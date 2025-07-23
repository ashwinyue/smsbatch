// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

package statustracking

import (
	"context"
	"fmt"
	"time"

	"github.com/onexstack/onexstack/pkg/log"
	"github.com/onexstack/onexstack/pkg/store/where"

	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	apiv1 "github.com/ashwinyue/dcp/pkg/api/nightwatch/v1"
)

// IStatusTrackingV1 定义状态跟踪业务逻辑接口，专门用于SMS批处理状态管理
type IStatusTrackingV1 interface {
	// UpdateJobStatus 更新SMS批处理状态
	UpdateJobStatus(ctx context.Context, req *apiv1.UpdateJobStatusRequest) (*apiv1.UpdateJobStatusResponse, error)
	// GetJobStatus 获取SMS批处理状态
	GetJobStatus(ctx context.Context, req *apiv1.GetJobStatusRequest) (*apiv1.GetJobStatusResponse, error)
	// TrackRunningJobs 跟踪运行中的SMS批处理
	TrackRunningJobs(ctx context.Context, req *apiv1.TrackRunningJobsRequest) (*apiv1.TrackRunningJobsResponse, error)
	// GetJobStatistics 获取SMS批处理统计信息
	GetJobStatistics(ctx context.Context, req *apiv1.GetJobStatisticsRequest) (*apiv1.GetJobStatisticsResponse, error)
	// GetBatchStatistics 获取批处理统计信息
	GetBatchStatistics(ctx context.Context, req *apiv1.GetBatchStatisticsRequest) (*apiv1.GetBatchStatisticsResponse, error)
	// JobHealthCheck SMS批处理健康检查
	JobHealthCheck(ctx context.Context, req *apiv1.JobHealthCheckRequest) (*apiv1.JobHealthCheckResponse, error)
	// GetSystemMetrics 获取系统指标
	GetSystemMetrics(ctx context.Context, req *apiv1.GetSystemMetricsRequest) (*apiv1.GetSystemMetricsResponse, error)
	// CleanupExpiredStatus 清理过期状态
	CleanupExpiredStatus(ctx context.Context, req *apiv1.CleanupExpiredStatusRequest) (*apiv1.CleanupExpiredStatusResponse, error)
	// StartStatusWatcher 启动状态监控器
	StartStatusWatcher(ctx context.Context, req *apiv1.StartStatusWatcherRequest) (*apiv1.StartStatusWatcherResponse, error)
	// StopStatusWatcher 停止状态监控器
	StopStatusWatcher(ctx context.Context, req *apiv1.StopStatusWatcherRequest) (*apiv1.StopStatusWatcherResponse, error)
	// UpdateHeartbeat 更新任务心跳
	UpdateHeartbeat(ctx context.Context, jobID, partitionKey, status string, metrics map[string]string) error
	// GetHeartbeatStatus 获取心跳状态
	GetHeartbeatStatus(ctx context.Context, jobID, partitionKey string) (*HeartbeatInfo, error)
	// GetAllHeartbeats 获取所有心跳信息
	GetAllHeartbeats(ctx context.Context) (map[string]*HeartbeatInfo, error)
}

type statusTrackingV1 struct {
	store            store.IStore
	heartbeatMonitor *HeartbeatMonitor
}

// New 创建状态跟踪业务逻辑实例
func New(store store.IStore) IStatusTrackingV1 {
	heartbeatMonitor := NewHeartbeatMonitor(store)
	return &statusTrackingV1{
		store:            store,
		heartbeatMonitor: heartbeatMonitor,
	}
}

