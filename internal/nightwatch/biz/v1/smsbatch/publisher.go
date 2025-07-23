package smsbatch

import (
	"context"
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/messaging/sender"
)

// EventPublisher handles all SMS batch event publishing
type EventPublisher struct {
	batchPublisher *sender.BatchEventPublisher
}

// NewEventPublisher creates a new EventPublisher instance
func NewEventPublisher(batchPublisher *sender.BatchEventPublisher) *EventPublisher {
	return &EventPublisher{
		batchPublisher: batchPublisher,
	}
}

// SetBatchPublisher sets the batch publisher
func (ep *EventPublisher) SetBatchPublisher(batchPublisher interface{}) {
	if publisher, ok := batchPublisher.(*sender.BatchEventPublisher); ok {
		ep.batchPublisher = publisher
	}
}

// PublishPreparationProgress publishes preparation progress update
func (ep *EventPublisher) PublishPreparationProgress(ctx context.Context, batchID, status string, processed, success, failed int64, progress float64) error {
	if ep.batchPublisher == nil {
		return nil
	}

	return ep.batchPublisher.PublishSmsBatchStatusChanged(ctx, batchID, "preparation_running", "preparation", progress, processed, success, failed)
}

// PublishPreparationCompleted publishes preparation completed event
func (ep *EventPublisher) PublishPreparationCompleted(ctx context.Context, batchID, status string, processed, success, failed int64) error {
	if ep.batchPublisher == nil {
		return nil
	}

	return ep.batchPublisher.PublishSmsBatchStatusChanged(ctx, batchID, status, "preparation", 100.0, processed, success, failed)
}

// PublishDeliveryProgress publishes delivery progress update
func (ep *EventPublisher) PublishDeliveryProgress(ctx context.Context, batchID string, processed, success, failed int64, progress float64) error {
	if ep.batchPublisher == nil {
		return nil
	}

	return ep.batchPublisher.PublishSmsBatchStatusChanged(ctx, batchID, "delivery_running", "delivery", progress, processed, success, failed)
}

// PublishDeliveryCompleted publishes delivery completed event
func (ep *EventPublisher) PublishDeliveryCompleted(ctx context.Context, batchID, status string, processed, success, failed int64) error {
	if ep.batchPublisher == nil {
		return nil
	}

	return ep.batchPublisher.PublishSmsBatchStatusChanged(ctx, batchID, status, "delivery", 100.0, processed, success, failed)
}

// PublishBatchSuccess publishes batch success event
func (ep *EventPublisher) PublishBatchSuccess(ctx context.Context, batchID string, processed, success, failed int64) error {
	if ep.batchPublisher == nil {
		return nil
	}

	return ep.batchPublisher.PublishSmsBatchStatusChanged(ctx, batchID, "succeeded", "completed", 100.0, processed, success, failed)
}

// PublishBatchFailure publishes batch failure event
func (ep *EventPublisher) PublishBatchFailure(ctx context.Context, batchID string, processed, success, failed int64, errorMessage string) error {
	if ep.batchPublisher == nil {
		return nil
	}

	return ep.batchPublisher.PublishSmsBatchStatusChanged(ctx, batchID, "failed", "failed", 0.0, processed, success, failed)
}

// PublishBatchOperation publishes batch operation event
func (ep *EventPublisher) PublishBatchOperation(ctx context.Context, batchID, operation, phase, reason, operatorID string) error {
	if ep.batchPublisher == nil {
		return nil
	}

	switch operation {
	case "pause":
		return ep.batchPublisher.PublishSmsBatchPaused(ctx, batchID, phase, reason, operatorID)
	case "resume":
		return ep.batchPublisher.PublishSmsBatchResumed(ctx, batchID, phase, reason, operatorID)
	default:
		// For other operations, use the generic method
		req := &sender.BatchOperationRequest{
			RequestID:   fmt.Sprintf("%s-%s-%d", batchID, operation, time.Now().Unix()),
			BatchID:     batchID,
			Operation:   operation,
			Phase:       phase,
			Reason:      reason,
			OperatorID:  operatorID,
			RequestedAt: time.Now(),
		}
		return ep.batchPublisher.PublishBatchOperation(ctx, req)
	}
}

// PublishSmsBatchPaused publishes SMS batch paused event
func (ep *EventPublisher) PublishSmsBatchPaused(ctx context.Context, batchID, currentPhase, pauseReason, operatorID string) error {
	if ep.batchPublisher == nil {
		return nil
	}

	return ep.batchPublisher.PublishSmsBatchPaused(ctx, batchID, currentPhase, pauseReason, operatorID)
}
