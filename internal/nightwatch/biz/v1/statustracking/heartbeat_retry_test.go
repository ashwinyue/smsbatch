// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

package statustracking

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeartbeatMonitor_WithRetryManager(t *testing.T) {
	// 创建带重试管理器的心跳监控器
	monitor := NewHeartbeatMonitorWithConfig(100*time.Millisecond, 200*time.Millisecond, nil)

	// 启动监控器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor.Start(ctx)
	defer monitor.Stop()

	// 验证重试管理器已启动
	assert.True(t, monitor.retryManager.isRunning, "Retry manager should be running")

	// 验证重试配置
	retryConfig := monitor.GetRetryConfig()
	assert.NotNil(t, retryConfig)
	assert.Equal(t, 2, retryConfig.MaxRetries)
	assert.Equal(t, FixedInterval, retryConfig.Strategy)
}

func TestHeartbeatMonitor_TimeoutWithRetry(t *testing.T) {
	retryAttempts := 0
	var mu sync.Mutex
	timeoutCalled := false

	// 创建心跳监控器
	monitor := NewHeartbeatMonitorWithConfig(50*time.Millisecond, 100*time.Millisecond, func(jobID, partitionKey string) {
		timeoutCalled = true
	})

	// 模拟重试失败
	originalReschedule := monitor.RescheduleJob
	monitor.RescheduleJob = func(ctx context.Context, job *HeartbeatInfo) error {
		mu.Lock()
		retryAttempts++
		mu.Unlock()
		// 模拟重试失败
		return assert.AnError
	}
	defer func() {
		monitor.RescheduleJob = originalReschedule
	}()

	// 启动监控器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor.Start(ctx)
	defer monitor.Stop()

	// 添加心跳
	jobID := "retry-test-job"
	partitionKey := "retry-partition"
	monitor.UpdateHeartbeat(jobID, partitionKey, "processing", nil)

	// 等待超时和重试
	time.Sleep(500 * time.Millisecond)

	// 验证超时回调被调用
	assert.True(t, timeoutCalled, "Timeout callback should be called")

	// 验证重试尝试
	mu.Lock()
	finalAttempts := retryAttempts
	mu.Unlock()
	assert.True(t, finalAttempts > 0, "Should have retry attempts")

	// 验证重试指标
	metrics := monitor.GetRetryMetrics()
	assert.NotNil(t, metrics)
	assert.True(t, metrics.TotalTasks > 0, "Should have retry tasks")
}

func TestHeartbeatMonitor_SuccessfulRetry(t *testing.T) {
	retryAttempts := 0
	var mu sync.Mutex
	timeoutCalled := false

	// 创建心跳监控器
	monitor := NewHeartbeatMonitorWithConfig(50*time.Millisecond, 100*time.Millisecond, func(jobID, partitionKey string) {
		timeoutCalled = true
	})

	// 设置更短的重试间隔用于测试
	testRetryConfig := &RetryConfig{
		MaxRetries:      5,
		InitialInterval: 100 * time.Millisecond, // 更短的重试间隔
		MaxInterval:     1 * time.Second,
		Strategy:        FixedInterval, // 使用固定间隔避免指数退避
		JitterFactor:    0,
		BackoffFactor:   1.0,
		Timeout:         5 * time.Second,
	}
	monitor.SetRetryConfig(testRetryConfig)

	// 模拟第一次失败，第二次成功
	originalReschedule := monitor.RescheduleJob
	monitor.RescheduleJob = func(ctx context.Context, job *HeartbeatInfo) error {
		mu.Lock()
		retryAttempts++
		currentAttempt := retryAttempts
		mu.Unlock()

		if currentAttempt == 1 {
			return assert.AnError // 第一次失败
		}
		// 第二次成功
		return nil
	}
	defer func() {
		monitor.RescheduleJob = originalReschedule
	}()

	// 启动监控器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor.Start(ctx)
	defer monitor.Stop()

	// 添加心跳
	jobID := "success-retry-job"
	partitionKey := "success-partition"
	monitor.UpdateHeartbeat(jobID, partitionKey, "processing", nil)

	// 等待超时和重试（增加等待时间以确保第二次重试能够执行）
	time.Sleep(1500 * time.Millisecond)

	// 验证超时回调被调用
	assert.True(t, timeoutCalled, "Timeout callback should be called")

	// 验证重试尝试
	mu.Lock()
	finalAttempts := retryAttempts
	mu.Unlock()
	assert.Equal(t, 2, finalAttempts, "Should have exactly 2 retry attempts")

	// 验证任务恢复健康状态
	heartbeatInfo, exists := monitor.GetHeartbeatInfo(jobID, partitionKey)
	require.True(t, exists)
	assert.True(t, heartbeatInfo.IsHealthy, "Job should be healthy after successful retry")
	assert.True(t, heartbeatInfo.RetryCount > 0, "Should have retry count")
}

