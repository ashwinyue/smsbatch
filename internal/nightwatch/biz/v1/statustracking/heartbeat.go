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
	"sync"
	"time"

	"github.com/onexstack/onexstack/pkg/log"
	"github.com/onexstack/onexstack/pkg/store/where"

	"github.com/ashwinyue/dcp/internal/nightwatch/store"
)

// HeartbeatMonitor 心跳监控器
type HeartbeatMonitor struct {
	store           store.IStore
	heartbeatMap    map[string]*HeartbeatInfo                   // 存储心跳信息
	mutex           sync.RWMutex                                // 保护心跳信息的读写
	timeoutInterval time.Duration                               // 心跳超时间隔
	checkInterval   time.Duration                               // 检查间隔
	stopChan        chan struct{}                               // 停止信号
	isRunning       bool                                        // 是否正在运行
	timeoutCallback func(string, string)                        // 超时回调函数
	retryManager    *RetryManager                               // 重试管理器
	retryConfig     *RetryConfig                                // 重试配置
	RescheduleJob   func(context.Context, *HeartbeatInfo) error // 重新调度任务函数
}

// HeartbeatInfo 心跳信息
type HeartbeatInfo struct {
	JobID         string            // 任务ID
	PartitionKey  string            // 分区键
	LastHeartbeat time.Time         // 最后心跳时间
	Status        string            // 当前状态
	ErrorCount    int64             // 错误计数
	RetryCount    int64             // 重试计数
	Metrics       map[string]string // 自定义指标
	IsHealthy     bool              // 是否健康
}

// NewHeartbeatMonitor 创建心跳监控器
func NewHeartbeatMonitor(store store.IStore) *HeartbeatMonitor {
	retryManager := NewRetryManager(3) // 3个工作协程
	retryConfig := DefaultRetryConfig()
	// 针对心跳超时调整重试配置
	retryConfig.MaxRetries = 3
	retryConfig.InitialInterval = 30 * time.Second
	retryConfig.MaxInterval = 5 * time.Minute
	retryConfig.Strategy = ExponentialBackoff

	hm := &HeartbeatMonitor{
		store:           store,
		heartbeatMap:    make(map[string]*HeartbeatInfo),
		timeoutInterval: 10 * time.Minute, // 默认10分钟超时
		checkInterval:   30 * time.Second, // 默认30秒检查一次
		stopChan:        make(chan struct{}),
		isRunning:       false,
		retryManager:    retryManager,
		retryConfig:     retryConfig,
	}
	hm.RescheduleJob = hm.defaultRescheduleJob
	return hm
}

// NewHeartbeatMonitorWithConfig 创建带配置的心跳监控器（用于测试）
func NewHeartbeatMonitorWithConfig(checkInterval, timeoutDuration time.Duration, timeoutCallback func(string, string)) *HeartbeatMonitor {
	retryManager := NewRetryManager(2) // 测试用较少的工作协程
	retryConfig := DefaultRetryConfig()
	// 测试用快速重试配置
	retryConfig.MaxRetries = 2
	retryConfig.InitialInterval = 100 * time.Millisecond
	retryConfig.MaxInterval = 1 * time.Second
	retryConfig.Strategy = FixedInterval

	hm := &HeartbeatMonitor{
		heartbeatMap:    make(map[string]*HeartbeatInfo),
		checkInterval:   checkInterval,
		timeoutInterval: timeoutDuration,
		stopChan:        make(chan struct{}),
		timeoutCallback: timeoutCallback,
		retryManager:    retryManager,
		retryConfig:     retryConfig,
	}
	hm.RescheduleJob = hm.defaultRescheduleJob
	return hm
}

// Start 启动心跳监控
func (hm *HeartbeatMonitor) Start(ctx context.Context) error {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	if hm.isRunning {
		return fmt.Errorf("heartbeat monitor is already running")
	}

	hm.isRunning = true
	log.Infow("Starting heartbeat monitor", "timeout_interval", hm.timeoutInterval, "check_interval", hm.checkInterval)

	// 启动重试管理器
	if hm.retryManager != nil {
		err := hm.retryManager.Start(ctx)
		if err != nil {
			hm.isRunning = false
			return fmt.Errorf("failed to start retry manager: %w", err)
		}
	}

	// 启动监控goroutine
	go hm.monitorLoop(ctx)

	return nil
}

