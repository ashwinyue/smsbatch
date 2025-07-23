package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/pkg/log"
	"github.com/ashwinyue/dcp/pkg/queue"
	"github.com/segmentio/kafka-go"
)

// UnifiedMessagingService combines message sending and consuming functionality
type UnifiedMessagingService struct {
	ctx      context.Context
	writer   *kafka.Writer
	consumer *queue.KQueue
	store    store.IStore

	// Metrics
	messagesProcessed int64
	errorCount        int64
	startTime         time.Time
}

// Config holds configuration for the unified messaging service
type Config struct {
	KafkaConfig    *KafkaConfig       `json:"kafka_config"`
	ConsumerConfig *queue.KafkaConfig `json:"consumer_config"`
	Topic          string             `json:"topic"`
	ConsumerGroup  string             `json:"consumer_group"`
	Store          store.IStore       `json:"-"`
}

// KafkaConfig holds Kafka producer configuration
type KafkaConfig struct {
	Brokers      []string      `json:"brokers"`
	Topic        string        `json:"topic"`
	Compression  string        `json:"compression,omitempty"`
	BatchSize    int           `json:"batch_size,omitempty"`
	BatchTimeout time.Duration `json:"batch_timeout,omitempty"`
	MaxAttempts  int           `json:"max_attempts,omitempty"`
	Async        bool          `json:"async,omitempty"`
}

