package messaging

import (
	"context"
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/messaging/sender"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// MessagingService provides message sending functionality
type MessagingService struct {
	ctx    context.Context
	sender *sender.KafkaSender

	// Metrics
	messagesProcessed int64
	errorCount        int64
	startTime         time.Time
}

// MessagingConfig holds configuration for the messaging service
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

	return &MessagingService{
		ctx:       ctx,
		sender:    kafkaSender,
		startTime: time.Now(),
	}, nil
}

// Start starts the messaging service
func (s *MessagingService) Start() error {
	log.Infow("Starting messaging service")
	log.Infow("Messaging service started successfully")
	return nil
}

// Stop stops the messaging service
func (s *MessagingService) Stop() error {
	log.Infow("Stopping messaging service")

	if s.sender != nil {
		s.sender.Close()
	}

	log.Infow("Messaging service stopped successfully")
	return nil
}

// SendMessage sends a single message
func (s *MessagingService) SendMessage(msg *sender.Message) error {
	return s.sender.SendMessage(msg)
}

// SendMessages sends multiple messages
func (s *MessagingService) SendMessages(messages []*sender.Message) error {
	return s.sender.SendMessages(messages)
}

// SendBatchMessage sends a batch processing message
func (s *MessagingService) SendBatchMessage(batchMsg *sender.BatchMessage) error {
	msg := &sender.Message{
		Key:   batchMsg.BatchID,
		Value: batchMsg,
		Headers: map[string]string{
			"message_type": "batch_message",
			"batch_type":   batchMsg.BatchType,
		},
	}
	return s.SendMessage(msg)
}

// SendBatchStatusUpdate sends a batch status update
func (s *MessagingService) SendBatchStatusUpdate(statusUpdate *sender.BatchStatusUpdate) error {
	msg := &sender.Message{
		Key:   statusUpdate.BatchID,
		Value: statusUpdate,
		Headers: map[string]string{
			"message_type": "batch_status_update",
			"batch_id":     statusUpdate.BatchID,
		},
	}
	return s.SendMessage(msg)
}

// SendSMSMessage sends an SMS message
func (s *MessagingService) SendSMSMessage(requestID, phoneNumber, content, templateCode string, providers []string) error {
	return s.sender.SendSMSMessage(requestID, phoneNumber, content, templateCode, providers)
}

// SendUplinkMessage sends an uplink message
func (s *MessagingService) SendUplinkMessage(requestID, phoneNumber, content, destCode string) error {
	return s.sender.SendUplinkMessage(requestID, phoneNumber, content, destCode)
}

// SendJSONMessage sends a JSON message
func (s *MessagingService) SendJSONMessage(key string, data interface{}, headers map[string]string) error {
	return s.sender.SendJSONMessage(key, data, headers)
}

// SendTextMessage sends a text message
func (s *MessagingService) SendTextMessage(key, text string, headers map[string]string) error {
	return s.sender.SendTextMessage(key, text, headers)
}

// IsHealthy checks if the service is healthy
func (s *MessagingService) IsHealthy() bool {
	return s.sender != nil
}

// GetMetrics returns service metrics
func (s *MessagingService) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"service_status":     "running",
		"healthy":            s.IsHealthy(),
		"messages_processed": s.messagesProcessed,
		"error_count":        s.errorCount,
		"uptime":             time.Since(s.startTime),
		"timestamp":          time.Now(),
	}
}

// GetSenderStats returns Kafka sender statistics
func (s *MessagingService) GetSenderStats() interface{} {
	if s.sender != nil {
		return s.sender.GetStats()
	}
	return nil
}