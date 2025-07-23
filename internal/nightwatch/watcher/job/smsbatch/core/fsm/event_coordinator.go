package fsm

import (
	"context"

	"go.uber.org/ratelimit"

	"github.com/ashwinyue/dcp/internal/nightwatch/mqs"
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

// NewEventCoordinator creates a new EventCoordinator instance using Wire dependency injection
func NewEventCoordinator(
	preparationProcessor *processor.PreparationProcessor,
	deliveryProcessor *processor.DeliveryProcessor,
	stateManager *manager.StateManager,
	eventPublisher *publisher.EventPublisher,
	validator *Validator,
	partitionManager *manager.PartitionManager,
	rateLimiter *RateLimiter,
) *EventCoordinator {
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
