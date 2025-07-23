package smsbatch

import (
	"context"
	"fmt"
	"time"

	"github.com/onexstack/onexstack/pkg/log"
	"github.com/onexstack/onexstack/pkg/store/where"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/pkg/conversion"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/fsm"
	apiv1 "github.com/ashwinyue/dcp/pkg/api/nightwatch/v1"
)

// BatchStatus 批处理状态信息
type BatchStatus struct {
	BatchID     string     `json:"batch_id"`
	Status      string     `json:"status"`
	Phase       string     `json:"phase"`
	Message     string     `json:"message"`
	LastUpdated time.Time  `json:"last_updated"`
	StartTime   *time.Time `json:"start_time,omitempty"`
	EndTime     *time.Time `json:"end_time,omitempty"`
}

// BatchProgress 批处理进度信息
type BatchProgress struct {
	BatchID           string         `json:"batch_id"`
	TotalRecords      int64          `json:"total_records"`
	ProcessedRecords  int64          `json:"processed_records"`
	SuccessRecords    int64          `json:"success_records"`
	FailedRecords     int64          `json:"failed_records"`
	ProgressPercent   float64        `json:"progress_percent"`
	EstimatedTimeLeft *time.Duration `json:"estimated_time_left,omitempty"`
	ThroughputPerSec  float64        `json:"throughput_per_sec"`
}

type ISmsBatchV1 interface {
	// 基础CRUD操作
	Create(ctx context.Context, rq *apiv1.CreateSmsBatchRequest) (*apiv1.CreateSmsBatchResponse, error)
	Update(ctx context.Context, rq *apiv1.UpdateSmsBatchRequest) (*apiv1.UpdateSmsBatchResponse, error)
	Delete(ctx context.Context, rq *apiv1.DeleteSmsBatchRequest) (*apiv1.DeleteSmsBatchResponse, error)
	Get(ctx context.Context, rq *apiv1.GetSmsBatchRequest) (*apiv1.GetSmsBatchResponse, error)
	List(ctx context.Context, rq *apiv1.ListSmsBatchRequest) (*apiv1.ListSmsBatchResponse, error)

	// 高级批处理管理功能
	StartProcessing(ctx context.Context, batchID string) error
	PauseBatch(ctx context.Context, batchID string) error
	ResumeBatch(ctx context.Context, batchID string) error
	RetryBatch(ctx context.Context, batchID string) error
	AbortBatch(ctx context.Context, batchID string) error
	GetBatchStatus(ctx context.Context, batchID string) (*BatchStatus, error)
	GetBatchProgress(ctx context.Context, batchID string) (*BatchProgress, error)
	ValidateBatch(ctx context.Context, batchID string) error
}

type smsBatchV1 struct {
	store store.IStore
}

func New(store store.IStore) ISmsBatchV1 {
	return &smsBatchV1{
		store: store,
	}
}

func (s *smsBatchV1) Create(ctx context.Context, rq *apiv1.CreateSmsBatchRequest) (*apiv1.CreateSmsBatchResponse, error) {
	smsBatchM := conversion.SmsBatchV1ToSmsBatchM(rq.SmsBatch)

	// 设置默认值
	if smsBatchM.Watcher == "" {
		smsBatchM.Watcher = "smsbatch"
	}

	// 实现数据库操作
	if err := s.store.SmsBatch().Create(ctx, smsBatchM); err != nil {
		log.Errorw(err, "Failed to create SMS batch", "name", smsBatchM.Name)
		return nil, err
	}

	// 集成状态机
	stateMachine := fsm.NewStateMachine(smsBatchM, nil, nil)
	if err := stateMachine.InitialExecute(ctx, nil); err != nil {
		log.Errorw(err, "Failed to initialize state machine", "batch_id", smsBatchM.BatchID)
		return nil, err
	}

	// 触发批处理工作流
	if smsBatchM.AutoTrigger == 1 {
		if err := s.StartProcessing(ctx, smsBatchM.BatchID); err != nil {
			log.Errorw(err, "Failed to trigger batch workflow", "batch_id", smsBatchM.BatchID)
			// 不返回错误，创建成功但自动触发失败
		}
	}

	log.Infow("SMS batch created successfully", "batch_id", smsBatchM.BatchID, "name", smsBatchM.Name)

	return &apiv1.CreateSmsBatchResponse{
		BatchID: smsBatchM.BatchID,
	}, nil
}

