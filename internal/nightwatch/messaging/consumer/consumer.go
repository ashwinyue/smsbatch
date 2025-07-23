package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/pkg/log"
	"github.com/ashwinyue/dcp/pkg/queue"
)

// MessageConsumer handles incoming messages
type MessageConsumer struct {
	store store.IStore
	ctx   context.Context
}

// NewMessageConsumer creates a new message consumer
func NewMessageConsumer(ctx context.Context, store store.IStore) *MessageConsumer {
	return &MessageConsumer{
		store: store,
		ctx:   ctx,
	}
}

// Consume processes incoming messages (implements queue.ConsumeHandler)
func (c *MessageConsumer) Consume(message any) error {
	// Process the message based on its type
	switch msg := message.(type) {
	case []byte:
		return c.processRawMessage(msg)
	case string:
		return c.processRawMessage([]byte(msg))
	default:
		log.Infow("Processing message", "type", fmt.Sprintf("%T", message))
		return nil
	}
}

// processRawMessage processes raw byte messages
func (c *MessageConsumer) processRawMessage(msgBytes []byte) error {
	log.Infow("Processing raw message", "size", len(msgBytes))
	
	// Try to parse as JSON to determine message type
	var msgData map[string]interface{}
	if err := json.Unmarshal(msgBytes, &msgData); err != nil {
		log.Warnw("Failed to parse message as JSON, treating as raw text", "error", err)
		return c.processTextMessage(string(msgBytes))
	}
	
	// Check message type from headers or content
	if msgType, ok := msgData["message_type"].(string); ok {
		switch msgType {
		case "batch_message":
			return c.processBatchMessage(msgData)
		case "batch_status_update":
			return c.processBatchStatusUpdate(msgData)
		case "template_sms":
			return c.processSMSMessage(msgData)
		case "uplink_sms":
			return c.processUplinkMessage(msgData)
		default:
			log.Warnw("Unknown message type", "type", msgType)
			return c.processGenericMessage(msgData)
		}
	}
	
	// If no message type specified, treat as generic message
	return c.processGenericMessage(msgData)
}

// processBatchMessage processes batch processing messages
func (c *MessageConsumer) processBatchMessage(msgData map[string]interface{}) error {
	log.Infow("Processing batch message", "data", msgData)
	
	// Extract batch information
	batchID, _ := msgData["batch_id"].(string)
	userID, _ := msgData["user_id"].(string)
	batchType, _ := msgData["batch_type"].(string)
	
	log.Infow("Batch message details", 
		"batch_id", batchID,
		"user_id", userID,
		"batch_type", batchType)
	
	// TODO: Implement actual batch processing logic
	// This could involve:
	// 1. Validating batch parameters
	// 2. Breaking down batch into individual tasks
	// 3. Scheduling tasks for execution
	// 4. Updating batch status
	
	return nil
}

// processBatchStatusUpdate processes batch status update messages
func (c *MessageConsumer) processBatchStatusUpdate(msgData map[string]interface{}) error {
	log.Infow("Processing batch status update", "data", msgData)
	
	// Extract status information
	batchID, _ := msgData["batch_id"].(string)
	status, _ := msgData["status"].(string)
	progress, _ := msgData["progress"].(float64)
	
	log.Infow("Batch status update details",
		"batch_id", batchID,
		"status", status,
		"progress", progress)
	
	// TODO: Implement status update logic
	// This could involve:
	// 1. Updating batch status in database
	// 2. Notifying interested parties
	// 3. Triggering next phase if needed
	
	return nil
}

// processSMSMessage processes SMS messages
func (c *MessageConsumer) processSMSMessage(msgData map[string]interface{}) error {
	log.Infow("Processing SMS message", "data", msgData)
	
	// Extract SMS information
	requestID, _ := msgData["request_id"].(string)
	phoneNumber, _ := msgData["phone_number"].(string)
	content, _ := msgData["content"].(string)
	
	log.Infow("SMS message details",
		"request_id", requestID,
		"phone_number", phoneNumber,
		"content_length", len(content))
	
	// TODO: Implement SMS processing logic
	// This could involve:
	// 1. Validating phone number and content
	// 2. Selecting appropriate SMS provider
	// 3. Sending SMS via provider API
	// 4. Handling delivery status
	
	return nil
}

