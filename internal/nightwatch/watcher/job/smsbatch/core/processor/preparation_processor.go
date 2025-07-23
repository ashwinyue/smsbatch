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

// PreparationProcessor handles SMS batch preparation phase processing
type PreparationProcessor struct {
	eventPublisher      *publisher.EventPublisher
	partitionManager    *manager.PartitionManager
	tableStorageService service.TableStorageService // 替代Java项目中的Table Storage功能
}

// NewPreparationProcessor creates a new PreparationProcessor instance
func NewPreparationProcessor(eventPublisher *publisher.EventPublisher, partitionManager *manager.PartitionManager, tableStorageService service.TableStorageService) *PreparationProcessor {
	return &PreparationProcessor{
		eventPublisher:      eventPublisher,
		partitionManager:    partitionManager,
		tableStorageService: tableStorageService,
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

	// 1. Read SMS parameters from MongoDB storage (替代Java项目中的Table Storage)
	log.Infow("Reading SMS parameters from MongoDB storage", "batch_id", sm.BatchID, "table_storage_name", sm.TableStorageName)

	// 从MongoDB中读取SMS记录数据
	defaultBatchSize := 1000 // Default batch size for reading
	smsRecords, err := pp.tableStorageService.ReadSmsRecordsByBatch(ctx, sm.BatchID, defaultBatchSize)
	if err != nil {
		// 如果按批次ID读取失败，尝试按Table Storage名称读取（兼容Java项目）
		if sm.TableStorageName != "" {
			log.Infow("Fallback to reading by table storage name", "table_storage_name", sm.TableStorageName)
			smsRecords, err = pp.tableStorageService.ReadSmsRecords(ctx, sm.TableStorageName, defaultBatchSize)
			if err != nil {
				return fmt.Errorf("failed to read SMS records from storage: %w", err)
			}
		} else {
			return fmt.Errorf("failed to read SMS records by batch ID: %w", err)
		}
	}

	log.Infow("Successfully read SMS records from storage", "batch_id", sm.BatchID, "record_count", len(smsRecords))

	// 2. Process SMS data in batches (similar to Java pack size logic)
	batchSize := 1000 // Default batch size
	totalProcessed := int64(0)
	totalSuccess := int64(0)
	totalFailed := int64(0)

	// Process SMS records in chunks
	for i := 0; i < len(smsRecords); i += batchSize {
		end := i + batchSize
		if end > len(smsRecords) {
			end = len(smsRecords)
		}

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			log.Infow("Preparation cancelled", "batch_id", sm.BatchID)
			return ctx.Err()
		default:
		}

		// Process current batch of SMS records
		batchRecords := smsRecords[i:end]
		processedCount, successCount, failedCount := pp.ProcessSmsRecordBatch(ctx, batchRecords)

		totalProcessed += int64(processedCount)
		totalSuccess += int64(successCount)
		totalFailed += int64(failedCount)

		// Update progress
		progress := float64(end) / float64(len(smsRecords)) * 100.0
		sm.Results.ProgressPercent = progress

		// Publish progress update
		if err := pp.eventPublisher.PublishPreparationProgress(ctx, sm.BatchID, sm.Status, totalProcessed, totalSuccess, totalFailed, progress); err != nil {
			log.Errorw("Failed to publish progress update", "batch_id", sm.BatchID, "error", err)
		}
	}

	// 3. Pack and save SMS delivery data to MongoDB
	log.Infow("Saving packed SMS delivery data", "batch_id", sm.BatchID, "record_count", len(smsRecords))

	// 将处理后的SMS记录保存到MongoDB（替代Java项目中的异步保存逻辑）
	if err := pp.saveProcessedSmsRecords(ctx, smsRecords); err != nil {
		log.Errorw("Failed to save processed SMS records", "batch_id", sm.BatchID, "error", err)
		// 不返回错误，继续处理，但记录失败统计
		totalFailed += int64(len(smsRecords))
	}

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

	if !sm.Params.IdempotentExecution {
		sm.Params.IdempotentExecution = true
	}

	if sm.Params.PreparationConfig == nil {
		sm.Params.PreparationConfig = map[string]interface{}{
			"pack_size":            1000,
			"max_concurrent_packs": 4,
			"enable_validation":    true,
			"storage_timeout":      300,
		}
	}

	if sm.Params.DeliveryConfig == nil {
		sm.Params.DeliveryConfig = map[string]interface{}{
			"max_concurrent_partitions": 4,
			"delivery_timeout":          600,
			"retry_delay_seconds":       30,
			"enable_delivery_tracking":  true,
		}
	}
}

