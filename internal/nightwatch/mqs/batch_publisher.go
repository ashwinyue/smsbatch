package mqs

import (
	"context"
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/message"
	"github.com/ashwinyue/dcp/internal/pkg/log"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

// BatchEventPublisher publishes batch-related events to Kafka
type BatchEventPublisher struct {
	kafkaSender *message.KafkaSender
	topic       string
}

// NewBatchEventPublisher creates a new batch event publisher
func NewBatchEventPublisher(kafkaSender *message.KafkaSender, topic string) *BatchEventPublisher {
	return &BatchEventPublisher{
		kafkaSender: kafkaSender,
		topic:       topic,
	}
}

// PublishBatchRequest publishes a batch processing request
func (p *BatchEventPublisher) PublishBatchRequest(ctx context.Context, req *BatchMessageRequest) error {
	if req.RequestID == "" {
		req.RequestID = uuid.New().String()
	}
	req.CreatedAt = time.Now()

	headers := []kafka.Header{
		{Key: "message_type", Value: []byte("batch_request")},
		{Key: "batch_type", Value: []byte(req.BatchType)},
		{Key: "batch_id", Value: []byte(req.BatchID)},
		{Key: "user_id", Value: []byte(req.UserID)},
		{Key: "timestamp", Value: []byte(fmt.Sprintf("%d", time.Now().Unix()))},
	}

	// Convert headers to map[string]string
	headerMap := make(map[string]string)
	for _, header := range headers {
		headerMap[header.Key] = string(header.Value)
	}

	msg := &message.Message{
		Key:       req.BatchID,
		Value:     req,
		Headers:   headerMap,
		Timestamp: time.Now(),
	}

	err := p.kafkaSender.SendMessage(msg)
	if err != nil {
		log.Errorw("Failed to publish batch request", "error", err, "batch_id", req.BatchID)
		return err
	}

	log.Infow("Published batch request", "batch_id", req.BatchID, "request_id", req.RequestID)
	return nil
}

// PublishBatchStatusUpdate publishes a batch status update event
func (p *BatchEventPublisher) PublishBatchStatusUpdate(ctx context.Context, req *BatchStatusUpdate) error {
	if req.RequestID == "" {
		req.RequestID = uuid.New().String()
	}
	req.UpdatedAt = time.Now()

	headers := []kafka.Header{
		{Key: "message_type", Value: []byte("batch_status_update")},
		{Key: "batch_id", Value: []byte(req.BatchID)},
		{Key: "status", Value: []byte(req.Status)},
		{Key: "current_phase", Value: []byte(req.CurrentPhase)},
		{Key: "timestamp", Value: []byte(fmt.Sprintf("%d", time.Now().Unix()))},
	}

	// Convert headers to map[string]string
	headerMap := make(map[string]string)
	for _, header := range headers {
		headerMap[header.Key] = string(header.Value)
	}

	msg := &message.Message{
		Key:       req.BatchID,
		Value:     req,
		Headers:   headerMap,
		Timestamp: time.Now(),
	}

	err := p.kafkaSender.SendMessage(msg)
	if err != nil {
		log.Errorw("Failed to publish batch status update", "error", err, "batch_id", req.BatchID)
		return err
	}

	log.Infow("Published batch status update", "batch_id", req.BatchID, "status", req.Status, "progress", req.Progress)
	return nil
}

// PublishBatchOperation publishes a batch operation command
func (p *BatchEventPublisher) PublishBatchOperation(ctx context.Context, req *BatchOperationCommand) error {
	if req.RequestID == "" {
		req.RequestID = uuid.New().String()
	}
	req.RequestedAt = time.Now()

	headers := []kafka.Header{
		{Key: "message_type", Value: []byte("batch_operation")},
		{Key: "batch_id", Value: []byte(req.BatchID)},
		{Key: "operation", Value: []byte(req.Operation)},
		{Key: "phase", Value: []byte(req.Phase)},
		{Key: "operator_id", Value: []byte(req.OperatorID)},
		{Key: "timestamp", Value: []byte(fmt.Sprintf("%d", time.Now().Unix()))},
	}

	// Convert headers to map[string]string
	headerMap := make(map[string]string)
	for _, header := range headers {
		headerMap[header.Key] = string(header.Value)
	}

	msg := &message.Message{
		Key:       req.BatchID,
		Value:     req,
		Headers:   headerMap,
		Timestamp: time.Now(),
	}

	err := p.kafkaSender.SendMessage(msg)
	if err != nil {
		log.Errorw("Failed to publish batch operation", "error", err, "batch_id", req.BatchID)
		return err
	}

	log.Infow("Published batch operation", "batch_id", req.BatchID, "operation", req.Operation, "operator_id", req.OperatorID)
	return nil
}

// PublishSmsBatchCreated publishes an SMS batch creation event
func (p *BatchEventPublisher) PublishSmsBatchCreated(ctx context.Context, batchID, userID string, recipients []string, template string, params map[string]interface{}) error {
	req := &BatchMessageRequest{
		BatchID:     batchID,
		UserID:      userID,
		BatchType:   "sms",
		MessageType: "template",
		Recipients:  recipients,
		Template:    template,
		Params:      params,
		Priority:    5,   // default priority
		MaxRetries:  3,   // default max retries
		Timeout:     300, // 5 minutes default timeout
	}

	return p.PublishBatchRequest(ctx, req)
}

// PublishSmsBatchStatusChanged publishes an SMS batch status change event
func (p *BatchEventPublisher) PublishSmsBatchStatusChanged(ctx context.Context, batchID, status, phase string, progress float64, processed, success, failed int64) error {
	req := &BatchStatusUpdate{
		BatchID:      batchID,
		Status:       status,
		CurrentPhase: phase,
		Progress:     progress,
		Processed:    processed,
		Success:      success,
		Failed:       failed,
	}

	return p.PublishBatchStatusUpdate(ctx, req)
}

// PublishSmsBatchPaused publishes an SMS batch pause event
func (p *BatchEventPublisher) PublishSmsBatchPaused(ctx context.Context, batchID, phase, reason, operatorID string) error {
	req := &BatchOperationRequest{
		BatchID:    batchID,
		Operation:  "pause",
		Phase:      phase,
		Reason:     reason,
		OperatorID: operatorID,
	}

	return p.PublishBatchOperation(ctx, req)
}

// PublishSmsBatchResumed publishes an SMS batch resume event
func (p *BatchEventPublisher) PublishSmsBatchResumed(ctx context.Context, batchID, phase, reason, operatorID string) error {
	req := &BatchOperationRequest{
		BatchID:    batchID,
		Operation:  "resume",
		Phase:      phase,
		Reason:     reason,
		OperatorID: operatorID,
	}

	return p.PublishBatchOperation(ctx, req)
}

// PublishSmsBatchAborted publishes an SMS batch abort event
func (p *BatchEventPublisher) PublishSmsBatchAborted(ctx context.Context, batchID, reason, operatorID string) error {
	req := &BatchOperationRequest{
		BatchID:    batchID,
		Operation:  "abort",
		Reason:     reason,
		OperatorID: operatorID,
	}

	return p.PublishBatchOperation(ctx, req)
}

// PublishSmsBatchRetried publishes an SMS batch retry event
func (p *BatchEventPublisher) PublishSmsBatchRetried(ctx context.Context, batchID, phase, reason, operatorID string) error {
	req := &BatchOperationRequest{
		BatchID:    batchID,
		Operation:  "retry",
		Phase:      phase,
		Reason:     reason,
		OperatorID: operatorID,
	}

	return p.PublishBatchOperation(ctx, req)
}
