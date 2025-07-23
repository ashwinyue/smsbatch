package messaging

import (
	"context"

	"github.com/google/wire"

	"github.com/ashwinyue/dcp/internal/nightwatch/messaging/sender"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
)

// ProviderSet is the provider set for messaging service
var ProviderSet = wire.NewSet(
	NewUnifiedMessagingServiceWithDefaults,
)

// ProvideDefaultConfig provides default configuration for messaging service
func ProvideDefaultConfig(store store.IStore) *MessagingConfig {
	return &MessagingConfig{
		KafkaConfig: &sender.KafkaSenderConfig{
			Brokers:     []string{"localhost:9092"},
			Topic:       "nightwatch-messages",
			Compression: "gzip",
			BatchSize:   100,
			MaxAttempts: 3,
			Async:       false,
		},
		Topic: "nightwatch-messages",
	}
}

// NewUnifiedMessagingServiceWithDefaults creates a new messaging service with default config
func NewUnifiedMessagingServiceWithDefaults(store store.IStore) (*MessagingService, error) {
	config := ProvideDefaultConfig(store)
	return NewMessagingService(context.Background(), config)
}
