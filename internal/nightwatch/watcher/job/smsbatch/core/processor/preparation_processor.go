package processor

import (
	"context"
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/manager"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/publisher"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// PreparationProcessor handles SMS batch preparation phase processing
type PreparationProcessor struct {
	eventPublisher   *publisher.EventPublisher
	partitionManager *manager.PartitionManager
}

// NewPreparationProcessor creates a new PreparationProcessor instance
func NewPreparationProcessor(eventPublisher *publisher.EventPublisher, partitionManager *manager.PartitionManager) *PreparationProcessor {
	return &PreparationProcessor{
		eventPublisher:   eventPublisher,
		partitionManager: partitionManager,
	}
}

// Execute handles the SMS preparation phase
func (pp *PreparationProcessor) Execute(ctx context.Context, sm *model.SmsBatchM) error {
	// Set default SMS batch params
	pp.setDefaultSmsBatchParams(sm)

	// Skip the preparation if the operation has already been performed (idempotency check)
	if pp.shouldSkipOnIdempotency(sm, "preparation") {
		return nil
	}

	// Initialize SMS batch results if they are not already set
	if sm.Results == nil {
		sm.Results = &model.SmsBatchResults{}
	}

	// Initialize SMS batch phase stats
	if sm.Results.PreparationStats == nil {
		sm.Results.PreparationStats = &model.SmsBatchPhaseStats{}
	}

	// Basic validation of batch parameters
	if sm == nil {
		return fmt.Errorf("SMS batch is nil")
	}
	if sm.BatchID == "" {
		return fmt.Errorf("batch ID is empty")
	}

	// Start preparation phase
	startTime := time.Now()
	sm.Results.PreparationStats.StartTime = &startTime
	sm.Results.CurrentPhase = "preparation"

	// 1. Read SMS parameters from table storage
	log.Infow("Reading SMS parameters from storage", "batch_id", sm.BatchID)
	// TODO: Implement actual data reading from table storage
	data := []string{"sms1", "sms2", "sms3", "sms4", "sms5"} // Simulated data

	// 2. Process SMS data in batches (similar to Java pack size logic)
	batchSize := 1000 // Default batch size
	totalProcessed := int64(0)
	totalSuccess := int64(0)
	totalFailed := int64(0)

	// Process data in chunks
	for i := 0; i < len(data); i += batchSize {
		end := i + batchSize
		if end > len(data) {
			end = len(data)
		}

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			log.Infow("Preparation cancelled", "batch_id", sm.BatchID)
			return ctx.Err()
		default:
		}

		// Process current batch
		batchData := data[i:end]
		processedCount, successCount, failedCount := pp.ProcessSmsPreparationBatch(ctx, batchData)

		totalProcessed += int64(processedCount)
		totalSuccess += int64(successCount)
		totalFailed += int64(failedCount)

		// Update progress
		progress := float64(end) / float64(len(data)) * 100.0
		sm.Results.ProgressPercent = progress

		// Publish progress update
		if err := pp.eventPublisher.PublishPreparationProgress(ctx, sm.BatchID, sm.Status, totalProcessed, totalSuccess, totalFailed, progress); err != nil {
			log.Errorw("Failed to publish progress update", "batch_id", sm.BatchID, "error", err)
		}
	}

	// 3. Pack and save SMS data
	log.Infow("Saving packed data", "batch_id", sm.BatchID, "data_count", len(data))
	// TODO: Implement actual data saving to storage

	// 4. Update final statistics
	now := time.Now()
	sm.Results.PreparationStats = &model.SmsBatchPhaseStats{
		Processed: totalProcessed,
		Success:   totalSuccess,
		Failed:    totalFailed,
		StartTime: &startTime,
		EndTime:   &now,
	}
	sm.Results.ProcessedMessages = totalProcessed
	sm.Results.SuccessMessages = totalSuccess
	sm.Results.FailedMessages = totalFailed
	sm.Results.ProgressPercent = 100.0

	// Publish preparation completed event
	if err := pp.eventPublisher.PublishPreparationCompleted(ctx, sm.BatchID, sm.Status, totalProcessed, totalSuccess, totalFailed); err != nil {
		log.Errorw("Failed to publish preparation completed event", "batch_id", sm.BatchID, "error", err)
	}

	log.Infow("SMS batch preparation completed successfully",
		"batch_id", sm.BatchID,
		"processed", totalProcessed,
		"success", totalSuccess,
		"failed", totalFailed)

	return nil
}

// setDefaultSmsBatchParams sets default parameters for SMS batch
func (pp *PreparationProcessor) setDefaultSmsBatchParams(sm *model.SmsBatchM) {
	if sm.Params == nil {
		sm.Params = &model.SmsBatchParams{}
	}

	if sm.Params.BatchSize == 0 {
		sm.Params.BatchSize = 1000
	}

	if sm.Params.PartitionCount == 0 {
		sm.Params.PartitionCount = 10
	}

	if sm.Params.JobTimeout == 0 {
		sm.Params.JobTimeout = 3600 // 1 hour
	}

	if sm.Params.MaxRetries == 0 {
		sm.Params.MaxRetries = 3
	}
}

// shouldSkipOnIdempotency determines whether an SMS batch should skip execution based on idempotency conditions.
func (pp *PreparationProcessor) shouldSkipOnIdempotency(smsBatch *model.SmsBatchM, condType string) bool {
	// If idempotent execution is not set, allow execution regardless of conditions.
	if !pp.isIdempotentExecution(smsBatch) {
		return false
	}

	// Check if the condition has already been satisfied
	if smsBatch.Conditions == nil {
		return false
	}

	// Convert conditions to map for easier checking
	conditions := (*model.JobConditions)(smsBatch.Conditions)
	if conditions == nil {
		return false
	}

	// Check specific condition types
	switch condType {
	case "preparation":
		// Skip if preparation has already been executed successfully
		if pp.isConditionTrue(conditions, "preparation_execute") {
			return true
		}
	case "delivery":
		// Skip if delivery has already been executed successfully
		if pp.isConditionTrue(conditions, "delivery_execute") {
			return true
		}
	default:
		// For unknown condition types, check if the condition exists and is true
		if pp.isConditionTrue(conditions, condType) {
			return true
		}
	}

	// Additional checks based on batch status and current state
	if smsBatch.Status == "completed" && (condType == "preparation" || condType == "delivery") {
		return true
	}

	return false
}

// isIdempotentExecution checks if the SMS batch is configured for idempotent execution.
func (pp *PreparationProcessor) isIdempotentExecution(smsBatch *model.SmsBatchM) bool {
	// For now, assume all SMS batches are idempotent
	return true
}

// ProcessSmsPreparationBatch processes a batch of SMS data
func (pp *PreparationProcessor) ProcessSmsPreparationBatch(ctx context.Context, batchData []string) (processed, success, failed int) {
	// Simulate processing logic
	processed = len(batchData)
	success = int(float64(processed) * 0.95) // 95% success rate
	failed = processed - success

	// Add small delay to simulate processing
	time.Sleep(100 * time.Millisecond)

	return processed, success, failed
}

// isConditionTrue checks if a condition is true in the job conditions
func (pp *PreparationProcessor) isConditionTrue(conditions *model.JobConditions, conditionName string) bool {
	if conditions == nil {
		return false
	}
	// Simple implementation - in a real scenario, this would check the actual condition structure
	// For now, we'll assume conditions are not set, so return false
	return false
}
