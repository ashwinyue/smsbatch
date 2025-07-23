package watcher

import (
	"context"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// BaseWatcher provides common functionality for all watchers
type BaseWatcher struct {
	mu     sync.RWMutex
	store  store.IStore
	db     *gorm.DB
	config *AggregateConfig

	// Metrics and monitoring
	lastRunTime time.Time
	runCount    int64
	errorCount  int64
}

// EnhancedWatcher extends BaseWatcher with monitoring and resilience features
type EnhancedWatcher struct {
	BaseWatcher
	monitor       *WatcherMonitor
	retryConfig   *RetryConfig
	circuitBreaker *CircuitBreaker
	watcherName   string
}

// BaseWatcher provides common functionality but doesn't implement the full Watcher interface
// Individual watchers should embed BaseWatcher and implement the Run() and Spec() methods

// NewEnhancedWatcher creates a new enhanced watcher
func NewEnhancedWatcher(name string) *EnhancedWatcher {
	return &EnhancedWatcher{
		watcherName: name,
	}
}

// SetMonitor implements WantsEnhancedFeatures interface
func (e *EnhancedWatcher) SetMonitor(monitor *WatcherMonitor) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.monitor = monitor
}

// SetRetryConfig implements WantsEnhancedFeatures interface
func (e *EnhancedWatcher) SetRetryConfig(config *RetryConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.retryConfig = config
}

// SetCircuitBreaker implements WantsEnhancedFeatures interface
func (e *EnhancedWatcher) SetCircuitBreaker(cb *CircuitBreaker) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.circuitBreaker = cb
}

// Name returns the watcher name
func (e *EnhancedWatcher) Name() string {
	return e.watcherName
}

// RunWithResilience executes the watcher run function with monitoring and resilience
func (e *EnhancedWatcher) RunWithResilience(ctx context.Context, runFunc func(context.Context) error) {
	start := time.Now()
	var err error

	// Execute with circuit breaker if available
	if e.circuitBreaker != nil {
		err = e.circuitBreaker.Execute(func() error {
			// Execute with retry if available
			if e.retryConfig != nil {
				return RetryWithBackoff(ctx, e.retryConfig, func() error {
					return runFunc(ctx)
				})
			}
			return runFunc(ctx)
		})
	} else {
		// Execute with retry if available
		if e.retryConfig != nil {
			err = RetryWithBackoff(ctx, e.retryConfig, func() error {
				return runFunc(ctx)
			})
		} else {
			err = runFunc(ctx)
		}
	}

	// Record metrics if monitor is available
	if e.monitor != nil {
		e.monitor.RecordRun(e.watcherName, time.Since(start), err)
	}

	// Update base metrics
	e.RunWithMetrics(ctx, func(context.Context) error {
		return err // Just return the error for base metrics
	})
}

// SetStore implements WantsStore interface
func (b *BaseWatcher) SetStore(store store.IStore) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.store = store
}

// SetDB implements WantsDB interface
func (b *BaseWatcher) SetDB(db *gorm.DB) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.db = db
}

// SetAggregateConfig implements WantsAggregateConfig interface
func (b *BaseWatcher) SetAggregateConfig(config *AggregateConfig) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.config = config
}

// GetStore returns the store instance safely
func (b *BaseWatcher) GetStore() store.IStore {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.store
}

// GetDB returns the database instance safely
func (b *BaseWatcher) GetDB() *gorm.DB {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.db
}

// GetConfig returns the aggregate config safely
func (b *BaseWatcher) GetConfig() *AggregateConfig {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.config
}

// RunWithMetrics wraps the actual run logic with metrics collection
func (b *BaseWatcher) RunWithMetrics(ctx context.Context, runFunc func(context.Context) error) {
	start := time.Now()
	defer func() {
		b.mu.Lock()
		b.lastRunTime = start
		b.runCount++
		b.mu.Unlock()
	}()

	if err := runFunc(ctx); err != nil {
		b.mu.Lock()
		b.errorCount++
		b.mu.Unlock()
		log.Errorw("Watcher run failed", "error", err, "duration", time.Since(start))
		return
	}

	log.Debugw("Watcher run completed", "duration", time.Since(start))
}

// GetMetrics returns watcher metrics
func (b *BaseWatcher) GetMetrics() WatcherMetrics {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return WatcherMetrics{
		LastRunTime: b.lastRunTime,
		RunCount:    b.runCount,
		ErrorCount:  b.errorCount,
	}
}

// WatcherMetrics holds metrics for a watcher
type WatcherMetrics struct {
	LastRunTime time.Time
	RunCount    int64
	ErrorCount  int64
}

// IsHealthy returns true if the watcher is considered healthy
func (m WatcherMetrics) IsHealthy() bool {
	if m.RunCount == 0 {
		return true // New watcher, considered healthy
	}
	errorRate := float64(m.ErrorCount) / float64(m.RunCount)
	return errorRate < 0.1 // Less than 10% error rate
}