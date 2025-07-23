// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/onexstack/onexstack/pkg/log"

	"github.com/ashwinyue/dcp/internal/nightwatch/biz"
	"github.com/ashwinyue/dcp/internal/nightwatch/pkg/validation"
	apiv1 "github.com/ashwinyue/dcp/pkg/api/nightwatch/v1"
)

// Handler 处理博客模块的请求.
type Handler struct {
	biz biz.IBiz
	val *validation.Validator
}

// NewHandler 创建新的 Handler 实例.
func NewHandler(biz biz.IBiz, val *validation.Validator) *Handler {
	return &Handler{
		biz: biz,
		val: val,
	}
}

// UpdateJobStatus 更新任务状态 HTTP 接口
func (h *Handler) UpdateJobStatus(c *gin.Context) {
	log.Infow("Update job status function called.")

	var req apiv1.UpdateJobStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw(err, "Failed to bind update job status request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.biz.StatusTrackingV1().UpdateJobStatus(c.Request.Context(), &req)
	if err != nil {
		log.Errorw(err, "Failed to update job status")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetJobStatus 获取任务状态 HTTP 接口
func (h *Handler) GetJobStatus(c *gin.Context) {
	log.Infow("Get job status function called.")

	jobId := c.Param("jobId")
	if jobId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jobId is required"})
		return
	}

	req := &apiv1.GetJobStatusRequest{
		JobId: jobId,
	}

	resp, err := h.biz.StatusTrackingV1().GetJobStatus(c.Request.Context(), req)
	if err != nil {
		log.Errorw(err, "Failed to get job status")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// TrackRunningJobs 跟踪运行中的任务 HTTP 接口
func (h *Handler) TrackRunningJobs(c *gin.Context) {
	log.Infow("Track running jobs function called.")

	var req apiv1.TrackRunningJobsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw(err, "Failed to bind track running jobs request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.biz.StatusTrackingV1().TrackRunningJobs(c.Request.Context(), &req)
	if err != nil {
		log.Errorw(err, "Failed to track running jobs")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetJobStatistics 获取任务统计信息 HTTP 接口
func (h *Handler) GetJobStatistics(c *gin.Context) {
	log.Infow("Get job statistics function called.")

	var req apiv1.GetJobStatisticsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw(err, "Failed to bind get job statistics request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.biz.StatusTrackingV1().GetJobStatistics(c.Request.Context(), &req)
	if err != nil {
		log.Errorw(err, "Failed to get job statistics")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetBatchStatistics 获取批处理统计信息 HTTP 接口
func (h *Handler) GetBatchStatistics(c *gin.Context) {
	log.Infow("Get batch statistics function called.")

	var req apiv1.GetBatchStatisticsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw(err, "Failed to bind get batch statistics request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.biz.StatusTrackingV1().GetBatchStatistics(c.Request.Context(), &req)
	if err != nil {
		log.Errorw(err, "Failed to get batch statistics")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// JobHealthCheck 任务健康检查 HTTP 接口
func (h *Handler) JobHealthCheck(c *gin.Context) {
	log.Infow("Job health check function called.")

	var req apiv1.JobHealthCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw(err, "Failed to bind job health check request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.biz.StatusTrackingV1().JobHealthCheck(c.Request.Context(), &req)
	if err != nil {
		log.Errorw(err, "Failed to perform job health check")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetSystemMetrics 获取系统指标 HTTP 接口
func (h *Handler) GetSystemMetrics(c *gin.Context) {
	log.Infow("Get system metrics function called.")

	var req apiv1.GetSystemMetricsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw(err, "Failed to bind get system metrics request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.biz.StatusTrackingV1().GetSystemMetrics(c.Request.Context(), &req)
	if err != nil {
		log.Errorw(err, "Failed to get system metrics")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// CleanupExpiredStatus 清理过期状态 HTTP 接口
func (h *Handler) CleanupExpiredStatus(c *gin.Context) {
	log.Infow("Cleanup expired status function called.")

	var req apiv1.CleanupExpiredStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw(err, "Failed to bind cleanup expired status request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.biz.StatusTrackingV1().CleanupExpiredStatus(c.Request.Context(), &req)
	if err != nil {
		log.Errorw(err, "Failed to cleanup expired status")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// StartStatusWatcher 启动状态监控器 HTTP 接口
func (h *Handler) StartStatusWatcher(c *gin.Context) {
	log.Infow("Start status watcher function called.")

	var req apiv1.StartStatusWatcherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw(err, "Failed to bind start status watcher request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.biz.StatusTrackingV1().StartStatusWatcher(c.Request.Context(), &req)
	if err != nil {
		log.Errorw(err, "Failed to start status watcher")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// StopStatusWatcher 停止状态监控器 HTTP 接口
func (h *Handler) StopStatusWatcher(c *gin.Context) {
	log.Infow("Stop status watcher function called.")

	var req apiv1.StopStatusWatcherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw(err, "Failed to bind stop status watcher request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.biz.StatusTrackingV1().StopStatusWatcher(c.Request.Context(), &req)
	if err != nil {
		log.Errorw(err, "Failed to stop status watcher")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
