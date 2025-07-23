// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

package http

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/onexstack/onexstack/pkg/core"

	"github.com/ashwinyue/dcp/internal/pkg/errno"
	"github.com/onexstack/onexstack/pkg/log"
)

// HeartbeatUpdateRequest 心跳更新请求
type HeartbeatUpdateRequest struct {
	JobID        string            `json:"jobId" binding:"required"`
	PartitionKey string            `json:"partitionKey"`
	Status       string            `json:"status" binding:"required"`
	Metrics      map[string]string `json:"metrics,omitempty"`
}

// HeartbeatResponse 心跳响应
type HeartbeatResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// HeartbeatStatusResponse 心跳状态响应
type HeartbeatStatusResponse struct {
	JobID         string            `json:"jobId"`
	PartitionKey  string            `json:"partitionKey"`
	LastHeartbeat int64             `json:"lastHeartbeat"`
	Status        string            `json:"status"`
	ErrorCount    int64             `json:"errorCount"`
	RetryCount    int64             `json:"retryCount"`
	Metrics       map[string]string `json:"metrics"`
	IsHealthy     bool              `json:"isHealthy"`
}

// AllHeartbeatsResponse 所有心跳信息响应
type AllHeartbeatsResponse struct {
	Heartbeats map[string]*HeartbeatStatusResponse `json:"heartbeats"`
	Count      int                                 `json:"count"`
	Timestamp  int64                               `json:"timestamp"`
}

// UpdateHeartbeat 更新心跳
func (h *Handler) UpdateHeartbeat(c *gin.Context) {
	log.Infow("UpdateHeartbeat handler called")

	var req HeartbeatUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw(err, "Invalid request body")
		core.WriteResponse(c, nil, err)
		return
	}

	// 如果没有指定分区键，使用默认值
	if req.PartitionKey == "" {
		req.PartitionKey = "default"
	}

	// 更新心跳
	err := h.biz.StatusTrackingV1().UpdateHeartbeat(c.Request.Context(), req.JobID, req.PartitionKey, req.Status, req.Metrics)
	if err != nil {
		log.Errorw(err, "Failed to update heartbeat", "job_id", req.JobID, "partition_key", req.PartitionKey)
		core.WriteResponse(c, nil, err)
		return
	}

	response := HeartbeatResponse{
		Success: true,
		Message: "Heartbeat updated successfully",
	}

	log.Infow("Heartbeat updated successfully", "job_id", req.JobID, "partition_key", req.PartitionKey)
	core.WriteResponse(c, response, nil)
}

// GetHeartbeatStatus 获取心跳状态
func (h *Handler) GetHeartbeatStatus(c *gin.Context) {
	jobID := c.Param("jobId")
	partitionKey := c.Query("partitionKey")

	if jobID == "" {
		log.Infow("Job ID is required")
		core.WriteResponse(c, nil, errno.ErrInvalidArgument)
		return
	}

	// 如果没有指定分区键，使用默认值
	if partitionKey == "" {
		partitionKey = "default"
	}

	log.Infow("Getting heartbeat status", "job_id", jobID, "partition_key", partitionKey)

	heartbeatInfo, err := h.biz.StatusTrackingV1().GetHeartbeatStatus(c.Request.Context(), jobID, partitionKey)
	if err != nil {
		log.Errorw(err, "Failed to get heartbeat status", "job_id", jobID, "partition_key", partitionKey)
		core.WriteResponse(c, nil, err)
		return
	}

	response := &HeartbeatStatusResponse{
		JobID:         heartbeatInfo.JobID,
		PartitionKey:  heartbeatInfo.PartitionKey,
		LastHeartbeat: heartbeatInfo.LastHeartbeat.Unix(),
		Status:        heartbeatInfo.Status,
		ErrorCount:    heartbeatInfo.ErrorCount,
		RetryCount:    heartbeatInfo.RetryCount,
		Metrics:       heartbeatInfo.Metrics,
		IsHealthy:     heartbeatInfo.IsHealthy,
	}

	log.Infow("Heartbeat status retrieved successfully", "job_id", jobID, "is_healthy", heartbeatInfo.IsHealthy)
	core.WriteResponse(c, response, nil)
}

// GetAllHeartbeats 获取所有心跳信息
func (h *Handler) GetAllHeartbeats(c *gin.Context) {
	log.Infow("Getting all heartbeat information")

	heartbeats, err := h.biz.StatusTrackingV1().GetAllHeartbeats(c.Request.Context())
	if err != nil {
		log.Errorw(err, "Failed to get all heartbeats")
		core.WriteResponse(c, nil, err)
		return
	}

	// 转换为响应格式
	responseHeartbeats := make(map[string]*HeartbeatStatusResponse)
	for key, heartbeatInfo := range heartbeats {
		responseHeartbeats[key] = &HeartbeatStatusResponse{
			JobID:         heartbeatInfo.JobID,
			PartitionKey:  heartbeatInfo.PartitionKey,
			LastHeartbeat: heartbeatInfo.LastHeartbeat.Unix(),
			Status:        heartbeatInfo.Status,
			ErrorCount:    heartbeatInfo.ErrorCount,
			RetryCount:    heartbeatInfo.RetryCount,
			Metrics:       heartbeatInfo.Metrics,
			IsHealthy:     heartbeatInfo.IsHealthy,
		}
	}

	response := &AllHeartbeatsResponse{
		Heartbeats: responseHeartbeats,
		Count:      len(responseHeartbeats),
		Timestamp:  time.Now().Unix(),
	}

	log.Infow("All heartbeat information retrieved successfully", "count", len(responseHeartbeats))
	core.WriteResponse(c, response, nil)
}

// GetHeartbeatMetrics 获取心跳监控指标
func (h *Handler) GetHeartbeatMetrics(c *gin.Context) {
	log.Infow("Getting heartbeat metrics")

	heartbeats, err := h.biz.StatusTrackingV1().GetAllHeartbeats(c.Request.Context())
	if err != nil {
		log.Errorw(err, "Failed to get heartbeats for metrics")
		core.WriteResponse(c, nil, err)
		return
	}

	// 计算指标
	totalJobs := len(heartbeats)
	healthyJobs := 0
	unhealthyJobs := 0
	statusCount := make(map[string]int)

	for _, heartbeatInfo := range heartbeats {
		if heartbeatInfo.IsHealthy {
			healthyJobs++
		} else {
			unhealthyJobs++
		}
		statusCount[heartbeatInfo.Status]++
	}

	// 计算健康率
	healthRate := 0.0
	if totalJobs > 0 {
		healthRate = float64(healthyJobs) / float64(totalJobs) * 100
	}

	metrics := map[string]interface{}{
		"total_jobs":     totalJobs,
		"healthy_jobs":   healthyJobs,
		"unhealthy_jobs": unhealthyJobs,
		"health_rate":    healthRate,
		"status_count":   statusCount,
		"timestamp":      time.Now().Unix(),
	}

	log.Infow("Heartbeat metrics calculated",
		"total_jobs", totalJobs,
		"healthy_jobs", healthyJobs,
		"health_rate", healthRate)

	core.WriteResponse(c, metrics, nil)
}
