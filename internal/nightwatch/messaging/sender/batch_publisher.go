package sender

import (
	"context"
	"fmt"
	"time"
)

// BatchEventPublisher handles batch event publishing
type BatchEventPublisher struct {
	sender *KafkaSender
}

// NewBatchEventPublisher creates a new BatchEventPublisher
func NewBatchEventPublisher(sender *KafkaSender) *BatchEventPublisher {
	return &BatchEventPublisher{
		sender: sender,
	}
}

// PublishSmsBatchStatusChanged publishes SMS batch status change event
func (p *BatchEventPublisher) PublishSmsBatchStatusChanged(ctx context.Context, batchID, status, phase string, progress float64, processed, success, failed int64) error {
	if p.sender == nil {
		return fmt.Errorf("sender is not configured")
	}

	statusUpdate := map[string]interface{}{
		"request_id":    fmt.Sprintf("%s-status-%d", batchID, time.Now().Unix()),
		"batch_id":      batchID,
		"status":        status,
		"current_phase": phase,
		"progress":      progress,
		"processed":     processed,
		"success":       success,
		"failed":        failed,
		"updated_at":    time.Now(),
		"event_type":    "batch_status_update",
	}

	msg := &Message{
		Key:   batchID,
		Value: statusUpdate,
		Headers: map[string]string{
			"event_type": "batch_status_update",
			"batch_id":   batchID,
			"status":     status,
		},
	}

	return p.sender.SendMessage(msg)
}

// PublishSmsBatchPaused publishes SMS batch paused event
func (p *BatchEventPublisher) PublishSmsBatchPaused(ctx context.Context, batchID, currentPhase, pauseReason, operatorID string) error {
	if p.sender == nil {
		return fmt.Errorf("sender is not configured")
	}

	pauseEvent := map[string]interface{}{
		"request_id":     fmt.Sprintf("%s-pause-%d", batchID, time.Now().Unix()),
		"batch_id":       batchID,
		"operation":      "pause",
		"phase":          currentPhase,
		"reason":         pauseReason,
		"operator_id":    operatorID,
		"requested_at":   time.Now(),
		"event_type":     "batch_operation",
	}

	msg := &Message{
		Key:   batchID,
		Value: pauseEvent,
		Headers: map[string]string{
			"event_type":  "batch_operation",
			"batch_id":    batchID,
			"operation":   "pause",
			"operator_id": operatorID,
		},
	}

	return p.sender.SendMessage(msg)
}

// PublishSmsBatchResumed publishes SMS batch resumed event
func (p *BatchEventPublisher) PublishSmsBatchResumed(ctx context.Context, batchID, currentPhase, resumeReason, operatorID string) error {
	if p.sender == nil {
		return fmt.Errorf("sender is not configured")
	}

	resumeEvent := map[string]interface{}{
		"request_id":     fmt.Sprintf("%s-resume-%d", batchID, time.Now().Unix()),
		"batch_id":       batchID,
		"operation":      "resume",
		"phase":          currentPhase,
		"reason":         resumeReason,
		"operator_id":    operatorID,
		"requested_at":   time.Now(),
		"event_type":     "batch_operation",
	}

	msg := &Message{
		Key:   batchID,
		Value: resumeEvent,
		Headers: map[string]string{
			"event_type":  "batch_operation",
			"batch_id":    batchID,
			"operation":   "resume",
			"operator_id": operatorID,
		},
	}

	return p.sender.SendMessage(msg)
}

// BatchOperationRequest represents a batch operation request
type BatchOperationRequest struct {
	RequestID   string                 `json:"request_id"`
	BatchID     string                 `json:"batch_id"`
	Operation   string                 `json:"operation"`
	Phase       string                 `json:"phase,omitempty"`
	Reason      string                 `json:"reason,omitempty"`
	OperatorID  string                 `json:"operator_id"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	RequestedAt time.Time              `json:"requested_at"`
}

// PublishBatchOperation publishes batch operation event
func (p *BatchEventPublisher) PublishBatchOperation(ctx context.Context, req *BatchOperationRequest) error {
	if p.sender == nil {
		return fmt.Errorf("sender is not configured")
	}

	msg := &Message{
		Key:   req.BatchID,
		Value: req,
		Headers: map[string]string{
			"event_type":  "batch_operation",
			"batch_id":    req.BatchID,
			"operation":   req.Operation,
			"operator_id": req.OperatorID,
		},
	}

	return p.sender.SendMessage(msg)
}