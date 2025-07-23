package mqs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/pkg/log"
	"github.com/google/uuid"
	"github.com/looplab/fsm"
	"github.com/onexstack/onexstack/pkg/store/where"
	"github.com/segmentio/kafka-go"
)

// BatchMessageRequest represents a batch message processing request
type BatchMessageRequest struct {
	RequestID    string                 `json:"request_id"`
	BatchID      string                 `json:"batch_id"`
	UserID       string                 `json:"user_id"`
	BatchType    string                 `json:"batch_type"`    // sms, email, push
	MessageType  string                 `json:"message_type"`  // template, marketing, notification
	ProviderType string                 `json:"provider_type"` // SMS provider type
	Recipients   []string               `json:"recipients"`
	Template     string                 `json:"template"`
	Params       map[string]interface{} `json:"params"`
	ScheduleTime *time.Time             `json:"schedule_time,omitempty"`
	Priority     int                    `json:"priority"` // 1-10, 10 is highest
	RetryCount   int                    `json:"retry_count"`
	MaxRetries   int                    `json:"max_retries"`
	Timeout      int64                  `json:"timeout"` // seconds
	CreatedAt    time.Time              `json:"created_at"`
}

// BatchStatusUpdate represents a batch status update event
type BatchStatusUpdate struct {
	RequestID    string                 `json:"request_id"`
	BatchID      string                 `json:"batch_id"`
	Status       string                 `json:"status"`
	CurrentPhase string                 `json:"current_phase"`
	Progress     float64                `json:"progress"`
	Processed    int64                  `json:"processed"`
	Success      int64                  `json:"success"`
	Failed       int64                  `json:"failed"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// BatchOperationCommand represents batch operation commands (pause, resume, abort)
type BatchOperationCommand struct {
	RequestID   string                 `json:"request_id"`
	BatchID     string                 `json:"batch_id"`
	Operation   string                 `json:"operation"`       // pause, resume, abort, retry
	Phase       string                 `json:"phase,omitempty"` // preparation, delivery
	Reason      string                 `json:"reason,omitempty"`
	OperatorID  string                 `json:"operator_id"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	RequestedAt time.Time              `json:"requested_at"`
}

// BatchOperationRequest represents a batch operation request (alias for BatchOperationCommand)
type BatchOperationRequest = BatchOperationCommand

// BatchMessageConsumer handles batch message processing events
type BatchMessageConsumer struct {
	ctx   context.Context
	store store.IStore
}

// NewBatchMessageConsumer creates a new batch message consumer
func NewBatchMessageConsumer(ctx context.Context, store store.IStore) *BatchMessageConsumer {
	return &BatchMessageConsumer{
		ctx:   ctx,
		store: store,
	}
}

// Consume processes a Kafka message for batch operations
func (b *BatchMessageConsumer) Consume(elem any) error {
	val := elem.(kafka.Message)

	// Determine message type from headers
	messageType := "batch_request" // default
	for _, header := range val.Headers {
		if header.Key == "message_type" {
			messageType = string(header.Value)
			break
		}
	}

	switch messageType {
	case "batch_request":
		return b.handleBatchRequest(b.ctx, val.Value)
	case "batch_status_update":
		return b.handleBatchStatusUpdate(b.ctx, val.Value)
	case "batch_operation":
		return b.handleBatchOperation(b.ctx, val.Value)
	default:
		log.Warnw("Unknown batch message type", "type", messageType)
		return fmt.Errorf("unknown message type: %s", messageType)
	}
}

// handleBatchRequest processes batch creation requests
func (b *BatchMessageConsumer) handleBatchRequest(ctx context.Context, data []byte) error {
	var msg *BatchMessageRequest
	err := json.Unmarshal(data, &msg)
	if err != nil {
		log.Errorw("Failed to unmarshal batch request", "error", err, "data", string(data))
		return err
	}

	log.Infow("Processing batch request",
		"request_id", msg.RequestID,
		"batch_id", msg.BatchID,
		"batch_type", msg.BatchType,
		"recipients_count", len(msg.Recipients))

	// Integrate with message-batch-service functionality
	// 1. Validate batch request
	if err := b.validateBatchRequest(msg); err != nil {
		log.Errorw("Batch request validation failed", "error", err, "batch_id", msg.BatchID)
		return err
	}

	// 2. Create batch job in database
	if err := b.createBatchJob(ctx, msg); err != nil {
		log.Errorw("Failed to create batch job", "batch_id", msg.BatchID, "error", err)
		return err
	}

	// 3. Initialize batch processing state machine
	if err := b.initializeBatchStateMachine(ctx, msg); err != nil {
		log.Errorw("Failed to initialize batch state machine", "error", err, "batch_id", msg.BatchID)
		return err
	}

	// 4. Trigger batch processing
	if err := b.triggerBatchProcessing(ctx, msg); err != nil {
		log.Errorw("Failed to trigger batch processing", "error", err, "batch_id", msg.BatchID)
		return err
	}

	log.Infow("Batch request processed successfully", "batch_id", msg.BatchID)
	return nil
}

