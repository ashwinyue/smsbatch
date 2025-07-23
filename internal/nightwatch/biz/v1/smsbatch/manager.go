package smsbatch

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/provider"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// PartitionManager handles SMS batch partition creation and processing
type PartitionManager struct {
	store           store.IStore
	providerFactory *provider.ProviderFactory
}

// NewPartitionManager creates a new partition manager
func NewPartitionManager(store store.IStore, providerFactory *provider.ProviderFactory) *PartitionManager {
	return &PartitionManager{
		store:           store,
		providerFactory: providerFactory,
	}
}

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

// Stats provides thread-safe statistics tracking for SMS batch processing
type Stats struct {
	processed int64
	success   int64
	failed    int64
}

// NewStats creates a new Stats instance
func NewStats() *Stats {
	return &Stats{}
}

// Add atomically adds values to the statistics
func (s *Stats) Add(processed, success, failed int64) {
	atomic.AddInt64(&s.processed, processed)
	atomic.AddInt64(&s.success, success)
	atomic.AddInt64(&s.failed, failed)
}

// Get returns current statistics values
func (s *Stats) Get() (processed, success, failed int64) {
	return atomic.LoadInt64(&s.processed), atomic.LoadInt64(&s.success), atomic.LoadInt64(&s.failed)
}

// Reset resets all statistics to zero
func (s *Stats) Reset() {
	atomic.StoreInt64(&s.processed, 0)
	atomic.StoreInt64(&s.success, 0)
	atomic.StoreInt64(&s.failed, 0)
}
