// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

package statustracking

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeartbeatMonitor(t *testing.T) {
	// 创建心跳监控器
	monitor := NewHeartbeatMonitorWithConfig(1*time.Second, 3*time.Second, func(jobID, partitionKey string) {
		t.Logf("Timeout callback triggered for job %s partition %s", jobID, partitionKey)
	})

	// 启动监控器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor.Start(ctx)
	defer monitor.Stop()

	// 测试更新心跳
	jobID := "test-job-001"
	partitionKey := "partition-1"
	status := "processing"
	metrics := map[string]string{
		"processed_count": "100",
		"error_count":     "0",
	}

	// 更新心跳
	monitor.UpdateHeartbeat(jobID, partitionKey, status, metrics)

	// 验证心跳信息
	heartbeatInfo, exists := monitor.GetHeartbeatInfo(jobID, partitionKey)
	require.True(t, exists, "Heartbeat info should exist")
	assert.Equal(t, jobID, heartbeatInfo.JobID)
	assert.Equal(t, partitionKey, heartbeatInfo.PartitionKey)
	assert.Equal(t, status, heartbeatInfo.Status)
	assert.Equal(t, metrics, heartbeatInfo.Metrics)
	assert.True(t, heartbeatInfo.IsHealthy)
	assert.Equal(t, int64(0), heartbeatInfo.ErrorCount)
	assert.Equal(t, int64(0), heartbeatInfo.RetryCount)

	// 测试获取所有心跳信息
	allHeartbeats := monitor.GetAllHeartbeats()
	assert.Len(t, allHeartbeats, 1)

	// 测试移除心跳
	monitor.RemoveHeartbeat(jobID, partitionKey)
	_, exists = monitor.GetHeartbeatInfo(jobID, partitionKey)
	assert.False(t, exists, "Heartbeat info should be removed")

	// 验证所有心跳信息为空
	allHeartbeats = monitor.GetAllHeartbeats()
	assert.Len(t, allHeartbeats, 0)
}

func TestHeartbeatMonitorTimeout(t *testing.T) {
	timeoutCalled := false
	timeoutJobID := ""
	timeoutPartitionKey := ""

	// 创建心跳监控器，设置较短的超时时间
	monitor := NewHeartbeatMonitorWithConfig(100*time.Millisecond, 200*time.Millisecond, func(jobID, partitionKey string) {
		timeoutCalled = true
		timeoutJobID = jobID
		timeoutPartitionKey = partitionKey
	})

	// 启动监控器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor.Start(ctx)
	defer monitor.Stop()

	// 添加心跳
	jobID := "timeout-test-job"
	partitionKey := "timeout-partition"
	monitor.UpdateHeartbeat(jobID, partitionKey, "processing", nil)

	// 等待超时触发
	time.Sleep(300 * time.Millisecond)

	// 验证超时回调被调用
	assert.True(t, timeoutCalled, "Timeout callback should be called")
	assert.Equal(t, jobID, timeoutJobID)
	assert.Equal(t, partitionKey, timeoutPartitionKey)

	// 验证心跳信息仍然存在但标记为不健康
	heartbeatInfo, exists := monitor.GetHeartbeatInfo(jobID, partitionKey)
	require.True(t, exists, "Heartbeat info should still exist")
	assert.False(t, heartbeatInfo.IsHealthy, "Heartbeat should be marked as unhealthy")
}

func TestHeartbeatMonitorConcurrency(t *testing.T) {
	// 创建心跳监控器
	monitor := NewHeartbeatMonitorWithConfig(1*time.Second, 3*time.Second, func(jobID, partitionKey string) {
		t.Logf("Timeout for job %s partition %s", jobID, partitionKey)
	})

	// 启动监控器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor.Start(ctx)
	defer monitor.Stop()

	// 并发更新多个心跳
	numJobs := 10
	numPartitions := 3

	for i := 0; i < numJobs; i++ {
		for j := 0; j < numPartitions; j++ {
			go func(jobIndex, partitionIndex int) {
				jobID := fmt.Sprintf("job-%d", jobIndex)
				partitionKey := fmt.Sprintf("partition-%d", partitionIndex)
				monitor.UpdateHeartbeat(jobID, partitionKey, "processing", map[string]string{
					"job_index":       fmt.Sprintf("%d", jobIndex),
					"partition_index": fmt.Sprintf("%d", partitionIndex),
				})
			}(i, j)
		}
	}

	// 等待所有goroutine完成
	time.Sleep(100 * time.Millisecond)

	// 验证所有心跳都被正确记录
	allHeartbeats := monitor.GetAllHeartbeats()
	expectedCount := numJobs * numPartitions
	assert.Equal(t, expectedCount, len(allHeartbeats), "Should have correct number of heartbeats")

	// 验证每个心跳信息的正确性
	for key, heartbeatInfo := range allHeartbeats {
		assert.True(t, heartbeatInfo.IsHealthy, "Heartbeat %s should be healthy", key)
		assert.NotEmpty(t, heartbeatInfo.JobID, "Job ID should not be empty")
		assert.NotEmpty(t, heartbeatInfo.PartitionKey, "Partition key should not be empty")
		assert.Equal(t, "processing", heartbeatInfo.Status, "Status should be processing")
	}
}

func TestHeartbeatMonitorErrorHandling(t *testing.T) {
	// 创建心跳监控器
	monitor := NewHeartbeatMonitorWithConfig(1*time.Second, 3*time.Second, func(jobID, partitionKey string) {
		t.Logf("Timeout for job %s partition %s", jobID, partitionKey)
	})

	// 启动监控器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor.Start(ctx)
	defer monitor.Stop()

	jobID := "error-test-job"
	partitionKey := "error-partition"

	// 更新心跳并模拟错误
	monitor.UpdateHeartbeat(jobID, partitionKey, "error", map[string]string{
		"error_message": "Test error",
	})

	// 获取心跳信息
	heartbeatInfo, exists := monitor.GetHeartbeatInfo(jobID, partitionKey)
	require.True(t, exists, "Heartbeat info should exist")

	// 验证错误状态
	assert.Equal(t, "error", heartbeatInfo.Status)
	assert.False(t, heartbeatInfo.IsHealthy, "Should be unhealthy when status is error")
	assert.Equal(t, int64(1), heartbeatInfo.ErrorCount, "Error count should be 1 after first error")

	// 多次更新错误状态
	for i := 0; i < 5; i++ {
		monitor.UpdateHeartbeat(jobID, partitionKey, "error", map[string]string{
			"error_count": fmt.Sprintf("%d", i+1),
		})
	}

	// 验证错误计数
	heartbeatInfo, exists = monitor.GetHeartbeatInfo(jobID, partitionKey)
	require.True(t, exists, "Heartbeat info should exist")
	assert.True(t, heartbeatInfo.ErrorCount > 0, "Error count should be greater than 0")
}