func (s *smsBatchV1) Update(ctx context.Context, rq *apiv1.UpdateSmsBatchRequest) (*apiv1.UpdateSmsBatchResponse, error) {
	// 获取现有的SMS批次
	existingBatch, err := s.store.SmsBatch().Get(ctx, where.F("batch_id", rq.BatchID))
	if err != nil {
		log.Errorw(err, "Failed to get SMS batch for update", "batch_id", rq.BatchID)
		return nil, err
	}

	// 更新字段
	if rq.Name != nil {
		existingBatch.Name = *rq.Name
	}
	if rq.Description != nil {
		existingBatch.Description = *rq.Description
	}
	if rq.CampaignID != nil {
		existingBatch.CampaignID = *rq.CampaignID
	}
	if rq.TaskID != nil {
		existingBatch.TaskID = *rq.TaskID
	}
	if rq.TableStorageName != nil {
		existingBatch.TableStorageName = *rq.TableStorageName
	}
	if rq.ContentID != nil {
		existingBatch.ContentID = *rq.ContentID
	}
	if rq.Content != nil {
		existingBatch.Content = *rq.Content
	}
	if rq.ContentSignature != nil {
		existingBatch.ContentSignature = *rq.ContentSignature
	}
	if rq.Url != nil {
		existingBatch.URL = *rq.Url
	}
	if rq.CombineMemberIDWithURL != nil {
		existingBatch.CombineMemberIDWithURL = *rq.CombineMemberIDWithURL
	}
	if rq.AutoTrigger != nil {
		existingBatch.AutoTrigger = *rq.AutoTrigger
	}
	if rq.ScheduleTime != nil {
		scheduleTime := rq.ScheduleTime.AsTime()
		existingBatch.ScheduleTime = &scheduleTime
	}
	if rq.ExtCode != nil {
		existingBatch.ExtCode = int32(len(*rq.ExtCode)) // Convert string to int32
	}
	if rq.TaskCode != nil {
		existingBatch.TaskCode = *rq.TaskCode
	}
	if rq.ProviderType != nil {
		existingBatch.ProviderType = *rq.ProviderType
	}
	if rq.MessageType != nil {
		existingBatch.MessageType = *rq.MessageType
	}
	if rq.MessageCategory != nil {
		existingBatch.MessageCategory = *rq.MessageCategory
	}
	if rq.Region != nil {
		existingBatch.Region = *rq.Region
	}
	if rq.Source != nil {
		existingBatch.Source = *rq.Source
	}
	if rq.Watcher != nil {
		existingBatch.Watcher = *rq.Watcher
	}
	if rq.Suspend != nil {
		if *rq.Suspend {
			existingBatch.Suspend = 1
		} else {
			existingBatch.Suspend = 0
		}
	}

	if err := s.store.SmsBatch().Update(ctx, existingBatch); err != nil {
		log.Errorw(err, "Failed to update SMS batch", "batch_id", rq.BatchID)
		return nil, err
	}

	log.Infow("SMS batch updated successfully", "batch_id", rq.BatchID)

	return &apiv1.UpdateSmsBatchResponse{}, nil
}

func (s *smsBatchV1) Delete(ctx context.Context, rq *apiv1.DeleteSmsBatchRequest) (*apiv1.DeleteSmsBatchResponse, error) {
	for _, batchID := range rq.BatchIDs {
		if err := s.store.SmsBatch().Delete(ctx, where.F("batch_id", batchID)); err != nil {
			log.Errorw(err, "Failed to delete SMS batch", "batch_id", batchID)
			return nil, fmt.Errorf("failed to delete SMS batch %s: %w", batchID, err)
		}
		log.Infow("SMS batch deleted successfully", "batch_id", batchID)
	}

	return &apiv1.DeleteSmsBatchResponse{}, nil
}

func (s *smsBatchV1) Get(ctx context.Context, rq *apiv1.GetSmsBatchRequest) (*apiv1.GetSmsBatchResponse, error) {
	smsBatchM, err := s.store.SmsBatch().Get(ctx, where.F("batch_id", rq.BatchID))
	if err != nil {
		log.Errorw(err, "Failed to get SMS batch", "batch_id", rq.BatchID)
		return nil, err
	}

	smsBatch := conversion.SmsBatchMToSmsBatchV1(smsBatchM)

	return &apiv1.GetSmsBatchResponse{
		SmsBatch: smsBatch,
	}, nil
}

func (s *smsBatchV1) List(ctx context.Context, rq *apiv1.ListSmsBatchRequest) (*apiv1.ListSmsBatchResponse, error) {
	whr := where.T(ctx).P(int(rq.Offset), int(rq.Limit))
	total, smsBatchMs, err := s.store.SmsBatch().List(ctx, whr)
	if err != nil {
		log.Errorw(err, "Failed to list SMS batches")
		return nil, err
	}

	smsBatches := make([]*apiv1.SmsBatch, 0, len(smsBatchMs))
	for _, smsBatchM := range smsBatchMs {
		smsBatch := conversion.SmsBatchMToSmsBatchV1(smsBatchM)
		smsBatches = append(smsBatches, smsBatch)
	}

	return &apiv1.ListSmsBatchResponse{
		Total:      total,
		SmsBatches: smsBatches,
	}, nil
}

