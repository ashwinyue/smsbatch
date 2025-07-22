// Copyright 2022 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/onex.
//

package kafka

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/klog/v2"
)

// RunConsumerDemo runs a Kafka consumer demo.
// This function demonstrates how to use the Kafka consumer in nightwatch.
func RunConsumerDemo(brokers []string, topic string, groupID string) error {
	klog.InfoS("Starting Kafka consumer demo",
		"brokers", brokers,
		"topic", topic,
		"groupID", groupID,
	)

	// Create consumer demo instance
	consumer, err := NewConsumerDemo(brokers, topic, groupID)
	if err != nil {
		return fmt.Errorf("failed to create consumer demo: %w", err)
	}

	// Start consuming
	consumer.Start()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan
	klog.InfoS("Received shutdown signal")

	// Stop consumer
	consumer.Stop()

	// Give some time for graceful shutdown
	time.Sleep(2 * time.Second)
	klog.InfoS("Kafka consumer demo stopped")

	return nil
}

// ProduceDemoMessages produces some demo messages to Kafka for testing.
// This is a helper function to generate test data.
func ProduceDemoMessages(brokers []string, topic string, count int) error {
	klog.InfoS("Producing demo messages",
		"brokers", brokers,
		"topic", topic,
		"count", count,
	)

	// This would require a Kafka producer implementation
	// For now, we'll just log what would be produced
	for i := 0; i < count; i++ {
		demoMsg := DemoMessage{
			ID:        fmt.Sprintf("demo-msg-%d", i),
			Content:   fmt.Sprintf("This is demo message number %d", i),
			Timestamp: time.Now(),
			Source:    "nightwatch-demo",
		}

		msgBytes, err := json.Marshal(demoMsg)
		if err != nil {
			return fmt.Errorf("failed to marshal demo message: %w", err)
		}

		klog.InfoS("Would produce message",
			"id", demoMsg.ID,
			"content", demoMsg.Content,
			"json", string(msgBytes),
		)
	}

	return nil
}

// Example usage function that can be called from main or tests.
func ExampleUsage() {
	// Example configuration
	brokers := []string{"localhost:9092"}
	topic := "nightwatch-demo"
	groupID := "nightwatch-consumer-group"

	// Run the demo
	if err := RunConsumerDemo(brokers, topic, groupID); err != nil {
		klog.ErrorS(err, "Failed to run consumer demo")
	}
}
