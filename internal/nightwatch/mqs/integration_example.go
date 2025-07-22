package mqs

import (
	"context"
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/message"
	"github.com/ashwinyue/dcp/internal/nightwatch/queue"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// BatchEventIntegration demonstrates how to integrate batch events with existing SMS batch system
type BatchEventIntegration struct {
	batchService *BatchEventService
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewBatchEventIntegration creates a new batch event integration instance
func NewBatchEventIntegration() (*BatchEventIntegration, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Configure Kafka for batch events
	kafkaConfig := &message.KafkaSenderConfig{
		Brokers:      []string{"localhost:9092"},
		Topic:        "sms-batch-events",
		Compression:  "gzip",
		BatchSize:    100,
		BatchTimeout: time.Second,
		MaxAttempts:  3,
		Async:        false,
	}

	// Configure Kafka consumer for batch events
	consumerConfig := &queue.KafkaConfig{
		Brokers:       []string{"localhost:9092"},
		Topic:         "sms-batch-events",
		GroupID:       "sms-batch-event-processor",
		Processors:    8,
		Consumers:     4,
		ForceCommit:   true,
		QueueCapacity: 200,
		MinBytes:      1024,
		MaxBytes:      1024 * 1024, // 1MB
	}

	// Create batch event service configuration
	serviceConfig := &BatchEventServiceConfig{
		KafkaConfig:    kafkaConfig,
		ConsumerConfig: consumerConfig,
		Topic:          "sms-batch-events",
		ConsumerGroup:  "sms-batch-event-processor",
	}

	// Create batch event service
	batchService, err := NewBatchEventService(ctx, serviceConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create batch event service: %w", err)
	}

	return &BatchEventIntegration{
		batchService: batchService,
		ctx:          ctx,
		cancel:       cancel,
	}, nil
}

// Start starts the batch event integration
func (i *BatchEventIntegration) Start() error {
	log.Infow("Starting batch event integration")

	err := i.batchService.Start()
	if err != nil {
		return fmt.Errorf("failed to start batch service: %w", err)
	}

	log.Infow("Batch event integration started successfully")
	return nil
}

// Stop stops the batch event integration
func (i *BatchEventIntegration) Stop() error {
	log.Infow("Stopping batch event integration")

	i.cancel()
	err := i.batchService.Stop()
	if err != nil {
		log.Errorw("Failed to stop batch service", "error", err)
	}

	log.Infow("Batch event integration stopped")
	return nil
}

// GetBatchService returns the batch event service
func (i *BatchEventIntegration) GetBatchService() *BatchEventService {
	return i.batchService
}

// Example integration methods that can be called from existing SMS batch system

// OnSmsBatchCreated should be called when a new SMS batch is created
func (i *BatchEventIntegration) OnSmsBatchCreated(batchID, userID string, recipients []string, template string, params map[string]interface{}) error {
	log.Infow("SMS batch created event", "batch_id", batchID, "recipients_count", len(recipients))

	err := i.batchService.PublishBatchCreated(batchID, userID, "sms", recipients, template, params)
	if err != nil {
		log.Errorw("Failed to publish batch created event", "error", err, "batch_id", batchID)
		return err
	}

	return nil
}

// OnSmsBatchStatusChanged should be called when SMS batch status changes
func (i *BatchEventIntegration) OnSmsBatchStatusChanged(batchID, status, phase string, progress float64, processed, success, failed int64, errorMessage string) error {
	log.Infow("SMS batch status changed",
		"batch_id", batchID,
		"status", status,
		"phase", phase,
		"progress", progress,
		"processed", processed,
		"success", success,
		"failed", failed)

	err := i.batchService.PublishBatchStatusChanged(batchID, status, phase, progress, processed, success, failed, errorMessage)
	if err != nil {
		log.Errorw("Failed to publish batch status changed event", "error", err, "batch_id", batchID)
		return err
	}

	return nil
}

// OnSmsBatchPaused should be called when SMS batch is paused
func (i *BatchEventIntegration) OnSmsBatchPaused(batchID, phase, reason, operatorID string) error {
	log.Infow("SMS batch paused", "batch_id", batchID, "phase", phase, "reason", reason)

	err := i.batchService.PublishSmsBatchPaused(batchID, phase, reason, operatorID)
	if err != nil {
		log.Errorw("Failed to publish batch paused event", "error", err, "batch_id", batchID)
		return err
	}

	return nil
}

// OnSmsBatchResumed should be called when SMS batch is resumed
func (i *BatchEventIntegration) OnSmsBatchResumed(batchID, phase, reason, operatorID string) error {
	log.Infow("SMS batch resumed", "batch_id", batchID, "phase", phase, "reason", reason)

	err := i.batchService.PublishSmsBatchResumed(batchID, phase, reason, operatorID)
	if err != nil {
		log.Errorw("Failed to publish batch resumed event", "error", err, "batch_id", batchID)
		return err
	}

	return nil
}

// OnSmsBatchAborted should be called when SMS batch is aborted
func (i *BatchEventIntegration) OnSmsBatchAborted(batchID, reason, operatorID string) error {
	log.Infow("SMS batch aborted", "batch_id", batchID, "reason", reason)

	err := i.batchService.PublishSmsBatchAborted(batchID, reason, operatorID)
	if err != nil {
		log.Errorw("Failed to publish batch aborted event", "error", err, "batch_id", batchID)
		return err
	}

	return nil
}

// OnSmsBatchRetried should be called when SMS batch is retried
func (i *BatchEventIntegration) OnSmsBatchRetried(batchID, phase, reason, operatorID string) error {
	log.Infow("SMS batch retried", "batch_id", batchID, "phase", phase, "reason", reason)

	err := i.batchService.PublishSmsBatchRetried(batchID, phase, reason, operatorID)
	if err != nil {
		log.Errorw("Failed to publish batch retried event", "error", err, "batch_id", batchID)
		return err
	}

	return nil
}

// Example usage function
func ExampleBatchEventIntegration() error {
	// Create integration instance
	integration, err := NewBatchEventIntegration()
	if err != nil {
		return fmt.Errorf("failed to create integration: %w", err)
	}

	// Start the integration
	err = integration.Start()
	if err != nil {
		return fmt.Errorf("failed to start integration: %w", err)
	}

	// Simulate SMS batch events
	batchID := "batch-12345"
	userID := "user-67890"
	recipients := []string{"+1234567890", "+0987654321"}
	template := "Hello {{name}}, your verification code is {{code}}"
	params := map[string]interface{}{
		"name": "John",
		"code": "123456",
	}

	// Publish batch created event
	err = integration.OnSmsBatchCreated(batchID, userID, recipients, template, params)
	if err != nil {
		return fmt.Errorf("failed to publish batch created: %w", err)
	}

	// Simulate status changes
	time.Sleep(1 * time.Second)
	err = integration.OnSmsBatchStatusChanged(batchID, "preparation_running", "preparation", 0.5, 1, 0, 0, "")
	if err != nil {
		return fmt.Errorf("failed to publish status change: %w", err)
	}

	time.Sleep(1 * time.Second)
	err = integration.OnSmsBatchStatusChanged(batchID, "delivery_running", "delivery", 0.8, 2, 1, 1, "")
	if err != nil {
		return fmt.Errorf("failed to publish status change: %w", err)
	}

	time.Sleep(1 * time.Second)
	err = integration.OnSmsBatchStatusChanged(batchID, "completed", "delivery", 1.0, 2, 2, 0, "")
	if err != nil {
		return fmt.Errorf("failed to publish status change: %w", err)
	}

	// Wait a bit before stopping
	time.Sleep(2 * time.Second)

	// Stop the integration
	err = integration.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop integration: %w", err)
	}

	log.Infow("Batch event integration example completed successfully")
	return nil
}