func TestHeartbeatMonitor_ForceRetry(t *testing.T) {
	retryExecuted := false
	var mu sync.Mutex

	// 创建心跳监控器
	monitor := NewHeartbeatMonitorWithConfig(1*time.Second, 2*time.Second, nil)

	// 模拟成功的重试
	originalReschedule := monitor.RescheduleJob
	monitor.RescheduleJob = func(ctx context.Context, job *HeartbeatInfo) error {
		mu.Lock()
		retryExecuted = true
		mu.Unlock()
		return nil
	}
	defer func() {
		monitor.RescheduleJob = originalReschedule
	}()

	// 启动监控器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor.Start(ctx)
	defer monitor.Stop()

	// 添加心跳
	jobID := "force-retry-job"
	partitionKey := "force-partition"
	monitor.UpdateHeartbeat(jobID, partitionKey, "error", nil)

	// 强制重试
	err := monitor.ForceRetryJob(ctx, jobID, partitionKey)
	assert.NoError(t, err)

	// 等待重试执行
	time.Sleep(300 * time.Millisecond)

	// 验证重试被执行
	mu.Lock()
	assert.True(t, retryExecuted, "Force retry should be executed")
	mu.Unlock()

	// 验证任务状态
	heartbeatInfo, exists := monitor.GetHeartbeatInfo(jobID, partitionKey)
	require.True(t, exists)
	assert.True(t, heartbeatInfo.RetryCount > 0, "Should have retry count after force retry")
}

func TestHeartbeatMonitor_CircuitBreakerIntegration(t *testing.T) {
	// 创建心跳监控器
	monitor := NewHeartbeatMonitorWithConfig(50*time.Millisecond, 100*time.Millisecond, nil)

	// 设置较低的失败阈值
	monitor.retryManager.circuitBreaker.failureThreshold = 2
	monitor.retryManager.circuitBreaker.timeout = 200 * time.Millisecond

	// 启动监控器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor.Start(ctx)
	defer monitor.Stop()

	// 验证初始状态
	assert.Equal(t, Closed, monitor.GetCircuitBreakerState())

	// 模拟多次失败触发熔断器
	originalReschedule := monitor.RescheduleJob
	monitor.RescheduleJob = func(ctx context.Context, job *HeartbeatInfo) error {
		return assert.AnError // 总是失败
	}
	defer func() {
		monitor.RescheduleJob = originalReschedule
	}()

	// 添加多个超时任务
	for i := 0; i < 3; i++ {
		jobID := fmt.Sprintf("circuit-test-job-%d", i)
		partitionKey := "circuit-partition"
		monitor.UpdateHeartbeat(jobID, partitionKey, "processing", nil)
	}

	// 等待超时和失败
	time.Sleep(400 * time.Millisecond)

	// 验证熔断器状态（可能是Open或HalfOpen，取决于时间）
	state := monitor.GetCircuitBreakerState()
	assert.True(t, state == Open || state == HalfOpen, "Circuit breaker should be open or half-open")
}

func TestHeartbeatMonitor_RetryMetrics(t *testing.T) {
	// 创建心跳监控器
	monitor := NewHeartbeatMonitorWithConfig(50*time.Millisecond, 100*time.Millisecond, nil)

	// 启动监控器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor.Start(ctx)
	defer monitor.Stop()

	// 获取初始指标
	initialMetrics := monitor.GetRetryMetrics()
	assert.NotNil(t, initialMetrics)
	initialTotalTasks := initialMetrics.TotalTasks

	// 模拟成功的重试
	originalReschedule := monitor.RescheduleJob
	monitor.RescheduleJob = func(ctx context.Context, job *HeartbeatInfo) error {
		return nil // 成功
	}
	defer func() {
		monitor.RescheduleJob = originalReschedule
	}()

	// 添加心跳并等待超时
	jobID := "metrics-test-job"
	partitionKey := "metrics-partition"
	monitor.UpdateHeartbeat(jobID, partitionKey, "processing", nil)

	// 等待超时和重试
	time.Sleep(300 * time.Millisecond)

	// 验证指标更新
	finalMetrics := monitor.GetRetryMetrics()
	assert.True(t, finalMetrics.TotalTasks > initialTotalTasks, "Should have more total tasks")
	assert.True(t, finalMetrics.SuccessfulTasks > 0, "Should have successful tasks")
}

func TestHeartbeatMonitor_RetryConfigUpdate(t *testing.T) {
	// 创建心跳监控器
	monitor := NewHeartbeatMonitorWithConfig(1*time.Second, 2*time.Second, nil)

	// 获取初始配置
	initialConfig := monitor.GetRetryConfig()
	assert.NotNil(t, initialConfig)
	initialMaxRetries := initialConfig.MaxRetries

	// 更新配置
	newConfig := &RetryConfig{
		MaxRetries:      10,
		InitialInterval: 500 * time.Millisecond,
		MaxInterval:     10 * time.Minute,
		Strategy:        ExponentialBackoff,
		JitterFactor:    0.2,
		BackoffFactor:   3.0,
		Timeout:         1 * time.Minute,
	}

	monitor.SetRetryConfig(newConfig)

	// 验证配置更新
	updatedConfig := monitor.GetRetryConfig()
	assert.NotEqual(t, initialMaxRetries, updatedConfig.MaxRetries)
	assert.Equal(t, 10, updatedConfig.MaxRetries)
	assert.Equal(t, ExponentialBackoff, updatedConfig.Strategy)
	assert.Equal(t, 0.2, updatedConfig.JitterFactor)
}