// UpdateJobStatus 更新SMS批处理状态
func (s *statusTrackingV1) UpdateJobStatus(ctx context.Context, req *apiv1.UpdateJobStatusRequest) (*apiv1.UpdateJobStatusResponse, error) {
	log.Infow("Updating SMS batch status", "batch_id", req.JobId, "status", req.Status)

	// 根据批次ID查询SMS批处理信息
	smsBatch, err := s.store.SmsBatch().Get(ctx, where.NewWhere(where.WithQuery("batch_id = ?", req.JobId)))
	if err != nil {
		log.Errorw(err, "Failed to get SMS batch", "batch_id", req.JobId)
		return &apiv1.UpdateJobStatusResponse{
			Success: false,
			Message: fmt.Sprintf("SMS batch not found: %v", err),
		}, nil
	}

	// 更新状态
	smsBatch.Status = req.Status
	smsBatch.UpdatedAt = time.Now()

	// 如果提供了时间戳，使用提供的时间戳
	if req.Timestamp > 0 {
		smsBatch.UpdatedAt = time.Unix(req.Timestamp, 0)
	}

	// 保存更新
	err = s.store.SmsBatch().Update(ctx, smsBatch)
	if err != nil {
		log.Errorw(err, "Failed to update SMS batch status", "batch_id", req.JobId)
		return &apiv1.UpdateJobStatusResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to update SMS batch status: %v", err),
		}, nil
	}

	log.Infow("SMS batch status updated successfully", "batch_id", req.JobId, "status", req.Status)

	return &apiv1.UpdateJobStatusResponse{
		Success: true,
		Message: "SMS batch status updated successfully",
	}, nil
}

// GetJobStatus 获取SMS批处理状态
func (s *statusTrackingV1) GetJobStatus(ctx context.Context, req *apiv1.GetJobStatusRequest) (*apiv1.GetJobStatusResponse, error) {
	log.Infow("Getting SMS batch status", "batch_id", req.JobId)

	// 根据批次ID查询SMS批处理信息
	smsBatch, err := s.store.SmsBatch().Get(ctx, where.NewWhere(where.WithQuery("batch_id = ?", req.JobId)))
	if err != nil {
		log.Errorw(err, "Failed to get SMS batch", "batch_id", req.JobId)
		return nil, fmt.Errorf("SMS batch not found: %w", err)
	}

	// 构建响应
	response := &apiv1.GetJobStatusResponse{
		JobId:     smsBatch.BatchID,
		Status:    smsBatch.Status,
		UpdatedAt: smsBatch.UpdatedAt.Unix(),
	}

	// 设置开始时间
	if !smsBatch.StartedAt.IsZero() {
		response.StartedAt = smsBatch.StartedAt.Unix()
	}

	// 设置结束时间
	if !smsBatch.EndedAt.IsZero() {
		response.EndedAt = smsBatch.EndedAt.Unix()
	}

	// 计算运行时间
	if !smsBatch.StartedAt.IsZero() {
		endTime := time.Now()
		if !smsBatch.EndedAt.IsZero() {
			endTime = smsBatch.EndedAt
		}
		response.RuntimeSeconds = int64(endTime.Sub(smsBatch.StartedAt).Seconds())
	}

	log.Infow("SMS batch status retrieved successfully", "batch_id", req.JobId, "status", smsBatch.Status)

	return response, nil
}

// UpdateHeartbeat 更新任务心跳
func (s *statusTrackingV1) UpdateHeartbeat(ctx context.Context, jobID, partitionKey, status string, metrics map[string]string) error {
	log.Debugw("Updating heartbeat", "job_id", jobID, "partition_key", partitionKey, "status", status)

	s.heartbeatMonitor.UpdateHeartbeat(jobID, partitionKey, status, metrics)

	return nil
}

// GetHeartbeatStatus 获取心跳状态
func (s *statusTrackingV1) GetHeartbeatStatus(ctx context.Context, jobID, partitionKey string) (*HeartbeatInfo, error) {
	log.Debugw("Getting heartbeat status", "job_id", jobID, "partition_key", partitionKey)

	heartbeatInfo, exists := s.heartbeatMonitor.GetHeartbeatInfo(jobID, partitionKey)
	if !exists {
		return nil, fmt.Errorf("heartbeat info not found for job %s partition %s", jobID, partitionKey)
	}

	return heartbeatInfo, nil
}

// GetAllHeartbeats 获取所有心跳信息
func (s *statusTrackingV1) GetAllHeartbeats(ctx context.Context) (map[string]*HeartbeatInfo, error) {
	log.Debugw("Getting all heartbeat information")

	heartbeats := s.heartbeatMonitor.GetAllHeartbeats()

	log.Infow("Retrieved all heartbeat information", "count", len(heartbeats))

	return heartbeats, nil
}

