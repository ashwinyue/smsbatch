package processor

import (
	"context"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/manager"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/publisher"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// DeliveryProcessor handles SMS batch delivery phase processing
type DeliveryProcessor struct {
	partitionManager *manager.PartitionManager
	eventPublisher   *publisher.EventPublisher
}

// NewDeliveryProcessor creates a new DeliveryProcessor instance
func NewDeliveryProcessor(partitionManager *manager.PartitionManager, eventPublisher *publisher.EventPublisher) *DeliveryProcessor {
	return &DeliveryProcessor{
		partitionManager: partitionManager,
		eventPublisher:   eventPublisher,
	}
}

// Execute handles the SMS delivery phase
func (dp *DeliveryProcessor) Execute(ctx context.Context, sm *model.SmsBatchM) error {
	if dp.shouldSkipOnIdempotency(sm, "delivery") {
		return nil
	}

	log.Infow("Starting SMS batch delivery", "batch_id", sm.BatchID)

	// Initialize SMS batch results if they are not already set
	if sm.Results == nil {
		sm.Results = &model.SmsBatchResults{}
	}

	// Initialize delivery statistics
	startTime := time.Now()
	sm.Results.CurrentPhase = "delivery"
	if sm.Results.DeliveryStats == nil {
		sm.Results.DeliveryStats = &model.SmsBatchPhaseStats{}
	}
	sm.Results.DeliveryStats.StartTime = &startTime

	// Read packed data (simulate reading from storage)
	log.Infow("Reading packed data for delivery", "batch_id", sm.BatchID)
	// TODO: Implement actual data reading from storage
	data := []string{"sms1", "sms2", "sms3", "sms4", "sms5"} // Simulated data

	log.Infow("Processing SMS delivery with partitioning", "batch_id", sm.BatchID, "total_data", len(data))

	// Get partition configuration from params
	partitionCount := int32(4) // Default partition count
	if sm.Params != nil && sm.Params.PartitionCount > 0 {
		partitionCount = sm.Params.PartitionCount
	}

	// Create delivery partitions (similar to Java's SmsBatchPartitionTask)
	partitions := dp.partitionManager.CreateDeliveryPartitions(data, partitionCount)
	totalProcessed := int64(0)
	totalSuccess := int64(0)
	totalFailed := int64(0)

	// Initialize partition statuses
	sm.Results.PartitionStatuses = make([]*model.SmsBatchPartitionStatus, len(partitions))

	// Process partitions concurrently using processDeliveryPartitions
	workerCount := 4 // Default worker count
	if sm.Params != nil && sm.Params.PartitionCount > 0 {
		workerCount = int(sm.Params.PartitionCount)
	}

	processed, success, failed, err := dp.partitionManager.ProcessDeliveryPartitions(ctx, sm, partitions, workerCount)
	if err != nil {
		log.Errorw("Failed to process delivery partitions", "error", err, "batch_id", sm.BatchID)
		return err
	}

	totalProcessed = processed
	totalSuccess = success
	totalFailed = failed

	// Update progress to 100% after all partitions are processed
	sm.Results.ProgressPercent = 100.0

	// Publish final progress update
	if err := dp.eventPublisher.PublishDeliveryProgress(ctx, sm.BatchID, totalProcessed, totalSuccess, totalFailed, 100.0); err != nil {
		log.Errorw("Failed to publish final progress update", "batch_id", sm.BatchID, "error", err)
	}

	// Update final delivery statistics
	endTime := time.Now()
	sm.Results.DeliveryStats.Processed = totalProcessed
	sm.Results.DeliveryStats.Success = totalSuccess
	sm.Results.DeliveryStats.Failed = totalFailed
	sm.Results.DeliveryStats.EndTime = &endTime
	sm.Results.DeliveryStats.DurationSeconds = int64(endTime.Sub(startTime).Seconds())

	// Update overall batch statistics
	sm.Results.ProcessedMessages = totalProcessed
	sm.Results.SuccessMessages = totalSuccess
	sm.Results.FailedMessages = totalFailed
	sm.Results.ProgressPercent = 100.0

	// Publish final status update
	if err := dp.eventPublisher.PublishDeliveryCompleted(ctx, sm.BatchID, sm.Status, totalProcessed, totalSuccess, totalFailed); err != nil {
		log.Errorw("Failed to publish final delivery status update", "batch_id", sm.BatchID, "error", err)
	}

	log.Infow("SMS batch delivery completed successfully",
		"batch_id", sm.BatchID,
		"processed", totalProcessed,
		"success", totalSuccess,
		"failed", totalFailed,
		"partitions", len(partitions))

	return nil
}

// shouldSkipOnIdempotency determines whether an SMS batch should skip execution based on idempotency conditions.
func (dp *DeliveryProcessor) shouldSkipOnIdempotency(sm *model.SmsBatchM, condType string) bool {
	// If idempotent execution is not set, allow execution regardless of conditions.
	if !dp.isIdempotentExecution(sm) {
		return false
	}

	// Check if the condition has already been satisfied
	if sm.Conditions == nil {
		return false
	}

	// Convert conditions to map for easier checking
	conditions := (*model.JobConditions)(sm.Conditions)
	if conditions == nil {
		return false
	}

	// Check specific condition types
	switch condType {
	case "preparation":
		// Skip if preparation has already been executed successfully
		if dp.isConditionTrue(conditions, "preparation_execute") {
			return true
		}
	case "delivery":
		// Skip if delivery has already been executed successfully
		if dp.isConditionTrue(conditions, "delivery_execute") {
			return true
		}
	default:
		// For unknown condition types, check if the condition exists and is true
		if dp.isConditionTrue(conditions, condType) {
			return true
		}
	}

	// Additional checks based on batch status and current state
	if sm.Status == "completed" && (condType == "preparation" || condType == "delivery") {
		return true
	}

	return false
}

// isIdempotentExecution checks if the SMS batch is configured for idempotent execution.
func (dp *DeliveryProcessor) isIdempotentExecution(sm *model.SmsBatchM) bool {
	// For now, assume all SMS batches are idempotent
	return true
}

// isConditionTrue checks if a condition is true in the job conditions
func (dp *DeliveryProcessor) isConditionTrue(conditions *model.JobConditions, conditionName string) bool {
	if conditions == nil {
		return false
	}
	// Simple implementation - in a real scenario, this would check the actual condition structure
	// For now, we'll assume conditions are not set, so return false
	return false
}