// processUplinkMessage processes uplink SMS messages
func (c *MessageConsumer) processUplinkMessage(msgData map[string]interface{}) error {
	log.Infow("Processing uplink message", "data", msgData)
	
	// Extract uplink information
	requestID, _ := msgData["request_id"].(string)
	phoneNumber, _ := msgData["phone_number"].(string)
	content, _ := msgData["content"].(string)
	destCode, _ := msgData["dest_code"].(string)
	
	log.Infow("Uplink message details",
		"request_id", requestID,
		"phone_number", phoneNumber,
		"dest_code", destCode,
		"content_length", len(content))
	
	// TODO: Implement uplink processing logic
	// This could involve:
	// 1. Parsing uplink content
	// 2. Routing to appropriate handler
	// 3. Generating response if needed
	// 4. Logging for analytics
	
	return nil
}

// processTextMessage processes plain text messages
func (c *MessageConsumer) processTextMessage(content string) error {
	log.Infow("Processing text message", "content_length", len(content))
	
	// TODO: Implement text message processing
	// This is a fallback for messages that can't be parsed as JSON
	
	return nil
}

// processGenericMessage processes generic JSON messages
func (c *MessageConsumer) processGenericMessage(msgData map[string]interface{}) error {
	log.Infow("Processing generic message", "keys", getMapKeys(msgData))
	
	// TODO: Implement generic message processing
	// This handles JSON messages without a specific type
	
	return nil
}

// Helper function to get map keys for logging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ConsumerService manages message consumption
type ConsumerService struct {
	ctx      context.Context
	consumer *queue.KQueue
	handler  *MessageConsumer
	store    store.IStore
	
	// Metrics
	messagesProcessed int64
	errorCount        int64
	startTime         time.Time
}

// ConsumerConfig holds consumer configuration
type ConsumerConfig struct {
	KafkaConfig   *queue.KafkaConfig `json:"kafka_config"`
	ConsumerGroup string             `json:"consumer_group"`
	Topics        []string           `json:"topics"`
	Store         store.IStore       `json:"-"`
}

// NewConsumerService creates a new consumer service
func NewConsumerService(ctx context.Context, config *ConsumerConfig) (*ConsumerService, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.Store == nil {
		return nil, fmt.Errorf("store cannot be nil")
	}
	if config.KafkaConfig == nil {
		return nil, fmt.Errorf("kafka config cannot be nil")
	}
	
	// Create message handler
	handler := NewMessageConsumer(ctx, config.Store)
	
	// Create consumer
	consumer, err := queue.NewKQueue(config.KafkaConfig, handler)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}
	
	return &ConsumerService{
		ctx:       ctx,
		consumer:  consumer,
		handler:   handler,
		store:     config.Store,
		startTime: time.Now(),
	}, nil
}

// Start starts the consumer service
func (s *ConsumerService) Start() error {
	log.Infow("Starting consumer service")
	
	go s.consumer.Start()
	
	log.Infow("Consumer service started successfully")
	return nil
}

// Stop stops the consumer service
func (s *ConsumerService) Stop() error {
	log.Infow("Stopping consumer service")
	
	s.consumer.Stop()
	
	log.Infow("Consumer service stopped successfully")
	return nil
}

// IsHealthy checks if the consumer service is healthy
func (s *ConsumerService) IsHealthy() bool {
	return s.consumer != nil
}

// GetMetrics returns consumer service metrics
func (s *ConsumerService) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"service_status":     "running",
		"healthy":            s.IsHealthy(),
		"messages_processed": s.messagesProcessed,
		"error_count":        s.errorCount,
		"uptime":             time.Since(s.startTime),
		"timestamp":          time.Now(),
	}
}