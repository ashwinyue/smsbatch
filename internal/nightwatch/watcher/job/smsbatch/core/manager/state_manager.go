package manager

import (
	"context"
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// StateManager handles SMS batch state management and transitions
type StateManager struct {
	store store.IStore
}

// NewStateManager creates a new StateManager instance
func NewStateManager(store store.IStore) *StateManager {
	return &StateManager{
		store: store,
	}
}

// UpdateBatch updates the SMS batch in the store
func (stm *StateManager) UpdateBatch(ctx context.Context, batch *model.SmsBatchM) error {
	return stm.store.SmsBatch().Update(ctx, batch)
}

// UpdateBatchStatus updates the batch status
func (stm *StateManager) UpdateBatchStatus(batch *model.SmsBatchM, status string) {
	batch.Status = status
}

// InitializeBatchResults initializes SMS batch results if not already set
func (stm *StateManager) InitializeBatchResults(batch *model.SmsBatchM, phase string) {
	if batch.Results == nil {
		batch.Results = &model.SmsBatchResults{}
	}
	batch.Results.CurrentPhase = phase
}

// InitializePhaseStats initializes phase statistics
func (stm *StateManager) InitializePhaseStats(batch *model.SmsBatchM, phase string) {
	now := time.Now()
	switch phase {
	case "preparation":
		if batch.Results.PreparationStats == nil {
			batch.Results.PreparationStats = &model.SmsBatchPhaseStats{
				StartTime: &now,
			}
		}
	case "delivery":
		if batch.Results.DeliveryStats == nil {
			batch.Results.DeliveryStats = &model.SmsBatchPhaseStats{
				StartTime: &now,
			}
		}
	}
}

// FinalizePhaseStats finalizes phase statistics
func (stm *StateManager) FinalizePhaseStats(batch *model.SmsBatchM, phase string, processed, success, failed int64) {
	now := time.Now()
	switch phase {
	case "preparation":
		if batch.Results.PreparationStats != nil {
			batch.Results.PreparationStats.Processed = processed
			batch.Results.PreparationStats.Success = success
			batch.Results.PreparationStats.Failed = failed
			batch.Results.PreparationStats.EndTime = &now
			if batch.Results.PreparationStats.StartTime != nil {
				batch.Results.PreparationStats.DurationSeconds = int64(now.Sub(*batch.Results.PreparationStats.StartTime).Seconds())
			}
		}
	case "delivery":
		if batch.Results.DeliveryStats != nil {
			batch.Results.DeliveryStats.Processed = processed
			batch.Results.DeliveryStats.Success = success
			batch.Results.DeliveryStats.Failed = failed
			batch.Results.DeliveryStats.EndTime = &now
			if batch.Results.DeliveryStats.StartTime != nil {
				batch.Results.DeliveryStats.DurationSeconds = int64(now.Sub(*batch.Results.DeliveryStats.StartTime).Seconds())
			}
		}
	}
}

// UpdateBatchProgress updates batch progress and overall statistics
func (stm *StateManager) UpdateBatchProgress(batch *model.SmsBatchM, processed, success, failed int64, progress float64) {
	batch.Results.ProcessedMessages = processed
	batch.Results.SuccessMessages = success
	batch.Results.FailedMessages = failed
	batch.Results.ProgressPercent = progress
}

// SetBatchCompleted sets the batch as completed with final statistics
func (stm *StateManager) SetBatchCompleted(batch *model.SmsBatchM) {
	now := time.Now()
	batch.Status = "completed"
	batch.EndedAt = now

	if batch.Results != nil {
		batch.Results.CurrentPhase = "completed"
		batch.Results.ProgressPercent = 100.0

		// Calculate total duration
		if !batch.StartedAt.IsZero() {
			duration := now.Sub(batch.StartedAt)
			log.Infow("Batch completion duration", "batch_id", batch.BatchID, "duration", duration)
		}

		// Calculate success rate
		if batch.Results.ProcessedMessages > 0 {
			successRate := float64(batch.Results.SuccessMessages) / float64(batch.Results.ProcessedMessages) * 100.0
			log.Infow("Batch completion statistics",
				"batch_id", batch.BatchID,
				"total_processed", batch.Results.ProcessedMessages,
				"success_count", batch.Results.SuccessMessages,
				"failed_count", batch.Results.FailedMessages,
				"success_rate", fmt.Sprintf("%.2f%%", successRate))
		}
	}
}

// SetBatchStarted sets the batch start time
func (stm *StateManager) SetBatchStarted(batch *model.SmsBatchM) {
	batch.StartedAt = time.Now()
}

// SetBatchEnded sets the batch end time
func (stm *StateManager) SetBatchEnded(batch *model.SmsBatchM) {
	batch.EndedAt = time.Now()
}

// LogBatchEvent logs batch events
func (stm *StateManager) LogBatchEvent(batchID, event, message string) {
	log.Infow("SMS batch event", "batch_id", batchID, "event", event, "message", message)
}
