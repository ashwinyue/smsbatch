package mqs

import (
	"context"
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/message"
	"github.com/ashwinyue/dcp/internal/nightwatch/queue"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// BatchEventService manages batch event processing
type BatchEventService struct {
	ctx       context.Context
	consumer  *BatchMessageConsumer
	publisher *BatchEventPublisher
	kqueue    *queue.KQueue
	// 指标统计
	messagesProcessed int64
	errorCount        int64
	startTime         time.Time
}

// BatchEventServiceConfig configuration for batch event service
type BatchEventServiceConfig struct {
	KafkaConfig    *message.KafkaSenderConfig `json:"kafka_config"`
	ConsumerConfig *queue.KafkaConfig         `json:"consumer_config"`
	Topic          string                     `json:"topic"`
	ConsumerGroup  string                     `json:"consumer_group"`
}

// NewBatchEventService creates a new batch event service
func NewBatchEventService(ctx context.Context, config *BatchEventServiceConfig) (*BatchEventService, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Create Kafka sender for publishing
	kafkaSender, err := message.NewKafkaSender(config.KafkaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka sender: %w", err)
	}

	// Create publisher
	publisher := NewBatchEventPublisher(kafkaSender, config.Topic)

	// Create consumer
	consumer := NewBatchMessageConsumer(ctx)

	// Create KQueue for consuming messages
	kqueue, err := queue.NewKQueue(config.ConsumerConfig, consumer)
	if err != nil {
		return nil, fmt.Errorf("failed to create kqueue: %w", err)
	}

	return &BatchEventService{
		ctx:       ctx,
		consumer:  consumer,
		publisher: publisher,
		kqueue:    kqueue,
		startTime: time.Now(),
	}, nil
}

// Start starts the batch event service
func (s *BatchEventService) Start() error {
	log.Infow("Starting batch event service")

	// Start consuming messages
	go s.kqueue.Start()

	log.Infow("Batch event service started successfully")
	return nil
}

// Stop stops the batch event service
func (s *BatchEventService) Stop() error {
	log.Infow("Stopping batch event service")

	s.kqueue.Stop()

	log.Infow("Batch event service stopped successfully")
	return nil
}

// GetPublisher returns the batch event publisher
func (s *BatchEventService) GetPublisher() *BatchEventPublisher {
	return s.publisher
}

// GetConsumer returns the batch message consumer
func (s *BatchEventService) GetConsumer() *BatchMessageConsumer {
	return s.consumer
}

// PublishBatchCreated publishes a batch creation event
func (s *BatchEventService) PublishBatchCreated(batchID, userID string, batchType string, recipients []string, template string, params map[string]interface{}) error {
	req := &BatchMessageRequest{
		BatchID:     batchID,
		UserID:      userID,
		BatchType:   batchType,
		MessageType: "template",
		Recipients:  recipients,
		Template:    template,
		Params:      params,
		Priority:    5,
		MaxRetries:  3,
		Timeout:     300,
		CreatedAt:   time.Now(),
	}

	return s.publisher.PublishBatchRequest(s.ctx, req)
}

// PublishBatchStatusChanged publishes a batch status change event
func (s *BatchEventService) PublishBatchStatusChanged(batchID, status, phase string, progress float64, processed, success, failed int64, errorMessage string) error {
	req := &BatchStatusUpdate{
		BatchID:      batchID,
		Status:       status,
		CurrentPhase: phase,
		Progress:     progress,
		Processed:    processed,
		Success:      success,
		Failed:       failed,
		ErrorMessage: errorMessage,
		UpdatedAt:    time.Now(),
	}

	return s.publisher.PublishBatchStatusUpdate(s.ctx, req)
}

// PublishBatchOperation publishes a batch operation command
func (s *BatchEventService) PublishBatchOperation(batchID, operation, phase, reason, operatorID string, metadata map[string]interface{}) error {
	req := &BatchOperationRequest{
		BatchID:     batchID,
		Operation:   operation,
		Phase:       phase,
		Reason:      reason,
		OperatorID:  operatorID,
		Metadata:    metadata,
		RequestedAt: time.Now(),
	}

	return s.publisher.PublishBatchOperation(s.ctx, req)
}

// PublishSmsBatchPaused publishes an SMS batch pause event
func (s *BatchEventService) PublishSmsBatchPaused(batchID, phase, reason, operatorID string) error {
	return s.PublishBatchOperation(batchID, "pause", phase, reason, operatorID, nil)
}

// PublishSmsBatchResumed publishes an SMS batch resume event
func (s *BatchEventService) PublishSmsBatchResumed(batchID, phase, reason, operatorID string) error {
	return s.PublishBatchOperation(batchID, "resume", phase, reason, operatorID, nil)
}

// PublishSmsBatchAborted publishes an SMS batch abort event
func (s *BatchEventService) PublishSmsBatchAborted(batchID, reason, operatorID string) error {
	return s.PublishBatchOperation(batchID, "abort", "", reason, operatorID, nil)
}

// PublishSmsBatchRetried publishes an SMS batch retry event
func (s *BatchEventService) PublishSmsBatchRetried(batchID, phase, reason, operatorID string) error {
	return s.PublishBatchOperation(batchID, "retry", phase, reason, operatorID, nil)
}

// Health check methods

// IsHealthy checks if the batch event service is healthy
func (s *BatchEventService) IsHealthy() bool {
	// Check if KQueue is running
	if s.kqueue == nil {
		return false
	}

	// Additional health checks can be added here
	// For example, check Kafka connectivity, database connectivity, etc.

	return true
}

// GetMetrics returns service metrics
func (s *BatchEventService) GetMetrics() map[string]interface{} {
	metrics := make(map[string]interface{})

	// Add basic metrics
	metrics["service_status"] = "running"
	metrics["healthy"] = s.IsHealthy()
	metrics["timestamp"] = time.Now()

	// 添加详细指标
	metrics["messages_processed"] = s.messagesProcessed
	metrics["error_count"] = s.errorCount
	metrics["uptime_seconds"] = time.Since(s.startTime).Seconds()

	// 计算处理速率
	uptime := time.Since(s.startTime).Seconds()
	if uptime > 0 {
		metrics["messages_per_second"] = float64(s.messagesProcessed) / uptime
		metrics["error_rate"] = float64(s.errorCount) / float64(s.messagesProcessed)
	} else {
		metrics["messages_per_second"] = 0.0
		metrics["error_rate"] = 0.0
	}

	// 队列深度（如果KQueue支持的话）
	if s.kqueue != nil {
		// 注意：这里假设KQueue有GetQueueDepth方法，实际需要根据queue包的实现调整
		metrics["queue_depth"] = "N/A" // 需要KQueue支持才能获取
	}

	return metrics
}
