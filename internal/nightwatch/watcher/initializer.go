package watcher

import (
	"context"
	"sync"

	"github.com/onexstack/onexstack/pkg/watch/initializer"
	"github.com/onexstack/onexstack/pkg/watch/registry"

	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// WatcherInitializer is used for initialization of the onex specific watcher plugins.
type WatcherInitializer struct {
	*AggregateConfig
	monitor       *WatcherMonitor
	retryConfig   *RetryConfig
	circuitBreakers map[string]*CircuitBreaker
	mu            sync.RWMutex
}

// Ensure that WatcherInitializer implements the initializer.WatcherInitializer interface.
var _ initializer.WatcherInitializer = &WatcherInitializer{}

// NewInitializer creates and returns a new WatcherInitializer instance.
func NewInitializer(aggregate *AggregateConfig) *WatcherInitializer {
	monitor := NewWatcherMonitor()
	
	// Set up health change callback
	monitor.SetHealthChangeCallback(func(watcherName string, oldStatus, newStatus HealthStatus) {
		log.Warnw("Watcher health status changed",
			"watcher", watcherName,
			"old_status", oldStatus,
			"new_status", newStatus,
		)
	})

	return &WatcherInitializer{
		AggregateConfig: aggregate,
		monitor:         monitor,
		retryConfig:     DefaultRetryConfig(),
		circuitBreakers: make(map[string]*CircuitBreaker),
	}
}

// Initialize configures the provided watcher by injecting dependencies
// such as the Store and AggregateConfig when supported by the watcher.
func (w *WatcherInitializer) Initialize(wc registry.Watcher) {
	// Inject dependencies
	if wants, ok := wc.(WantsStore); ok {
		wants.SetStore(w.Store)
	}

	if wants, ok := wc.(WantsAggregateConfig); ok {
		wants.SetAggregateConfig(w.AggregateConfig)
	}

	if wants, ok := wc.(WantsDB); ok {
		wants.SetDB(w.DB)
	}

	// Set up monitoring and resilience features
	if enhanced, ok := wc.(WantsEnhancedFeatures); ok {
		enhanced.SetMonitor(w.monitor)
		enhanced.SetRetryConfig(w.retryConfig)
		
		// Create circuit breaker for this watcher
		watcherName := getWatcherName(wc)
		cb := NewCircuitBreaker(5, 30) // 5 failures, 30 second timeout
		w.mu.Lock()
		w.circuitBreakers[watcherName] = cb
		w.mu.Unlock()
		enhanced.SetCircuitBreaker(cb)
	}

	log.Infow("Watcher initialized", "watcher", getWatcherName(wc))
}

// GetMonitor returns the watcher monitor
func (w *WatcherInitializer) GetMonitor() *WatcherMonitor {
	return w.monitor
}

// StartHealthCheck starts the health check process
func (w *WatcherInitializer) StartHealthCheck(ctx context.Context) {
	w.monitor.StartHealthCheck(ctx)
}

// GetCircuitBreaker returns the circuit breaker for a watcher
func (w *WatcherInitializer) GetCircuitBreaker(watcherName string) *CircuitBreaker {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.circuitBreakers[watcherName]
}

func getWatcherName(wc registry.Watcher) string {
	// Try to get name from watcher if it implements a Name() method
	if named, ok := wc.(interface{ Name() string }); ok {
		return named.Name()
	}
	// Fallback to type name
	return "unknown"
}
