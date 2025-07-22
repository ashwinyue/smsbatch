package smsbatch

import (
	"context"
	"fmt"
	"time"

	"github.com/looplab/fsm"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
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
	return nil
}

// EnterState handles the state transition of the state machine
// and updates the SmsBatch's status and conditions based on the event.
func (sm *StateMachine) EnterState(ctx context.Context, event *fsm.Event) error {
	sm.SmsBatch.Status = event.FSM.Current()

	// Record the start time of the SMS batch
	if sm.SmsBatch.Status == SmsBatchPreparationReady {
		sm.SmsBatch.StartedAt = time.Now()
	}

	// Unified handling logic for SMS batch failure
	if event.Err != nil || isSmsBatchTimeout(sm.SmsBatch) {
		sm.SmsBatch.Status = SmsBatchFailed
		sm.SmsBatch.EndedAt = time.Now()

		var cond *v1.JobCondition
		if isSmsBatchTimeout(sm.SmsBatch) {
			log.Infow("SMS batch task timeout")
			cond = jobconditionsutil.FalseCondition(event.FSM.Current(), "SMS batch task exceeded timeout seconds")
		} else {
			cond = jobconditionsutil.FalseCondition(event.FSM.Current(), event.Err.Error())
		}

		sm.SmsBatch.Conditions = (*model.SmsBatchConditions)(jobconditionsutil.Set((*model.JobConditions)(sm.SmsBatch.Conditions), cond))
	}

	// Record completion time for final states
	if sm.SmsBatch.Status == SmsBatchSucceeded || sm.SmsBatch.Status == SmsBatchFailed || sm.SmsBatch.Status == SmsBatchAborted {
		sm.SmsBatch.EndedAt = time.Now()
	}

	if err := sm.Watcher.Store.SmsBatch().Update(ctx, sm.SmsBatch); err != nil {
		return err
	}

	return nil
}
