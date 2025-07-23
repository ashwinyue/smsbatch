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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryManager_BasicFunctionality(t *testing.T) {
	// 创建重试管理器
	retryManager := NewRetryManager(2)

	// 启动管理器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := retryManager.Start(ctx)
	require.NoError(t, err)
	defer retryManager.Stop()

	// 测试成功任务
	successCalled := false
	task := &RetryTask{
		ID:           "test-task-1",
		JobID:        "job-1",
		PartitionKey: "partition-1",
		Config: &RetryConfig{
			MaxRetries:      3,
			InitialInterval: 100 * time.Millisecond,
			Strategy:        FixedInterval,
			Timeout:         1 * time.Second,
		},
		Executor: func(ctx context.Context) error {
			return nil // 成功
		},
		OnSuccess: func() {
			successCalled = true
		},
	}

	err = retryManager.AddTask(task)
	require.NoError(t, err)

	// 等待任务执行
	time.Sleep(500 * time.Millisecond)

	// 验证成功回调被调用
	assert.True(t, successCalled, "Success callback should be called")

	// 验证任务被移除
	_, exists := retryManager.GetTask("test-task-1")
	assert.False(t, exists, "Task should be removed after success")
}

func TestRetryManager_RetryOnFailure(t *testing.T) {
	retryManager := NewRetryManager(1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := retryManager.Start(ctx)
	require.NoError(t, err)
	defer retryManager.Stop()

	// 测试重试任务
	attempts := 0
	var mu sync.Mutex
	maxRetriesCalled := false

	task := &RetryTask{
		ID:           "test-task-2",
		JobID:        "job-2",
		PartitionKey: "partition-2",
		Config: &RetryConfig{
			MaxRetries:      3,
			InitialInterval: 50 * time.Millisecond,
			Strategy:        FixedInterval,
			Timeout:         1 * time.Second,
		},
		Executor: func(ctx context.Context) error {
			mu.Lock()
			attempts++
			mu.Unlock()
			return errors.New("simulated failure")
		},
		OnMaxRetries: func() {
			maxRetriesCalled = true
		},
	}

	err = retryManager.AddTask(task)
	require.NoError(t, err)

	// 等待所有重试完成
	time.Sleep(1 * time.Second)

	// 验证重试次数
	mu.Lock()
	finalAttempts := attempts
	mu.Unlock()

	assert.Equal(t, 3, finalAttempts, "Should attempt exactly 3 times")
	assert.True(t, maxRetriesCalled, "Max retries callback should be called")

	// 验证任务被移除
	_, exists := retryManager.GetTask("test-task-2")
	assert.False(t, exists, "Task should be removed after max retries")
}

func TestRetryManager_ExponentialBackoff(t *testing.T) {
	retryManager := NewRetryManager(1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := retryManager.Start(ctx)
	require.NoError(t, err)
	defer retryManager.Stop()

	// 记录执行时间
	executionTimes := make([]time.Time, 0)
	var mu sync.Mutex

	task := &RetryTask{
		ID:           "test-task-3",
		JobID:        "job-3",
		PartitionKey: "partition-3",
		Config: &RetryConfig{
			MaxRetries:      3,
			InitialInterval: 100 * time.Millisecond,
			Strategy:        ExponentialBackoff,
			BackoffFactor:   2.0,
			Timeout:         1 * time.Second,
		},
		Executor: func(ctx context.Context) error {
			mu.Lock()
			executionTimes = append(executionTimes, time.Now())
			mu.Unlock()
			return errors.New("simulated failure")
		},
	}

	err = retryManager.AddTask(task)
	require.NoError(t, err)

	// 等待所有重试完成
	time.Sleep(2 * time.Second)

	// 验证执行时间间隔递增
	mu.Lock()
	times := make([]time.Time, len(executionTimes))
	copy(times, executionTimes)
	mu.Unlock()

	assert.Len(t, times, 3, "Should have 3 execution times")

	if len(times) >= 2 {
		interval1 := times[1].Sub(times[0])
		assert.True(t, interval1 >= 100*time.Millisecond, "First retry interval should be at least 100ms")
	}

	if len(times) >= 3 {
		interval2 := times[2].Sub(times[1])
		interval1 := times[1].Sub(times[0])
		assert.True(t, interval2 > interval1, "Second retry interval should be longer than first")
	}
}

func TestRetryManager_CircuitBreaker(t *testing.T) {
	retryManager := NewRetryManager(1)

	// 设置较低的失败阈值用于测试
	retryManager.circuitBreaker.failureThreshold = 2
	retryManager.circuitBreaker.timeout = 1 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := retryManager.Start(ctx)
	require.NoError(t, err)
	defer retryManager.Stop()

	// 添加失败任务触发熔断器
	for i := 0; i < 3; i++ {
		task := &RetryTask{
			ID:           fmt.Sprintf("test-task-%d", i),
			JobID:        fmt.Sprintf("job-%d", i),
			PartitionKey: "partition",
			Config: &RetryConfig{
				MaxRetries:      1,
				InitialInterval: 10 * time.Millisecond,
				Strategy:        FixedInterval,
				Timeout:         1 * time.Second,
			},
			Executor: func(ctx context.Context) error {
				return errors.New("simulated failure")
			},
		}

		if i < 2 {
			// 前两个任务应该成功添加
			err = retryManager.AddTask(task)
			assert.NoError(t, err)
		} else {
			// 等待熔断器打开
			time.Sleep(500 * time.Millisecond)
			// 检查熔断器状态
			state := retryManager.circuitBreaker.GetState()
			t.Logf("Circuit breaker state before adding third task: %v", state)
			// 第三个任务应该被熔断器拒绝
			err = retryManager.AddTask(task)
			if err != nil {
				assert.Contains(t, err.Error(), "circuit breaker is open")
			} else {
				t.Errorf("Expected error but got nil, circuit breaker state: %v", state)
			}
		}
	}

	// 等待熔断器超时后恢复
	time.Sleep(1200 * time.Millisecond)

	// 现在应该可以添加任务了
	task := &RetryTask{
		ID:           "test-task-recovery",
		JobID:        "job-recovery",
		PartitionKey: "partition",
		Config: &RetryConfig{
			MaxRetries:      1,
			InitialInterval: 10 * time.Millisecond,
			Strategy:        FixedInterval,
			Timeout:         1 * time.Second,
		},
		Executor: func(ctx context.Context) error {
			return nil // 成功
		},
	}

	err = retryManager.AddTask(task)
	assert.NoError(t, err, "Should be able to add task after circuit breaker recovery")
}

func TestRetryManager_Metrics(t *testing.T) {
	retryManager := NewRetryManager(2)

	// 重置熔断器状态，避免之前测试的影响
	retryManager.circuitBreaker.state = Closed
	retryManager.circuitBreaker.failureCount = 0

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := retryManager.Start(ctx)
	require.NoError(t, err)
	defer retryManager.Stop()

	// 添加成功任务
	successTask := &RetryTask{
		ID:           "success-task",
		JobID:        "job-success",
		PartitionKey: "partition",
		Config: &RetryConfig{
			MaxRetries:      3,
			InitialInterval: 10 * time.Millisecond,
			Strategy:        FixedInterval,
			Timeout:         1 * time.Second,
		},
		Executor: func(ctx context.Context) error {
			return nil
		},
	}

	err = retryManager.AddTask(successTask)
	require.NoError(t, err)

	// 添加失败任务
	failTask := &RetryTask{
		ID:           "fail-task",
		JobID:        "job-fail",
		PartitionKey: "partition",
		Config: &RetryConfig{
			MaxRetries:      2,
			InitialInterval: 10 * time.Millisecond,
			Strategy:        FixedInterval,
			Timeout:         1 * time.Second,
		},
		Executor: func(ctx context.Context) error {
			return errors.New("simulated failure")
		},
	}

	err = retryManager.AddTask(failTask)
	require.NoError(t, err)

	// 等待任务完成，给足够时间让失败任务完成所有重试
	time.Sleep(2 * time.Second)

	// 检查指标
	metrics := retryManager.GetMetrics()
	assert.Equal(t, int64(2), metrics.TotalTasks, "Should have 2 total tasks")
	assert.Equal(t, int64(1), metrics.SuccessfulTasks, "Should have 1 successful task")
	assert.Equal(t, int64(1), metrics.MaxRetriesTasks, "Should have 1 max retries task")
	assert.True(t, metrics.TotalRetries > 0, "Should have some retries")
}

func TestRetryConfig_DefaultValues(t *testing.T) {
	config := DefaultRetryConfig()

	assert.Equal(t, 5, config.MaxRetries)
	assert.Equal(t, 1*time.Second, config.InitialInterval)
	assert.Equal(t, 5*time.Minute, config.MaxInterval)
	assert.Equal(t, ExponentialBackoff, config.Strategy)
	assert.Equal(t, 0.1, config.JitterFactor)
	assert.Equal(t, 2.0, config.BackoffFactor)
	assert.Equal(t, 30*time.Second, config.Timeout)
}

func TestRetryManager_ConcurrentExecution(t *testing.T) {
	retryManager := NewRetryManager(3)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := retryManager.Start(ctx)
	require.NoError(t, err)
	defer retryManager.Stop()

	// 添加多个并发任务
	var wg sync.WaitGroup
	completedTasks := make(map[string]bool)
	var mu sync.Mutex

	for i := 0; i < 5; i++ {
		wg.Add(1)
		taskID := fmt.Sprintf("concurrent-task-%d", i)

		task := &RetryTask{
			ID:           taskID,
			JobID:        fmt.Sprintf("job-%d", i),
			PartitionKey: "partition",
			Config: &RetryConfig{
				MaxRetries:      1,
				InitialInterval: 50 * time.Millisecond,
				Strategy:        FixedInterval,
				Timeout:         1 * time.Second,
			},
			Executor: func(ctx context.Context) error {
				// 模拟一些工作
				time.Sleep(100 * time.Millisecond)
				return nil
			},
			OnSuccess: func() {
				mu.Lock()
				completedTasks[taskID] = true
				mu.Unlock()
				wg.Done()
			},
		}

		err = retryManager.AddTask(task)
		require.NoError(t, err)
	}

	// 等待所有任务完成
	wg.Wait()

	// 验证所有任务都完成了
	mu.Lock()
	assert.Len(t, completedTasks, 5, "All tasks should complete")
	mu.Unlock()
}
