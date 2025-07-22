// Copyright 2022 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/onex.
//

package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"k8s.io/klog/v2"

	kafkaconnector "github.com/ashwinyue/dcp/pkg/streams/connector/kafka"
)

// DemoMessage represents a demo message structure.
type DemoMessage struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
}

// ConsumerDemo demonstrates Kafka consumer functionality in nightwatch.
type ConsumerDemo struct {
	ctx    context.Context
	cancel context.CancelFunc
	source *kafkaconnector.KafkaSource
}

// NewConsumerDemo creates a new Kafka consumer demo instance.
func NewConsumerDemo(brokers []string, topic string, groupID string) (*ConsumerDemo, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Configure Kafka reader
	readerConfig := kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	}

	// Create Kafka source using the streams connector
	source, err := kafkaconnector.NewKafkaSource(ctx, readerConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create Kafka source: %w", err)
	}

	return &ConsumerDemo{
		ctx:    ctx,
		cancel: cancel,
		source: source,
	}, nil
}

// Start begins consuming messages from Kafka.
func (cd *ConsumerDemo) Start() {
	klog.InfoS("Starting Kafka consumer demo")

	// Start consuming messages
	go cd.consumeMessages()
}

// Stop gracefully stops the consumer.
func (cd *ConsumerDemo) Stop() {
	klog.InfoS("Stopping Kafka consumer demo")
	cd.cancel()
}

// consumeMessages processes incoming Kafka messages.
func (cd *ConsumerDemo) consumeMessages() {
	for {
		select {
		case <-cd.ctx.Done():
			klog.InfoS("Consumer demo context cancelled")
			return
		case msg, ok := <-cd.source.Out():
			if !ok {
				klog.InfoS("Kafka source channel closed")
				return
			}

			// Process the message
			cd.processMessage(msg)
		}
	}
}

// processMessage handles individual Kafka messages.
func (cd *ConsumerDemo) processMessage(msg any) {
	kafkaMsg, ok := msg.(kafka.Message)
	if !ok {
		klog.ErrorS(nil, "Received non-Kafka message", "message", msg)
		return
	}

	klog.InfoS("Received Kafka message",
		"topic", kafkaMsg.Topic,
		"partition", kafkaMsg.Partition,
		"offset", kafkaMsg.Offset,
		"key", string(kafkaMsg.Key),
		"value_size", len(kafkaMsg.Value),
	)

	// Try to parse as DemoMessage
	var demoMsg DemoMessage
	if err := json.Unmarshal(kafkaMsg.Value, &demoMsg); err != nil {
		klog.V(2).InfoS("Message is not a valid DemoMessage, treating as raw text",
			"error", err,
			"raw_value", string(kafkaMsg.Value),
		)
		// Handle as raw message
		cd.handleRawMessage(kafkaMsg)
	} else {
		// Handle as structured message
		cd.handleDemoMessage(demoMsg, kafkaMsg)
	}
}

// handleDemoMessage processes structured demo messages.
func (cd *ConsumerDemo) handleDemoMessage(demoMsg DemoMessage, kafkaMsg kafka.Message) {
	klog.InfoS("Processing demo message",
		"id", demoMsg.ID,
		"content", demoMsg.Content,
		"timestamp", demoMsg.Timestamp,
		"source", demoMsg.Source,
		"kafka_topic", kafkaMsg.Topic,
		"kafka_partition", kafkaMsg.Partition,
		"kafka_offset", kafkaMsg.Offset,
	)

	// Here you can add your business logic
	// For example: save to database, trigger workflows, etc.
	// This is just a demo, so we'll just log the message
}

// handleRawMessage processes raw text messages.
func (cd *ConsumerDemo) handleRawMessage(kafkaMsg kafka.Message) {
	klog.InfoS("Processing raw message",
		"content", string(kafkaMsg.Value),
		"topic", kafkaMsg.Topic,
		"partition", kafkaMsg.Partition,
		"offset", kafkaMsg.Offset,
	)

	// Handle raw message logic here
}

// GetStats returns consumer statistics (placeholder for future implementation).
func (cd *ConsumerDemo) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"status":       "running",
		"context_done": cd.ctx.Err() != nil,
	}
}