// Stop 停止心跳监控
func (hm *HeartbeatMonitor) Stop() error {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	if !hm.isRunning {
		return fmt.Errorf("heartbeat monitor is not running")
	}

	log.Infow("Stopping heartbeat monitor")

	// 停止重试管理器
	if hm.retryManager != nil {
		err := hm.retryManager.Stop()
		if err != nil {
			log.Warnw("Failed to stop retry manager", "error", err)
		}
	}

	close(hm.stopChan)
	hm.isRunning = false

	return nil
}

// UpdateHeartbeat 更新心跳信息
func (hm *HeartbeatMonitor) UpdateHeartbeat(jobID, partitionKey, status string, metrics map[string]string) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	key := fmt.Sprintf("%s_%s", jobID, partitionKey)
	heartbeatInfo, exists := hm.heartbeatMap[key]

	if !exists {
		heartbeatInfo = &HeartbeatInfo{
			JobID:        jobID,
			PartitionKey: partitionKey,
			Metrics:      make(map[string]string),
		}
		hm.heartbeatMap[key] = heartbeatInfo
	}

	heartbeatInfo.LastHeartbeat = time.Now()
	heartbeatInfo.Status = status
	heartbeatInfo.IsHealthy = status != "failed" && status != "error"

	// 如果状态为错误，增加错误计数
	if status == "error" {
		heartbeatInfo.ErrorCount++
	}

	// 更新自定义指标
	if metrics != nil {
		for k, v := range metrics {
			heartbeatInfo.Metrics[k] = v
		}
	}

	log.Debugw("Heartbeat updated", "job_id", jobID, "partition_key", partitionKey, "status", status)
}

// GetHeartbeatInfo 获取心跳信息
func (hm *HeartbeatMonitor) GetHeartbeatInfo(jobID, partitionKey string) (*HeartbeatInfo, bool) {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	key := fmt.Sprintf("%s_%s", jobID, partitionKey)
	heartbeatInfo, exists := hm.heartbeatMap[key]
	if !exists {
		return nil, false
	}

	// 返回副本以避免并发修改
	copy := *heartbeatInfo
	copy.Metrics = make(map[string]string)
	for k, v := range heartbeatInfo.Metrics {
		copy.Metrics[k] = v
	}

	return &copy, true
}

// GetAllHeartbeats 获取所有心跳信息
func (hm *HeartbeatMonitor) GetAllHeartbeats() map[string]*HeartbeatInfo {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	result := make(map[string]*HeartbeatInfo)
	for key, heartbeatInfo := range hm.heartbeatMap {
		// 返回副本以避免并发修改
		copy := *heartbeatInfo
		copy.Metrics = make(map[string]string)
		for k, v := range heartbeatInfo.Metrics {
			copy.Metrics[k] = v
		}
		result[key] = &copy
	}

	return result
}

// RemoveHeartbeat 移除心跳信息
func (hm *HeartbeatMonitor) RemoveHeartbeat(jobID, partitionKey string) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	key := fmt.Sprintf("%s_%s", jobID, partitionKey)
	delete(hm.heartbeatMap, key)

	log.Debugw("Heartbeat removed", "job_id", jobID, "partition_key", partitionKey)
}

// monitorLoop 监控循环
func (hm *HeartbeatMonitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(hm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Infow("Heartbeat monitor stopped due to context cancellation")
			return
		case <-hm.stopChan:
			log.Infow("Heartbeat monitor stopped")
			return
		case <-ticker.C:
			hm.checkTimeouts(ctx)
		}
	}
}

// checkTimeouts 检查超时的心跳
func (hm *HeartbeatMonitor) checkTimeouts(ctx context.Context) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	now := time.Now()
	timeoutJobs := make([]*HeartbeatInfo, 0)
	jobsToRemove := make([]string, 0)

	// 检查所有心跳信息
	for key, heartbeatInfo := range hm.heartbeatMap {
		if now.Sub(heartbeatInfo.LastHeartbeat) > hm.timeoutInterval {
			// 心跳超时
			heartbeatInfo.IsHealthy = false
			heartbeatInfo.ErrorCount++
			timeoutJobs = append(timeoutJobs, heartbeatInfo)

			log.Warnw("Job heartbeat timeout detected",
				"job_id", heartbeatInfo.JobID,
				"partition_key", heartbeatInfo.PartitionKey,
				"last_heartbeat", heartbeatInfo.LastHeartbeat,
				"timeout_duration", now.Sub(heartbeatInfo.LastHeartbeat))

			// 调用超时回调函数
			if hm.timeoutCallback != nil {
				hm.timeoutCallback(heartbeatInfo.JobID, heartbeatInfo.PartitionKey)
			}

			// 如果错误次数过多，标记为待删除（但不立即删除，让重试逻辑先执行）
			if heartbeatInfo.ErrorCount > 5 {
				jobsToRemove = append(jobsToRemove, key)
			}
		}
	}

	// 先处理超时任务（包括重试逻辑）
	if len(timeoutJobs) > 0 {
		hm.handleTimeoutJobs(ctx, timeoutJobs)
	}

	// 然后移除错误次数过多的任务
	for _, key := range jobsToRemove {
		if heartbeatInfo, exists := hm.heartbeatMap[key]; exists {
			delete(hm.heartbeatMap, key)
			log.Infow("Removing job due to excessive timeout errors",
				"job_id", heartbeatInfo.JobID,
				"partition_key", heartbeatInfo.PartitionKey,
				"error_count", heartbeatInfo.ErrorCount)
		}
	}
}