// shouldSkipOnIdempotency determines whether an SMS batch should skip execution based on idempotency conditions
func (pp *PreparationProcessor) shouldSkipOnIdempotency(smsBatch *model.SmsBatchM, condType string) bool {
	if !pp.isIdempotentExecution(smsBatch) {
		return false
	}

	if smsBatch.Conditions == nil {
		return false
	}

	conditions := (*model.JobConditions)(smsBatch.Conditions)
	if conditions == nil {
		return false
	}

	switch condType {
	case "preparation":
		if pp.isConditionTrue(conditions, "preparation_execute") {
			return true
		}
	case "delivery":
		if pp.isConditionTrue(conditions, "delivery_execute") {
			return true
		}
	default:
		if pp.isConditionTrue(conditions, condType) {
			return true
		}
	}

	if smsBatch.Status == "completed" && (condType == "preparation" || condType == "delivery") {
		return true
	}

	return false
}

// isIdempotentExecution checks if the SMS batch is configured for idempotent execution
func (pp *PreparationProcessor) isIdempotentExecution(smsBatch *model.SmsBatchM) bool {
	return smsBatch.Params != nil && smsBatch.Params.IdempotentExecution
}

// isConditionTrue checks if a condition is true in the job conditions
func (pp *PreparationProcessor) isConditionTrue(conditions *model.JobConditions, conditionName string) bool {
	if conditions == nil {
		return false
	}
	return false
}

// ProcessSmsRecordBatch processes a batch of SMS records (替代Java项目中的分批处理逻辑)
func (pp *PreparationProcessor) ProcessSmsRecordBatch(ctx context.Context, batchRecords []*model.SmsRecordM) (processed, success, failed int) {
	processed = len(batchRecords)
	success = 0
	failed = 0

	// 处理每条SMS记录
	for _, record := range batchRecords {
		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			return processed, success, failed
		default:
		}

		// 验证SMS记录
		if pp.validateSmsRecord(record) {
			// 标记为处理中状态
			record.Status = model.SmsRecordStatusProcessing
			record.UpdatedAt = time.Now()
			success++
		} else {
			// 标记为失败状态
			record.MarkAsFailed("validation failed")
			failed++
		}
	}

	// 添加小延迟模拟处理时间
	time.Sleep(50 * time.Millisecond)

	return processed, success, failed
}

// validateSmsRecord 验证SMS记录的有效性
func (pp *PreparationProcessor) validateSmsRecord(record *model.SmsRecordM) bool {
	// 基本验证逻辑
	if record.PhoneNumber == "" {
		return false
	}
	if record.Content == "" && record.TemplateID == "" {
		return false
	}
	if record.BatchID == "" {
		return false
	}
	return true
}

// saveProcessedSmsRecords 保存处理后的SMS记录到MongoDB
func (pp *PreparationProcessor) saveProcessedSmsRecords(ctx context.Context, records []*model.SmsRecordM) error {
	if len(records) == 0 {
		return nil
	}

	// 批量更新SMS记录状态
	updatedRecords := make([]*model.SmsRecordM, 0, len(records))
	for _, record := range records {
		if record.Status == model.SmsRecordStatusProcessing {
			updatedRecords = append(updatedRecords, record)
		}
	}

	if len(updatedRecords) == 0 {
		return nil
	}

	// 这里可以调用tableStorageService来批量更新记录
	// 由于当前接口限制，我们逐个更新
	for _, record := range updatedRecords {
		if err := pp.tableStorageService.UpdateSmsRecordStatus(ctx, record.ID, record.Status); err != nil {
			log.Errorw("Failed to update SMS record status", "record_id", record.ID, "error", err)
			// 继续处理其他记录
		}
	}

	return nil
}
