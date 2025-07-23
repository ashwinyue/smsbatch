package smsbatch

import (
	"context"

	"github.com/gammazero/workerpool"
	"github.com/onexstack/onexstack/pkg/store/where"
	"github.com/onexstack/onexstack/pkg/watch/registry"
	"go.uber.org/ratelimit"

	"github.com/ashwinyue/dcp/internal/nightwatch/mqs"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher"
	"github.com/ashwinyue/dcp/internal/pkg/client/minio"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// Ensure Watcher implements the registry.Watcher interface.
var _ registry.Watcher = (*Watcher)(nil)

// Limiter holds rate limiters for different operations.
type Limiter struct {
	Preparation ratelimit.Limiter
	Delivery    ratelimit.Limiter
}

// Watcher monitors and processes SMS batch jobs.
type Watcher struct {
	Minio minio.IMinio
	Store store.IStore

	// Maximum number of concurrent workers.
	MaxWorkers int64
	// Rate limiters for operations.
	Limiter Limiter
	// Event publisher for batch events.
	EventPublisher *mqs.BatchEventPublisher
}

// Run executes the watcher logic to process jobs.
func (w *Watcher) Run() {
	// Define the phases that the watcher can handle.
	runablePhase := []string{
		SmsBatchInitial,
		SmsBatchPreparationReady,
		SmsBatchPreparationRunning,
		SmsBatchPreparationPaused,
		SmsBatchDeliveryReady,
		SmsBatchDeliveryRunning,
		SmsBatchDeliveryPaused,
	}

	_, smsBatches, err := w.Store.SmsBatch().List(context.Background(), where.F(
		"scope", SmsBatchJobScope,
		"watcher", SmsBatchWatcher,
		"current_state", runablePhase,
		"suspend", JobNonSuspended,
	))
	if err != nil {
		log.Errorw("Failed to get runnable SMS batches", "error", err)
		return
	}

	wp := workerpool.New(int(w.MaxWorkers))
	for _, smsBatch := range smsBatches {
		ctx := context.Background()
		log.Infow("Start to process SMS batch", "batch_id", smsBatch.BatchID, "current_state", smsBatch.CurrentState)

		wp.Submit(func() {
			sm := NewStateMachine(smsBatch.CurrentState, w, smsBatch)
			if err := sm.FSM.Event(ctx, smsBatch.CurrentState); err != nil {
				log.Errorw("Failed to process SMS batch", "batch_id", smsBatch.BatchID, "error", err)
				return
			}
		})
	}

	wp.StopWait()
}

// Spec returns the cron job specification for scheduling.
func (w *Watcher) Spec() string {
	return "@every 5s"
}

// GetStore returns the store instance
func (w *Watcher) GetStore() store.IStore {
	return w.Store
}

// SetAggregateConfig configures the watcher with the provided aggregate configuration.
func (w *Watcher) SetAggregateConfig(config *watcher.AggregateConfig) {
	w.Minio = config.Minio
	w.Store = config.Store
	w.Limiter = Limiter{
		Preparation: ratelimit.New(SmsBatchPreparationQPS),
		Delivery:    ratelimit.New(SmsBatchDeliveryQPS),
	}
}

// SetMaxWorkers sets the maximum number of concurrent workers for the watcher.
func (w *Watcher) SetMaxWorkers(maxWorkers int64) {
	// Use custom maxWorkers setting for SMS batch processing
	w.MaxWorkers = SmsBatchMaxWorkers
}

func init() {
	registry.Register(SmsBatchWatcher, &Watcher{})
}
