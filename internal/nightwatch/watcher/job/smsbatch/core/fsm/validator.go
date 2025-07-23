package fsm

import (
	"fmt"
	"github.com/ashwinyue/dcp/internal/nightwatch/model"
)

// Validator handles SMS batch parameter validation
type Validator struct{}

// NewValidator creates a new Validator instance
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateBatchParams validates SMS batch parameters
func (v *Validator) ValidateBatchParams(smsBatch *model.SmsBatchM) error {
	if smsBatch.BatchID == "" {
		return fmt.Errorf("batch ID is required")
	}
	if smsBatch.TableStorageName == "" {
		return fmt.Errorf("table storage name is required")
	}
	return nil
}
