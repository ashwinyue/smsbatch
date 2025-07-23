package smsbatch

import (
	"context"
	"fmt"

	"github.com/gammazero/workerpool"
	"github.com/onexstack/onexstack/pkg/store/where"
	"github.com/onexstack/onexstack/pkg/watch/registry"
	"go.uber.org/ratelimit"

	"github.com/ashwinyue/dcp/internal/nightwatch/messaging/sender"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher"
	"github.com/ashwinyue/dcp/internal/pkg/client/minio"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// Ensure Watcher implements the registry.Watcher interface.
var _ registry.Watcher = (*Watcher)(nil)
var _ watcher.WantsAggregateConfig = (*Watcher)(nil)

// Limiter holds rate limiters for different operations.
type Limiter struct {
	Preparation ratelimit.Limiter
	Delivery    ratelimit.Limiter
}

// Watcher monitors and processes SMS batch jobs.
type Watcher struct {
	*watcher.BaseWatcher

	// Maximum number of concurrent workers.
	MaxWorkers int64
	// Rate limiters for operations.
	Limiter Limiter
	// Event publisher for batch events.
	EventPublisher *sender.BatchEventPublisher
}

// NewWatcher creates a new SMS batch watcher
func NewWatcher() *Watcher {
	return &Watcher{
		BaseWatcher: watcher.NewBaseWatcher(),
	}
}

// Run executes the watcher logic to process jobs.
func (w *Watcher) Run() {
	w.RunWithMetrics(context.Background(), w.runLogic)
}

// runLogic contains the actual watcher logic
func (w *Watcher) runLogic(ctx context.Context) error {
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

	store := w.GetStore()
	if store == nil {
		return fmt.Errorf("store not initialized")
	}

	_, smsBatches, err := store.SmsBatch().List(ctx, where.F(
		"scope", SmsBatchJobScope,
		"watcher", SmsBatchWatcher,
		"current_state", runablePhase,
		"suspend", JobNonSuspended,
	))
	if err != nil {
		return fmt.Errorf("failed to get runnable SMS batches: %w", err)
	}

	wp := workerpool.New(int(w.MaxWorkers))
	for _, smsBatch := range smsBatches {
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
	return nil
}

// Spec returns the cron job specification for scheduling.
func (w *Watcher) Spec() string {
	return "@every 5s"
}

// SetAggregateConfig configures the watcher with the provided aggregate configuration.
func (w *Watcher) SetAggregateConfig(config *watcher.AggregateConfig) {
	w.BaseWatcher.SetAggregateConfig(config)
	w.Limiter = Limiter{
		Preparation: ratelimit.New(SmsBatchPreparationQPS),
		Delivery:    ratelimit.New(SmsBatchDeliveryQPS),
	}
}

// GetMinio returns the minio client from config
func (w *Watcher) GetMinio() minio.IMinio {
	config := w.GetConfig()
	if config != nil {
		return config.Minio
	}
	return nil
}

// SetMaxWorkers sets the maximum number of concurrent workers for the watcher.
func (w *Watcher) SetMaxWorkers(maxWorkers int64) {
	// Use custom maxWorkers setting for SMS batch processing
	w.MaxWorkers = SmsBatchMaxWorkers
}

func init() {
	registry.Register(SmsBatchWatcher, NewWatcher())
}
