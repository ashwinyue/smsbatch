package core

import (
	"github.com/google/wire"
	"go.uber.org/ratelimit"

	"github.com/ashwinyue/dcp/internal/nightwatch/service"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/fsm"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/manager"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/processor"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/publisher"
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

// NewRateLimiter creates a new rate limiter from config
func NewRateLimiter(config *RateLimiterConfig) *fsm.RateLimiter {
	return &fsm.RateLimiter{
		Preparation: ratelimit.New(config.PreparationRPS),
		Delivery:    ratelimit.New(config.DeliveryRPS),
	}
}

// CoreProviderSet is the Wire provider set for core components
var CoreProviderSet = wire.NewSet(
	processor.NewPreparationProcessor,
	processor.NewDeliveryProcessor,
	manager.NewStateManager,
	publisher.NewEventPublisher,
	fsm.NewValidator,
	manager.NewPartitionManager,
	NewRateLimiter,
	fsm.NewEventCoordinator,
)

// InitializeEventCoordinator initializes an EventCoordinator with all dependencies
func InitializeEventCoordinator(
	tableStorageService service.TableStorageService,
	storeInterface store.IStore,
	config *RateLimiterConfig,
) (*fsm.EventCoordinator, error) {
	// Create rate limiter
	rateLimiter := NewRateLimiter(config)

	// Create publisher first (needed by processors)
	eventPublisher := publisher.NewEventPublisher(nil) // Pass nil for now, can be set later

	// Create partition manager (needed by processors)
	partitionManager := manager.NewPartitionManager(storeInterface, nil) // Pass nil for provider factory for now

	// Create processors with correct parameters
	preparationProcessor := processor.NewPreparationProcessor(eventPublisher, partitionManager, tableStorageService)
	deliveryProcessor := processor.NewDeliveryProcessor(partitionManager, eventPublisher, tableStorageService)

	// Create state manager
	stateManager := manager.NewStateManager(storeInterface)

	// Create validator
	validator := fsm.NewValidator()

	// Create and return EventCoordinator
	return fsm.NewEventCoordinator(
		preparationProcessor,
		deliveryProcessor,
		stateManager,
		eventPublisher,
		validator,
		partitionManager,
		rateLimiter,
	), nil
}
