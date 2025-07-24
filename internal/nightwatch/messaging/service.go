package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/messaging/sender"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// MessagingService provides unified messaging capabilities
type MessagingService struct {
	ctx    context.Context
	cancel context.CancelFunc
	config *MessagingConfig
	sender *sender.KafkaSender
	publisher *sender.BatchEventPublisher
	mu     sync.RWMutex
	started bool
	metrics map[string]interface{}
}

// MessagingConfig holds configuration for messaging service
type MessagingConfig struct {
	KafkaConfig *sender.KafkaSenderConfig `json:"kafka_config"`
	Topic       string                    `json:"topic"`
}

// NewMessagingService creates a new messaging service
func NewMessagingService(ctx context.Context, config *MessagingConfig) (*MessagingService, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.KafkaConfig == nil {
		return nil, fmt.Errorf("kafka config cannot be nil")
	}

	// Create Kafka sender
	kafkaSender, err := sender.NewKafkaSender(config.KafkaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka sender: %w", err)
	}

	// Create batch event publisher
	batchPublisher := sender.NewBatchEventPublisher(kafkaSender)

	// Create service context
	serviceCtx, cancel := context.WithCancel(ctx)

	return &MessagingService{
		ctx:       serviceCtx,
		cancel:    cancel,
		config:    config,
		sender:    kafkaSender,
		publisher: batchPublisher,
		metrics:   make(map[string]interface{}),
	}, nil
}

// Start starts the messaging service
func (s *MessagingService) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("messaging service already started")
	}

	log.Infow("Starting messaging service", "topic", s.config.Topic)
	s.started = true
	s.updateMetrics("started_at", time.Now())

	return nil
}

// Stop stops the messaging service
func (s *MessagingService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return fmt.Errorf("messaging service not started")
	}

	log.Infow("Stopping messaging service")
	s.cancel()
	s.started = false
	s.updateMetrics("stopped_at", time.Now())

	// Close Kafka sender
	if s.sender != nil {
		if err := s.sender.Close(); err != nil {
			log.Warnw("Failed to close kafka sender", "error", err)
		}
	}

	return nil
}

// IsHealthy checks if the messaging service is healthy
func (s *MessagingService) IsHealthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.started && s.ctx.Err() == nil
}

// GetMetrics returns service metrics
func (s *MessagingService) GetMetrics() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics := make(map[string]interface{})
	for k, v := range s.metrics {
		metrics[k] = v
	}
	metrics["healthy"] = s.IsHealthy()
	return metrics
}

// SendMessage sends a single message
func (s *MessagingService) SendMessage(msg *sender.Message) error {
	if !s.IsHealthy() {
		return fmt.Errorf("messaging service not healthy")
	}

	err := s.sender.SendMessage(msg)
	if err != nil {
		s.updateMetrics("send_errors", s.getMetricValue("send_errors")+1)
		return err
	}

	s.updateMetrics("messages_sent", s.getMetricValue("messages_sent")+1)
	return nil
}

// SendMessages sends multiple messages
func (s *MessagingService) SendMessages(messages []*sender.Message) error {
	if !s.IsHealthy() {
		return fmt.Errorf("messaging service not healthy")
	}

	err := s.sender.SendMessages(messages)
	if err != nil {
		s.updateMetrics("send_errors", s.getMetricValue("send_errors")+1)
		return err
	}

	s.updateMetrics("messages_sent", s.getMetricValue("messages_sent")+int64(len(messages)))
	return nil
}

// SendBatchMessage sends a batch message
func (s *MessagingService) SendBatchMessage(batchMsg *sender.BatchMessage) error {
	if !s.IsHealthy() {
		return fmt.Errorf("messaging service not healthy")
	}

	msg := &sender.Message{
		Key:   batchMsg.BatchID,
		Value: batchMsg,
		Headers: map[string]string{
			"message_type": "batch_message",
			"batch_id":     batchMsg.BatchID,
			"batch_type":   batchMsg.BatchType,
		},
	}

	return s.SendMessage(msg)
}

// SendBatchStatusUpdate sends a batch status update
func (s *MessagingService) SendBatchStatusUpdate(statusUpdate *sender.BatchStatusUpdate) error {
	if !s.IsHealthy() {
		return fmt.Errorf("messaging service not healthy")
	}

	msg := &sender.Message{
		Key:   statusUpdate.BatchID,
		Value: statusUpdate,
		Headers: map[string]string{
			"message_type": "batch_status_update",
			"batch_id":     statusUpdate.BatchID,
			"status":       statusUpdate.Status,
		},
	}

	return s.SendMessage(msg)
}

// SendSMSMessage sends an SMS message
func (s *MessagingService) SendSMSMessage(requestID, phoneNumber, content, templateCode string, providers []string) error {
	if !s.IsHealthy() {
		return fmt.Errorf("messaging service not healthy")
	}

	smsData := map[string]interface{}{
		"request_id":    requestID,
		"phone_number":  phoneNumber,
		"content":       content,
		"template_code": templateCode,
		"providers":     providers,
		"timestamp":     time.Now(),
	}

	msg := &sender.Message{
		Key:   requestID,
		Value: smsData,
		Headers: map[string]string{
			"message_type":  "template_sms",
			"request_id":    requestID,
			"phone_number":  phoneNumber,
			"template_code": templateCode,
		},
	}

	return s.SendMessage(msg)
}

// SendUplinkMessage sends an uplink message
func (s *MessagingService) SendUplinkMessage(requestID, phoneNumber, content, destCode string) error {
	if !s.IsHealthy() {
		return fmt.Errorf("messaging service not healthy")
	}

	uplinkData := map[string]interface{}{
		"request_id":   requestID,
		"phone_number": phoneNumber,
		"content":      content,
		"dest_code":    destCode,
		"timestamp":    time.Now(),
	}

	msg := &sender.Message{
		Key:   requestID,
		Value: uplinkData,
		Headers: map[string]string{
			"message_type":  "uplink_sms",
			"request_id":    requestID,
			"phone_number":  phoneNumber,
			"dest_code":     destCode,
		},
	}

	return s.SendMessage(msg)
}

// GetBatchPublisher returns the batch event publisher
func (s *MessagingService) GetBatchPublisher() *sender.BatchEventPublisher {
	return s.publisher
}

// updateMetrics updates service metrics (thread-safe)
func (s *MessagingService) updateMetrics(key string, value interface{}) {
	s.metrics[key] = value
}

// getMetricValue gets a metric value as int64
func (s *MessagingService) getMetricValue(key string) int64 {
	if val, ok := s.metrics[key]; ok {
		if intVal, ok := val.(int64); ok {
			return intVal
		}
	}
	return 0
}