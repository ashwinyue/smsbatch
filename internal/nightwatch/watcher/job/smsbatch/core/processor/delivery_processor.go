package processor

import (
	"context"
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/service"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/manager"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/publisher"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// DeliveryProcessor handles SMS batch delivery phase processing
type DeliveryProcessor struct {
	partitionManager    *manager.PartitionManager
	eventPublisher      *publisher.EventPublisher
	tableStorageService service.TableStorageService // 替代Java项目中的Table Storage功能
}

// NewDeliveryProcessor creates a new DeliveryProcessor instance
func NewDeliveryProcessor(partitionManager *manager.PartitionManager, eventPublisher *publisher.EventPublisher, tableStorageService service.TableStorageService) *DeliveryProcessor {
	return &DeliveryProcessor{
		partitionManager:    partitionManager,
		eventPublisher:      eventPublisher,
		tableStorageService: tableStorageService,
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

	// Read SMS records from MongoDB storage (替代Java项目中的Table Storage)
	log.Infow("Reading SMS records for delivery from MongoDB storage", "batch_id", sm.BatchID)

	// 从MongoDB中读取准备好的SMS记录数据
	smsRecords, err := dp.tableStorageService.ReadSmsRecordsByBatch(ctx, sm.BatchID, 0) // 0表示读取所有记录
	if err != nil {
		// 如果按批次ID读取失败，尝试按Table Storage名称读取（兼容Java项目）
		if sm.TableStorageName != "" {
			log.Infow("Fallback to reading by table storage name for delivery", "table_storage_name", sm.TableStorageName)
			smsRecords, err = dp.tableStorageService.ReadSmsRecords(ctx, sm.TableStorageName, 0)
			if err != nil {
				return fmt.Errorf("failed to read SMS records from storage for delivery: %w", err)
			}
		} else {
			return fmt.Errorf("failed to read SMS records by batch ID for delivery: %w", err)
		}
	}

	// 过滤出状态为处理中的SMS记录（已经通过准备阶段的记录）
	deliveryRecords := make([]*model.SmsRecordM, 0)
	for _, record := range smsRecords {
		if record.Status == model.SmsRecordStatusProcessing || record.Status == model.SmsRecordStatusPending {
			deliveryRecords = append(deliveryRecords, record)
		}
	}

	log.Infow("Successfully read SMS records for delivery", "batch_id", sm.BatchID, "total_records", len(smsRecords), "delivery_records", len(deliveryRecords))

	log.Infow("Processing SMS delivery with partitioning", "batch_id", sm.BatchID, "delivery_records", len(deliveryRecords))

	// Get partition configuration from params
	partitionCount := int32(4) // Default partition count
	if sm.Params != nil && sm.Params.PartitionCount > 0 {
		partitionCount = sm.Params.PartitionCount
	}

	// Create delivery partitions using SMS records (替代Java项目中的分区逻辑)
	partitions := dp.partitionManager.CreateSmsRecordDeliveryPartitions(deliveryRecords, partitionCount)
	totalProcessed := int64(0)
	totalSuccess := int64(0)
	totalFailed := int64(0)

	// Initialize partition statuses
	sm.Results.PartitionStatuses = make([]*model.SmsBatchPartitionStatus, len(partitions))

	// Process SMS record partitions concurrently (替代Java项目中的分区处理)
	workerCount := 4 // Default worker count
	if sm.Params != nil && sm.Params.PartitionCount > 0 {
		workerCount = int(sm.Params.PartitionCount)
	}

	processed, success, failed, err := dp.partitionManager.ProcessSmsRecordDeliveryPartitions(ctx, sm, partitions, workerCount)
	if err != nil {
		log.Errorw("Failed to process SMS record delivery partitions", "error", err, "batch_id", sm.BatchID)
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

// shouldSkipOnIdempotency determines whether an SMS batch should skip execution based on idempotency conditions
func (dp *DeliveryProcessor) shouldSkipOnIdempotency(sm *model.SmsBatchM, condType string) bool {
	if !dp.isIdempotentExecution(sm) {
		return false
	}

	if sm.Conditions == nil {
		return false
	}

	conditions := (*model.JobConditions)(sm.Conditions)
	if conditions == nil {
		return false
	}

	switch condType {
	case "preparation":
		if dp.isConditionTrue(conditions, "preparation_execute") {
			return true
		}
	case "delivery":
		if dp.isConditionTrue(conditions, "delivery_execute") {
			return true
		}
	default:
		if dp.isConditionTrue(conditions, condType) {
			return true
		}
	}

	if sm.Status == "completed" && (condType == "preparation" || condType == "delivery") {
		return true
	}

	return false
}

// isIdempotentExecution checks if the SMS batch is configured for idempotent execution
func (dp *DeliveryProcessor) isIdempotentExecution(sm *model.SmsBatchM) bool {
	return sm.Params != nil && sm.Params.IdempotentExecution
}

// isConditionTrue checks if a condition is true in the job conditions
func (dp *DeliveryProcessor) isConditionTrue(conditions *model.JobConditions, conditionName string) bool {
	if conditions == nil {
		return false
	}
	return false
}