// Message represents a unified message structure
type Message struct {
	Key       string                 `json:"key,omitempty"`
	Value     interface{}            `json:"value"`
	Headers   map[string]string      `json:"headers,omitempty"`
	Partition int                    `json:"partition,omitempty"`
	Timestamp time.Time              `json:"timestamp,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// BatchMessage represents batch processing messages
type BatchMessage struct {
	BatchID     string                 `json:"batch_id"`
	UserID      string                 `json:"user_id"`
	BatchType   string                 `json:"batch_type"`
	MessageType string                 `json:"message_type"`
	Recipients  []string               `json:"recipients"`
	Template    string                 `json:"template"`
	Params      map[string]interface{} `json:"params"`
	Priority    int                    `json:"priority"`
	MaxRetries  int                    `json:"max_retries"`
	Timeout     int                    `json:"timeout"`
	CreatedAt   time.Time              `json:"created_at"`
}

// BatchStatusUpdate represents batch status change events
type BatchStatusUpdate struct {
	BatchID      string    `json:"batch_id"`
	Status       string    `json:"status"`
	CurrentPhase string    `json:"current_phase"`
	Progress     float64   `json:"progress"`
	Processed    int64     `json:"processed"`
	Success      int64     `json:"success"`
	Failed       int64     `json:"failed"`
	ErrorMessage string    `json:"error_message,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NewUnifiedMessagingService creates a new unified messaging service
func NewUnifiedMessagingService(ctx context.Context, config *Config) (*UnifiedMessagingService, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.Store == nil {
		return nil, fmt.Errorf("store cannot be nil")
	}

	// Create Kafka writer
	writer, err := createKafkaWriter(config.KafkaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka writer: %w", err)
	}

	// Create consumer (if needed)
	var consumer *queue.KQueue
	if config.ConsumerConfig != nil {
		consumerHandler := &MessageConsumer{store: config.Store}
		consumer, err = queue.NewKQueue(config.ConsumerConfig, consumerHandler)
		if err != nil {
			return nil, fmt.Errorf("failed to create consumer: %w", err)
		}
	}

	return &UnifiedMessagingService{
		ctx:       ctx,
		writer:    writer,
		consumer:  consumer,
		store:     config.Store,
		startTime: time.Now(),
	}, nil
}

// Start starts the messaging service
func (s *UnifiedMessagingService) Start() error {
	log.Infow("Starting unified messaging service")

	if s.consumer != nil {
		go s.consumer.Start()
	}

	log.Infow("Unified messaging service started successfully")
	return nil
}

// Stop stops the messaging service
func (s *UnifiedMessagingService) Stop() error {
	log.Infow("Stopping unified messaging service")

	if s.consumer != nil {
		s.consumer.Stop()
	}

	if s.writer != nil {
		s.writer.Close()
	}

	log.Infow("Unified messaging service stopped successfully")
	return nil
}

// SendMessage sends a single message
func (s *UnifiedMessagingService) SendMessage(msg *Message) error {
	return s.SendMessages([]*Message{msg})
}

// SendMessages sends multiple messages
func (s *UnifiedMessagingService) SendMessages(messages []*Message) error {
	if len(messages) == 0 {
		return fmt.Errorf("messages cannot be empty")
	}

	kafkaMessages := make([]kafka.Message, 0, len(messages))

	for _, msg := range messages {
		valueBytes, err := s.serializeValue(msg.Value)
		if err != nil {
			return fmt.Errorf("failed to serialize message value: %w", err)
		}

		kafkaMsg := kafka.Message{
			Key:   []byte(msg.Key),
			Value: valueBytes,
			Time:  msg.Timestamp,
		}

		if msg.Timestamp.IsZero() {
			kafkaMsg.Time = time.Now()
		}

		if msg.Partition > 0 {
			kafkaMsg.Partition = msg.Partition
		}

		if len(msg.Headers) > 0 {
			headers := make([]kafka.Header, 0, len(msg.Headers))
			for k, v := range msg.Headers {
				headers = append(headers, kafka.Header{
					Key:   k,
					Value: []byte(v),
				})
			}
			kafkaMsg.Headers = headers
		}

		kafkaMessages = append(kafkaMessages, kafkaMsg)
	}

	err := s.writer.WriteMessages(s.ctx, kafkaMessages...)
	if err != nil {
		log.Errorw("Failed to send kafka messages", "error", err, "count", len(kafkaMessages))
		return fmt.Errorf("failed to send kafka messages: %w", err)
	}

	log.Infow("Successfully sent kafka messages", "count", len(kafkaMessages))
	return nil
}

// SendBatchMessage sends a batch processing message
func (s *UnifiedMessagingService) SendBatchMessage(batchMsg *BatchMessage) error {
	msg := &Message{
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
func (s *UnifiedMessagingService) SendBatchStatusUpdate(statusUpdate *BatchStatusUpdate) error {
	msg := &Message{
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
func (s *UnifiedMessagingService) SendSMSMessage(requestID, phoneNumber, content, templateCode string, providers []string) error {
	smsMsg := map[string]interface{}{
		"request_id":    requestID,
		"phone_number":  phoneNumber,
		"content":       content,
		"template_code": templateCode,
		"providers":     providers,
		"send_time":     time.Now(),
		"message_type":  "template_sms",
	}

	msg := &Message{
		Key:   requestID,
		Value: smsMsg,
		Headers: map[string]string{
			"message_type": "template_sms",
			"phone_number": phoneNumber,
		},
	}

	return s.SendMessage(msg)
}

// IsHealthy checks if the service is healthy
func (s *UnifiedMessagingService) IsHealthy() bool {
	return s.writer != nil
}

// GetMetrics returns service metrics
func (s *UnifiedMessagingService) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"service_status":     "running",
		"healthy":            s.IsHealthy(),
		"messages_processed": s.messagesProcessed,
		"error_count":        s.errorCount,
		"uptime":             time.Since(s.startTime),
		"timestamp":          time.Now(),
	}
}

// Helper functions

func createKafkaWriter(config *KafkaConfig) (*kafka.Writer, error) {
	if len(config.Brokers) == 0 {
		return nil, fmt.Errorf("brokers cannot be empty")
	}
	if config.Topic == "" {
		return nil, fmt.Errorf("topic cannot be empty")
	}

	// Set defaults
	if config.BatchSize == 0 {
		config.BatchSize = 100
	}
	if config.BatchTimeout == 0 {
		config.BatchTimeout = time.Second
	}
	if config.MaxAttempts == 0 {
		config.MaxAttempts = 3
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(config.Brokers...),
		Topic:        config.Topic,
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    config.BatchSize,
		BatchTimeout: config.BatchTimeout,
		MaxAttempts:  config.MaxAttempts,
		Async:        config.Async,
	}

	// Set compression
	switch config.Compression {
	case "gzip":
		writer.Compression = kafka.Gzip
	case "snappy":
		writer.Compression = kafka.Snappy
	case "lz4":
		writer.Compression = kafka.Lz4
	case "zstd":
		writer.Compression = kafka.Zstd
	}

	return writer, nil
}

func (s *UnifiedMessagingService) serializeValue(value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	default:
		return json.Marshal(v)
	}
}

// MessageConsumer handles incoming messages
type MessageConsumer struct {
	store store.IStore
}

// Consume processes incoming messages (implements queue.ConsumeHandler)
func (c *MessageConsumer) Consume(message any) error {
	// Process the message based on its type
	// This is a simplified implementation
	if msgBytes, ok := message.([]byte); ok {
		log.Infow("Processing message", "size", len(msgBytes))
	} else {
		log.Infow("Processing message", "type", fmt.Sprintf("%T", message))
	}
	return nil
}