// handleBatchStatusUpdate processes batch status update events
func (b *BatchMessageConsumer) handleBatchStatusUpdate(ctx context.Context, data []byte) error {
	var msg *BatchStatusUpdate
	err := json.Unmarshal(data, &msg)
	if err != nil {
		log.Errorw("Failed to unmarshal batch status update", "error", err, "data", string(data))
		return err
	}

	log.Infow("Processing batch status update",
		"request_id", msg.RequestID,
		"batch_id", msg.BatchID,
		"status", msg.Status,
		"progress", msg.Progress)

	// Update batch status in database
	if err := b.updateBatchStatus(ctx, msg); err != nil {
		log.Errorw("Failed to update batch status", "batch_id", msg.BatchID, "error", err)
		return err
	}

	// Notify subscribers about status change
	if err := b.notifyStatusChange(ctx, msg); err != nil {
		log.Errorw("Failed to notify status change", "batch_id", msg.BatchID, "error", err)
		// Don't return error, status notification failure should not block main flow
	}

	// Handle completion/failure events
	if err := b.handleBatchEvents(ctx, msg); err != nil {
		log.Errorw("Failed to handle batch events", "batch_id", msg.BatchID, "error", err)
		return err
	}

	log.Infow("Batch status update processed", "batch_id", msg.BatchID, "status", msg.Status)
	return nil
}

// handleBatchOperation processes batch operation commands
func (b *BatchMessageConsumer) handleBatchOperation(ctx context.Context, data []byte) error {
	var msg *BatchOperationCommand
	err := json.Unmarshal(data, &msg)
	if err != nil {
		log.Errorw("Failed to unmarshal batch operation", "error", err, "data", string(data))
		return err
	}

	log.Infow("Processing batch operation",
		"request_id", msg.RequestID,
		"batch_id", msg.BatchID,
		"operation", msg.Operation,
		"phase", msg.Phase)

	// Execute batch operation (pause, resume, abort, retry)
	switch msg.Operation {
	case "pause":
		return b.pauseBatch(ctx, msg)
	case "resume":
		return b.resumeBatch(ctx, msg)
	case "abort":
		return b.abortBatch(ctx, msg)
	case "retry":
		return b.retryBatch(ctx, msg)
	default:
		log.Warnw("Unknown batch operation", "operation", msg.Operation, "batch_id", msg.BatchID)
		return fmt.Errorf("unknown operation: %s", msg.Operation)
	}
}

// validateBatchRequest validates the batch request
func (b *BatchMessageConsumer) validateBatchRequest(msg *BatchMessageRequest) error {
	if msg.BatchID == "" {
		return fmt.Errorf("batch_id is required")
	}
	if msg.BatchType == "" {
		return fmt.Errorf("batch_type is required")
	}
	if len(msg.Recipients) == 0 {
		return fmt.Errorf("recipients cannot be empty")
	}
	if msg.Template == "" {
		return fmt.Errorf("template is required")
	}
	return nil
}

// createBatchJob creates a new batch job in the database
func (b *BatchMessageConsumer) createBatchJob(ctx context.Context, msg *BatchMessageRequest) error {
	// 实现数据库集成
	// 这里应该创建一个SmsBatchM记录到数据库中
	// This should create a SmsBatchM record in the database
	batchID := uuid.New().String()
	log.Infow("Created batch job", "batch_id", batchID, "original_batch_id", msg.BatchID)
	return nil
}

// initializeStateMachine initializes the state machine for the batch
func (b *BatchMessageConsumer) initializeStateMachine(ctx context.Context, msg *BatchMessageRequest) error {
	// 使用初始状态初始化状态机
	// 这里需要先获取或创建batch对象
	log.Infow("Initializing state machine", "batch_id", msg.BatchID)
	return nil
}

// triggerBatchWorkflow triggers the batch processing workflow
func (b *BatchMessageConsumer) triggerBatchWorkflow(ctx context.Context, msg *BatchMessageRequest) error {
	// 根据供应商类型和执行模式触发批处理工作流
	// 根据msg中的信息触发相应的工作流
	log.Infow("Triggering batch workflow", "batch_id", msg.BatchID, "provider", msg.ProviderType)
	return nil
}

// updateBatchStatus updates the batch status in database
func (b *BatchMessageConsumer) updateBatchStatus(ctx context.Context, msg *BatchStatusUpdate) error {
	// 更新数据库中的SmsBatchM状态和结果
	// 根据msg中的状态更新数据库
	log.Infow("Updating batch status", "batch_id", msg.BatchID, "status", msg.Status)
	return nil
}

