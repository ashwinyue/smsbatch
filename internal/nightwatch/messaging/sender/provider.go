package sender

import (
	"github.com/google/wire"
)

// ProviderSet is the Wire provider set for sender components
var ProviderSet = wire.NewSet(
	NewKafkaSender,
	NewBatchEventPublisher,
	ProvideDefaultKafkaConfig,
)

// ProvideDefaultKafkaConfig provides default Kafka configuration
func ProvideDefaultKafkaConfig() *KafkaSenderConfig {
	return &KafkaSenderConfig{
		Brokers:     []string{"localhost:9092"},
		Topic:       "nightwatch-messages",
		Compression: "gzip",
		BatchSize:   100,
		MaxAttempts: 3,
		Async:       false,
	}
}