package messaging

import (
	"context"
	"errors"

	"github.com/ashwinyue/dcp/internal/nightwatch/messaging/consumer"
	"github.com/ashwinyue/dcp/internal/nightwatch/messaging/sender"
)

// Errors
var (
	ErrMessagingServiceNotConfigured = errors.New("messaging service not configured")
	ErrConsumerServiceNotConfigured  = errors.New("consumer service not configured")
)

// Re-export commonly used types from sender package
type SenderMessage = sender.Message
type SenderBatchMessage = sender.BatchMessage
type SenderBatchStatusUpdate = sender.BatchStatusUpdate
type SenderKafkaSender = sender.KafkaSender
type SenderKafkaSenderConfig = sender.KafkaSenderConfig

// Re-export functions from sender package
var NewKafkaSender = sender.NewKafkaSender

// Re-export types from consumer package
type ConsumerMessageConsumer = consumer.MessageConsumer
type ConsumerService = consumer.ConsumerService
type ConsumerConfig = consumer.ConsumerConfig

// Re-export functions from consumer package
var NewMessageConsumer = consumer.NewMessageConsumer
var NewConsumerService = consumer.NewConsumerService

// UnifiedService combines both messaging and consumer services
type UnifiedService struct {
	messagingService *MessagingService
	consumerService  *ConsumerService
}

// UnifiedConfig holds configuration for both messaging and consumer services
type UnifiedConfig struct {
	MessagingConfig *MessagingConfig `json:"messaging_config"`
	ConsumerConfig  *ConsumerConfig  `json:"consumer_config"`
}

// NewUnifiedService creates a new unified service with both messaging and consumer capabilities
func NewUnifiedService(ctx context.Context, config *UnifiedConfig) (*UnifiedService, error) {
	var messagingService *MessagingService
	var consumerService *ConsumerService
	var err error

	// Create messaging service if config provided
	if config.MessagingConfig != nil {
		messagingService, err = NewMessagingService(ctx, config.MessagingConfig)
		if err != nil {
			return nil, err
		}
	}

	// Create consumer service if config provided
	if config.ConsumerConfig != nil {
		consumerService, err = NewConsumerService(ctx, config.ConsumerConfig)
		if err != nil {
			return nil, err
		}
	}

	return &UnifiedService{
		messagingService: messagingService,
		consumerService:  consumerService,
	}, nil
}

// Start starts both messaging and consumer services
func (s *UnifiedService) Start() error {
	if s.messagingService != nil {
		if err := s.messagingService.Start(); err != nil {
			return err
		}
	}

	if s.consumerService != nil {
		if err := s.consumerService.Start(); err != nil {
			return err
		}
	}

	return nil
}

// Stop stops both messaging and consumer services
func (s *UnifiedService) Stop() error {
	if s.consumerService != nil {
		if err := s.consumerService.Stop(); err != nil {
			return err
		}
	}

	if s.messagingService != nil {
		if err := s.messagingService.Stop(); err != nil {
			return err
		}
	}

	return nil
}

// GetMessagingService returns the messaging service
func (s *UnifiedService) GetMessagingService() *MessagingService {
	return s.messagingService
}

// GetConsumerService returns the consumer service
func (s *UnifiedService) GetConsumerService() *ConsumerService {
	return s.consumerService
}

// IsHealthy checks if both services are healthy
func (s *UnifiedService) IsHealthy() bool {
	messagingHealthy := s.messagingService == nil || s.messagingService.IsHealthy()
	consumerHealthy := s.consumerService == nil || s.consumerService.IsHealthy()
	return messagingHealthy && consumerHealthy
}

// GetMetrics returns combined metrics from both services
func (s *UnifiedService) GetMetrics() map[string]interface{} {
	metrics := make(map[string]interface{})

	if s.messagingService != nil {
		metrics["messaging"] = s.messagingService.GetMetrics()
	}

	if s.consumerService != nil {
		metrics["consumer"] = s.consumerService.GetMetrics()
	}

	metrics["unified_service_healthy"] = s.IsHealthy()

	return metrics
}

// Convenience methods for messaging operations

// SendMessage sends a single message (requires messaging service)
func (s *UnifiedService) SendMessage(msg *sender.Message) error {
	if s.messagingService == nil {
		return ErrMessagingServiceNotConfigured
	}
	return s.messagingService.SendMessage(msg)
}

// SendMessages sends multiple messages (requires messaging service)
func (s *UnifiedService) SendMessages(messages []*sender.Message) error {
	if s.messagingService == nil {
		return ErrMessagingServiceNotConfigured
	}
	return s.messagingService.SendMessages(messages)
}

// SendBatchMessage sends a batch message (requires messaging service)
func (s *UnifiedService) SendBatchMessage(batchMsg *sender.BatchMessage) error {
	if s.messagingService == nil {
		return ErrMessagingServiceNotConfigured
	}
	return s.messagingService.SendBatchMessage(batchMsg)
}

// SendBatchStatusUpdate sends a batch status update (requires messaging service)
func (s *UnifiedService) SendBatchStatusUpdate(statusUpdate *sender.BatchStatusUpdate) error {
	if s.messagingService == nil {
		return ErrMessagingServiceNotConfigured
	}
	return s.messagingService.SendBatchStatusUpdate(statusUpdate)
}

// SendSMSMessage sends an SMS message (requires messaging service)
func (s *UnifiedService) SendSMSMessage(requestID, phoneNumber, content, templateCode string, providers []string) error {
	if s.messagingService == nil {
		return ErrMessagingServiceNotConfigured
	}
	return s.messagingService.SendSMSMessage(requestID, phoneNumber, content, templateCode, providers)
}

// SendUplinkMessage sends an uplink message (requires messaging service)
func (s *UnifiedService) SendUplinkMessage(requestID, phoneNumber, content, destCode string) error {
	if s.messagingService == nil {
		return ErrMessagingServiceNotConfigured
	}
	return s.messagingService.SendUplinkMessage(requestID, phoneNumber, content, destCode)
}