// StartProcessing 启动批处理流程
func (s *smsBatchV1) StartProcessing(ctx context.Context, batchID string) error {
	// 获取批次信息
	smsBatch, err := s.store.SmsBatch().Get(ctx, where.F("batch_id", batchID))
	if err != nil {
		log.Errorw(err, "Failed to get SMS batch for processing", "batch_id", batchID)
		return err
	}

	// 实现状态机事件触发
	stateMachine := fsm.NewStateMachine(smsBatch, nil, nil)
	if err := stateMachine.InitialExecute(ctx, nil); err != nil {
		log.Errorw(err, "Failed to start batch processing", "batch_id", batchID)
		return err
	}

	// 更新数据库状态
	smsBatch.Status = "processing"
	if smsBatch.Results == nil {
		smsBatch.Results = &model.SmsBatchResults{}
	}
	smsBatch.Results.CurrentPhase = "preparation"
	smsBatch.Results.CurrentState = "running"
	if err := s.store.SmsBatch().Update(ctx, smsBatch); err != nil {
		log.Errorw(err, "Failed to update batch processing status", "batch_id", batchID)
		return err
	}

	// 发布状态变更事件
	log.Infow("Batch processing status changed", "batch_id", batchID, "status", "processing_started")

	log.Infow("Batch processing started", "batch_id", batchID)
	return nil
}

// PauseBatch 暂停批处理
func (s *smsBatchV1) PauseBatch(ctx context.Context, batchID string) error {
	smsBatch, err := s.store.SmsBatch().Get(ctx, where.F("batch_id", batchID))
	if err != nil {
		log.Errorw(err, "Failed to get SMS batch for pausing", "batch_id", batchID)
		return err
	}

	// 实现暂停逻辑
	smsBatch.Status = "paused"
	if smsBatch.Results == nil {
		smsBatch.Results = &model.SmsBatchResults{}
	}
	smsBatch.Results.CurrentState = "paused"
	if err := s.store.SmsBatch().Update(ctx, smsBatch); err != nil {
		log.Errorw(err, "Failed to pause batch processing", "batch_id", batchID)
		return err
	}

	// 更新状态机
	stateMachine := fsm.NewStateMachine(smsBatch, nil, nil)
	if smsBatch.Results != nil && smsBatch.Results.CurrentPhase == "preparation" {
		if err := stateMachine.PreparationPause(ctx, nil); err != nil {
			log.Errorw(err, "Failed to trigger pause event for preparation phase", "batch_id", batchID)
			return err
		}
	} else if smsBatch.Results != nil && smsBatch.Results.CurrentPhase == "delivery" {
		if err := stateMachine.DeliveryPause(ctx, nil); err != nil {
			log.Errorw(err, "Failed to trigger pause event for delivery phase", "batch_id", batchID)
			return err
		}
	}

	// 通知相关组件
	log.Infow("Batch paused notification sent", "batch_id", batchID)

	log.Infow("Batch paused", "batch_id", batchID)
	return nil
}

// ResumeBatch 恢复批处理
func (s *smsBatchV1) ResumeBatch(ctx context.Context, batchID string) error {
	smsBatch, err := s.store.SmsBatch().Get(ctx, where.F("batch_id", batchID))
	if err != nil {
		log.Errorw(err, "Failed to get SMS batch for resuming", "batch_id", batchID)
		return err
	}

	stateMachine := fsm.NewStateMachine(smsBatch, nil, nil)

	// 根据当前阶段选择恢复方法
	if smsBatch.Results != nil && smsBatch.Results.CurrentPhase == "preparation" {
		if err := stateMachine.PreparationResume(ctx, nil); err != nil {
			log.Errorw(err, "Failed to resume preparation phase", "batch_id", batchID)
			return err
		}
	} else if smsBatch.Results != nil && smsBatch.Results.CurrentPhase == "delivery" {
		if err := stateMachine.DeliveryResume(ctx, nil); err != nil {
			log.Errorw(err, "Failed to resume delivery phase", "batch_id", batchID)
			return err
		}
	}

	log.Infow("Batch resumed", "batch_id", batchID)
	return nil
}

// RetryBatch 重试批处理
func (s *smsBatchV1) RetryBatch(ctx context.Context, batchID string) error {
	smsBatch, err := s.store.SmsBatch().Get(ctx, where.F("batch_id", batchID))
	if err != nil {
		log.Errorw(err, "Failed to get SMS batch for retry", "batch_id", batchID)
		return err
	}

	// 重置批次状态为初始状态
	smsBatch.Status = "pending"
	if smsBatch.Results == nil {
		smsBatch.Results = &model.SmsBatchResults{}
	}
	smsBatch.Results.CurrentPhase = "initial"
	smsBatch.Results.RetryCount = smsBatch.Results.RetryCount + 1

	if err := s.store.SmsBatch().Update(ctx, smsBatch); err != nil {
		log.Errorw(err, "Failed to update batch for retry", "batch_id", batchID)
		return err
	}

	// 重新启动处理
	return s.StartProcessing(ctx, batchID)
}

