package smsbatch

import (
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	jobconditionsutil "github.com/ashwinyue/dcp/internal/pkg/util/jobconditions"
)

// SMS Batch State Constants
const (
	// Initial state
	SmsBatchInitial = "sms_batch_initial"

	// Preparation states
	SmsBatchPreparationReady     = "sms_batch_preparation_ready"
	SmsBatchPreparationRunning   = "sms_batch_preparation_running"
	SmsBatchPreparationCompleted = "sms_batch_preparation_completed"
	SmsBatchPreparationPaused    = "sms_batch_preparation_paused"

	// Delivery states
	SmsBatchDeliveryReady     = "sms_batch_delivery_ready"
	SmsBatchDeliveryRunning   = "sms_batch_delivery_running"
	SmsBatchDeliveryCompleted = "sms_batch_delivery_completed"
	SmsBatchDeliveryPaused    = "sms_batch_delivery_paused"

	// Final states
	SmsBatchSucceeded = "sms_batch_succeeded"
	SmsBatchFailed    = "sms_batch_failed"
	SmsBatchAborted   = "sms_batch_aborted"

	// Events
	SmsBatchPausePreparation  = "pause_preparation"
	SmsBatchResumePreparation = "resume_preparation"
	SmsBatchPauseDelivery     = "pause_delivery"
	SmsBatchResumeDelivery    = "resume_delivery"
	SmsBatchRetryPreparation  = "retry_preparation"
	SmsBatchRetryDelivery     = "retry_delivery"
	SmsBatchPreparationFailed = "preparation_failed"
	SmsBatchDeliveryFailed    = "delivery_failed"
	SmsBatchAbort             = "abort"

	// Configuration constants
	SmsBatchJobScope       = "sms_batch"
	SmsBatchWatcher        = "sms_batch_watcher"
	SmsBatchTimeout        = 3600 // 1 hour in seconds
	SmsBatchPreparationQPS = 10
	SmsBatchDeliveryQPS    = 20
	SmsBatchMaxWorkers     = 5
	JobNonSuspended        = false
	IdempotentExecution    = "idempotent"
)

// Note: We now use v1.MessageBatchResults and v1.MessageBatchPhaseStats
// instead of custom structures for better integration with the existing system

// isSmsBatchTimeout checks if the SMS batch has exceeded its allowed execution time.
func isSmsBatchTimeout(smsBatch *model.SmsBatchM) bool {
	duration := time.Now().Unix() - smsBatch.StartedAt.Unix()
	timeout := getSmsBatchTimeout(smsBatch)

	return duration > timeout
}

// getSmsBatchTimeout returns the timeout value for the SMS batch.
func getSmsBatchTimeout(smsBatch *model.SmsBatchM) int64 {
	// Try to get timeout from SMS batch params if available
	if smsBatch.Params != nil {
		// Assuming SMS batch params might have a timeout field
		// This would need to be adjusted based on actual model structure
		return SmsBatchTimeout
	}
	return SmsBatchTimeout
}

// ShouldSkipOnIdempotency determines whether an SMS batch should skip execution based on idempotency conditions.
func ShouldSkipOnIdempotency(smsBatch *model.SmsBatchM, condType string) bool {
	// If idempotent execution is not set, allow execution regardless of conditions.
	if !isIdempotentExecution(smsBatch) {
		return false
	}

	return jobconditionsutil.IsTrue((*model.JobConditions)(smsBatch.Conditions), condType)
}

// isIdempotentExecution checks if the SMS batch is configured for idempotent execution.
func isIdempotentExecution(smsBatch *model.SmsBatchM) bool {
	// This would need to be adjusted based on actual SMS batch params structure
	// For now, assume all SMS batches are idempotent
	return true
}

// SetDefaultSmsBatchParams sets default parameters for the SMS batch if they are not already set.
func SetDefaultSmsBatchParams(smsBatch *model.SmsBatchM) {
	// Set default timeout if not specified
	// This would need to be adjusted based on actual SMS batch params structure
	if smsBatch.Params == nil {
		// Initialize params if needed
		// smsBatch.Params = &model.SmsBatchParams{}
	}

	// Set other default parameters as needed
}

// Legacy functions for backward compatibility
// isJobTimeout checks if the job has exceeded its allowed execution time.
func isJobTimeout(job *model.JobM) bool {
	duration := time.Now().Unix() - job.StartedAt.Unix()
	timeout := getJobTimeout(job)

	return duration > timeout
}

// getJobTimeout returns the timeout value for the job.
func getJobTimeout(job *model.JobM) int64 {
	// Try to get timeout from job params if available
	if job.Params != nil {
		// Assuming job params might have a timeout field
		// This would need to be adjusted based on actual model structure
		return SmsBatchTimeout
	}
	return SmsBatchTimeout
}

// GetCurrentStep returns the current step based on the job status.
func GetCurrentStep(status string) string {
	switch {
	case status == SmsBatchInitial ||
		status == SmsBatchPreparationReady ||
		status == SmsBatchPreparationRunning ||
		status == SmsBatchPreparationCompleted ||
		status == SmsBatchPreparationPaused:
		return "SMS_PREPARATION"
	case status == SmsBatchDeliveryReady ||
		status == SmsBatchDeliveryRunning ||
		status == SmsBatchDeliveryCompleted ||
		status == SmsBatchDeliveryPaused:
		return "SMS_DELIVERY"
	default:
		return "UNKNOWN"
	}
}

// IsRunningState checks if the given status represents a running state.
func IsRunningState(status string) bool {
	return status == SmsBatchPreparationRunning || status == SmsBatchDeliveryRunning
}

// IsPausedState checks if the given status represents a paused state.
func IsPausedState(status string) bool {
	return status == SmsBatchPreparationPaused || status == SmsBatchDeliveryPaused
}

// IsFinalState checks if the given status represents a final state.
func IsFinalState(status string) bool {
	return status == SmsBatchSucceeded || status == SmsBatchFailed || status == SmsBatchAborted
}
