package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// TableStorageService 提供Table Storage功能的服务
// 这个服务替代了Java项目中的TableStorageTemplate功能
type TableStorageService interface {
	// ReadSmsRecords 从存储中读取SMS记录（替代Java中的Table Storage读取）
	ReadSmsRecords(ctx context.Context, tableStorageName string, batchSize int) ([]*model.SmsRecordM, error)
	// ReadSmsRecordsByBatch 根据批次ID读取SMS记录
	ReadSmsRecordsByBatch(ctx context.Context, batchID string, batchSize int) ([]*model.SmsRecordM, error)
	// ReadSmsRecordsInBatches 分批读取SMS记录（类似Java中的分页读取）
	ReadSmsRecordsInBatches(ctx context.Context, tableStorageName string, batchSize int, processor func([]*model.SmsRecordM) error) error
	// CreateSmsRecords 创建SMS记录到存储中
	CreateSmsRecords(ctx context.Context, records []*model.SmsRecordM) error
	// UpdateSmsRecordStatus 更新SMS记录状态
	UpdateSmsRecordStatus(ctx context.Context, recordID int64, status string) error
	// GetSmsRecordStats 获取SMS记录统计信息
	GetSmsRecordStats(ctx context.Context, batchID string) (*SmsRecordStats, error)
}

// SmsRecordStats SMS记录统计信息
type SmsRecordStats struct {
	BatchID         string `json:"batch_id"`
	TotalCount      int64  `json:"total_count"`
	PendingCount    int64  `json:"pending_count"`
	ProcessingCount int64  `json:"processing_count"`
	SentCount       int64  `json:"sent_count"`
	DeliveredCount  int64  `json:"delivered_count"`
	FailedCount     int64  `json:"failed_count"`
	CanceledCount   int64  `json:"canceled_count"`
}

// tableStorageService 是 TableStorageService 的具体实现
type tableStorageService struct {
	smsRecordStore store.SmsRecordMongoStore
}

// NewTableStorageService 创建一个新的 TableStorageService 实例
func NewTableStorageService(smsRecordStore store.SmsRecordMongoStore) TableStorageService {
	return &tableStorageService{
		smsRecordStore: smsRecordStore,
	}
}

// ReadSmsRecords 从存储中读取SMS记录（替代Java中的Table Storage读取）
func (s *tableStorageService) ReadSmsRecords(ctx context.Context, tableStorageName string, batchSize int) ([]*model.SmsRecordM, error) {
	log.Infow("Reading SMS records from storage", "table_storage_name", tableStorageName, "batch_size", batchSize)

	records, _, err := s.smsRecordStore.GetByTableStorageName(ctx, tableStorageName, batchSize, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to read SMS records from table storage %s: %w", tableStorageName, err)
	}

	log.Infow("Successfully read SMS records", "table_storage_name", tableStorageName, "count", len(records))
	return records, nil
}

// ReadSmsRecordsByBatch 根据批次ID读取SMS记录
func (s *tableStorageService) ReadSmsRecordsByBatch(ctx context.Context, batchID string, batchSize int) ([]*model.SmsRecordM, error) {
	log.Infow("Reading SMS records by batch ID", "batch_id", batchID, "batch_size", batchSize)

	records, _, err := s.smsRecordStore.GetByBatchID(ctx, batchID, batchSize, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to read SMS records for batch %s: %w", batchID, err)
	}

	log.Infow("Successfully read SMS records by batch", "batch_id", batchID, "count", len(records))
	return records, nil
}

