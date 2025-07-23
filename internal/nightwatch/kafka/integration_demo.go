package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/mqs"
	"github.com/ashwinyue/dcp/internal/nightwatch/provider"
	"github.com/ashwinyue/dcp/internal/nightwatch/queue"
	"github.com/ashwinyue/dcp/internal/nightwatch/types"
	"github.com/ashwinyue/dcp/internal/pkg/log"
	"github.com/segmentio/kafka-go"
)

// Mock implementations for demo purposes
type mockDataStore struct{}
type mockIdempotentChecker struct{}
type mockHistoryLogger struct{}

// Implement DataStore interface
// (empty implementation for demo)

// Implement IdempotentChecker interface
func (m *mockIdempotentChecker) Check(ctx context.Context, key string) (bool, error) {
	return true, nil // Always return true for demo
}

func (m *mockIdempotentChecker) Mark(ctx context.Context, key string) error {
	return nil // No-op for demo
}

// Implement HistoryLogger interface
func (m *mockHistoryLogger) Log(ctx context.Context, entry interface{}) error {
	return nil // No-op for demo
}

func (m *mockHistoryLogger) Query(ctx context.Context, filter interface{}) ([]interface{}, error) {
	return nil, nil // Return empty for demo
}

func (m *mockHistoryLogger) WriteHistory(ctx context.Context, userID, action, resource, details string) error {
	return nil // No-op for demo
}

// IntegrationDemo demonstrates the integration of monster project patterns
type IntegrationDemo struct {
	queue  *queue.KQueue
	writer *kafka.Writer
	ctx    context.Context
	cancel context.CancelFunc
}

// NewIntegrationDemo creates a new integration demo instance
func NewIntegrationDemo(brokers []string, topic string, groupID string) (*IntegrationDemo, error) {
	// Create Kafka writer for producing messages
	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}

	// Create context
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize provider factory
	providers := provider.InitializeProviders()

	// Create mock implementations for demo
	mockStore := &mockDataStore{}
	mockIdempotent := &mockIdempotentChecker{}
	mockLogger := &mockHistoryLogger{}

	// Create message consumer
	consumer := mqs.NewCommonMessageConsumer(ctx, providers, mockStore, mockIdempotent, mockLogger)

	// Create Kafka queue configuration
	config := &queue.KafkaConfig{
		Brokers:       brokers,
		Topic:         topic,
		GroupID:       groupID,
		Processors:    4,
		Consumers:     4,
		ForceCommit:   true,
		QueueCapacity: 100,
		MinBytes:      1,
		MaxBytes:      1024 * 1024, // 1MB
	}

	// Create Kafka queue
	kQueue, err := queue.NewKQueue(config, consumer)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka queue: %w", err)
	}

	return &IntegrationDemo{
		queue:  kQueue,
		writer: writer,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Start begins the integration demo
func (d *IntegrationDemo) Start() {
	log.Infow("Starting integration demo")
	go d.queue.Start()
}

// Stop gracefully stops the integration demo
func (d *IntegrationDemo) Stop() {
	log.Infow("Stopping integration demo")
	d.cancel()
	d.queue.Stop()
	if d.writer != nil {
		d.writer.Close()
	}
}

// ProduceTemplateMessage produces a template message to Kafka
func (d *IntegrationDemo) ProduceTemplateMessage(requestID, phoneNumber, content, templateCode string, providers []string) error {
	msg := &types.TemplateMsgRequest{
		RequestId:    requestID,
		PhoneNumber:  phoneNumber,
		Content:      content,
		TemplateCode: templateCode,
		Providers:    providers,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	kafkaMsg := kafka.Message{
		Key:   []byte(requestID),
		Value: msgBytes,
		Time:  time.Now(),
	}

	err = d.writer.WriteMessages(d.ctx, kafkaMsg)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	log.Infow("Template message produced", "request_id", requestID, "phone", phoneNumber)
	return nil
}

// ProduceUplinkMessage produces an uplink message to Kafka
func (d *IntegrationDemo) ProduceUplinkMessage(requestID, phoneNumber, content, destCode string) error {
	msg := &types.UplinkMsgRequest{
		RequestId:   requestID,
		PhoneNumber: phoneNumber,
		Content:     content,
		DestCode:    destCode,
		SendTime:    time.Now(),
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal uplink message: %w", err)
	}

	kafkaMsg := kafka.Message{
		Key:   []byte(requestID),
		Value: msgBytes,
		Time:  time.Now(),
	}

	err = d.writer.WriteMessages(d.ctx, kafkaMsg)
	if err != nil {
		return fmt.Errorf("failed to write uplink message: %w", err)
	}

	log.Infow("Uplink message produced", "request_id", requestID, "phone", phoneNumber)
	return nil
}

// RunIntegrationDemo runs a complete integration demo
func RunIntegrationDemo(brokers []string, topic string, groupID string) error {
	demo, err := NewIntegrationDemo(brokers, topic, groupID)
	if err != nil {
		return fmt.Errorf("failed to create integration demo: %w", err)
	}

	// Start the demo
	demo.Start()
	defer demo.Stop()

	// Wait a moment for the consumer to be ready
	time.Sleep(2 * time.Second)

	// Produce some template messages
	for i := 0; i < 3; i++ {
		err := demo.ProduceTemplateMessage(
			fmt.Sprintf("template-req-%d", i),
			fmt.Sprintf("1380000000%d", i),
			fmt.Sprintf("Hello from template message %d", i),
			"SMS_TEMPLATE_001",
			[]string{"ALIYUN", "TENCENT"},
		)
		if err != nil {
			log.Errorw("Failed to produce template message", "error", err)
		}
		time.Sleep(1 * time.Second)
	}

	// Produce some uplink messages
	for i := 0; i < 2; i++ {
		err := demo.ProduceUplinkMessage(
			fmt.Sprintf("uplink-req-%d", i),
			fmt.Sprintf("1390000000%d", i),
			fmt.Sprintf("Reply from user %d", i),
			"10086",
		)
		if err != nil {
			log.Errorw("Failed to produce uplink message", "error", err)
		}
		time.Sleep(1 * time.Second)
	}

	// Let messages process
	time.Sleep(5 * time.Second)

	log.Infow("Integration demo completed")
	return nil
}