// watchStatus 监控状态的内部方法
func (s *statusTrackingV1) watchStatus(ctx context.Context, intervalSeconds int32) {
	// 实现状态监控逻辑
	log.Infow("Status watcher running", "interval_seconds", intervalSeconds)

	// 定期检查运行中的任务并更新心跳
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Infow("Status watcher stopped due to context cancellation")
			return
		case <-ticker.C:
			s.checkAndUpdateRunningJobs(ctx)
		}
	}
}

// checkAndUpdateRunningJobs 检查并更新运行中的任务
func (s *statusTrackingV1) checkAndUpdateRunningJobs(ctx context.Context) {
	// 查询运行中的SMS批处理
	whereOpts := where.NewWhere(
		where.WithQuery("status IN ?", []string{"processing", "retrying", "pending"}),
	)

	_, runningBatches, err := s.store.SmsBatch().List(ctx, whereOpts)
	if err != nil {
		log.Errorw(err, "Failed to list running SMS batches")
		return
	}

	// 为每个运行中的批次更新心跳
	for _, batch := range runningBatches {
		metrics := map[string]string{
			"batch_status": batch.Status,
			"updated_at":   batch.UpdatedAt.Format(time.RFC3339),
		}

		if !batch.StartedAt.IsZero() {
			metrics["started_at"] = batch.StartedAt.Format(time.RFC3339)
			processingTime := time.Since(batch.StartedAt)
			metrics["processing_time_ms"] = fmt.Sprintf("%d", processingTime.Milliseconds())
		}

		s.heartbeatMonitor.UpdateHeartbeat(batch.BatchID, "default", batch.Status, metrics)
	}

	log.Debugw("Updated heartbeats for running jobs", "count", len(runningBatches))
}

// TrackRunningJobs 跟踪运行中的SMS批处理
func (s *statusTrackingV1) TrackRunningJobs(ctx context.Context, req *apiv1.TrackRunningJobsRequest) (*apiv1.TrackRunningJobsResponse, error) {
	log.Infow("Tracking running SMS batches")

	// 构建查询条件
	whereOpts := where.NewWhere(where.WithQuery("status IN ?", []string{"processing", "pending", "retrying"}))

	// 如果提供了作用域过滤
	if req.Scope != nil && *req.Scope != "" {
		whereOpts = where.NewWhere(
			where.WithQuery("status IN ? AND scope = ?", []string{"processing", "pending", "retrying"}, *req.Scope),
		)
	}

	// 如果提供了监控器过滤
	if req.Watcher != nil && *req.Watcher != "" {
		if req.Scope != nil && *req.Scope != "" {
			whereOpts = where.NewWhere(
				where.WithQuery("status IN ? AND scope = ? AND watcher = ?", []string{"processing", "pending", "retrying"}, *req.Scope, *req.Watcher),
			)
		} else {
			whereOpts = where.NewWhere(
				where.WithQuery("status IN ? AND watcher = ?", []string{"processing", "pending", "retrying"}, *req.Watcher),
			)
		}
	}

	// 查询运行中的SMS批处理
	count, smsBatches, err := s.store.SmsBatch().List(ctx, whereOpts)
	if err != nil {
		log.Errorw(err, "Failed to list running SMS batches")
		return nil, fmt.Errorf("failed to track running SMS batches: %w", err)
	}

	// 构建响应
	runningJobs := make([]*apiv1.RunningJobInfo, 0, len(smsBatches))
	for _, batch := range smsBatches {
		jobInfo := &apiv1.RunningJobInfo{
			JobId:  batch.BatchID,
			Status: batch.Status,
		}

		// 设置开始时间和运行时间
		if !batch.StartedAt.IsZero() {
			jobInfo.StartedAt = batch.StartedAt.Unix()
			jobInfo.RuntimeSeconds = int64(time.Since(batch.StartedAt).Seconds())
		}

		// 设置健康状态（简单判断：如果有错误则不健康）
		jobInfo.IsHealthy = batch.Status != "failed"

		runningJobs = append(runningJobs, jobInfo)
	}

	response := &apiv1.TrackRunningJobsResponse{
		TotalCount:  count,
		RunningJobs: runningJobs,
		TrackTime:   time.Now().Unix(),
	}

	log.Infow("Running SMS batches tracked successfully", "count", count)

	return response, nil
}

