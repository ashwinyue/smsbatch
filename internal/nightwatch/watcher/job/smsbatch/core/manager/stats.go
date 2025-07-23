package manager

import (
	"sync/atomic"
)

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
