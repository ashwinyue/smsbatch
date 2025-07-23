package batch

import (
	"context"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/pkg/log"
	"github.com/ashwinyue/dcp/pkg/queue"
)



// BatchEventService manages batch event processing
type BatchEventService struct {
	ctx       context.Context
	consumer  *BatchMessageConsumer
	kqueue    *queue.KQueue
	store     store.IStore
	// 指标统计
	messagesProcessed int64
	errorCount        int64
	startTime         time.Time
}

// BatchEventServiceConfig configuration for batch event service
type BatchEventServiceConfig struct {
	KafkaConfig    *KafkaSenderConfig `json:"kafka_config"`
	ConsumerConfig *queue.KafkaConfig         `json:"consumer_config"`
	Topic          string                     `json:"topic"`
	ConsumerGroup  string                     `json:"consumer_group"`
	Store          store.IStore               `json:"-"`
}



// Start starts the batch event service
func (s *BatchEventService) Start() error {
	log.Infow("Starting batch event service")

	// Start consuming messages
	go s.kqueue.Start()

	log.Infow("Batch event service started successfully")
	return nil
}

// Stop stops the batch event service
func (s *BatchEventService) Stop() error {
	log.Infow("Stopping batch event service")

	s.kqueue.Stop()

	log.Infow("Batch event service stopped successfully")
	return nil
}


// GetConsumer returns the batch message consumer
func (s *BatchEventService) GetConsumer() *BatchMessageConsumer {
	return s.consumer
}







// Health check methods

// IsHealthy checks if the batch event service is healthy
func (s *BatchEventService) IsHealthy() bool {
	// Check if KQueue is running
	if s.kqueue == nil {
		return false
	}

	// Additional health checks can be added here
	// For example, check Kafka connectivity, database connectivity, etc.

	return true
}

// GetMetrics returns service metrics
func (s *BatchEventService) GetMetrics() map[string]interface{} {
	metrics := make(map[string]interface{})

	// Add basic metrics
	metrics["service_status"] = "running"
	metrics["healthy"] = s.IsHealthy()
	metrics["timestamp"] = time.Now()

	// 添加详细指标
	metrics["messages_processed"] = s.messagesProcessed
	metrics["error_count"] = s.errorCount
	metrics["uptime_seconds"] = time.Since(s.startTime).Seconds()

	// 计算处理速率
	uptime := time.Since(s.startTime).Seconds()
	if uptime > 0 {
		metrics["messages_per_second"] = float64(s.messagesProcessed) / uptime
		metrics["error_rate"] = float64(s.errorCount) / float64(s.messagesProcessed)
	} else {
		metrics["messages_per_second"] = 0.0
		metrics["error_rate"] = 0.0
	}

	// 队列深度（如果KQueue支持的话）
	if s.kqueue != nil {
		// 注意：这里假设KQueue有GetQueueDepth方法，实际需要根据queue包的实现调整
		metrics["queue_depth"] = "N/A" // 需要KQueue支持才能获取
	}

	return metrics
}