// GetJobStatistics 获取SMS批处理统计信息
func (s *statusTrackingV1) GetJobStatistics(ctx context.Context, req *apiv1.GetJobStatisticsRequest) (*apiv1.GetJobStatisticsResponse, error) {
	log.Infow("Getting SMS batch statistics")

	// 查询所有SMS批处理
	_, smsBatches, err := s.store.SmsBatch().List(ctx, where.NewWhere())
	if err != nil {
		log.Errorw(err, "Failed to list SMS batches for statistics")
		return nil, fmt.Errorf("failed to get SMS batch statistics: %w", err)
	}

	// 统计各种状态的数量
	var totalJobs, pendingJobs, runningJobs, succeededJobs, failedJobs int64
	var totalRuntimeSeconds int64
	var runtimeCount int64

	for _, batch := range smsBatches {
		totalJobs++
		switch batch.Status {
		case "pending":
			pendingJobs++
		case "processing", "retrying":
			runningJobs++
		case "completed":
			succeededJobs++
		case "failed":
			failedJobs++
		}

		// 计算运行时间
		if !batch.StartedAt.IsZero() && !batch.EndedAt.IsZero() {
			runtimeSeconds := int64(batch.EndedAt.Sub(batch.StartedAt).Seconds())
			totalRuntimeSeconds += runtimeSeconds
			runtimeCount++
		}
	}

	// 计算平均运行时间
	var avgRuntimeSeconds int64
	if runtimeCount > 0 {
		avgRuntimeSeconds = totalRuntimeSeconds / runtimeCount
	}

	// 计算成功率
	var successRate float64
	if totalJobs > 0 {
		successRate = float64(succeededJobs) / float64(totalJobs) * 100
	}

	response := &apiv1.GetJobStatisticsResponse{
		TotalJobs:         totalJobs,
		PendingJobs:       pendingJobs,
		RunningJobs:       runningJobs,
		SucceededJobs:     succeededJobs,
		FailedJobs:        failedJobs,
		AvgRuntimeSeconds: avgRuntimeSeconds,
		SuccessRate:       successRate,
		GeneratedAt:       time.Now().Unix(),
	}

	log.Infow("SMS batch statistics retrieved successfully", "total_batches", len(smsBatches))

	return response, nil
}

// GetBatchStatistics 获取批处理统计信息
func (s *statusTrackingV1) GetBatchStatistics(ctx context.Context, req *apiv1.GetBatchStatisticsRequest) (*apiv1.GetBatchStatisticsResponse, error) {
	log.Infow("Getting batch statistics")

	// 查询所有SMS批处理
	_, smsBatches, err := s.store.SmsBatch().List(ctx, where.NewWhere())
	if err != nil {
		log.Errorw(err, "Failed to list SMS batches for batch statistics")
		return nil, fmt.Errorf("failed to get batch statistics: %w", err)
	}

	// 构建批处理状态信息
	batchStatuses := make([]*apiv1.BatchStatusInfo, 0, len(smsBatches))
	for _, batch := range smsBatches {
		batchInfo := &apiv1.BatchStatusInfo{
			BatchId: batch.BatchID,
			Status:  batch.Status,
		}

		if !batch.StartedAt.IsZero() {
			batchInfo.StartedAt = batch.StartedAt.Unix()
		}

		// BatchStatusInfo没有EndedAt字段，使用UpdatedAt代替
		if !batch.UpdatedAt.IsZero() {
			batchInfo.UpdatedAt = batch.UpdatedAt.Unix()
		}

		batchStatuses = append(batchStatuses, batchInfo)
	}

	response := &apiv1.GetBatchStatisticsResponse{
		BatchStats:   batchStatuses,
		TotalBatches: int64(len(smsBatches)),
		GeneratedAt:  time.Now().Unix(),
	}

	log.Infow("Batch statistics retrieved successfully", "total_batches", len(smsBatches))

	return response, nil
}

