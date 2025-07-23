package watcher

import (
	"context"
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// RetryConfig defines retry behavior
type RetryConfig struct {
	MaxRetries    int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

// DefaultRetryConfig returns a sensible default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
	}
}

// RetryableError represents an error that can be retried
type RetryableError struct {
	Err       error
	Retryable bool
}

func (r *RetryableError) Error() string {
	return r.Err.Error()
}

func (r *RetryableError) Unwrap() error {
	return r.Err
}

// NewRetryableError creates a new retryable error
func NewRetryableError(err error, retryable bool) *RetryableError {
	return &RetryableError{
		Err:       err,
		Retryable: retryable,
	}
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if retryableErr, ok := err.(*RetryableError); ok {
		return retryableErr.Retryable
	}
	// Default: database errors and network errors are usually retryable
	// Application logic errors are usually not retryable
	return true
}

// RetryWithBackoff executes a function with exponential backoff retry
func RetryWithBackoff(ctx context.Context, config *RetryConfig, operation func() error) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}

			// Calculate next delay with exponential backoff
			delay = time.Duration(float64(delay) * config.BackoffFactor)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
		}

		err := operation()
		if err == nil {
			if attempt > 0 {
				log.Infow("Operation succeeded after retry", "attempts", attempt+1)
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryable(err) {
			log.Debugw("Error is not retryable, stopping", "error", err)
			return err
		}

		if attempt < config.MaxRetries {
			log.Debugw("Operation failed, will retry", "error", err, "attempt", attempt+1, "next_delay", delay)
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxRetries+1, lastErr)
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

// CircuitBreaker implements a simple circuit breaker pattern
type CircuitBreaker struct {
	state         CircuitBreakerState
	failureCount  int
	lastFailTime  time.Time
	successCount  int
	maxFailures   int
	timeout       time.Duration
	maxSuccesses  int
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:        CircuitClosed,
		maxFailures:  maxFailures,
		timeout:      timeout,
		maxSuccesses: 3, // Number of successes needed to close circuit
	}
}

// Execute runs the operation through the circuit breaker
func (cb *CircuitBreaker) Execute(operation func() error) error {
	if cb.state == CircuitOpen {
		if time.Since(cb.lastFailTime) > cb.timeout {
			cb.state = CircuitHalfOpen
			cb.successCount = 0
		} else {
			return fmt.Errorf("circuit breaker is open")
		}
	}

	err := operation()
	if err != nil {
		cb.onFailure()
		return err
	}

	cb.onSuccess()
	return nil
}

func (cb *CircuitBreaker) onSuccess() {
	if cb.state == CircuitHalfOpen {
		cb.successCount++
		if cb.successCount >= cb.maxSuccesses {
			cb.state = CircuitClosed
			cb.failureCount = 0
		}
	} else {
		cb.failureCount = 0
	}
}

func (cb *CircuitBreaker) onFailure() {
	cb.failureCount++
	cb.lastFailTime = time.Now()

	if cb.failureCount >= cb.maxFailures {
		cb.state = CircuitOpen
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	return cb.state
}