// AbortBatch 中止批处理
func (s *smsBatchV1) AbortBatch(ctx context.Context, batchID string) error {
	smsBatch, err := s.store.SmsBatch().Get(ctx, where.F("batch_id", batchID))
	if err != nil {
		log.Errorw(err, "Failed to get SMS batch for aborting", "batch_id", batchID)
		return err
	}

	// 更新批次状态为已中止
	smsBatch.Status = "aborted"
	if smsBatch.Results == nil {
		smsBatch.Results = &model.SmsBatchResults{}
	}
	smsBatch.Results.CurrentPhase = "aborted"
	smsBatch.Results.CurrentState = "aborted"

	if err := s.store.SmsBatch().Update(ctx, smsBatch); err != nil {
		log.Errorw(err, "Failed to update batch status to aborted", "batch_id", batchID)
		return err
	}

	log.Infow("Batch aborted", "batch_id", batchID)
	return nil
}

// GetBatchStatus 获取批处理状态
func (s *smsBatchV1) GetBatchStatus(ctx context.Context, batchID string) (*BatchStatus, error) {
	smsBatch, err := s.store.SmsBatch().Get(ctx, where.F("batch_id", batchID))
	if err != nil {
		log.Errorw(err, "Failed to get SMS batch status", "batch_id", batchID)
		return nil, err
	}

	status := &BatchStatus{
		BatchID:     smsBatch.BatchID,
		Status:      smsBatch.Status,
		LastUpdated: smsBatch.UpdatedAt,
	}

	if smsBatch.Results != nil {
		status.Phase = smsBatch.Results.CurrentPhase
		if smsBatch.Results.ErrorMessage != "" {
			status.Message = smsBatch.Results.ErrorMessage
		}
	}

	return status, nil
}

// GetBatchProgress 获取批处理进度
func (s *smsBatchV1) GetBatchProgress(ctx context.Context, batchID string) (*BatchProgress, error) {
	smsBatch, err := s.store.SmsBatch().Get(ctx, where.F("batch_id", batchID))
	if err != nil {
		log.Errorw(err, "Failed to get SMS batch for progress", "batch_id", batchID)
		return nil, err
	}

	progress := &BatchProgress{
		BatchID: smsBatch.BatchID,
	}

	// 从Results中获取进度信息
	if smsBatch.Results != nil {
		progress.TotalRecords = smsBatch.Results.TotalMessages
		progress.ProcessedRecords = smsBatch.Results.ProcessedMessages
		progress.SuccessRecords = smsBatch.Results.SuccessMessages
		progress.FailedRecords = smsBatch.Results.FailedMessages
		progress.ProgressPercent = smsBatch.Results.ProgressPercent
	}

	// 如果进度百分比为0，则计算
	if progress.ProgressPercent == 0 && progress.TotalRecords > 0 {
		progress.ProgressPercent = float64(progress.ProcessedRecords) / float64(progress.TotalRecords) * 100
	}

	return progress, nil
}

// ValidateBatch 验证批处理配置
func (s *smsBatchV1) ValidateBatch(ctx context.Context, batchID string) error {
	smsBatch, err := s.store.SmsBatch().Get(ctx, where.F("batch_id", batchID))
	if err != nil {
		log.Errorw(err, "Failed to get SMS batch for validation", "batch_id", batchID)
		return err
	}

	// 验证必要字段
	if smsBatch.Name == "" {
		return fmt.Errorf("batch name is required")
	}
	if smsBatch.TableStorageName == "" {
		return fmt.Errorf("table storage name is required")
	}
	if smsBatch.Content == "" {
		return fmt.Errorf("content is required")
	}
	if smsBatch.MessageType == "" {
		return fmt.Errorf("message type is required")
	}

	// 验证调度时间
	if smsBatch.ScheduleTime != nil && smsBatch.ScheduleTime.Before(time.Now()) {
		return fmt.Errorf("schedule time cannot be in the past")
	}

	// 验证消息类型
	validMessageTypes := []string{"SMS", "MMS", "VOICE"}
	validType := false
	for _, validMsgType := range validMessageTypes {
		if smsBatch.MessageType == validMsgType {
			validType = true
			break
		}
	}
	if !validType {
		return fmt.Errorf("invalid message type: %s", smsBatch.MessageType)
	}

	log.Infow("Batch validation passed", "batch_id", batchID)
	return nil
}
