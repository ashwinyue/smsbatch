// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

package app

import (
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/message"
	"github.com/spf13/cobra"
)

// NewMessageCommand creates a new message command.
func NewMessageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "message",
		Short: "Message sending commands",
		Long:  `Message sending commands for nightwatch service.`,
	}

	// Add subcommands
	cmd.AddCommand(NewKafkaSendCommand())

	return cmd
}

// NewKafkaSendCommand creates the kafka send command
func NewKafkaSendCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kafka-send",
		Short: "Send messages to Kafka",
		Long:  `Send various types of messages to Kafka using the message sender.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			brokers, _ := cmd.Flags().GetStringSlice("brokers")
			topic, _ := cmd.Flags().GetString("topic")
			messageType, _ := cmd.Flags().GetString("type")
			key, _ := cmd.Flags().GetString("key")
			value, _ := cmd.Flags().GetString("value")
			count, _ := cmd.Flags().GetInt("count")
			compression, _ := cmd.Flags().GetString("compression")
			batchSize, _ := cmd.Flags().GetInt("batch-size")
			async, _ := cmd.Flags().GetBool("async")

			if len(brokers) == 0 {
				return fmt.Errorf("brokers cannot be empty")
			}
			if topic == "" {
				return fmt.Errorf("topic cannot be empty")
			}

			// 创建Kafka发送器配置
			config := &message.KafkaSenderConfig{
				Brokers:      brokers,
				Topic:        topic,
				Compression:  compression,
				BatchSize:    batchSize,
				BatchTimeout: time.Second,
				MaxAttempts:  3,
				Async:        async,
			}

			// 创建发送器
			sender, err := message.NewKafkaSender(config)
			if err != nil {
				return fmt.Errorf("failed to create kafka sender: %w", err)
			}
			defer sender.Close()

			// 根据消息类型发送消息
			switch messageType {
			case "text":
				return sendTextMessages(sender, key, value, count)
			case "sms":
				return sendSMSMessages(sender, count)
			case "uplink":
				return sendUplinkMessages(sender, count)
			case "json":
				return sendJSONMessages(sender, key, value, count)
			default:
				return fmt.Errorf("unsupported message type: %s", messageType)
			}
		},
	}

	// Add flags
	cmd.Flags().StringSliceP("brokers", "b", []string{"localhost:9092"}, "Kafka broker addresses")
	cmd.Flags().StringP("topic", "t", "nightwatch-messages", "Kafka topic name")
	cmd.Flags().StringP("type", "T", "text", "Message type (text, sms, uplink, json)")
	cmd.Flags().StringP("key", "k", "", "Message key")
	cmd.Flags().StringP("value", "v", "Hello Kafka!", "Message value")
	cmd.Flags().IntP("count", "n", 1, "Number of messages to send")
	cmd.Flags().StringP("compression", "c", "none", "Compression type (none, gzip, snappy, lz4, zstd)")
	cmd.Flags().IntP("batch-size", "s", 100, "Batch size for sending messages")
	cmd.Flags().BoolP("async", "a", false, "Send messages asynchronously")

	return cmd
}

// sendTextMessages 发送文本消息
func sendTextMessages(sender *message.KafkaSender, key, value string, count int) error {
	for i := 0; i < count; i++ {
		msgKey := fmt.Sprintf("%s-%d", key, i)
		if key == "" {
			msgKey = fmt.Sprintf("text-msg-%d", i)
		}
		msgValue := fmt.Sprintf("%s [%d]", value, i)

		err := sender.SendTextMessage(msgKey, msgValue, map[string]string{
			"message_type": "text",
			"sequence":     fmt.Sprintf("%d", i),
		})
		if err != nil {
			return fmt.Errorf("failed to send text message %d: %w", i, err)
		}
		fmt.Printf("Sent text message %d: key=%s, value=%s\n", i, msgKey, msgValue)
	}
	return nil
}

// sendSMSMessages 发送短信消息
func sendSMSMessages(sender *message.KafkaSender, count int) error {
	for i := 0; i < count; i++ {
		requestID := fmt.Sprintf("sms-req-%d-%d", time.Now().Unix(), i)
		phoneNumber := fmt.Sprintf("1380000000%d", i%10)
		content := fmt.Sprintf("您的验证码是：%06d，请在5分钟内使用。", 100000+i)
		templateCode := "SMS_VERIFY_CODE"
		providers := []string{"ALIYUN", "TENCENT"}

		err := sender.SendSMSMessage(requestID, phoneNumber, content, templateCode, providers)
		if err != nil {
			return fmt.Errorf("failed to send SMS message %d: %w", i, err)
		}
		fmt.Printf("Sent SMS message %d: request_id=%s, phone=%s\n", i, requestID, phoneNumber)
	}
	return nil
}

// sendUplinkMessages 发送上行消息
func sendUplinkMessages(sender *message.KafkaSender, count int) error {
	for i := 0; i < count; i++ {
		requestID := fmt.Sprintf("uplink-req-%d-%d", time.Now().Unix(), i)
		phoneNumber := fmt.Sprintf("1390000000%d", i%10)
		content := fmt.Sprintf("用户回复内容：TD %d", i)
		destCode := "10086"

		err := sender.SendUplinkMessage(requestID, phoneNumber, content, destCode)
		if err != nil {
			return fmt.Errorf("failed to send uplink message %d: %w", i, err)
		}
		fmt.Printf("Sent uplink message %d: request_id=%s, phone=%s\n", i, requestID, phoneNumber)
	}
	return nil
}

// sendJSONMessages 发送JSON消息
func sendJSONMessages(sender *message.KafkaSender, key, value string, count int) error {
	for i := 0; i < count; i++ {
		msgKey := fmt.Sprintf("%s-%d", key, i)
		if key == "" {
			msgKey = fmt.Sprintf("json-msg-%d", i)
		}

		// 创建JSON数据
		jsonData := map[string]interface{}{
			"id":        i,
			"message":   value,
			"timestamp": time.Now(),
			"source":    "nightwatch-cli",
			"metadata": map[string]interface{}{
				"sequence": i,
				"total":    count,
			},
		}

		err := sender.SendJSONMessage(msgKey, jsonData, map[string]string{
			"message_type": "json",
			"sequence":     fmt.Sprintf("%d", i),
		})
		if err != nil {
			return fmt.Errorf("failed to send JSON message %d: %w", i, err)
		}
		fmt.Printf("Sent JSON message %d: key=%s\n", i, msgKey)
	}
	return nil
}
