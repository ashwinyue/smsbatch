package smsbatch

import (
	"context"

	"github.com/ashwinyue/dcp/internal/nightwatch/message"
	"github.com/ashwinyue/dcp/internal/nightwatch/mqs"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// InitializeEventPublisher initializes the event publisher for the watcher
func (w *Watcher) InitializeEventPublisher(kafkaSender *message.KafkaSender, topic string) {
	w.EventPublisher = mqs.NewBatchEventPublisher(kafkaSender, topic)
	log.Infow("Event publisher initialized for SMS batch watcher", "topic", topic)
}

// PublishBatchCreatedEvent publishes an event when a new batch is created
func (w *Watcher) PublishBatchCreatedEvent(ctx context.Context, batchID string) error {
	if w.EventPublisher == nil {
		log.Warnw("Event publisher not initialized", "batch_id", batchID)
		return nil
	}

	return w.EventPublisher.PublishBatchRequest(ctx, &mqs.BatchMessageRequest{
		RequestID:   batchID + "-created",
		BatchID:     batchID,
		BatchType:   "sms",
		MessageType: "notification",
		Recipients:  []string{}, // Will be populated from database
		Template:    "default",
		Params:      make(map[string]interface{}),
		Priority:    5,
		MaxRetries:  3,
		Timeout:     300, // 5 minutes
	})
}

// PublishBatchStartedEvent publishes an event when batch processing starts
func (w *Watcher) PublishBatchStartedEvent(ctx context.Context, batchID string) error {
	if w.EventPublisher == nil {
		return nil
	}

	return w.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
		RequestID:    batchID + "-started",
		BatchID:      batchID,
		Status:       SmsBatchPreparationReady,
		CurrentPhase: "preparation",
		Progress:     0.0,
		Processed:    0,
		Success:      0,
		Failed:       0,
	})
}

// PublishBatchRetryEvent publishes an event when a batch is retried
func (w *Watcher) PublishBatchRetryEvent(ctx context.Context, batchID, phase, reason string) error {
	if w.EventPublisher == nil {
		return nil
	}

	return w.EventPublisher.PublishBatchOperation(ctx, &mqs.BatchOperationRequest{
		RequestID:  batchID + "-retry",
		BatchID:    batchID,
		Operation:  "retry",
		Phase:      phase,
		Reason:     reason,
		OperatorID: "system",
	})
}

// GetEventPublisher returns the event publisher instance
func (w *Watcher) GetEventPublisher() *mqs.BatchEventPublisher {
	return w.EventPublisher
}

// IsEventPublisherEnabled checks if event publishing is enabled
func (w *Watcher) IsEventPublisherEnabled() bool {
	return w.EventPublisher != nil
}
