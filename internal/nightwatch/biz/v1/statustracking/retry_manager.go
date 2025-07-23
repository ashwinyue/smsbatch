// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

package statustracking

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/onexstack/onexstack/pkg/log"
)

// RetryStrategy 重试策略
type RetryStrategy string

const (
	// FixedInterval 固定间隔重试
	FixedInterval RetryStrategy = "fixed"
	// ExponentialBackoff 指数退避重试
	ExponentialBackoff RetryStrategy = "exponential"
	// LinearBackoff 线性退避重试
	LinearBackoff RetryStrategy = "linear"
)

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries      int           // 最大重试次数
	InitialInterval time.Duration // 初始重试间隔
	MaxInterval     time.Duration // 最大重试间隔
	Strategy        RetryStrategy // 重试策略
	JitterFactor    float64       // 抖动因子 (0-1)
	BackoffFactor   float64       // 退避因子
	Timeout         time.Duration // 单次重试超时时间
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:      5,
		InitialInterval: 1 * time.Second,
		MaxInterval:     5 * time.Minute,
		Strategy:        ExponentialBackoff,
		JitterFactor:    0.1,
		BackoffFactor:   2.0,
		Timeout:         30 * time.Second,
	}
}

// RetryTask 重试任务
type RetryTask struct {
	ID           string                      // 任务ID
	JobID        string                      // 作业ID
	PartitionKey string                      // 分区键
	Attempts     int                         // 已尝试次数
	LastAttempt  time.Time                   // 最后尝试时间
	NextAttempt  time.Time                   // 下次尝试时间
	Config       *RetryConfig                // 重试配置
	Executor     func(context.Context) error // 执行函数
	OnSuccess    func()                      // 成功回调
	OnFailure    func(error)                 // 失败回调
	OnMaxRetries func()                      // 达到最大重试次数回调
	Metadata     map[string]interface{}      // 元数据
}

// RetryManager 重试管理器
type RetryManager struct {
	mu             sync.RWMutex
	tasks          map[string]*RetryTask // 重试任务映射
	queue          chan *RetryTask       // 重试队列
	workerCount    int                   // 工作协程数量
	stopChan       chan struct{}         // 停止信号
	isRunning      bool                  // 是否运行中
	circuitBreaker *CircuitBreaker       // 熔断器
	metrics        *RetryMetrics         // 重试指标
}

// RetryMetrics 重试指标
type RetryMetrics struct {
	mu              sync.RWMutex
	TotalTasks      int64   // 总任务数
	SuccessfulTasks int64   // 成功任务数
	FailedTasks     int64   // 失败任务数
	MaxRetriesTasks int64   // 达到最大重试次数的任务数
	AverageRetries  float64 // 平均重试次数
	TotalRetries    int64   // 总重试次数
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	mu               sync.RWMutex
	failureCount     int           // 失败计数
	lastFailureTime  time.Time     // 最后失败时间
	state            CircuitState  // 熔断器状态
	failureThreshold int           // 失败阈值
	timeout          time.Duration // 超时时间
}

// CircuitState 熔断器状态
type CircuitState int

const (
	Closed   CircuitState = iota // 关闭状态
	Open                         // 开启状态
	HalfOpen                     // 半开状态
)

// NewRetryManager 创建重试管理器
func NewRetryManager(workerCount int) *RetryManager {
	return &RetryManager{
		tasks:       make(map[string]*RetryTask),
		queue:       make(chan *RetryTask, 1000),
		workerCount: workerCount,
		stopChan:    make(chan struct{}),
		circuitBreaker: &CircuitBreaker{
			failureThreshold: 10,
			timeout:          1 * time.Minute,
			state:            Closed,
		},
		metrics: &RetryMetrics{},
	}
}

// Start 启动重试管理器
func (rm *RetryManager) Start(ctx context.Context) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.isRunning {
		return fmt.Errorf("retry manager is already running")
	}

	rm.isRunning = true
	log.Infow("Starting retry manager", "worker_count", rm.workerCount)

	// 启动工作协程
	for i := 0; i < rm.workerCount; i++ {
		go rm.worker(ctx, i)
	}

	// 启动调度协程
	go rm.scheduler(ctx)

	return nil
}

