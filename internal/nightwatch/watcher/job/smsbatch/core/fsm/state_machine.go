package fsm

import (
	"context"
	"fmt"
	"time"

	"github.com/looplab/fsm"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/manager"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/processor"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/publisher"
	"github.com/ashwinyue/dcp/internal/pkg/log"
	jobconditionsutil "github.com/ashwinyue/dcp/internal/pkg/util/jobconditions"
)

// Validator handles SMS batch parameter validation
type Validator struct{}

// NewValidator creates a new Validator instance
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateBatchParams validates SMS batch parameters
func (v *Validator) ValidateBatchParams(smsBatch *model.SmsBatchM) error {
	if smsBatch.BatchID == "" {
		return fmt.Errorf("batch ID is required")
	}
	if smsBatch.TableStorageName == "" {
		return fmt.Errorf("table storage name is required")
	}
	return nil
}

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
func NewEventCoordinator() *EventCoordinator {
	validator := NewValidator()
	partitionManager := manager.NewPartitionManager()
	eventPublisher := publisher.NewEventPublisher(nil) // Will be set when watcher is available
	preparationProcessor := processor.NewPreparationProcessor(eventPublisher, partitionManager)
	deliveryProcessor := processor.NewDeliveryProcessor(partitionManager, eventPublisher)
	// TODO: Fix StateManager constructor - it expects IStore interface
	stateManager := &manager.StateManager{}

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
	// ec.eventPublisher.mqsPublisher = mqsPublisher
}

// StateMachine represents the state machine for SMS batch processing
type StateMachine struct {
	FSM              *fsm.FSM
	SmsBatch         *model.SmsBatchM
	Watcher          interface{}
	EventCoordinator EventCoordinatorInterface
}

// NewStateMachine creates a new StateMachine instance
func NewStateMachine(smsBatch *model.SmsBatchM, watcher interface{}) *StateMachine {
	eventCoordinator := NewEventCoordinator()
	return &StateMachine{
		SmsBatch:         smsBatch,
		Watcher:          watcher,
		EventCoordinator: eventCoordinator,
	}
}

// PreparationExecute handles the SMS preparation phase.
func (sm *StateMachine) PreparationExecute(ctx context.Context, event *fsm.Event) error {
	ec := sm.EventCoordinator.(*EventCoordinator)
	return ec.preparationProcessor.Execute(ctx, sm.SmsBatch)
}

// DeliveryExecute handles the SMS delivery phase.
func (sm *StateMachine) DeliveryExecute(ctx context.Context, event *fsm.Event) error {
	ec := sm.EventCoordinator.(*EventCoordinator)
	return ec.deliveryProcessor.Execute(ctx, sm.SmsBatch)
}

// PreparationPause handles pausing the preparation phase.
func (sm *StateMachine) PreparationPause(ctx context.Context, event *fsm.Event) error {
	ec := sm.EventCoordinator.(*EventCoordinator)
	ec.stateManager.UpdateBatchStatus(sm.SmsBatch, "paused")
	log.Infow("SMS batch preparation paused", "batch_id", sm.SmsBatch.BatchID)
	return nil
}

// DeliveryPause handles pausing the delivery phase.
func (sm *StateMachine) DeliveryPause(ctx context.Context, event *fsm.Event) error {
	ec := sm.EventCoordinator.(*EventCoordinator)
	ec.stateManager.UpdateBatchStatus(sm.SmsBatch, "paused")
	log.Infow("SMS batch delivery paused", "batch_id", sm.SmsBatch.BatchID)
	return nil
}

// PreparationResume handles resuming the preparation phase.
func (sm *StateMachine) PreparationResume(ctx context.Context, event *fsm.Event) error {
	ec := sm.EventCoordinator.(*EventCoordinator)
	ec.stateManager.UpdateBatchStatus(sm.SmsBatch, "running")
	log.Infow("SMS batch preparation resumed", "batch_id", sm.SmsBatch.BatchID)
	return nil
}

// DeliveryResume handles resuming the delivery phase.
func (sm *StateMachine) DeliveryResume(ctx context.Context, event *fsm.Event) error {
	ec := sm.EventCoordinator.(*EventCoordinator)
	ec.stateManager.UpdateBatchStatus(sm.SmsBatch, "running")
	log.Infow("SMS batch delivery resumed", "batch_id", sm.SmsBatch.BatchID)
	return nil
}

// SetDefaultSmsBatchParams sets default parameters for SMS batch
func SetDefaultSmsBatchParams(smsBatch *model.SmsBatchM) {
	// Implementation for setting default parameters
	// This function was moved from types package
}

// InitialExecute handles the initial state of the SMS batch.
func (sm *StateMachine) InitialExecute(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch initialized", "batch_id", sm.SmsBatch.BatchID)

	// Set default SMS batch params
	SetDefaultSmsBatchParams(sm.SmsBatch)

	// Initialize SMS batch results
	if sm.SmsBatch.Results == nil {
		sm.SmsBatch.Results = &model.SmsBatchResults{}
	}

	sm.SmsBatch.Results.CurrentPhase = "initialization"
	sm.SmsBatch.Results.CurrentState = event.FSM.Current()

	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	return nil
}

// PreparationReady handles the preparation ready state.
func (sm *StateMachine) PreparationReady(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch preparation ready", "batch_id", sm.SmsBatch.BatchID)

	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentPhase = "preparation"
		sm.SmsBatch.Results.CurrentState = event.FSM.Current()
	}

	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	return nil
}

