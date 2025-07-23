package watcher

import (
	"context"
	"sync"
	"time"

	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// HealthStatus represents the health status of a watcher
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// PerformanceMetrics holds performance metrics for a watcher
type PerformanceMetrics struct {
	mu sync.RWMutex

	// Execution metrics
	TotalRuns       int64
	SuccessfulRuns  int64
	FailedRuns      int64
	LastRunTime     time.Time
	LastSuccessTime time.Time
	LastFailureTime time.Time

	// Performance metrics
	AverageRunTime time.Duration
	MinRunTime     time.Duration
	MaxRunTime     time.Duration
	TotalRunTime   time.Duration

	// Error tracking
	ConsecutiveFailures int64
	ErrorRate           float64

	// Resource usage
	MemoryUsage int64
	CPUUsage    float64
}

// WatcherMonitor monitors watcher performance and health
type WatcherMonitor struct {
	mu      sync.RWMutex
	metrics map[string]*PerformanceMetrics

	// Health check configuration
	maxConsecutiveFailures int64
	maxErrorRate           float64
	maxRunTime             time.Duration
	healthCheckInterval    time.Duration

	// Callbacks
	onHealthChange func(watcherName string, oldStatus, newStatus HealthStatus)
}

// NewWatcherMonitor creates a new watcher monitor
func NewWatcherMonitor() *WatcherMonitor {
	return &WatcherMonitor{
		metrics:                make(map[string]*PerformanceMetrics),
		maxConsecutiveFailures: 5,
		maxErrorRate:           0.2, // 20%
		maxRunTime:             30 * time.Second,
		healthCheckInterval:    1 * time.Minute,
	}
}

// SetHealthChangeCallback sets a callback for health status changes
func (m *WatcherMonitor) SetHealthChangeCallback(callback func(string, HealthStatus, HealthStatus)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onHealthChange = callback
}

// RecordRun records a watcher run
func (m *WatcherMonitor) RecordRun(watcherName string, duration time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics := m.getOrCreateMetrics(watcherName)
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	oldStatus := m.calculateHealthStatus(metrics)

	metrics.TotalRuns++
	metrics.LastRunTime = time.Now()
	metrics.TotalRunTime += duration

	// Update run time statistics
	if metrics.MinRunTime == 0 || duration < metrics.MinRunTime {
		metrics.MinRunTime = duration
	}
	if duration > metrics.MaxRunTime {
		metrics.MaxRunTime = duration
	}
	metrics.AverageRunTime = time.Duration(int64(metrics.TotalRunTime) / metrics.TotalRuns)

	if err != nil {
		metrics.FailedRuns++
		metrics.ConsecutiveFailures++
		metrics.LastFailureTime = time.Now()
	} else {
		metrics.SuccessfulRuns++
		metrics.ConsecutiveFailures = 0
		metrics.LastSuccessTime = time.Now()
	}

	// Calculate error rate
	metrics.ErrorRate = float64(metrics.FailedRuns) / float64(metrics.TotalRuns)

	newStatus := m.calculateHealthStatus(metrics)
	if oldStatus != newStatus && m.onHealthChange != nil {
		go m.onHealthChange(watcherName, oldStatus, newStatus)
	}
}

// GetMetrics returns the metrics for a watcher
func (m *WatcherMonitor) GetMetrics(watcherName string) *PerformanceMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := m.metrics[watcherName]
	if metrics == nil {
		return nil
	}

	// Return a copy to avoid race conditions
	metrics.mu.RLock()
	defer metrics.mu.RUnlock()

	return &PerformanceMetrics{
		TotalRuns:           metrics.TotalRuns,
		SuccessfulRuns:      metrics.SuccessfulRuns,
		FailedRuns:          metrics.FailedRuns,
		LastRunTime:         metrics.LastRunTime,
		LastSuccessTime:     metrics.LastSuccessTime,
		LastFailureTime:     metrics.LastFailureTime,
		AverageRunTime:      metrics.AverageRunTime,
		MinRunTime:          metrics.MinRunTime,
		MaxRunTime:          metrics.MaxRunTime,
		TotalRunTime:        metrics.TotalRunTime,
		ConsecutiveFailures: metrics.ConsecutiveFailures,
		ErrorRate:           metrics.ErrorRate,
		MemoryUsage:         metrics.MemoryUsage,
		CPUUsage:            metrics.CPUUsage,
	}
}

// GetHealthStatus returns the health status of a watcher
func (m *WatcherMonitor) GetHealthStatus(watcherName string) HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := m.metrics[watcherName]
	if metrics == nil {
		return HealthStatusHealthy // New watcher, assume healthy
	}

	metrics.mu.RLock()
	defer metrics.mu.RUnlock()

	return m.calculateHealthStatus(metrics)
}

// GetAllMetrics returns metrics for all watchers
func (m *WatcherMonitor) GetAllMetrics() map[string]*PerformanceMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*PerformanceMetrics)
	for name := range m.metrics {
		result[name] = m.GetMetrics(name)
	}
	return result
}

// StartHealthCheck starts periodic health checks
func (m *WatcherMonitor) StartHealthCheck(ctx context.Context) {
	ticker := time.NewTicker(m.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.performHealthCheck()
		}
	}
}

func (m *WatcherMonitor) getOrCreateMetrics(watcherName string) *PerformanceMetrics {
	if metrics, exists := m.metrics[watcherName]; exists {
		return metrics
	}

	metrics := &PerformanceMetrics{}
	m.metrics[watcherName] = metrics
	return metrics
}

func (m *WatcherMonitor) calculateHealthStatus(metrics *PerformanceMetrics) HealthStatus {
	// Check for critical issues
	if metrics.ConsecutiveFailures >= m.maxConsecutiveFailures {
		return HealthStatusUnhealthy
	}

	// Check if watcher hasn't run recently (assuming it should run at least every 5 minutes)
	if !metrics.LastRunTime.IsZero() && time.Since(metrics.LastRunTime) > 5*time.Minute {
		return HealthStatusUnhealthy
	}

	// Check error rate
	if metrics.TotalRuns > 10 && metrics.ErrorRate > m.maxErrorRate {
		return HealthStatusDegraded
	}

	// Check average run time
	if metrics.AverageRunTime > m.maxRunTime {
		return HealthStatusDegraded
	}

	return HealthStatusHealthy
}

func (m *WatcherMonitor) performHealthCheck() {
	m.mu.RLock()
	watcherNames := make([]string, 0, len(m.metrics))
	for name := range m.metrics {
		watcherNames = append(watcherNames, name)
	}
	m.mu.RUnlock()

	for _, name := range watcherNames {
		status := m.GetHealthStatus(name)
		if status != HealthStatusHealthy {
			metrics := m.GetMetrics(name)
			log.Warnw("Watcher health check failed",
				"watcher", name,
				"status", status,
				"consecutive_failures", metrics.ConsecutiveFailures,
				"error_rate", metrics.ErrorRate,
				"last_run", metrics.LastRunTime,
				"avg_run_time", metrics.AverageRunTime,
			)
		}
	}
}
