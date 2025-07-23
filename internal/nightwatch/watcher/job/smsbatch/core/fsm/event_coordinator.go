package fsm

import (
	"context"

	"go.uber.org/ratelimit"

	"github.com/ashwinyue/dcp/internal/nightwatch/mqs"
	"github.com/ashwinyue/dcp/internal/nightwatch/provider"
	"github.com/ashwinyue/dcp/internal/nightwatch/service"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/manager"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/processor"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/publisher"
)

// RateLimiter holds rate limiters for different operations
type RateLimiter struct {
	Preparation ratelimit.Limiter
	Delivery    ratelimit.Limiter
}

// EventCoordinator coordinates SMS batch event processing
type EventCoordinator struct {
	preparationProcessor *processor.PreparationProcessor
	deliveryProcessor    *processor.DeliveryProcessor
	stateManager         *manager.StateManager
	eventPublisher       *publisher.EventPublisher
	validator            *Validator
	partitionManager     *manager.PartitionManager
	rateLimiter          *RateLimiter
}

// EventCoordinatorInterface defines the interface for event coordination
type EventCoordinatorInterface interface {
	SetEventPublisher(mqsPublisher interface{})
}

// Ensure EventCoordinator implements EventCoordinatorInterface
var _ EventCoordinatorInterface = (*EventCoordinator)(nil)

// NewEventCoordinator creates a new EventCoordinator instance
func NewEventCoordinator(tableStorageService service.TableStorageService, storeInterface store.IStore) *EventCoordinator {
	validator := NewValidator()
	// Initialize provider factory
	providerFactory := provider.InitializeProviders()
	partitionManager := manager.NewPartitionManager(storeInterface, providerFactory)
	eventPublisher := publisher.NewEventPublisher(nil) // Will be set when watcher is available
	preparationProcessor := processor.NewPreparationProcessor(eventPublisher, partitionManager, tableStorageService)
	deliveryProcessor := processor.NewDeliveryProcessor(partitionManager, eventPublisher, tableStorageService)

	// Create StateManager with proper store interface
	stateManager := manager.NewStateManager(storeInterface)

	// Initialize rate limiter with default rates
	rateLimiter := &RateLimiter{
		Preparation: ratelimit.New(10), // 10 requests per second for preparation
		Delivery:    ratelimit.New(100), // 100 requests per second for delivery
	}

	return &EventCoordinator{
		preparationProcessor: preparationProcessor,
		deliveryProcessor:    deliveryProcessor,
		stateManager:         stateManager,
		eventPublisher:       eventPublisher,
		validator:            validator,
		partitionManager:     partitionManager,
		rateLimiter:          rateLimiter,
	}
}

// SetEventPublisher sets the event publisher with MQS service
func (ec *EventCoordinator) SetEventPublisher(mqsPublisher interface{}) {
	// This will be called when the watcher is initialized
	if publisher, ok := mqsPublisher.(*mqs.BatchEventPublisher); ok {
		ec.eventPublisher.SetMqsPublisher(publisher)
	}
}

// ApplyRateLimit applies rate limiting for the specified operation type
func (rl *RateLimiter) ApplyRateLimit(ctx context.Context, smsBatch interface{}) error {
	// Apply preparation rate limit by default
	rl.Preparation.Take()
	return nil
}
