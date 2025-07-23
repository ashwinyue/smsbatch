package smsbatch

import (
	"github.com/google/wire"

	"github.com/ashwinyue/dcp/internal/nightwatch/service"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
)

// ProviderSet is the Wire provider set for SMS batch business logic
var ProviderSet = wire.NewSet(
	CoreProviderSet,
	New, // SMS batch service
	DefaultRateLimiterConfig,
)

// NewSmsBatchWithDefaults creates a new SMS batch service with default configuration
func NewSmsBatchWithDefaults(store store.IStore) ISmsBatchV1 {
	return New(store)
}

// NewEventCoordinatorWithDefaults creates an EventCoordinator with default configuration
func NewEventCoordinatorWithDefaults(
	tableStorageService service.TableStorageService,
	storeInterface store.IStore,
) (*EventCoordinator, error) {
	config := DefaultRateLimiterConfig()
	return InitializeEventCoordinator(tableStorageService, storeInterface, config)
}