// JobHealthCheck SMS批处理健康检查
func (s *statusTrackingV1) JobHealthCheck(ctx context.Context, req *apiv1.JobHealthCheckRequest) (*apiv1.JobHealthCheckResponse, error) {
	log.Infow("Performing SMS batch health check", "batch_id", req.JobId)

	// 根据批次ID查询SMS批处理信息
	smsBatch, err := s.store.SmsBatch().Get(ctx, where.NewWhere(where.WithQuery("batch_id = ?", req.JobId)))
	if err != nil {
		log.Errorw(err, "Failed to get SMS batch for health check", "batch_id", req.JobId)
		return nil, fmt.Errorf("SMS batch not found: %w", err)
	}

	// 从心跳监控器获取心跳信息
	heartbeatInfo, hasHeartbeat := s.heartbeatMonitor.GetHeartbeatInfo(req.JobId, "default")

	// 构建健康状态
	healthStatus := &apiv1.JobHealthStatus{
		JobId:            smsBatch.BatchID,
		Status:           smsBatch.Status,
		IsHealthy:        smsBatch.Status != "failed" && smsBatch.Status != "error",
		LastHeartbeat:    time.Now().Unix(),
		ProcessingTimeMs: 0, // 可以根据需要计算
		Metrics:          make(map[string]string),
		ErrorCount:       0,
		RetryCount:       0,
	}

	// 如果有心跳信息，使用心跳数据
	if hasHeartbeat {
		healthStatus.LastHeartbeat = heartbeatInfo.LastHeartbeat.Unix()
		healthStatus.IsHealthy = heartbeatInfo.IsHealthy
		healthStatus.ErrorCount = heartbeatInfo.ErrorCount
		healthStatus.RetryCount = heartbeatInfo.RetryCount

		// 复制心跳指标
		for k, v := range heartbeatInfo.Metrics {
			healthStatus.Metrics[k] = v
		}

		// 计算处理时间
		if !smsBatch.StartedAt.IsZero() {
			processingTime := time.Since(smsBatch.StartedAt)
			healthStatus.ProcessingTimeMs = int64(processingTime.Milliseconds())
		}
	} else {
		// 没有心跳信息时，基于数据库状态判断
		if smsBatch.Status == "failed" {
			healthStatus.Metrics["error"] = "SMS batch execution failed"
			healthStatus.IsHealthy = false
		}
	}

	// 添加批次基本信息到指标中
	healthStatus.Metrics["batch_status"] = smsBatch.Status
	healthStatus.Metrics["created_at"] = smsBatch.CreatedAt.Format(time.RFC3339)
	if !smsBatch.StartedAt.IsZero() {
		healthStatus.Metrics["started_at"] = smsBatch.StartedAt.Format(time.RFC3339)
	}

	response := &apiv1.JobHealthCheckResponse{
		HealthStatus: healthStatus,
	}

	log.Infow("SMS batch health check completed",
		"batch_id", req.JobId,
		"is_healthy", healthStatus.IsHealthy,
		"has_heartbeat", hasHeartbeat,
		"error_count", healthStatus.ErrorCount)

	return response, nil
}

// GetSystemMetrics 获取系统指标
func (s *statusTrackingV1) GetSystemMetrics(ctx context.Context, req *apiv1.GetSystemMetricsRequest) (*apiv1.GetSystemMetricsResponse, error) {
	log.Infow("Getting system metrics")

	// 查询所有SMS批处理
	_, smsBatches, err := s.store.SmsBatch().List(ctx, where.NewWhere())
	if err != nil {
		log.Errorw(err, "Failed to list SMS batches for system metrics")
		return nil, fmt.Errorf("failed to get system metrics: %w", err)
	}

	// 计算系统指标
	var runningCount, completedCount, failedCount, pendingCount int64
	for _, batch := range smsBatches {
		switch batch.Status {
		case "processing", "retrying":
			runningCount++
		case "pending":
			pendingCount++
		case "completed":
			completedCount++
		case "failed":
			failedCount++
		}
	}

	// 计算成功率
	totalJobs := int64(len(smsBatches))
	var successRate float64
	if totalJobs > 0 {
		successRate = float64(completedCount) / float64(totalJobs) * 100
	}

	metrics := &apiv1.SystemMetrics{
		Timestamp:     time.Now().Unix(),
		ActiveJobs:    runningCount,
		CompletedJobs: completedCount,
		FailedJobs:    failedCount,
		PendingJobs:   pendingCount,
		SuccessRate:   successRate,
		ResourceUsage: make(map[string]string),
	}

	response := &apiv1.GetSystemMetricsResponse{
		SystemMetrics: metrics,
	}

	log.Infow("System metrics retrieved successfully", "active_jobs", runningCount, "completed_jobs", completedCount)

	return response, nil
}

