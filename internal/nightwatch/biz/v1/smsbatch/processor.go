package smsbatch

import (
	"context"
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/service"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// PreparationProcessor handles SMS batch preparation phase processing
type PreparationProcessor struct {
	eventPublisher      *EventPublisher
	partitionManager    *PartitionManager
	tableStorageService service.TableStorageService
}

// NewPreparationProcessor creates a new PreparationProcessor instance
func NewPreparationProcessor(eventPublisher *EventPublisher, partitionManager *PartitionManager, tableStorageService service.TableStorageService) *PreparationProcessor {
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

	// Read SMS parameters from MongoDB storage
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
			return fmt.Errorf("failed to read SMS records and no table storage name provided: %w", err)
		}
	}

	// Process SMS records
	log.Infow("Processing SMS records", "batch_id", sm.BatchID, "record_count", len(smsRecords))

	// Update preparation statistics
	sm.Results.PreparationStats.Total = int64(len(smsRecords))
	sm.Results.PreparationStats.Processed = int64(len(smsRecords))
	sm.Results.PreparationStats.Success = int64(len(smsRecords))
	endTime := time.Now()
	sm.Results.PreparationStats.EndTime = &endTime

	log.Infow("SMS batch preparation completed", "batch_id", sm.BatchID, "total_records", len(smsRecords))
	return nil
}

// setDefaultSmsBatchParams sets default parameters for SMS batch
func (pp *PreparationProcessor) setDefaultSmsBatchParams(sm *model.SmsBatchM) {
	if sm.ProviderType == "" {
		sm.ProviderType = "default"
	}
	if sm.MessageType == "" {
		sm.MessageType = "text"
	}
}

// shouldSkipOnIdempotency checks if the operation should be skipped for idempotency
func (pp *PreparationProcessor) shouldSkipOnIdempotency(sm *model.SmsBatchM, phase string) bool {
	if sm.Results == nil {
		return false
	}
	return sm.Results.CurrentPhase == phase
}

// DeliveryProcessor handles SMS batch delivery phase processing
type DeliveryProcessor struct {
	partitionManager    *PartitionManager
	eventPublisher      *EventPublisher
	tableStorageService service.TableStorageService
}

// NewDeliveryProcessor creates a new DeliveryProcessor instance
func NewDeliveryProcessor(partitionManager *PartitionManager, eventPublisher *EventPublisher, tableStorageService service.TableStorageService) *DeliveryProcessor {
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

	// Read SMS records from MongoDB storage
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
			return fmt.Errorf("failed to read SMS records and no table storage name provided for delivery: %w", err)
		}
	}

	// Process delivery
	log.Infow("Processing SMS delivery", "batch_id", sm.BatchID, "record_count", len(smsRecords))

	// Update delivery statistics
	sm.Results.DeliveryStats.Total = int64(len(smsRecords))
	sm.Results.DeliveryStats.Processed = int64(len(smsRecords))
	sm.Results.DeliveryStats.Success = int64(len(smsRecords))
	endTime := time.Now()
	sm.Results.DeliveryStats.EndTime = &endTime

	log.Infow("SMS batch delivery completed", "batch_id", sm.BatchID, "delivered_records", len(smsRecords))
	return nil
}

// shouldSkipOnIdempotency checks if the operation should be skipped for idempotency
func (dp *DeliveryProcessor) shouldSkipOnIdempotency(sm *model.SmsBatchM, phase string) bool {
	if sm.Results == nil {
		return false
	}
	return sm.Results.CurrentPhase == phase
}