// ReadSmsRecordsInBatches 分批读取SMS记录（类似Java中的分页读取）
func (s *tableStorageService) ReadSmsRecordsInBatches(ctx context.Context, tableStorageName string, batchSize int, processor func([]*model.SmsRecordM) error) error {
	log.Infow("Reading SMS records in batches", "table_storage_name", tableStorageName, "batch_size", batchSize)

	offset := 0
	totalProcessed := 0

	for {
		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 读取一批记录
		records, totalCount, err := s.smsRecordStore.GetByTableStorageName(ctx, tableStorageName, batchSize, offset)
		if err != nil {
			return fmt.Errorf("failed to read SMS records batch at offset %d: %w", offset, err)
		}

		// 如果没有更多记录，退出循环
		if len(records) == 0 {
			break
		}

		// 处理当前批次的记录
		if err := processor(records); err != nil {
			return fmt.Errorf("failed to process SMS records batch: %w", err)
		}

		totalProcessed += len(records)
		offset += batchSize

		log.Infow("Processed SMS records batch",
			"table_storage_name", tableStorageName,
			"batch_count", len(records),
			"total_processed", totalProcessed,
			"total_count", totalCount)

		// 如果已经处理完所有记录，退出循环
		if int64(totalProcessed) >= totalCount {
			break
		}

		// 添加小延迟，避免过度占用资源
		time.Sleep(10 * time.Millisecond)
	}

	log.Infow("Completed reading SMS records in batches",
		"table_storage_name", tableStorageName,
		"total_processed", totalProcessed)

	return nil
}

// CreateSmsRecords 创建SMS记录到存储中
func (s *tableStorageService) CreateSmsRecords(ctx context.Context, records []*model.SmsRecordM) error {
	if len(records) == 0 {
		return nil
	}

	log.Infow("Creating SMS records", "count", len(records))

	// 设置默认值
	for _, record := range records {
		if record.Status == "" {
			record.Status = model.SmsRecordStatusPending
		}
		if record.Priority == 0 {
			record.Priority = model.SmsRecordPriorityNormal
		}
		if record.MaxRetries == 0 {
			record.MaxRetries = 3
		}
	}

	err := s.smsRecordStore.CreateBatch(ctx, records)
	if err != nil {
		return fmt.Errorf("failed to create SMS records: %w", err)
	}

	log.Infow("Successfully created SMS records", "count", len(records))
	return nil
}

// UpdateSmsRecordStatus 更新SMS记录状态
func (s *tableStorageService) UpdateSmsRecordStatus(ctx context.Context, recordID int64, status string) error {
	log.Infow("Updating SMS record status", "record_id", recordID, "status", status)

	// 注意：这里需要扩展store接口来支持按ID查询
	// 暂时使用一个简化的实现
	record := &model.SmsRecordM{
		ID:     recordID,
		Status: status,
	}

	err := s.smsRecordStore.Update(ctx, record)
	if err != nil {
		return fmt.Errorf("failed to update SMS record status: %w", err)
	}

	log.Infow("Successfully updated SMS record status", "record_id", recordID, "status", status)
	return nil
}

// GetSmsRecordStats 获取SMS记录统计信息
func (s *tableStorageService) GetSmsRecordStats(ctx context.Context, batchID string) (*SmsRecordStats, error) {
	log.Infow("Getting SMS record statistics", "batch_id", batchID)

	stats := &SmsRecordStats{
		BatchID: batchID,
	}

	// 获取各种状态的记录数量
	statuses := []string{
		model.SmsRecordStatusPending,
		model.SmsRecordStatusProcessing,
		model.SmsRecordStatusSent,
		model.SmsRecordStatusDelivered,
		model.SmsRecordStatusFailed,
		model.SmsRecordStatusCanceled,
	}

	for _, status := range statuses {
		count, err := s.smsRecordStore.CountByStatus(ctx, batchID, status)
		if err != nil {
			return nil, fmt.Errorf("failed to count SMS records with status %s: %w", status, err)
		}

		switch status {
		case model.SmsRecordStatusPending:
			stats.PendingCount = count
		case model.SmsRecordStatusProcessing:
			stats.ProcessingCount = count
		case model.SmsRecordStatusSent:
			stats.SentCount = count
		case model.SmsRecordStatusDelivered:
			stats.DeliveredCount = count
		case model.SmsRecordStatusFailed:
			stats.FailedCount = count
		case model.SmsRecordStatusCanceled:
			stats.CanceledCount = count
		}

		stats.TotalCount += count
	}

	log.Infow("Successfully retrieved SMS record statistics",
		"batch_id", batchID,
		"total_count", stats.TotalCount,
		"pending_count", stats.PendingCount,
		"sent_count", stats.SentCount,
		"failed_count", stats.FailedCount)

	return stats, nil
}
