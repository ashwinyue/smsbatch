package fsm

import (
	"github.com/ashwinyue/dcp/internal/nightwatch/mqs"
	"github.com/ashwinyue/dcp/internal/nightwatch/service"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/manager"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/processor"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/publisher"
)

// EventCoordinator coordinates SMS batch event processing
type EventCoordinator struct {
	preparationProcessor *processor.PreparationProcessor
	deliveryProcessor    *processor.DeliveryProcessor
	stateManager         *manager.StateManager
	eventPublisher       *publisher.EventPublisher
	validator            *Validator
	partitionManager     *manager.PartitionManager
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
	partitionManager := manager.NewPartitionManager()
	eventPublisher := publisher.NewEventPublisher(nil) // Will be set when watcher is available
	preparationProcessor := processor.NewPreparationProcessor(eventPublisher, partitionManager, tableStorageService)
	deliveryProcessor := processor.NewDeliveryProcessor(partitionManager, eventPublisher, tableStorageService)

	// Create StateManager with proper store interface
	stateManager := manager.NewStateManager(storeInterface)

	return &EventCoordinator{
		preparationProcessor: preparationProcessor,
		deliveryProcessor:    deliveryProcessor,
		stateManager:         stateManager,
		eventPublisher:       eventPublisher,
		validator:            validator,
		partitionManager:     partitionManager,
	}
}

// SetEventPublisher sets the event publisher with MQS service
func (ec *EventCoordinator) SetEventPublisher(mqsPublisher interface{}) {
	// This will be called when the watcher is initialized
	if publisher, ok := mqsPublisher.(*mqs.BatchEventPublisher); ok {
		ec.eventPublisher.SetMqsPublisher(publisher)
	}
}