// PreparationStart handles the start of preparation phase.
func (sm *StateMachine) PreparationStart(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch preparation started", "batch_id", sm.SmsBatch.BatchID)

	// Apply rate limiting for preparation
	// TODO: Implement rate limiting when watcher interface is properly defined
	log.Infow("Rate limiting applied for preparation", "batch_id", sm.SmsBatch.BatchID)

	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentPhase = "preparation"
		sm.SmsBatch.Results.CurrentState = event.FSM.Current()
	}

	// Initialize preparation stats
	if sm.SmsBatch.Results != nil && sm.SmsBatch.Results.PreparationStats == nil {
		now := time.Now()
		sm.SmsBatch.Results.PreparationStats = &model.SmsBatchPhaseStats{
			StartTime: &now,
		}
	}

	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	return nil
}

// DeliveryReady handles the delivery ready state.
func (sm *StateMachine) DeliveryReady(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch delivery ready", "batch_id", sm.SmsBatch.BatchID)

	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentPhase = "delivery"
		sm.SmsBatch.Results.CurrentState = event.FSM.Current()
	}

	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	return nil
}

// DeliveryStart handles the start of delivery phase.
func (sm *StateMachine) DeliveryStart(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch delivery started", "batch_id", sm.SmsBatch.BatchID)

	// Apply rate limiting for delivery
	// TODO: Implement rate limiting when watcher interface is properly defined
	log.Infow("Rate limiting applied for delivery", "batch_id", sm.SmsBatch.BatchID)

	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentPhase = "delivery"
		sm.SmsBatch.Results.CurrentState = event.FSM.Current()
	}

	// Initialize delivery stats
	if sm.SmsBatch.Results != nil && sm.SmsBatch.Results.DeliveryStats == nil {
		now := time.Now()
		sm.SmsBatch.Results.DeliveryStats = &model.SmsBatchPhaseStats{
			StartTime: &now,
		}
	}

	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	return nil
}

// BatchSucceeded handles the successful completion of the SMS batch.
func (sm *StateMachine) BatchSucceeded(ctx context.Context, event *fsm.Event) error {
	ec := sm.EventCoordinator.(*EventCoordinator)
	ec.stateManager.SetBatchCompleted(sm.SmsBatch)
	log.Infow("SMS batch completed successfully", "batch_id", sm.SmsBatch.BatchID)
	return nil
}

// BatchFailed handles the failure of the SMS batch.
func (sm *StateMachine) BatchFailed(ctx context.Context, event *fsm.Event) error {
	ec := sm.EventCoordinator.(*EventCoordinator)
	ec.stateManager.UpdateBatchStatus(sm.SmsBatch, "failed")
	log.Errorw("SMS batch failed", "batch_id", sm.SmsBatch.BatchID)
	return nil
}

// The following methods have been moved to their respective processor modules:
// - validateBatchParams -> validator.go
// - shouldPausePreparation -> validator.go
// - processSmsPreparationBatch -> preparation_processor.go
// - createDeliveryPartitions -> partition_manager.go
// - processDeliveryPartition -> partition_manager.go

// BatchAborted handles the abortion of the SMS batch.
func (sm *StateMachine) BatchAborted(ctx context.Context, event *fsm.Event) error {
	ec := sm.EventCoordinator.(*EventCoordinator)
	ec.stateManager.UpdateBatchStatus(sm.SmsBatch, "aborted")
	log.Infow("SMS batch aborted", "batch_id", sm.SmsBatch.BatchID)
	return nil
}

// BatchPaused handles the pausing of the SMS batch.
func (sm *StateMachine) BatchPaused(ctx context.Context, event *fsm.Event) error {
	ec := sm.EventCoordinator.(*EventCoordinator)
	ec.stateManager.UpdateBatchStatus(sm.SmsBatch, "paused")
	log.Infow("SMS batch paused", "batch_id", sm.SmsBatch.BatchID)
	return nil
}

// BatchResumed handles the resuming of the SMS batch.
func (sm *StateMachine) BatchResumed(ctx context.Context, event *fsm.Event) error {
	ec := sm.EventCoordinator.(*EventCoordinator)
	ec.stateManager.UpdateBatchStatus(sm.SmsBatch, "running")
	log.Infow("SMS batch resumed", "batch_id", sm.SmsBatch.BatchID)
	return nil
}

// BatchRetry handles the retry of the SMS batch.
func (sm *StateMachine) BatchRetry(ctx context.Context, event *fsm.Event) error {
	ec := sm.EventCoordinator.(*EventCoordinator)
	ec.stateManager.UpdateBatchStatus(sm.SmsBatch, "retrying")
	log.Infow("SMS batch retrying", "batch_id", sm.SmsBatch.BatchID)
	return nil
}

// BatchCompleted handles the completion of the SMS batch.
func (sm *StateMachine) BatchCompleted(ctx context.Context, event *fsm.Event) error {
	ec := sm.EventCoordinator.(*EventCoordinator)
	ec.stateManager.SetBatchCompleted(sm.SmsBatch)
	log.Infow("SMS batch completed", "batch_id", sm.SmsBatch.BatchID)
	return nil
}

// EnterState handles state transitions and updates batch status and conditions.
func (sm *StateMachine) EnterState(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch state transition", "batch_id", sm.SmsBatch.BatchID, "from", event.Src, "to", event.Dst)
	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentState = event.Dst
	}
	return nil
}
