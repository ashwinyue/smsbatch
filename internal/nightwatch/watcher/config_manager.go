package watcher

import (
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/pkg/client/minio"
)

// ConfigManager manages configuration for all watchers
type ConfigManager struct {
	mu                    sync.RWMutex
	aggregateConfig       *AggregateConfig
	retryConfigs          map[string]*RetryConfig
	monitorConfigs        map[string]*MonitorConfig
	circuitBreakerConfigs map[string]*CircuitBreakerConfig
}

// MonitorConfig defines monitoring configuration for watchers
type MonitorConfig struct {
	Enabled              bool          `json:"enabled" yaml:"enabled"`
	HealthCheckInterval  time.Duration `json:"health_check_interval" yaml:"health_check_interval"`
	MetricsRetention     time.Duration `json:"metrics_retention" yaml:"metrics_retention"`
	PerformanceThreshold struct {
		MaxExecutionTime time.Duration `json:"max_execution_time" yaml:"max_execution_time"`
		MaxMemoryUsage   int64         `json:"max_memory_usage" yaml:"max_memory_usage"`
		MaxErrorRate     float64       `json:"max_error_rate" yaml:"max_error_rate"`
	} `json:"performance_threshold" yaml:"performance_threshold"`
}

// CircuitBreakerConfig defines circuit breaker configuration
type CircuitBreakerConfig struct {
	Enabled          bool          `json:"enabled" yaml:"enabled"`
	FailureThreshold int           `json:"failure_threshold" yaml:"failure_threshold"`
	SuccessThreshold int           `json:"success_threshold" yaml:"success_threshold"`
	Timeout          time.Duration `json:"timeout" yaml:"timeout"`
	MaxRequests      int           `json:"max_requests" yaml:"max_requests"`
	ResetTimeout     time.Duration `json:"reset_timeout" yaml:"reset_timeout"`
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(aggregateConfig *AggregateConfig) *ConfigManager {
	return &ConfigManager{
		aggregateConfig:       aggregateConfig,
		retryConfigs:          make(map[string]*RetryConfig),
		monitorConfigs:        make(map[string]*MonitorConfig),
		circuitBreakerConfigs: make(map[string]*CircuitBreakerConfig),
	}
}

// GetAggregateConfig returns the aggregate configuration
func (cm *ConfigManager) GetAggregateConfig() *AggregateConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.aggregateConfig
}

// GetStore returns the store instance
func (cm *ConfigManager) GetStore() store.IStore {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if cm.aggregateConfig == nil {
		return nil
	}
	return cm.aggregateConfig.Store
}

// GetDB returns the database instance
func (cm *ConfigManager) GetDB() *gorm.DB {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if cm.aggregateConfig == nil {
		return nil
	}
	return cm.aggregateConfig.DB
}

// GetMinio returns the Minio client
func (cm *ConfigManager) GetMinio() minio.IMinio {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if cm.aggregateConfig == nil {
		return nil
	}
	return cm.aggregateConfig.Minio
}

// GetMaxWorkers returns the maximum number of workers
func (cm *ConfigManager) GetMaxWorkers() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if cm.aggregateConfig == nil {
		return 1
	}
	return int(cm.aggregateConfig.UserWatcherMaxWorkers)
}

// SetRetryConfig sets retry configuration for a specific watcher
func (cm *ConfigManager) SetRetryConfig(watcherName string, config *RetryConfig) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.retryConfigs[watcherName] = config
}

// GetRetryConfig returns retry configuration for a specific watcher
func (cm *ConfigManager) GetRetryConfig(watcherName string) *RetryConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if config, exists := cm.retryConfigs[watcherName]; exists {
		return config
	}
	return DefaultRetryConfig()
}

// SetMonitorConfig sets monitoring configuration for a specific watcher
func (cm *ConfigManager) SetMonitorConfig(watcherName string, config *MonitorConfig) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.monitorConfigs[watcherName] = config
}

// GetMonitorConfig returns monitoring configuration for a specific watcher
func (cm *ConfigManager) GetMonitorConfig(watcherName string) *MonitorConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if config, exists := cm.monitorConfigs[watcherName]; exists {
		return config
	}
	return DefaultMonitorConfig()
}

// SetCircuitBreakerConfig sets circuit breaker configuration for a specific watcher
func (cm *ConfigManager) SetCircuitBreakerConfig(watcherName string, config *CircuitBreakerConfig) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.circuitBreakerConfigs[watcherName] = config
}

// GetCircuitBreakerConfig returns circuit breaker configuration for a specific watcher
func (cm *ConfigManager) GetCircuitBreakerConfig(watcherName string) *CircuitBreakerConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if config, exists := cm.circuitBreakerConfigs[watcherName]; exists {
		return config
	}
	return DefaultCircuitBreakerConfig()
}

