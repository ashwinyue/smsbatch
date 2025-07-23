package smsbatch

import (
	"context"
	"fmt"

	corefsm "github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/fsm"
)

// Validator handles SMS batch parameter validation
type Validator struct{}

// NewValidator creates a new Validator instance
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateBatchParams validates SMS batch parameters
func (v *Validator) ValidateBatchParams(sm *corefsm.StateMachine) error {
	if sm.SmsBatch.BatchID == "" {
		return fmt.Errorf("batch ID is required")
	}
	if sm.SmsBatch.TableStorageName == "" {
		return fmt.Errorf("table storage name is required")
	}
	// Add more validation as needed
	return nil
}

// ShouldPausePreparation checks if preparation should be paused
func (v *Validator) ShouldPausePreparation(ctx context.Context, sm *corefsm.StateMachine) bool {
	// Check if delivery phase is running (similar to Java logic)
	if sm.SmsBatch.CurrentPhase == "delivery" {
		return true
	}
	// Check if batch is paused
	if sm.SmsBatch.Status == "paused" {
		return true
	}
	return false
}