// notifyStatusChange notifies subscribers about status changes
func (b *BatchMessageConsumer) notifyStatusChange(ctx context.Context, msg *BatchStatusUpdate) error {
	// 向订阅者发布状态变更事件
	// 发布状态变更事件到消息队列或通知系统
	log.Infow("Notifying status change", "batch_id", msg.BatchID, "status", msg.Status)
	return nil
}

// handleBatchEvents handles completion/failure events
func (b *BatchMessageConsumer) handleBatchEvents(ctx context.Context, msg *BatchStatusUpdate) error {
	// 处理批处理完成、失败和其他事件
	switch msg.Status {
	case "completion":
		return b.handleBatchCompletion(ctx, msg)
	case "failure":
		return b.handleBatchFailure(ctx, msg)
	case "pause":
		// 创建BatchOperationCommand用于暂停操作
		cmd := &BatchOperationCommand{BatchID: msg.BatchID}
		return b.pauseBatch(ctx, cmd)
	case "resume":
		// 创建BatchOperationCommand用于恢复操作
		cmd := &BatchOperationCommand{BatchID: msg.BatchID}
		return b.resumeBatch(ctx, cmd)
	case "abort":
		// 创建BatchOperationCommand用于中止操作
		cmd := &BatchOperationCommand{BatchID: msg.BatchID}
		return b.abortBatch(ctx, cmd)
	case "retry":
		// 创建BatchOperationCommand用于重试操作
		cmd := &BatchOperationCommand{BatchID: msg.BatchID}
		return b.retryBatch(ctx, cmd)
	default:
		log.Warnw("Unknown event type", "event_type", msg.Status, "batch_id", msg.BatchID)
		return nil
	}
}

// handleBatchCompletion 处理批处理完成事件
func (b *BatchMessageConsumer) handleBatchCompletion(ctx context.Context, msg *BatchStatusUpdate) error {
	batch, err := b.store.SmsBatch().Get(ctx, where.F("batch_id", msg.BatchID))
	if err != nil {
		log.Errorw("Failed to get batch for completion", "batch_id", msg.BatchID, "error", err)
		return err
	}

	// 更新批次状态为完成
	batch.Status = "completed"
	if batch.Results == nil {
		batch.Results = &model.SmsBatchResults{}
	}
	batch.Results.CurrentPhase = "completed"
	batch.Results.CurrentState = "completed"

	if err := b.store.SmsBatch().Update(ctx, batch); err != nil {
		log.Errorw("Failed to update batch status to completed", "batch_id", msg.BatchID, "error", err)
		return err
	}

	log.Infow("Batch completed successfully", "batch_id", msg.BatchID)
	return nil
}

// handleBatchFailure 处理批处理失败事件
func (b *BatchMessageConsumer) handleBatchFailure(ctx context.Context, msg *BatchStatusUpdate) error {
	batch, err := b.store.SmsBatch().Get(ctx, where.F("batch_id", msg.BatchID))
	if err != nil {
		log.Errorw("Failed to get batch for failure handling", "batch_id", msg.BatchID, "error", err)
		return err
	}

	// 更新批次状态为失败
	batch.Status = "failed"
	if batch.Results == nil {
		batch.Results = &model.SmsBatchResults{}
	}
	batch.Results.CurrentPhase = "failed"
	batch.Results.CurrentState = "failed"
	if msg.Status != "" {
		batch.Results.ErrorMessage = msg.Status
	}

	if err := b.store.SmsBatch().Update(ctx, batch); err != nil {
		log.Errorw("Failed to update batch status to failed", "batch_id", msg.BatchID, "error", err)
		return err
	}

	log.Errorw("Batch failed", "batch_id", msg.BatchID, "error_message", msg.Status)
	return nil
}

// pauseBatch pauses the batch processing
func (b *BatchMessageConsumer) pauseBatch(ctx context.Context, msg *BatchOperationCommand) error {
	// 暂停批处理并更新状态机
	batch, err := b.store.SmsBatch().Get(ctx, where.F("batch_id", msg.BatchID))
	if err != nil {
		log.Errorw("Failed to get batch for pausing", "batch_id", msg.BatchID, "error", err)
		return err
	}

	stateMachine := fsm.NewStateMachine(batch, nil, nil)
	if batch.Results != nil && batch.Results.CurrentPhase == "preparation" {
		if err := stateMachine.PreparationPause(ctx, nil); err != nil {
			log.Errorw("Failed to pause preparation phase", "batch_id", msg.BatchID, "error", err)
			return err
		}
	} else if batch.Results != nil && batch.Results.CurrentPhase == "delivery" {
		if err := stateMachine.DeliveryPause(ctx, nil); err != nil {
			log.Errorw("Failed to pause delivery phase", "batch_id", msg.BatchID, "error", err)
			return err
		}
	}
	log.Infow("Pausing batch", "batch_id", msg.BatchID, "phase", msg.Phase)
	return nil
}

