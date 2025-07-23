package messaging

import (
	"context"

	"github.com/google/wire"

	"github.com/ashwinyue/dcp/internal/nightwatch/store"
)

// ProviderSet is the provider set for messaging service
var ProviderSet = wire.NewSet(
	NewUnifiedMessagingServiceWithDefaults,
)

// ProvideDefaultConfig provides default configuration for messaging service
func ProvideDefaultConfig(store store.IStore) *Config {
	return &Config{
		KafkaConfig: &KafkaConfig{
			Brokers:     []string{"localhost:9092"},
			Topic:       "nightwatch-messages",
			Compression: "gzip",
			BatchSize:   100,
			MaxAttempts: 3,
			Async:       false,
		},
		Topic:         "nightwatch-messages",
		ConsumerGroup: "nightwatch-consumer",
		Store:         store,
	}
}

// NewUnifiedMessagingServiceWithDefaults creates a new messaging service with default config
func NewUnifiedMessagingServiceWithDefaults(store store.IStore) (*UnifiedMessagingService, error) {
	config := ProvideDefaultConfig(store)
	return NewUnifiedMessagingService(context.Background(), config)
}