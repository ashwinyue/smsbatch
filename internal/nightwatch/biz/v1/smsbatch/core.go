package smsbatch

import (
	"github.com/google/wire"
	"go.uber.org/ratelimit"

	"github.com/ashwinyue/dcp/internal/nightwatch/mqs"
	"github.com/ashwinyue/dcp/internal/nightwatch/service"
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

// SetEventPublisher sets the event publisher with MQS service
func (ec *EventCoordinator) SetEventPublisher(mqsPublisher interface{}) {
	if publisher, ok := mqsPublisher.(*mqs.BatchEventPublisher); ok {
		ec.eventPublisher.SetMqsPublisher(publisher)
	}
}

// CoreProviderSet is the Wire provider set for core components
var CoreProviderSet = wire.NewSet(
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
func InitializeEventCoordinator(
	tableStorageService service.TableStorageService,
	storeInterface store.IStore,
	config *RateLimiterConfig,
) (*EventCoordinator, error) {
	// Create rate limiter
	rateLimiter := NewRateLimiter(config)

	// Create publisher first (needed by processors)
	eventPublisher := NewEventPublisher(nil) // Pass nil for now, can be set later

	// Create partition manager (needed by processors)
	partitionManager := NewPartitionManager(storeInterface, nil) // Pass nil for provider factory for now

	// Create processors with correct parameters
	preparationProcessor := NewPreparationProcessor(eventPublisher, partitionManager, tableStorageService)
	deliveryProcessor := NewDeliveryProcessor(partitionManager, eventPublisher, tableStorageService)

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