// handleTimeoutJobs 处理超时任务
func (hm *HeartbeatMonitor) handleTimeoutJobs(ctx context.Context, timeoutJobs []*HeartbeatInfo) {
	for _, job := range timeoutJobs {
		// 使用重试管理器处理超时任务
		if hm.retryManager != nil {
			taskID := fmt.Sprintf("%s_%s_timeout", job.JobID, job.PartitionKey)

			// 检查是否已经存在相同的重试任务
			if _, exists := hm.retryManager.GetTask(taskID); exists {
				log.Debugw("Retry task already exists, skipping", "task_id", taskID)
				continue
			}

			retryTask := &RetryTask{
				ID:           taskID,
				JobID:        job.JobID,
				PartitionKey: job.PartitionKey,
				Config:       hm.retryConfig,
				Executor: func(ctx context.Context) error {
					return hm.RescheduleJob(ctx, job)
				},
				OnSuccess: func() {
					job.RetryCount++
					job.IsHealthy = true                          // 重试成功后恢复健康状态
					job.ErrorCount = 0                            // 重置错误计数
					job.LastHeartbeat = time.Now().Add(time.Hour) // 设置一个很远的心跳时间，防止再次超时

					// 重新添加任务到heartbeatMap中（如果已被移除）
					hm.mutex.Lock()
					key := fmt.Sprintf("%s_%s", job.JobID, job.PartitionKey)
					if _, exists := hm.heartbeatMap[key]; !exists {
						hm.heartbeatMap[key] = job
					}
					hm.mutex.Unlock()

					log.Infow("Successfully rescheduled timeout job via retry manager",
						"job_id", job.JobID,
						"partition_key", job.PartitionKey,
						"retry_count", job.RetryCount)
					// 重试成功后从重试管理器中移除任务
					hm.retryManager.RemoveTask(taskID)
				},
				OnFailure: func(err error) {
					log.Warnw("Retry attempt failed for timeout job",
						"job_id", job.JobID,
						"partition_key", job.PartitionKey,
						"error", err.Error())
				},
				OnMaxRetries: func() {
					log.Errorw(errors.New("job reached max retries, marking as failed"),
						"job_id", job.JobID,
						"partition_key", job.PartitionKey,
						"max_retries", hm.retryConfig.MaxRetries)
					// 达到最大重试次数后从重试管理器中移除任务
					hm.retryManager.RemoveTask(taskID)
				},
				Metadata: map[string]interface{}{
					"timeout_duration": time.Since(job.LastHeartbeat),
					"error_count":      job.ErrorCount,
				},
			}

			err := hm.retryManager.AddTask(retryTask)
			if err != nil {
				log.Errorw(errors.New("failed to add timeout job to retry manager"),
					"job_id", job.JobID,
					"partition_key", job.PartitionKey,
					"error", err.Error())
				// 回退到直接重试
				hm.fallbackRetry(ctx, job)
			}
		} else {
			// 如果没有重试管理器，使用原有的直接重试方式
			hm.fallbackRetry(ctx, job)
		}
	}
}

// fallbackRetry 回退重试方法
func (hm *HeartbeatMonitor) fallbackRetry(ctx context.Context, job *HeartbeatInfo) {
	err := hm.RescheduleJob(ctx, job)
	if err != nil {
		log.Errorw(errors.New("failed to reschedule timeout job"),
			"job_id", job.JobID,
			"partition_key", job.PartitionKey,
			"error", err.Error())
	} else {
		job.RetryCount++
		log.Infow("Successfully rescheduled timeout job",
			"job_id", job.JobID,
			"partition_key", job.PartitionKey,
			"retry_count", job.RetryCount)
	}
}

