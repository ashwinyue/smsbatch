// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

package app

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ashwinyue/dcp/internal/nightwatch/kafka"
)

// NewKafkaCommand creates a new kafka command.
func NewKafkaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kafka",
		Short: "Kafka consumer demo commands",
		Long:  `Kafka consumer demo commands for nightwatch service.`,
	}

	// Add subcommands
	cmd.AddCommand(NewKafkaConsumerCommand())
	cmd.AddCommand(NewKafkaProducerCommand())
	cmd.AddCommand(NewKafkaIntegrationCommand())

	return cmd
}

// NewKafkaIntegrationCommand creates the kafka integration command
func NewKafkaIntegrationCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "integration",
		Short: "Run Kafka integration demo based on monster project patterns",
		Long:  `Run a comprehensive Kafka integration demo that demonstrates message production and consumption patterns from the monster project.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			brokers, _ := cmd.Flags().GetStringSlice("brokers")
			topic, _ := cmd.Flags().GetString("topic")
			groupID, _ := cmd.Flags().GetString("group-id")

			if len(brokers) == 0 {
				return fmt.Errorf("brokers cannot be empty")
			}
			if topic == "" {
				return fmt.Errorf("topic cannot be empty")
			}
			if groupID == "" {
				return fmt.Errorf("group-id cannot be empty")
			}

			return kafka.RunIntegrationDemo(brokers, topic, groupID)
		},
	}

	// Add flags
	cmd.Flags().StringSliceP("brokers", "b", []string{"localhost:9092"}, "Kafka broker addresses")
	cmd.Flags().StringP("topic", "t", "nightwatch-demo", "Kafka topic name")
	cmd.Flags().StringP("group-id", "g", "nightwatch-integration-group", "Kafka consumer group ID")

	return cmd
}

// NewKafkaConsumerCommand creates a new kafka consumer demo command.
func NewKafkaConsumerCommand() *cobra.Command {
	var (
		brokers []string
		topic   string
		groupID string
	)

	cmd := &cobra.Command{
		Use:   "consumer",
		Short: "Run Kafka consumer demo",
		Long: `Run Kafka consumer demo.

This command demonstrates how to consume messages from a Kafka topic
using the nightwatch Kafka consumer implementation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(brokers) == 0 {
				return fmt.Errorf("at least one broker must be specified")
			}
			if topic == "" {
				return fmt.Errorf("topic must be specified")
			}
			if groupID == "" {
				return fmt.Errorf("group ID must be specified")
			}

			return kafka.RunConsumerDemo(brokers, topic, groupID)
		},
	}

	// Add flags
	cmd.Flags().StringSliceVarP(&brokers, "brokers", "b", []string{"localhost:9092"}, "Kafka broker addresses")
	cmd.Flags().StringVarP(&topic, "topic", "t", "nightwatch-demo", "Kafka topic to consume from")
	cmd.Flags().StringVarP(&groupID, "group-id", "g", "nightwatch-consumer-group", "Kafka consumer group ID")

	return cmd
}

// NewKafkaProducerCommand creates a new kafka producer demo command.
func NewKafkaProducerCommand() *cobra.Command {
	var (
		brokers []string
		topic   string
		count   int
	)

	cmd := &cobra.Command{
		Use:   "producer",
		Short: "Run Kafka producer demo",
		Long: `Run Kafka producer demo.

This command demonstrates how to produce demo messages to a Kafka topic
for testing the consumer functionality.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(brokers) == 0 {
				return fmt.Errorf("at least one broker must be specified")
			}
			if topic == "" {
				return fmt.Errorf("topic must be specified")
			}
			if count <= 0 {
				return fmt.Errorf("count must be greater than 0")
			}

			return kafka.ProduceDemoMessages(brokers, topic, count)
		},
	}

	// Add flags
	cmd.Flags().StringSliceVarP(&brokers, "brokers", "b", []string{"localhost:9092"}, "Kafka broker addresses")
	cmd.Flags().StringVarP(&topic, "topic", "t", "nightwatch-demo", "Kafka topic to produce to")
	cmd.Flags().IntVarP(&count, "count", "n", 10, "Number of demo messages to produce")

	return cmd
}

// Example usage:
// ./nightwatch kafka consumer --brokers localhost:9092 --topic test-topic --group-id test-group
// ./nightwatch kafka producer --brokers localhost:9092 --topic test-topic --count 5
