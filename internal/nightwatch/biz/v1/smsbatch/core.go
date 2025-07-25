package smsbatch

import (
	"github.com/google/wire"
	"go.uber.org/ratelimit"

	"github.com/ashwinyue/dcp/internal/nightwatch/messaging/sender"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
)

// RateLimiterConfig holds configuration for rate limiting
type RateLimiterConfig struct {
	PreparationRPS int
	DeliveryRPS    int
}

// DefaultRateLimiterConfig returns default rate limiter configuration
func DefaultRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		PreparationRPS: 100,
		DeliveryRPS:    50,
	}
}

// RateLimiter holds rate limiters for different operations
type RateLimiter struct {
	Preparation ratelimit.Limiter
	Delivery    ratelimit.Limiter
}

// NewRateLimiter creates a new rate limiter from config
func NewRateLimiter(config *RateLimiterConfig) *RateLimiter {
	return &RateLimiter{
		Preparation: ratelimit.New(config.PreparationRPS),
		Delivery:    ratelimit.New(config.DeliveryRPS),
	}
}

// Validator provides validation functionality for SMS batch processing
type Validator struct{}

// NewValidator creates a new Validator instance
func NewValidator() *Validator {
	return &Validator{}
}

// EventCoordinator coordinates SMS batch processing events and state transitions
type EventCoordinator struct {
	preparationProcessor *PreparationProcessor
	deliveryProcessor    *DeliveryProcessor
	stateManager         *StateManager
	eventPublisher       *EventPublisher
	validator            *Validator
	rateLimiter          *RateLimiter
}

// NewEventCoordinator creates a new EventCoordinator instance
func NewEventCoordinator(
	preparationProcessor *PreparationProcessor,
	deliveryProcessor *DeliveryProcessor,
	stateManager *StateManager,
	eventPublisher *EventPublisher,
	validator *Validator,
	rateLimiter *RateLimiter,
) *EventCoordinator {
	return &EventCoordinator{
		preparationProcessor: preparationProcessor,
		deliveryProcessor:    deliveryProcessor,
		stateManager:         stateManager,
		eventPublisher:       eventPublisher,
		validator:            validator,
		rateLimiter:          rateLimiter,
	}
}

// SetEventPublisher sets the event publisher (deprecated - MQS service removed)
func (ec *EventCoordinator) SetEventPublisher(mqsPublisher interface{}) {
	// MQS service has been removed, this method is now a no-op
	// Event publishing is handled by the new unified messaging service
}

// CoreProviderSet is the Wire provider set for core components
var CoreProviderSet = wire.NewSet(
	sender.ProviderSet,
	NewPreparationProcessor,
	NewDeliveryProcessor,
	NewStateManager,
	NewEventPublisher,
	NewValidator,
	NewPartitionManager,
	NewRateLimiter,
	NewEventCoordinator,
)

// InitializeEventCoordinator initializes an EventCoordinator with all dependencies
// This function is now deprecated in favor of Wire dependency injection
// Use Wire to create EventCoordinator with proper dependency injection
func InitializeEventCoordinator(
	tableStorageStore store.TableStorageStore,
	storeInterface store.IStore,
	config *RateLimiterConfig,
) (*EventCoordinator, error) {
	// Create rate limiter
	rateLimiter := NewRateLimiter(config)

	// Create Kafka sender with default config
	kafkaConfig := sender.ProvideDefaultKafkaConfig()
	kafkaSender, err := sender.NewKafkaSender(kafkaConfig)
	if err != nil {
		return nil, err
	}

	// Create batch event publisher
	batchPublisher := sender.NewBatchEventPublisher(kafkaSender)

	// Create event publisher with proper dependency
	eventPublisher := NewEventPublisher(batchPublisher)

	// Create partition manager (needed by processors)
	partitionManager := NewPartitionManager(storeInterface, nil) // Pass nil for provider factory for now

	// Create processors with correct parameters
	preparationProcessor := NewPreparationProcessor(eventPublisher, partitionManager, tableStorageStore)
	deliveryProcessor := NewDeliveryProcessor(partitionManager, eventPublisher, tableStorageStore)

	// Create state manager
	stateManager := NewStateManager(storeInterface)

	// Create validator
	validator := NewValidator()

	// Create and return EventCoordinator
	return NewEventCoordinator(
		preparationProcessor,
		deliveryProcessor,
		stateManager,
		eventPublisher,
		validator,
		rateLimiter,
	), nil
}
