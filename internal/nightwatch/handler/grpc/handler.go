// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

package grpc

import (
	"context"

	"github.com/ashwinyue/dcp/internal/nightwatch/biz"
	apiv1 "github.com/ashwinyue/dcp/pkg/api/nightwatch/v1"
)

// Handler 负责处理博客模块的请求.
type Handler struct {
	apiv1.UnimplementedNightwatchServer

	biz biz.IBiz
}

// NewHandler 创建一个新的 Handler 实例.
func NewHandler(biz biz.IBiz) *Handler {
	return &Handler{
		biz: biz,
	}
}

// UpdateJobStatus 更新任务状态
func (h *Handler) UpdateJobStatus(ctx context.Context, req *apiv1.UpdateJobStatusRequest) (*apiv1.UpdateJobStatusResponse, error) {
	return h.biz.StatusTrackingV1().UpdateJobStatus(ctx, req)
}

// GetJobStatus 获取任务状态
func (h *Handler) GetJobStatus(ctx context.Context, req *apiv1.GetJobStatusRequest) (*apiv1.GetJobStatusResponse, error) {
	return h.biz.StatusTrackingV1().GetJobStatus(ctx, req)
}

// TrackRunningJobs 跟踪运行中的任务
func (h *Handler) TrackRunningJobs(ctx context.Context, req *apiv1.TrackRunningJobsRequest) (*apiv1.TrackRunningJobsResponse, error) {
	return h.biz.StatusTrackingV1().TrackRunningJobs(ctx, req)
}

// GetJobStatistics 获取任务统计信息
func (h *Handler) GetJobStatistics(ctx context.Context, req *apiv1.GetJobStatisticsRequest) (*apiv1.GetJobStatisticsResponse, error) {
	return h.biz.StatusTrackingV1().GetJobStatistics(ctx, req)
}

// GetBatchStatistics 获取批处理统计信息
func (h *Handler) GetBatchStatistics(ctx context.Context, req *apiv1.GetBatchStatisticsRequest) (*apiv1.GetBatchStatisticsResponse, error) {
	return h.biz.StatusTrackingV1().GetBatchStatistics(ctx, req)
}

// JobHealthCheck 任务健康检查
func (h *Handler) JobHealthCheck(ctx context.Context, req *apiv1.JobHealthCheckRequest) (*apiv1.JobHealthCheckResponse, error) {
	return h.biz.StatusTrackingV1().JobHealthCheck(ctx, req)
}

// GetSystemMetrics 获取系统指标
func (h *Handler) GetSystemMetrics(ctx context.Context, req *apiv1.GetSystemMetricsRequest) (*apiv1.GetSystemMetricsResponse, error) {
	return h.biz.StatusTrackingV1().GetSystemMetrics(ctx, req)
}

// CleanupExpiredStatus 清理过期状态
func (h *Handler) CleanupExpiredStatus(ctx context.Context, req *apiv1.CleanupExpiredStatusRequest) (*apiv1.CleanupExpiredStatusResponse, error) {
	return h.biz.StatusTrackingV1().CleanupExpiredStatus(ctx, req)
}

// StartStatusWatcher 启动状态监控器
func (h *Handler) StartStatusWatcher(ctx context.Context, req *apiv1.StartStatusWatcherRequest) (*apiv1.StartStatusWatcherResponse, error) {
	return h.biz.StatusTrackingV1().StartStatusWatcher(ctx, req)
}

// StopStatusWatcher 停止状态监控器
func (h *Handler) StopStatusWatcher(ctx context.Context, req *apiv1.StopStatusWatcherRequest) (*apiv1.StopStatusWatcherResponse, error) {
	return h.biz.StatusTrackingV1().StopStatusWatcher(ctx, req)
}