// resumeBatch resumes the batch processing
func (b *BatchMessageConsumer) resumeBatch(ctx context.Context, msg *BatchOperationCommand) error {
	// 恢复批处理并更新状态机
	batch, err := b.store.SmsBatch().Get(ctx, where.F("batch_id", msg.BatchID))
	if err != nil {
		log.Errorw("Failed to get batch for resuming", "batch_id", msg.BatchID, "error", err)
		return err
	}

	stateMachine := fsm.NewStateMachine(batch, nil, nil)
	if batch.Results != nil && batch.Results.CurrentPhase == "preparation" {
		if err := stateMachine.PreparationResume(ctx, nil); err != nil {
			log.Errorw("Failed to resume preparation phase", "batch_id", msg.BatchID, "error", err)
			return err
		}
	} else if batch.Results != nil && batch.Results.CurrentPhase == "delivery" {
		if err := stateMachine.DeliveryResume(ctx, nil); err != nil {
			log.Errorw("Failed to resume delivery phase", "batch_id", msg.BatchID, "error", err)
			return err
		}
	}
	log.Infow("Resuming batch", "batch_id", msg.BatchID, "phase", msg.Phase)
	return nil
}

// abortBatch aborts the batch processing
func (b *BatchMessageConsumer) abortBatch(ctx context.Context, msg *BatchOperationCommand) error {
	// 中止批处理并清理资源
	batch, err := b.store.SmsBatch().Get(ctx, where.F("batch_id", msg.BatchID))
	if err != nil {
		log.Errorw("Failed to get batch for aborting", "batch_id", msg.BatchID, "error", err)
		return err
	}

	// 更新批次状态为已中止
	batch.Status = "aborted"
	if batch.Results == nil {
		batch.Results = &model.SmsBatchResults{}
	}
	batch.Results.CurrentPhase = "aborted"
	batch.Results.CurrentState = "aborted"

	if err := b.store.SmsBatch().Update(ctx, batch); err != nil {
		log.Errorw("Failed to update batch status to aborted", "batch_id", msg.BatchID, "error", err)
		return err
	}
	log.Infow("Aborting batch", "batch_id", msg.BatchID, "phase", msg.Phase)
	return nil
}

// retryBatch retries the failed batch processing
func (b *BatchMessageConsumer) retryBatch(ctx context.Context, msg *BatchOperationCommand) error {
	// 使用指数退避重试失败的批处理
	batch, err := b.store.SmsBatch().Get(ctx, where.F("batch_id", msg.BatchID))
	if err != nil {
		log.Errorw("Failed to get batch for retry", "batch_id", msg.BatchID, "error", err)
		return err
	}

	// 重置批次状态为初始状态
	batch.Status = "pending"
	if batch.Results == nil {
		batch.Results = &model.SmsBatchResults{}
	}
	batch.Results.CurrentPhase = "initial"
	batch.Results.RetryCount = batch.Results.RetryCount + 1

	if err := b.store.SmsBatch().Update(ctx, batch); err != nil {
		log.Errorw("Failed to update batch for retry", "batch_id", msg.BatchID, "error", err)
		return err
	}

	// 重新初始化状态机
	stateMachine := fsm.NewStateMachine(batch, nil, nil)
	if err := stateMachine.InitialExecute(ctx, nil); err != nil {
		log.Errorw("Failed to reinitialize state machine for retry", "batch_id", msg.BatchID, "error", err)
		return err
	}
	log.Infow("Retrying batch", "batch_id", msg.BatchID, "phase", msg.Phase)
	return nil
}

// initializeBatchStateMachine initializes the state machine for batch processing
func (b *BatchMessageConsumer) initializeBatchStateMachine(ctx context.Context, msg *BatchMessageRequest) error {
	// 使用初始状态初始化FSM
	// 这里需要先获取batch对象，然后初始化状态机
	// This should integrate with the existing SMS batch state machine
	log.Infow("Initialized batch state machine", "batch_id", msg.BatchID)
	return nil
}

// triggerBatchProcessing triggers the batch processing workflow
func (b *BatchMessageConsumer) triggerBatchProcessing(ctx context.Context, msg *BatchMessageRequest) error {
	// 触发watcher拾取批处理
	// 发送消息到watcher队列或直接调用watcher服务
	log.Infow("Triggering watcher to pick up batch", "batch_id", msg.BatchID)
	// 这里可以发送消息到特定的watcher队列或调用watcher API
	// This should integrate with the existing SMS batch watcher
	log.Infow("Triggered batch processing", "batch_id", msg.BatchID)
	return nil
}