// Stop 停止重试管理器
func (rm *RetryManager) Stop() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.isRunning {
		return fmt.Errorf("retry manager is not running")
	}

	log.Infow("Stopping retry manager")
	close(rm.stopChan)
	rm.isRunning = false

	return nil
}

// AddTask 添加重试任务
func (rm *RetryManager) AddTask(task *RetryTask) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.isRunning {
		return fmt.Errorf("retry manager is not running")
	}

	// 检查熔断器状态
	if !rm.circuitBreaker.CanExecute() {
		return fmt.Errorf("circuit breaker is open, rejecting task")
	}

	// 设置默认配置
	if task.Config == nil {
		task.Config = DefaultRetryConfig()
	}

	// 设置初始时间
	task.LastAttempt = time.Now()
	task.NextAttempt = time.Now()

	// 添加到任务映射
	rm.tasks[task.ID] = task

	// 更新指标
	rm.metrics.mu.Lock()
	rm.metrics.TotalTasks++
	rm.metrics.mu.Unlock()

	log.Debugw("Added retry task", "task_id", task.ID, "job_id", task.JobID)

	// 立即尝试调度任务（如果队列有空间）
	select {
	case rm.queue <- task:
		log.Debugw("Immediately scheduled retry task", "task_id", task.ID)
	default:
		log.Debugw("Retry queue is full, task will be scheduled later", "task_id", task.ID)
	}

	return nil
}

// RemoveTask 移除重试任务
func (rm *RetryManager) RemoveTask(taskID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	delete(rm.tasks, taskID)
	log.Debugw("Removed retry task", "task_id", taskID)
}

// GetTask 获取重试任务
func (rm *RetryManager) GetTask(taskID string) (*RetryTask, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	task, exists := rm.tasks[taskID]
	return task, exists
}

// scheduler 调度器
func (rm *RetryManager) scheduler(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-rm.stopChan:
			return
		case <-ticker.C:
			rm.scheduleReadyTasks()
		}
	}
}

// scheduleReadyTasks 调度准备好的任务
func (rm *RetryManager) scheduleReadyTasks() {
	rm.mu.RLock()
	now := time.Now()
	readyTasks := make([]*RetryTask, 0)

	for _, task := range rm.tasks {
		if now.After(task.NextAttempt) && task.Attempts < task.Config.MaxRetries {
			readyTasks = append(readyTasks, task)
		}
	}
	rm.mu.RUnlock()

	// 将准备好的任务加入队列
	for _, task := range readyTasks {
		select {
		case rm.queue <- task:
			log.Debugw("Scheduled retry task", "task_id", task.ID, "attempt", task.Attempts+1)
		default:
			log.Warnw("Retry queue is full, skipping task", "task_id", task.ID)
		}
	}
}

// worker 工作协程
func (rm *RetryManager) worker(ctx context.Context, workerID int) {
	log.Debugw("Starting retry worker", "worker_id", workerID)

	for {
		select {
		case <-ctx.Done():
			return
		case <-rm.stopChan:
			return
		case task := <-rm.queue:
			rm.executeTask(ctx, task, workerID)
		}
	}
}

// executeTask 执行任务
func (rm *RetryManager) executeTask(ctx context.Context, task *RetryTask, workerID int) {
	// 创建带超时的上下文
	taskCtx, cancel := context.WithTimeout(ctx, task.Config.Timeout)
	defer cancel()

	// 更新尝试次数和时间
	task.Attempts++
	task.LastAttempt = time.Now()

	log.Infow("Executing retry task",
		"task_id", task.ID,
		"job_id", task.JobID,
		"attempt", task.Attempts,
		"worker_id", workerID)

	// 执行任务
	err := task.Executor(taskCtx)

	if err == nil {
		// 任务成功
		rm.handleTaskSuccess(task)
	} else {
		// 任务失败
		rm.handleTaskFailure(task, err)
	}
}

// handleTaskSuccess 处理任务成功
func (rm *RetryManager) handleTaskSuccess(task *RetryTask) {
	log.Infow("Retry task succeeded", "task_id", task.ID, "attempts", task.Attempts)

	// 更新指标
	rm.metrics.mu.Lock()
	rm.metrics.SuccessfulTasks++
	rm.metrics.TotalRetries += int64(task.Attempts)
	rm.metrics.mu.Unlock()

	// 调用成功回调
	if task.OnSuccess != nil {
		task.OnSuccess()
	}

	// 重置熔断器
	rm.circuitBreaker.OnSuccess()

	// 移除任务
	rm.RemoveTask(task.ID)
}

