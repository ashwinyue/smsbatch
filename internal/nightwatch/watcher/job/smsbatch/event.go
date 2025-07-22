package smsbatch

import (
	"context"
	"fmt"
	"time"

	"github.com/looplab/fsm"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/mqs"
	"github.com/ashwinyue/dcp/internal/nightwatch/pkg/path"
	fakeminio "github.com/ashwinyue/dcp/internal/pkg/client/minio/fake"
	"github.com/ashwinyue/dcp/internal/pkg/log"
	jobconditionsutil "github.com/ashwinyue/dcp/internal/pkg/util/jobconditions"
	v1 "github.com/ashwinyue/dcp/pkg/api/nightwatch/v1"
)

// PreparationExecute handles the SMS preparation phase.
func (sm *StateMachine) PreparationExecute(ctx context.Context, event *fsm.Event) error {
	// Set default SMS batch params.
	SetDefaultSmsBatchParams(sm.SmsBatch)

	// Skip the preparation if the operation has already been performed (idempotency check)
	if ShouldSkipOnIdempotency(sm.SmsBatch, event.FSM.Current()) {
		return nil
	}

	time.Sleep(2 * time.Second)

	// Initialize SMS batch results if they are not already set
	if sm.SmsBatch.Results == nil {
		sm.SmsBatch.Results = &model.SmsBatchResults{}
	}

	// Initialize SMS batch phase stats
	if sm.SmsBatch.Results.PreparationStats == nil {
		sm.SmsBatch.Results.PreparationStats = &model.SmsBatchPhaseStats{}
	}

	// Simulate SMS preparation process
	// 1. Read SMS parameters from table storage
	data, err := sm.Watcher.Minio.Read(ctx, fakeminio.FakeObjectName)
	if err != nil {
		return err
	}

	// 2. Process and pack SMS data
	packedDataPath := path.Job.Path(sm.SmsBatch.BatchID, "sms_packed_data")
	if err := sm.Watcher.Minio.Write(ctx, packedDataPath, data); err != nil {
		return err
	}

	// Update SMS batch results
	sm.SmsBatch.Results.CurrentPhase = "preparation"

	// 3. Update statistics
	now := time.Now()
	sm.SmsBatch.Results.PreparationStats = &model.SmsBatchPhaseStats{
		Processed: int64(len(data)),
		Success:   int64(len(data)),
		Failed:    int64(0),
		StartTime: &now,
		EndTime:   &now,
	}
	sm.SmsBatch.Results.ProcessedMessages = int64(len(data))
	sm.SmsBatch.Results.SuccessMessages = int64(len(data))
	sm.SmsBatch.Results.FailedMessages = int64(0)

	sm.SmsBatch.Conditions = (*model.SmsBatchConditions)(jobconditionsutil.Set((*model.JobConditions)(sm.SmsBatch.Conditions), jobconditionsutil.TrueCondition(event.FSM.Current())))

	// Publish preparation completed event
	if sm.Watcher.EventPublisher != nil {
		if err := sm.Watcher.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
			RequestID:    fmt.Sprintf("%s-prep-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:      sm.SmsBatch.BatchID,
			Status:       event.FSM.Current(),
			CurrentPhase: "preparation",
			Progress:     100.0,
			Processed:    int64(len(data)),
			Success:      int64(len(data)),
			Failed:       0,
			UpdatedAt:    now,
		}); err != nil {
			log.Errorw("Failed to publish preparation completed event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	return nil
}

// DeliveryExecute handles the SMS delivery phase.
func (sm *StateMachine) DeliveryExecute(ctx context.Context, event *fsm.Event) error {
	if ShouldSkipOnIdempotency(sm.SmsBatch, event.FSM.Current()) {
		return nil
	}

	// Initialize SMS batch results if they are not already set
	if sm.SmsBatch.Results == nil {
		sm.SmsBatch.Results = &model.SmsBatchResults{}
	}

	// Initialize SMS batch delivery stats
	if sm.SmsBatch.Results.DeliveryStats == nil {
		sm.SmsBatch.Results.DeliveryStats = &model.SmsBatchPhaseStats{}
	}

	// For delivery, we simulate reading packed data
	// In real implementation, you would store the path in preparation phase
	packedDataPath := path.Job.Path(sm.SmsBatch.BatchID, "sms_packed_data")
	packedData, err := sm.Watcher.Minio.Read(ctx, packedDataPath)
	if err != nil {
		return err
	}

	// Simulate SMS delivery process
	// 1. Create delivery tasks for each partition
	deliveryResults := make([]string, len(packedData))
	for i, data := range packedData {
		// Simulate delivery to SMS provider
		deliveryResults[i] = fmt.Sprintf("delivered_%d: %s", i, data)
	}

	// 2. Save delivery results
	deliveryResultPath := path.Job.Path(sm.SmsBatch.BatchID, "sms_delivery_results")
	if err := sm.Watcher.Minio.Write(ctx, deliveryResultPath, deliveryResults); err != nil {
		return err
	}

	// Update SMS batch results
	sm.SmsBatch.Results.CurrentPhase = "delivery"

	// 3. Update delivery statistics
	now := time.Now()
	sm.SmsBatch.Results.DeliveryStats = &model.SmsBatchPhaseStats{
		Processed: int64(len(packedData)),
		Success:   int64(len(packedData)),
		Failed:    int64(0),
		StartTime: &now,
		EndTime:   &now,
	}

	// Update total message counts
	totalProcessed := int64(0)
	if sm.SmsBatch.Results.PreparationStats != nil {
		totalProcessed += sm.SmsBatch.Results.PreparationStats.Processed
	}
	totalProcessed += int64(len(packedData))
	sm.SmsBatch.Results.ProcessedMessages = totalProcessed
	sm.SmsBatch.Results.TotalMessages = totalProcessed

	sm.SmsBatch.Conditions = (*model.SmsBatchConditions)(jobconditionsutil.Set((*model.JobConditions)(sm.SmsBatch.Conditions), jobconditionsutil.TrueCondition(event.FSM.Current())))

	// Publish delivery completed event
	if sm.Watcher.EventPublisher != nil {
		if err := sm.Watcher.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
			RequestID:    fmt.Sprintf("%s-deliv-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:      sm.SmsBatch.BatchID,
			Status:       event.FSM.Current(),
			CurrentPhase: "delivery",
			Progress:     100.0,
			Processed:    int64(len(packedData)),
			Success:      int64(len(packedData)),
			Failed:       0,
			UpdatedAt:    now,
		}); err != nil {
			log.Errorw("Failed to publish delivery completed event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	return nil
}

// PreparationPause handles pausing the preparation phase.
func (sm *StateMachine) PreparationPause(ctx context.Context, event *fsm.Event) error {
	log.Infow("Pausing SMS preparation", "batch_id", sm.SmsBatch.BatchID)

	// Save current state and progress
	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentState = "paused"
	}

	sm.SmsBatch.Conditions = (*model.SmsBatchConditions)(jobconditionsutil.Set((*model.JobConditions)(sm.SmsBatch.Conditions), jobconditionsutil.TrueCondition(event.FSM.Current())))

	// Publish preparation pause event
	if sm.Watcher.EventPublisher != nil {
		now := time.Now()
		if err := sm.Watcher.EventPublisher.PublishBatchOperation(ctx, &mqs.BatchOperationRequest{
			RequestID:   fmt.Sprintf("%s-pause-prep-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:     sm.SmsBatch.BatchID,
			Operation:   "pause",
			Phase:       "preparation",
			Reason:      "Manual pause operation",
			OperatorID:  "system",
			RequestedAt: now,
		}); err != nil {
			log.Errorw("Failed to publish preparation pause event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	return nil
}

// DeliveryPause handles pausing the delivery phase.
func (sm *StateMachine) DeliveryPause(ctx context.Context, event *fsm.Event) error {
	log.Infow("Pausing SMS delivery", "batch_id", sm.SmsBatch.BatchID)

	// Save current state and progress
	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentState = "paused"
	}

	sm.SmsBatch.Conditions = (*model.SmsBatchConditions)(jobconditionsutil.Set((*model.JobConditions)(sm.SmsBatch.Conditions), jobconditionsutil.TrueCondition(event.FSM.Current())))

	// Publish delivery pause event
	if sm.Watcher.EventPublisher != nil {
		now := time.Now()
		if err := sm.Watcher.EventPublisher.PublishBatchOperation(ctx, &mqs.BatchOperationRequest{
			RequestID:   fmt.Sprintf("%s-pause-deliv-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:     sm.SmsBatch.BatchID,
			Operation:   "pause",
			Phase:       "delivery",
			Reason:      "Manual pause operation",
			OperatorID:  "system",
			RequestedAt: now,
		}); err != nil {
			log.Errorw("Failed to publish delivery pause event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	return nil
}

// PreparationResume handles resuming the preparation phase.
func (sm *StateMachine) PreparationResume(ctx context.Context, event *fsm.Event) error {
	log.Infow("Resuming SMS preparation", "batch_id", sm.SmsBatch.BatchID)

	// Clear paused state
	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentState = "running"
	}

	sm.SmsBatch.Conditions = (*model.SmsBatchConditions)(jobconditionsutil.Set((*model.JobConditions)(sm.SmsBatch.Conditions), jobconditionsutil.TrueCondition(event.FSM.Current())))

	// Publish preparation resume event
	if sm.Watcher.EventPublisher != nil {
		now := time.Now()
		if err := sm.Watcher.EventPublisher.PublishBatchOperation(ctx, &mqs.BatchOperationRequest{
			RequestID:   fmt.Sprintf("%s-resume-prep-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:     sm.SmsBatch.BatchID,
			Operation:   "resume",
			Phase:       "preparation",
			Reason:      "Manual resume operation",
			OperatorID:  "system",
			RequestedAt: now,
		}); err != nil {
			log.Errorw("Failed to publish preparation resume event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	return nil
}

// DeliveryResume handles resuming the delivery phase.
func (sm *StateMachine) DeliveryResume(ctx context.Context, event *fsm.Event) error {
	log.Infow("Resuming SMS delivery", "batch_id", sm.SmsBatch.BatchID)

	// Clear paused state
	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentState = "running"
	}

	sm.SmsBatch.Conditions = (*model.SmsBatchConditions)(jobconditionsutil.Set((*model.JobConditions)(sm.SmsBatch.Conditions), jobconditionsutil.TrueCondition(event.FSM.Current())))

	// Publish delivery resume event
	if sm.Watcher.EventPublisher != nil {
		now := time.Now()
		if err := sm.Watcher.EventPublisher.PublishBatchOperation(ctx, &mqs.BatchOperationRequest{
			RequestID:   fmt.Sprintf("%s-resume-deliv-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:     sm.SmsBatch.BatchID,
			Operation:   "resume",
			Phase:       "delivery",
			Reason:      "Manual resume operation",
			OperatorID:  "system",
			RequestedAt: now,
		}); err != nil {
			log.Errorw("Failed to publish delivery resume event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	return nil
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

	sm.SmsBatch.Conditions = (*model.SmsBatchConditions)(jobconditionsutil.Set((*model.JobConditions)(sm.SmsBatch.Conditions), jobconditionsutil.TrueCondition(event.FSM.Current())))

	return nil
}

// PreparationReady handles the preparation ready state.
func (sm *StateMachine) PreparationReady(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch preparation ready", "batch_id", sm.SmsBatch.BatchID)

	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentPhase = "preparation"
		sm.SmsBatch.Results.CurrentState = event.FSM.Current()
	}

	sm.SmsBatch.Conditions = (*model.SmsBatchConditions)(jobconditionsutil.Set((*model.JobConditions)(sm.SmsBatch.Conditions), jobconditionsutil.TrueCondition(event.FSM.Current())))

	return nil
}

// PreparationStart handles the start of preparation phase.
func (sm *StateMachine) PreparationStart(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch preparation started", "batch_id", sm.SmsBatch.BatchID)

	// Apply rate limiting for preparation
	if sm.Watcher.Limiter.Preparation != nil {
		sm.Watcher.Limiter.Preparation.Take()
	}

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

	sm.SmsBatch.Conditions = (*model.SmsBatchConditions)(jobconditionsutil.Set((*model.JobConditions)(sm.SmsBatch.Conditions), jobconditionsutil.TrueCondition(event.FSM.Current())))

	return nil
}

// DeliveryReady handles the delivery ready state.
func (sm *StateMachine) DeliveryReady(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch delivery ready", "batch_id", sm.SmsBatch.BatchID)

	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentPhase = "delivery"
		sm.SmsBatch.Results.CurrentState = event.FSM.Current()
	}

	sm.SmsBatch.Conditions = (*model.SmsBatchConditions)(jobconditionsutil.Set((*model.JobConditions)(sm.SmsBatch.Conditions), jobconditionsutil.TrueCondition(event.FSM.Current())))

	return nil
}

// DeliveryStart handles the start of delivery phase.
func (sm *StateMachine) DeliveryStart(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch delivery started", "batch_id", sm.SmsBatch.BatchID)

	// Apply rate limiting for delivery
	if sm.Watcher.Limiter.Delivery != nil {
		sm.Watcher.Limiter.Delivery.Take()
	}

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

	sm.SmsBatch.Conditions = (*model.SmsBatchConditions)(jobconditionsutil.Set((*model.JobConditions)(sm.SmsBatch.Conditions), jobconditionsutil.TrueCondition(event.FSM.Current())))

	return nil
}

// BatchSucceeded handles the successful completion of the SMS batch.
func (sm *StateMachine) BatchSucceeded(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch succeeded", "batch_id", sm.SmsBatch.BatchID)

	now := time.Now()
	sm.SmsBatch.EndedAt = now

	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentPhase = "completed"
		sm.SmsBatch.Results.CurrentState = event.FSM.Current()
		sm.SmsBatch.Results.ProgressPercent = 100.0
	}

	sm.SmsBatch.Conditions = (*model.SmsBatchConditions)(jobconditionsutil.Set((*model.JobConditions)(sm.SmsBatch.Conditions), jobconditionsutil.TrueCondition(event.FSM.Current())))

	// Publish batch success event
	if sm.Watcher.EventPublisher != nil {
		if err := sm.Watcher.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
			RequestID:    fmt.Sprintf("%s-success-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:      sm.SmsBatch.BatchID,
			Status:       SmsBatchSucceeded,
			CurrentPhase: "completed",
			Progress:     100.0,
			Processed:    sm.SmsBatch.Results.ProcessedMessages,
			Success:      sm.SmsBatch.Results.SuccessMessages,
			Failed:       sm.SmsBatch.Results.FailedMessages,
			UpdatedAt:    now,
		}); err != nil {
			log.Errorw("Failed to publish batch success event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	return nil
}

// BatchFailed handles the failure of the SMS batch.
func (sm *StateMachine) BatchFailed(ctx context.Context, event *fsm.Event) error {
	log.Errorw("SMS batch failed", "batch_id", sm.SmsBatch.BatchID, "error", event.Err)

	now := time.Now()
	sm.SmsBatch.EndedAt = now

	errorMessage := ""
	if event.Err != nil {
		errorMessage = event.Err.Error()
	}

	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentPhase = "failed"
		sm.SmsBatch.Results.CurrentState = event.FSM.Current()
		sm.SmsBatch.Results.ErrorMessage = errorMessage
	}

	cond := jobconditionsutil.FalseCondition(event.FSM.Current(), errorMessage)
	sm.SmsBatch.Conditions = (*model.SmsBatchConditions)(jobconditionsutil.Set((*model.JobConditions)(sm.SmsBatch.Conditions), cond))

	// Publish batch failure event
	if sm.Watcher.EventPublisher != nil {
		if err := sm.Watcher.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
			RequestID:    fmt.Sprintf("%s-failed-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:      sm.SmsBatch.BatchID,
			Status:       SmsBatchFailed,
			CurrentPhase: "failed",
			Progress:     0.0,
			Processed:    sm.SmsBatch.Results.ProcessedMessages,
			Success:      sm.SmsBatch.Results.SuccessMessages,
			Failed:       sm.SmsBatch.Results.FailedMessages,
			ErrorMessage: errorMessage,
			UpdatedAt:    now,
		}); err != nil {
			log.Errorw("Failed to publish batch failure event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	return nil
}

// BatchAborted handles the abortion of the SMS batch.
func (sm *StateMachine) BatchAborted(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch aborted", "batch_id", sm.SmsBatch.BatchID)

	now := time.Now()
	sm.SmsBatch.EndedAt = now

	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentPhase = "aborted"
		sm.SmsBatch.Results.CurrentState = event.FSM.Current()
	}

	sm.SmsBatch.Conditions = (*model.SmsBatchConditions)(jobconditionsutil.Set((*model.JobConditions)(sm.SmsBatch.Conditions), jobconditionsutil.TrueCondition(event.FSM.Current())))

	// Publish batch abort event
	if sm.Watcher.EventPublisher != nil {
		if err := sm.Watcher.EventPublisher.PublishBatchOperation(ctx, &mqs.BatchOperationRequest{
			RequestID:   fmt.Sprintf("%s-abort-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:     sm.SmsBatch.BatchID,
			Operation:   "abort",
			Reason:      "Batch processing aborted",
			OperatorID:  "system",
			RequestedAt: now,
		}); err != nil {
			log.Errorw("Failed to publish batch abort event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	return nil
}

// EnterState handles the state transition of the state machine
// and updates the SmsBatch's status and conditions based on the event.
func (sm *StateMachine) EnterState(ctx context.Context, event *fsm.Event) error {
	sm.SmsBatch.Status = event.FSM.Current()
	now := time.Now()

	// Record the start time of the SMS batch
	if sm.SmsBatch.Status == SmsBatchPreparationReady {
		sm.SmsBatch.StartedAt = now
	}

	// Unified handling logic for SMS batch failure
	if event.Err != nil || isSmsBatchTimeout(sm.SmsBatch) {
		sm.SmsBatch.Status = SmsBatchFailed
		sm.SmsBatch.EndedAt = now

		var cond *v1.JobCondition
		errorMessage := ""
		if isSmsBatchTimeout(sm.SmsBatch) {
			log.Infow("SMS batch task timeout")
			errorMessage = "SMS batch task exceeded timeout seconds"
			cond = jobconditionsutil.FalseCondition(event.FSM.Current(), errorMessage)
		} else {
			errorMessage = event.Err.Error()
			cond = jobconditionsutil.FalseCondition(event.FSM.Current(), errorMessage)
		}

		sm.SmsBatch.Conditions = (*model.SmsBatchConditions)(jobconditionsutil.Set((*model.JobConditions)(sm.SmsBatch.Conditions), cond))

		// Publish batch failure event
		if sm.Watcher.EventPublisher != nil {
			if err := sm.Watcher.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
				RequestID:    fmt.Sprintf("%s-failed-%d", sm.SmsBatch.BatchID, now.Unix()),
				BatchID:      sm.SmsBatch.BatchID,
				Status:       SmsBatchFailed,
				CurrentPhase: sm.SmsBatch.Results.CurrentPhase,
				Progress:     0.0,
				Processed:    sm.SmsBatch.Results.ProcessedMessages,
				Success:      sm.SmsBatch.Results.SuccessMessages,
				Failed:       sm.SmsBatch.Results.FailedMessages,
				ErrorMessage: errorMessage,
				UpdatedAt:    now,
			}); err != nil {
				log.Errorw("Failed to publish batch failure event", "batch_id", sm.SmsBatch.BatchID, "error", err)
			}
		}
	}

	// Record completion time for final states
	if sm.SmsBatch.Status == SmsBatchSucceeded || sm.SmsBatch.Status == SmsBatchFailed || sm.SmsBatch.Status == SmsBatchAborted {
		sm.SmsBatch.EndedAt = now

		// Publish batch completion event for successful batches
		if sm.SmsBatch.Status == SmsBatchSucceeded && sm.Watcher.EventPublisher != nil {
			if err := sm.Watcher.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
				RequestID:    fmt.Sprintf("%s-success-%d", sm.SmsBatch.BatchID, now.Unix()),
				BatchID:      sm.SmsBatch.BatchID,
				Status:       SmsBatchSucceeded,
				CurrentPhase: "completed",
				Progress:     100.0,
				Processed:    sm.SmsBatch.Results.ProcessedMessages,
				Success:      sm.SmsBatch.Results.SuccessMessages,
				Failed:       sm.SmsBatch.Results.FailedMessages,
				UpdatedAt:    now,
			}); err != nil {
				log.Errorw("Failed to publish batch success event", "batch_id", sm.SmsBatch.BatchID, "error", err)
			}
		}

		// Publish batch abort event
		if sm.SmsBatch.Status == SmsBatchAborted && sm.Watcher.EventPublisher != nil {
			if err := sm.Watcher.EventPublisher.PublishBatchOperation(ctx, &mqs.BatchOperationRequest{
				RequestID:   fmt.Sprintf("%s-abort-%d", sm.SmsBatch.BatchID, now.Unix()),
				BatchID:     sm.SmsBatch.BatchID,
				Operation:   "abort",
				Reason:      "Batch processing aborted",
				OperatorID:  "system",
				RequestedAt: now,
			}); err != nil {
				log.Errorw("Failed to publish batch abort event", "batch_id", sm.SmsBatch.BatchID, "error", err)
			}
		}
	}

	if err := sm.Watcher.Store.SmsBatch().Update(ctx, sm.SmsBatch); err != nil {
		return err
	}

	return nil
}