// CleanupExpiredStatus 清理过期状态
func (s *statusTrackingV1) CleanupExpiredStatus(ctx context.Context, req *apiv1.CleanupExpiredStatusRequest) (*apiv1.CleanupExpiredStatusResponse, error) {
	log.Infow("Cleaning up expired SMS batch status")

	// 计算过期时间
	expiredBefore := time.Now().Add(-time.Duration(req.RetentionHours) * time.Hour)
	log.Infow("Cleaning up expired status", "retention_hours", req.RetentionHours)

	// 查询过期的已完成或失败的SMS批处理
	whereOpts := where.NewWhere(
		where.WithQuery("status IN ? AND updated_at < ?", []string{"completed", "failed", "cancelled"}, expiredBefore),
	)

	count, expiredBatches, err := s.store.SmsBatch().List(ctx, whereOpts)
	if err != nil {
		log.Errorw(err, "Failed to list expired SMS batches")
		return nil, fmt.Errorf("failed to cleanup expired status: %w", err)
	}

	// 删除过期的SMS批处理记录
	var cleanedCount int64
	for _, batch := range expiredBatches {
		err := s.store.SmsBatch().Delete(ctx, where.F("id", batch.ID))
		if err != nil {
			log.Errorw(err, "Failed to delete expired SMS batch", "batch_id", batch.BatchID)
			continue
		}
		cleanedCount++
	}

	response := &apiv1.CleanupExpiredStatusResponse{
		CleanedCount: cleanedCount,
		Message:      fmt.Sprintf("Cleaned up %d expired SMS batch records", cleanedCount),
	}

	log.Infow("Expired SMS batch status cleanup completed", "cleaned_count", cleanedCount, "total_expired", count)

	return response, nil
}

// StartStatusWatcher 启动状态监控器
func (s *statusTrackingV1) StartStatusWatcher(ctx context.Context, req *apiv1.StartStatusWatcherRequest) (*apiv1.StartStatusWatcherResponse, error) {
	intervalSeconds := int32(30) // 默认30秒
	if req.IntervalSeconds != nil {
		intervalSeconds = *req.IntervalSeconds
	}
	log.Infow("Starting status watcher", "interval_seconds", intervalSeconds)

	// 设置心跳监控器的检查间隔
	checkInterval := time.Duration(intervalSeconds) * time.Second
	s.heartbeatMonitor.SetCheckInterval(checkInterval)

	// 启动心跳监控器
	err := s.heartbeatMonitor.Start(ctx)
	if err != nil {
		log.Errorw(err, "Failed to start heartbeat monitor")
		return &apiv1.StartStatusWatcherResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to start heartbeat monitor: %v", err),
		}, nil
	}

	// 启动传统的状态监控
	go s.watchStatus(ctx, intervalSeconds)

	response := &apiv1.StartStatusWatcherResponse{
		Success: true,
		Message: fmt.Sprintf("Status watcher and heartbeat monitor started successfully with interval %d seconds", intervalSeconds),
	}

	log.Infow("Status watcher and heartbeat monitor started successfully", "interval_seconds", intervalSeconds)

	return response, nil
}

// StopStatusWatcher 停止状态监控器
func (s *statusTrackingV1) StopStatusWatcher(ctx context.Context, req *apiv1.StopStatusWatcherRequest) (*apiv1.StopStatusWatcherResponse, error) {
	log.Infow("Stopping status watcher")

	// 停止心跳监控器
	err := s.heartbeatMonitor.Stop()
	if err != nil {
		log.Errorw(err, "Failed to stop heartbeat monitor")
		return &apiv1.StopStatusWatcherResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to stop heartbeat monitor: %v", err),
		}, nil
	}

	// 这里可以实现停止传统状态监控器的逻辑
	// 例如发送信号给监控goroutine

	response := &apiv1.StopStatusWatcherResponse{
		Success: true,
		Message: "Status watcher and heartbeat monitor stopped successfully",
	}

	log.Infow("Status watcher and heartbeat monitor stopped successfully")

	return response, nil
}
