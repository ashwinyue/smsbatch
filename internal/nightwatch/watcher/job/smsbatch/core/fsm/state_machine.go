package fsm

import (
	"context"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/service"
	"github.com/ashwinyue/dcp/internal/pkg/known/smsbatch"
	"github.com/ashwinyue/dcp/internal/pkg/log"
	jobconditionsutil "github.com/ashwinyue/dcp/internal/pkg/util/jobconditions"
	"github.com/looplab/fsm"
)

// StateMachine represents the state machine for SMS batch processing
type StateMachine struct {
	FSM              *fsm.FSM
	SmsBatch         *model.SmsBatchM
	Watcher          interface{}
	EventCoordinator EventCoordinatorInterface
}

// NewStateMachine creates a new StateMachine instance
func NewStateMachine(smsBatch *model.SmsBatchM, watcher interface{}, tableStorageService service.TableStorageService) *StateMachine {
	// Create a basic EventCoordinator
	// Full dependency injection with all processors is handled at the core package level
	eventCoordinator := &EventCoordinator{}

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

// PreparationCompleted handles the completion of SMS preparation phase.
func (sm *StateMachine) PreparationCompleted(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch preparation completed", "batch_id", sm.SmsBatch.BatchID)

	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentPhase = "preparation"
		sm.SmsBatch.Results.CurrentState = event.FSM.Current()

		// Finalize preparation stats
		if sm.SmsBatch.Results.PreparationStats != nil {
			now := time.Now()
			sm.SmsBatch.Results.PreparationStats.EndTime = &now
			if sm.SmsBatch.Results.PreparationStats.StartTime != nil {
				sm.SmsBatch.Results.PreparationStats.DurationSeconds = int64(now.Sub(*sm.SmsBatch.Results.PreparationStats.StartTime).Seconds())
			}
		}
	}

	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))
	return nil
}

// DeliveryCompleted handles the completion of SMS delivery phase.
func (sm *StateMachine) DeliveryCompleted(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch delivery completed", "batch_id", sm.SmsBatch.BatchID)

	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentPhase = "delivery"
		sm.SmsBatch.Results.CurrentState = event.FSM.Current()

		// Finalize delivery stats
		if sm.SmsBatch.Results.DeliveryStats != nil {
			now := time.Now()
			sm.SmsBatch.Results.DeliveryStats.EndTime = &now
			if sm.SmsBatch.Results.DeliveryStats.StartTime != nil {
				sm.SmsBatch.Results.DeliveryStats.DurationSeconds = int64(now.Sub(*sm.SmsBatch.Results.DeliveryStats.StartTime).Seconds())
			}
		}
	}

	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))
	return nil
}

// PreparationPause handles pausing the preparation phase.
func (sm *StateMachine) PreparationPause(ctx context.Context, event *fsm.Event) error {
	ec := sm.EventCoordinator.(*EventCoordinator)
	ec.stateManager.UpdateBatchStatus(sm.SmsBatch, smsbatch.BatchStatusPaused)
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
	ec.stateManager.UpdateBatchStatus(sm.SmsBatch, smsbatch.BatchStatusRunning)
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

// InitialExecute handles the initial state of the SMS batch.
func (sm *StateMachine) InitialExecute(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch initialized", "batch_id", sm.SmsBatch.BatchID)

	// Set default SMS batch params
	setDefaultSmsBatchParams(sm.SmsBatch)

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
	if err := sm.applyRateLimit(ctx); err != nil {
		log.Errorw("Failed to apply rate limit", "batch_id", sm.SmsBatch.BatchID, "error", err)
		return err
	}
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
	if err := sm.applyRateLimit(ctx); err != nil {
		log.Errorw("Failed to apply rate limit", "batch_id", sm.SmsBatch.BatchID, "error", err)
		return err
	}
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
	ec.stateManager.UpdateBatchStatus(sm.SmsBatch, smsbatch.BatchStatusFailed)
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
	ec.stateManager.UpdateBatchStatus(sm.SmsBatch, smsbatch.BatchStatusAborted)
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
	ec.stateManager.UpdateBatchStatus(sm.SmsBatch, smsbatch.BatchStatusRetrying)
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

// applyRateLimit applies rate limiting for the SMS batch
func (sm *StateMachine) applyRateLimit(ctx context.Context) error {
	ec := sm.EventCoordinator.(*EventCoordinator)
	return ec.rateLimiter.ApplyRateLimit(ctx, sm.SmsBatch)
}

// setDefaultSmsBatchParams sets default parameters for the SMS batch if they are not already set
func setDefaultSmsBatchParams(smsBatch *model.SmsBatchM) {
	if smsBatch.Params == nil {
		smsBatch.Params = &model.SmsBatchParams{}
	}

	if smsBatch.Params.BatchSize == 0 {
		smsBatch.Params.BatchSize = 1000
	}

	if smsBatch.Params.PartitionCount == 0 {
		smsBatch.Params.PartitionCount = 4
	}

	if smsBatch.Params.JobTimeout == 0 {
		smsBatch.Params.JobTimeout = 3600
	}

	if smsBatch.Params.MaxRetries == 0 {
		smsBatch.Params.MaxRetries = 3
	}

	if !smsBatch.Params.IdempotentExecution {
		smsBatch.Params.IdempotentExecution = true
	}

	if smsBatch.Params.PreparationConfig == nil {
		smsBatch.Params.PreparationConfig = map[string]interface{}{
			"pack_size":            1000,
			"max_concurrent_packs": 4,
			"enable_validation":    true,
			"storage_timeout":      300,
		}
	}

	if smsBatch.Params.DeliveryConfig == nil {
		smsBatch.Params.DeliveryConfig = map[string]interface{}{
			"max_concurrent_partitions": 4,
			"delivery_timeout":          600,
			"retry_delay_seconds":       30,
			"enable_delivery_tracking":  true,
		}
	}
}