// defaultRescheduleJob 默认的重新调度任务实现
func (hm *HeartbeatMonitor) defaultRescheduleJob(ctx context.Context, job *HeartbeatInfo) error {
	// 如果store为空（测试环境），直接返回成功
	if hm.store == nil {
		return nil
	}

	// 查询SMS批处理信息
	smsBatch, err := hm.store.SmsBatch().Get(ctx, where.NewWhere(where.WithQuery("batch_id = ?", job.JobID)))
	if err != nil {
		return fmt.Errorf("failed to get SMS batch: %w", err)
	}

	// 更新状态为重试
	smsBatch.Status = "retrying"
	smsBatch.UpdatedAt = time.Now()

	err = hm.store.SmsBatch().Update(ctx, smsBatch)
	if err != nil {
		return fmt.Errorf("failed to update SMS batch status: %w", err)
	}

	// 这里可以添加重新发送到消息队列的逻辑
	// 例如：发送到重试队列或重新触发处理流程

	return nil
}

// SetTimeoutInterval 设置超时间隔
func (hm *HeartbeatMonitor) SetTimeoutInterval(interval time.Duration) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	hm.timeoutInterval = interval
}

// SetCheckInterval 设置检查间隔
func (hm *HeartbeatMonitor) SetCheckInterval(interval time.Duration) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	hm.checkInterval = interval
}

// IsRunning 检查是否正在运行
func (hm *HeartbeatMonitor) IsRunning() bool {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()
	return hm.isRunning
}

// GetHealthyJobsCount 获取健康任务数量
func (hm *HeartbeatMonitor) GetHealthyJobsCount() int {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	count := 0
	for _, heartbeatInfo := range hm.heartbeatMap {
		if heartbeatInfo.IsHealthy {
			count++
		}
	}
	return count
}

// GetUnhealthyJobsCount 获取不健康任务数量
func (hm *HeartbeatMonitor) GetUnhealthyJobsCount() int {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	count := 0
	for _, heartbeatInfo := range hm.heartbeatMap {
		if !heartbeatInfo.IsHealthy {
			count++
		}
	}
	return count
}

// GetRetryMetrics 获取重试指标
func (hm *HeartbeatMonitor) GetRetryMetrics() *RetryMetrics {
	if hm.retryManager == nil {
		return &RetryMetrics{}
	}
	return hm.retryManager.GetMetrics()
}

// GetCircuitBreakerState 获取熔断器状态
func (hm *HeartbeatMonitor) GetCircuitBreakerState() CircuitState {
	if hm.retryManager == nil || hm.retryManager.circuitBreaker == nil {
		return Closed
	}
	return hm.retryManager.circuitBreaker.GetState()
}

// SetRetryConfig 设置重试配置
func (hm *HeartbeatMonitor) SetRetryConfig(config *RetryConfig) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	hm.retryConfig = config
}

// GetRetryConfig 获取重试配置
func (hm *HeartbeatMonitor) GetRetryConfig() *RetryConfig {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()
	return hm.retryConfig
}

// ForceRetryJob 强制重试指定任务
func (hm *HeartbeatMonitor) ForceRetryJob(ctx context.Context, jobID, partitionKey string) error {
	hm.mutex.RLock()
	key := fmt.Sprintf("%s_%s", jobID, partitionKey)
	heartbeatInfo, exists := hm.heartbeatMap[key]
	hm.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("heartbeat info not found for job %s partition %s", jobID, partitionKey)
	}

	if hm.retryManager == nil {
		return fmt.Errorf("retry manager is not available")
	}

	taskID := fmt.Sprintf("%s_%s_manual_retry", jobID, partitionKey)
	retryTask := &RetryTask{
		ID:           taskID,
		JobID:        jobID,
		PartitionKey: partitionKey,
		Config:       hm.retryConfig,
		Executor: func(ctx context.Context) error {
			return hm.RescheduleJob(ctx, heartbeatInfo)
		},
		OnSuccess: func() {
			heartbeatInfo.RetryCount++
			heartbeatInfo.IsHealthy = true
			log.Infow("Manual retry succeeded",
				"job_id", jobID,
				"partition_key", partitionKey,
				"retry_count", heartbeatInfo.RetryCount)
		},
		OnFailure: func(err error) {
			log.Warnw("Manual retry failed",
				"job_id", jobID,
				"partition_key", partitionKey,
				"error", err.Error())
		},
		Metadata: map[string]interface{}{
			"manual_retry": true,
			"trigger_time": time.Now(),
		},
	}

	return hm.retryManager.AddTask(retryTask)
}
