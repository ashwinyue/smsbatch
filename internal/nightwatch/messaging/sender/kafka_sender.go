package sender

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/pkg/log"
	"github.com/segmentio/kafka-go"
)

// KafkaSender 用于发送Kafka消息
type KafkaSender struct {
	writer *kafka.Writer
	ctx    context.Context
}

// KafkaSenderConfig Kafka发送器配置
type KafkaSenderConfig struct {
	Brokers      []string      `json:"brokers"`
	Topic        string        `json:"topic"`
	Compression  string        `json:"compression,omitempty"` // none, gzip, snappy, lz4, zstd
	BatchSize    int           `json:"batch_size,omitempty"`
	BatchTimeout time.Duration `json:"batch_timeout,omitempty"`
	MaxAttempts  int           `json:"max_attempts,omitempty"`
	Async        bool          `json:"async,omitempty"`
}

// NewKafkaSender 创建新的Kafka发送器
func NewKafkaSender(config *KafkaSenderConfig) (*KafkaSender, error) {
	if len(config.Brokers) == 0 {
		return nil, fmt.Errorf("brokers cannot be empty")
	}
	if config.Topic == "" {
		return nil, fmt.Errorf("topic cannot be empty")
	}

	// 设置默认值
	if config.BatchSize == 0 {
		config.BatchSize = 100
	}
	if config.BatchTimeout == 0 {
		config.BatchTimeout = time.Second
	}
	if config.MaxAttempts == 0 {
		config.MaxAttempts = 3
	}

	// 创建Kafka writer
	writer := &kafka.Writer{
		Addr:         kafka.TCP(config.Brokers...),
		Topic:        config.Topic,
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    config.BatchSize,
		BatchTimeout: config.BatchTimeout,
		MaxAttempts:  config.MaxAttempts,
		Async:        config.Async,
	}

	// 设置压缩算法
	switch config.Compression {
	case "gzip":
		writer.Compression = kafka.Gzip
	case "snappy":
		writer.Compression = kafka.Snappy
	case "lz4":
		writer.Compression = kafka.Lz4
	case "zstd":
		writer.Compression = kafka.Zstd
	case "none", "":
		// 不设置压缩，使用默认值
	default:
		// 不设置压缩，使用默认值
	}

	return &KafkaSender{
		writer: writer,
		ctx:    context.Background(),
	}, nil
}

// SendMessage 发送单条消息
func (s *KafkaSender) SendMessage(msg *Message) error {
	return s.SendMessages([]*Message{msg})
}

// SendMessages 批量发送消息
func (s *KafkaSender) SendMessages(messages []*Message) error {
	if len(messages) == 0 {
		return fmt.Errorf("messages cannot be empty")
	}

	kafkaMessages := make([]kafka.Message, 0, len(messages))

	for _, msg := range messages {
		// 序列化消息值
		valueBytes, err := s.serializeValue(msg.Value)
		if err != nil {
			return fmt.Errorf("failed to serialize message value: %w", err)
		}

		// 构建Kafka消息
		kafkaMsg := kafka.Message{
			Key:   []byte(msg.Key),
			Value: valueBytes,
			Time:  msg.Timestamp,
		}

		// 设置时间戳
		if msg.Timestamp.IsZero() {
			kafkaMsg.Time = time.Now()
		}

		// 设置分区
		if msg.Partition > 0 {
			kafkaMsg.Partition = msg.Partition
		}

		// 设置Headers
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

	// 发送消息
	err := s.writer.WriteMessages(s.ctx, kafkaMessages...)
	if err != nil {
		log.Errorw("Failed to send kafka messages", "error", err, "count", len(kafkaMessages))
		return fmt.Errorf("failed to send kafka messages: %w", err)
	}

	log.Infow("Successfully sent kafka messages", "count", len(kafkaMessages), "topic", s.writer.Topic)
	return nil
}

// SendJSONMessage 发送JSON格式消息
func (s *KafkaSender) SendJSONMessage(key string, data interface{}, headers map[string]string) error {
	msg := &Message{
		Key:     key,
		Value:   data,
		Headers: headers,
	}
	return s.SendMessage(msg)
}

// SendTextMessage 发送文本消息
func (s *KafkaSender) SendTextMessage(key, text string, headers map[string]string) error {
	msg := &Message{
		Key:     key,
		Value:   text,
		Headers: headers,
	}
	return s.SendMessage(msg)
}

// SendSMSMessage 发送短信消息
func (s *KafkaSender) SendSMSMessage(requestID, phoneNumber, content, templateCode string, providers []string) error {
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

// SendUplinkMessage 发送上行消息
func (s *KafkaSender) SendUplinkMessage(requestID, phoneNumber, content, destCode string) error {
	uplinkMsg := map[string]interface{}{
		"request_id":   requestID,
		"phone_number": phoneNumber,
		"content":      content,
		"dest_code":    destCode,
		"send_time":    time.Now(),
		"message_type": "uplink_sms",
	}

	msg := &Message{
		Key:   requestID,
		Value: uplinkMsg,
		Headers: map[string]string{
			"message_type": "uplink_sms",
			"phone_number": phoneNumber,
		},
	}

	return s.SendMessage(msg)
}

// serializeValue 序列化消息值
func (s *KafkaSender) serializeValue(value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	default:
		// 对于其他类型，使用JSON序列化
		return json.Marshal(v)
	}
}

// Close 关闭发送器
func (s *KafkaSender) Close() error {
	if s.writer != nil {
		return s.writer.Close()
	}
	return nil
}

// GetStats 获取发送统计信息
func (s *KafkaSender) GetStats() kafka.WriterStats {
	if s.writer != nil {
		return s.writer.Stats()
	}
	return kafka.WriterStats{}
}

// SetContext 设置上下文
func (s *KafkaSender) SetContext(ctx context.Context) {
	s.ctx = ctx
}