// handleTaskFailure 处理任务失败
func (rm *RetryManager) handleTaskFailure(task *RetryTask, err error) {
	log.Warnw("Retry task failed",
		"task_id", task.ID,
		"attempt", task.Attempts,
		"error", err.Error())

	// 调用失败回调
	if task.OnFailure != nil {
		task.OnFailure(err)
	}

	// 更新熔断器
	rm.circuitBreaker.OnFailure()

	if task.Attempts >= task.Config.MaxRetries {
		// 达到最大重试次数
		log.Errorw(errors.New("retry task reached max retries"),
			"task_id", task.ID,
			"job_id", task.JobID,
			"max_retries", task.Config.MaxRetries)

		// 更新指标
		rm.metrics.mu.Lock()
		rm.metrics.MaxRetriesTasks++
		rm.metrics.TotalRetries += int64(task.Attempts)
		rm.metrics.mu.Unlock()

		// 调用最大重试次数回调
		if task.OnMaxRetries != nil {
			task.OnMaxRetries()
		}

		// 移除任务
		rm.RemoveTask(task.ID)
	} else {
		// 计算下次重试时间
		task.NextAttempt = rm.calculateNextAttempt(task)
		log.Debugw("Scheduled next retry",
			"task_id", task.ID,
			"next_attempt", task.NextAttempt)
	}
}

// calculateNextAttempt 计算下次重试时间
func (rm *RetryManager) calculateNextAttempt(task *RetryTask) time.Time {
	var interval time.Duration

	switch task.Config.Strategy {
	case FixedInterval:
		interval = task.Config.InitialInterval
	case LinearBackoff:
		interval = time.Duration(task.Attempts) * task.Config.InitialInterval
	case ExponentialBackoff:
		fallthrough
	default:
		interval = time.Duration(float64(task.Config.InitialInterval) *
			math.Pow(task.Config.BackoffFactor, float64(task.Attempts-1)))
	}

	// 限制最大间隔
	if interval > task.Config.MaxInterval {
		interval = task.Config.MaxInterval
	}

	// 添加抖动
	if task.Config.JitterFactor > 0 {
		jitter := time.Duration(float64(interval) * task.Config.JitterFactor * (2*rand.Float64() - 1))
		interval += jitter
	}

	// 确保间隔为正数
	if interval < 0 {
		interval = task.Config.InitialInterval
	}

	return time.Now().Add(interval)
}

// GetMetrics 获取重试指标
func (rm *RetryManager) GetMetrics() *RetryMetrics {
	rm.metrics.mu.RLock()
	defer rm.metrics.mu.RUnlock()

	// 计算平均重试次数
	if rm.metrics.SuccessfulTasks+rm.metrics.MaxRetriesTasks > 0 {
		rm.metrics.AverageRetries = float64(rm.metrics.TotalRetries) /
			float64(rm.metrics.SuccessfulTasks+rm.metrics.MaxRetriesTasks)
	}

	return &RetryMetrics{
		TotalTasks:      rm.metrics.TotalTasks,
		SuccessfulTasks: rm.metrics.SuccessfulTasks,
		FailedTasks:     rm.metrics.FailedTasks,
		MaxRetriesTasks: rm.metrics.MaxRetriesTasks,
		AverageRetries:  rm.metrics.AverageRetries,
		TotalRetries:    rm.metrics.TotalRetries,
	}
}

// CanExecute 检查熔断器是否允许执行
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case Closed:
		return true
	case Open:
		// 检查是否可以转为半开状态
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.state = HalfOpen
			return true
		}
		return false
	case HalfOpen:
		return true
	default:
		return false
	}
}

// OnSuccess 熔断器成功回调
func (cb *CircuitBreaker) OnSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount = 0
	cb.state = Closed
}

// OnFailure 熔断器失败回调
func (cb *CircuitBreaker) OnFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailureTime = time.Now()

	if cb.failureCount >= cb.failureThreshold {
		cb.state = Open
	}
}

// GetState 获取熔断器状态
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}