// LoadFromFile loads configuration from a file (placeholder for future implementation)
func (cm *ConfigManager) LoadFromFile(filePath string) error {
	// TODO: Implement configuration loading from file
	return fmt.Errorf("configuration loading from file not implemented yet")
}

// SaveToFile saves configuration to a file (placeholder for future implementation)
func (cm *ConfigManager) SaveToFile(filePath string) error {
	// TODO: Implement configuration saving to file
	return fmt.Errorf("configuration saving to file not implemented yet")
}

// ValidateConfig validates all configurations
func (cm *ConfigManager) ValidateConfig() error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.aggregateConfig == nil {
		return fmt.Errorf("aggregate configuration is nil")
	}

	if cm.aggregateConfig.Store == nil {
		return fmt.Errorf("store is not configured")
	}

	if cm.aggregateConfig.UserWatcherMaxWorkers <= 0 {
		return fmt.Errorf("invalid max workers configuration: %d", cm.aggregateConfig.UserWatcherMaxWorkers)
	}

	// Validate retry configurations
	for name, config := range cm.retryConfigs {
		if err := cm.validateRetryConfig(name, config); err != nil {
			return err
		}
	}

	// Validate monitor configurations
	for name, config := range cm.monitorConfigs {
		if err := cm.validateMonitorConfig(name, config); err != nil {
			return err
		}
	}

	// Validate circuit breaker configurations
	for name, config := range cm.circuitBreakerConfigs {
		if err := cm.validateCircuitBreakerConfig(name, config); err != nil {
			return err
		}
	}

	return nil
}

func (cm *ConfigManager) validateRetryConfig(name string, config *RetryConfig) error {
	if config.MaxRetries < 0 {
		return fmt.Errorf("invalid max retries for watcher %s: %d", name, config.MaxRetries)
	}
	if config.InitialDelay <= 0 {
		return fmt.Errorf("invalid initial delay for watcher %s: %v", name, config.InitialDelay)
	}
	if config.MaxDelay <= 0 {
		return fmt.Errorf("invalid max delay for watcher %s: %v", name, config.MaxDelay)
	}
	if config.BackoffFactor <= 1.0 {
		return fmt.Errorf("invalid backoff factor for watcher %s: %f", name, config.BackoffFactor)
	}
	return nil
}

func (cm *ConfigManager) validateMonitorConfig(name string, config *MonitorConfig) error {
	if config.HealthCheckInterval <= 0 {
		return fmt.Errorf("invalid health check interval for watcher %s: %v", name, config.HealthCheckInterval)
	}
	if config.MetricsRetention <= 0 {
		return fmt.Errorf("invalid metrics retention for watcher %s: %v", name, config.MetricsRetention)
	}
	if config.PerformanceThreshold.MaxErrorRate < 0 || config.PerformanceThreshold.MaxErrorRate > 1 {
		return fmt.Errorf("invalid max error rate for watcher %s: %f", name, config.PerformanceThreshold.MaxErrorRate)
	}
	return nil
}

func (cm *ConfigManager) validateCircuitBreakerConfig(name string, config *CircuitBreakerConfig) error {
	if config.FailureThreshold <= 0 {
		return fmt.Errorf("invalid failure threshold for watcher %s: %d", name, config.FailureThreshold)
	}
	if config.SuccessThreshold <= 0 {
		return fmt.Errorf("invalid success threshold for watcher %s: %d", name, config.SuccessThreshold)
	}
	if config.Timeout <= 0 {
		return fmt.Errorf("invalid timeout for watcher %s: %v", name, config.Timeout)
	}
	if config.MaxRequests <= 0 {
		return fmt.Errorf("invalid max requests for watcher %s: %d", name, config.MaxRequests)
	}
	return nil
}

// DefaultMonitorConfig returns default monitoring configuration
func DefaultMonitorConfig() *MonitorConfig {
	config := &MonitorConfig{
		Enabled:             true,
		HealthCheckInterval: 30 * time.Second,
		MetricsRetention:    24 * time.Hour,
	}
	config.PerformanceThreshold.MaxExecutionTime = 5 * time.Minute
	config.PerformanceThreshold.MaxMemoryUsage = 100 * 1024 * 1024 // 100MB
	config.PerformanceThreshold.MaxErrorRate = 0.1                 // 10%
	return config
}

// DefaultCircuitBreakerConfig returns default circuit breaker configuration
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
		MaxRequests:      10,
		ResetTimeout:     60 * time.Second,
	}
}
