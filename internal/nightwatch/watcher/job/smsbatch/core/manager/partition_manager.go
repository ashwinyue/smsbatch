package manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/provider"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/nightwatch/types"
	"github.com/ashwinyue/dcp/internal/pkg/log"
	"github.com/onexstack/onexstack/pkg/store/where"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
)

// PartitionManager handles SMS batch partition creation and processing
type PartitionManager struct {
	store           store.IStore
	providerFactory *provider.ProviderFactory
}

// NewPartitionManager creates a new partition manager
func NewPartitionManager(store store.IStore, providerFactory *provider.ProviderFactory) *PartitionManager {
	return &PartitionManager{
		store:           store,
		providerFactory: providerFactory,
	}
}

// deliverSmsMessages delivers SMS messages using the configured provider
func (pm *PartitionManager) deliverSmsMessages(ctx context.Context, partition *model.SmsBatchPartitionTaskM) error {
	// 集成短信供应商功能
	// 1. 获取批次信息和供应商配置
	batchInfo, err := pm.getBatchInfo(ctx, partition.BatchPrimaryKey)
	if err != nil {
		log.Errorw("Failed to get batch info", "batch_primary_key", partition.BatchPrimaryKey, "error", err)
		return err
	}

	// 2. 加载分区中的短信记录
	smsRecords, err := pm.loadSmsRecords(ctx, partition.BatchPrimaryKey, partition.PartitionKey)
	if err != nil {
		log.Errorw("Failed to load SMS records", "batch_primary_key", partition.BatchPrimaryKey, "partition_key", partition.PartitionKey, "error", err)
		return err
	}

	// 3. 调用供应商API发送短信
	for _, record := range smsRecords {
		if err := pm.sendSingleSms(ctx, record, batchInfo.ProviderConfig); err != nil {
			log.Errorw("Failed to send SMS", "record_id", record.ID, "error", err)
			// 4. 处理发送失败，可能需要重试
			if err := pm.handleSendFailure(ctx, record, err); err != nil {
				log.Errorw("Failed to handle send failure", "record_id", record.ID, "error", err)
			}
			continue
		}
		// 处理发送成功
		if err := pm.handleSendSuccess(ctx, record); err != nil {
			log.Errorw("Failed to handle send success", "record_id", record.ID, "error", err)
		}
	}

	// 5. 更新投递状态
	if err := pm.updateDeliveryStatus(ctx, partition.BatchPrimaryKey, partition.PartitionKey); err != nil {
		log.Errorw("Failed to update delivery status", "batch_primary_key", partition.BatchPrimaryKey, "partition_key", partition.PartitionKey, "error", err)
		return err
	}

	log.Infow("SMS messages delivered successfully", "partition_key", partition.PartitionKey, "total_records", len(smsRecords))

	// 模拟短信发送过程
	time.Sleep(100 * time.Millisecond)

	return nil
}

// getBatchInfo 获取批次信息
func (pm *PartitionManager) getBatchInfo(ctx context.Context, batchPrimaryKey string) (*BatchInfo, error) {
	// 从数据库获取批次信息
	batch, err := pm.store.SmsBatch().Get(ctx, where.F("batch_id", batchPrimaryKey))
	if err != nil {
		log.Errorw("Failed to get batch info", "batch_id", batchPrimaryKey, "error", err)
		return nil, err
	}

	// 构建供应商配置
	providerConfig := &ProviderConfig{
		ProviderType: batch.ProviderType,
		APIEndpoint:  "https://api.example.com/sms", // 根据实际供应商配置
	}

	return &BatchInfo{
		ID:              1, // 临时设置，应该从数据库获取
		BatchPrimaryKey: batchPrimaryKey,
		ProviderConfig:  providerConfig,
	}, nil
}

// loadSmsRecords 加载短信记录
func (pm *PartitionManager) loadSmsRecords(ctx context.Context, batchPrimaryKey string, partitionKey string) ([]*SmsRecord, error) {
	// 从数据库加载分区中的短信记录
	log.Infow("Loading SMS records", "batch_id", batchPrimaryKey, "partition_id", partitionKey)

	// 查询待发送的短信记录
	records, _, err := pm.store.SmsRecord().GetByBatchID(ctx, batchPrimaryKey, 1000, 0)
	if err != nil {
		log.Errorw("Failed to load SMS records", "batch_id", batchPrimaryKey, "error", err)
		return nil, err
	}

	// 转换为内部格式
	smsRecords := make([]*SmsRecord, 0, len(records))
	for _, record := range records {
		smsRecords = append(smsRecords, &SmsRecord{
			ID:          record.ID,
			PhoneNumber: record.PhoneNumber,
			Content:     record.Content,
			Status:      record.Status,
		})
	}

	return smsRecords, nil
}

// sendSingleSms 发送单条短信
func (pm *PartitionManager) sendSingleSms(ctx context.Context, record *SmsRecord, config *ProviderConfig) error {
	// 调用实际的短信供应商API
	log.Infow("Sending SMS", "record_id", record.ID, "phone", record.PhoneNumber, "provider", config.ProviderType)

	// 获取短信供应商实例
	provider, err := pm.providerFactory.GetSMSTemplateProvider(types.ProviderType(config.ProviderType))
	if err != nil {
		log.Errorw("Failed to get SMS provider", "provider_type", config.ProviderType, "error", err)
		return err
	}

	// 构建发送请求
	req := &types.TemplateMsgRequest{
		RequestId:   fmt.Sprintf("%d", record.ID),
		PhoneNumber: record.PhoneNumber,
		Content:     record.Content,
	}

	// 发送短信
	resp, err := provider.Send(ctx, req)
	if err != nil {
		log.Errorw("Failed to send SMS", "record_id", record.ID, "error", err)
		return err
	}

	log.Infow("SMS sent successfully", "record_id", record.ID, "biz_id", resp.BizId, "code", resp.Code)
	return nil
}

// handleSendSuccess 处理发送成功
func (pm *PartitionManager) handleSendSuccess(ctx context.Context, record *SmsRecord) error {
	// 更新记录状态为成功
	log.Infow("SMS sent successfully", "record_id", record.ID)

	// 更新数据库中的记录状态
	// Note: Using Query method since Get method doesn't exist in SmsRecordMongoStore
	query := &model.SmsRecordQuery{
		ID:     record.ID,
		Limit:  1,
		Offset: 0,
	}
	records, _, err := pm.store.SmsRecord().Query(ctx, query)
	if err != nil {
		log.Errorw("Failed to query SMS record for update", "record_id", record.ID, "error", err)
		return err
	}
	if len(records) == 0 {
		log.Errorw("SMS record not found for update", "record_id", record.ID)
		return fmt.Errorf("SMS record %s not found", record.ID)
	}
	smsRecord := records[0]

	smsRecord.Status = "sent"
	now := time.Now()
	smsRecord.SentTime = &now
	smsRecord.UpdatedAt = now

	if err := pm.store.SmsRecord().Update(ctx, smsRecord); err != nil {
		log.Errorw("Failed to update SMS record status", "record_id", record.ID, "error", err)
		return err
	}

	return nil
}

// handleSendFailure 处理发送失败
func (pm *PartitionManager) handleSendFailure(ctx context.Context, record *SmsRecord, sendErr error) error {
	// 更新记录状态为失败，可能需要重试
	log.Errorw("SMS send failed", "record_id", record.ID, "error", sendErr)

	// 更新数据库中的记录状态
	// Note: Using Query method since Get method doesn't exist in SmsRecordMongoStore
	query := &model.SmsRecordQuery{
		ID:     record.ID,
		Limit:  1,
		Offset: 0,
	}
	records, _, err := pm.store.SmsRecord().Query(ctx, query)
	if err != nil {
		log.Errorw("Failed to query SMS record for update", "record_id", record.ID, "error", err)
		return err
	}
	if len(records) == 0 {
		log.Errorw("SMS record not found for update", "record_id", record.ID)
		return fmt.Errorf("SMS record %s not found", record.ID)
	}
	smsRecord := records[0]

	smsRecord.Status = "failed"
	smsRecord.ErrorMessage = sendErr.Error()
	smsRecord.RetryCount++

	// 如果重试次数未达到上限，设置为待重试状态
	if smsRecord.RetryCount < 3 {
		smsRecord.Status = "retry"
	}

	if err := pm.store.SmsRecord().Update(ctx, smsRecord); err != nil {
		log.Errorw("Failed to update SMS record status", "record_id", record.ID, "error", err)
		return err
	}

	return nil
}

// updateDeliveryStatus 更新投递状态
func (pm *PartitionManager) updateDeliveryStatus(ctx context.Context, batchPrimaryKey string, partitionKey string) error {
	// 更新分区任务的投递状态
	log.Infow("Updating delivery status", "batch_id", batchPrimaryKey, "partition_id", partitionKey)

	// 统计当前批次的发送状态
	records, _, err := pm.store.SmsRecord().GetByBatchID(ctx, batchPrimaryKey, 10000, 0)
	if err != nil {
		log.Errorw("Failed to get SMS records for status update", "batch_id", batchPrimaryKey, "error", err)
		return err
	}

	var totalCount, sentCount, failedCount int64
	for _, record := range records {
		totalCount++
		switch record.Status {
		case "sent":
			sentCount++
		case "failed":
			failedCount++
		}
	}

	// 更新批次的投递状态
	batch, err := pm.store.SmsBatch().Get(ctx, where.F("batch_id", batchPrimaryKey))
	if err != nil {
		log.Errorw("Failed to get batch for status update", "batch_id", batchPrimaryKey, "error", err)
		return err
	}

	if batch.Results == nil {
		batch.Results = &model.SmsBatchResults{}
	}
	batch.Results.TotalMessages = totalCount
	batch.Results.ProcessedMessages = sentCount + failedCount
	batch.Results.SuccessMessages = sentCount
	batch.Results.FailedMessages = failedCount
	batch.Results.ProgressPercent = float64(sentCount+failedCount) / float64(totalCount) * 100

	if err := pm.store.SmsBatch().Update(ctx, batch); err != nil {
		log.Errorw("Failed to update batch delivery status", "batch_id", batchPrimaryKey, "error", err)
		return err
	}

	return nil
}

// BatchInfo 批次信息
type BatchInfo struct {
	ID              int64
	BatchPrimaryKey string
	ProviderConfig  *ProviderConfig
}

// ProviderConfig 供应商配置
type ProviderConfig struct {
	ProviderType string
	APIEndpoint  string
}

// SmsRecord 短信记录
type SmsRecord struct {
	ID          int64
	PhoneNumber string
	Content     string
	Status      string
}

// CreateDeliveryPartitions creates delivery partitions from data
func (pm *PartitionManager) CreateDeliveryPartitions(data []string, partitionCount int32) [][]string {
	if len(data) == 0 || partitionCount <= 0 {
		return [][]string{}
	}

	partitions := make([][]string, partitionCount)
	partitionSize := len(data) / int(partitionCount)
	if partitionSize == 0 {
		partitionSize = 1
	}

	for i := 0; i < len(data); i++ {
		partitionIndex := i / partitionSize
		if partitionIndex >= int(partitionCount) {
			partitionIndex = int(partitionCount) - 1
		}
		partitions[partitionIndex] = append(partitions[partitionIndex], data[i])
	}

	// Remove empty partitions
	var result [][]string
	for _, partition := range partitions {
		if len(partition) > 0 {
			result = append(result, partition)
		}
	}

	return result
}

// ProcessDeliveryPartitions processes delivery partitions concurrently
func (pm *PartitionManager) ProcessDeliveryPartitions(ctx context.Context, sm interface{}, partitions [][]string, workerCount int) (int64, int64, int64, error) {
	if len(partitions) == 0 {
		return 0, 0, 0, nil
	}

	stats := NewStats()
	var wg sync.WaitGroup
	partitionChan := make(chan []string, len(partitions))
	errorChan := make(chan error, len(partitions))

	// Start workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for partition := range partitionChan {
				partitionID := fmt.Sprintf("worker-%d-partition-%d", workerID, time.Now().UnixNano())
				processed, success, failed, err := pm.ProcessDeliveryPartition(ctx, sm, partitionID, partition)
				if err != nil {
					errorChan <- err
					return
				}
				stats.Add(int64(processed), int64(success), int64(failed))
				log.Debugw("Worker completed partition", "worker_id", workerID, "partition_size", len(partition))
			}
		}(i)
	}

	// Send work to workers
	go func() {
		defer close(partitionChan)
		for _, partition := range partitions {
			partitionChan <- partition
		}
	}()

	wg.Wait()
	close(errorChan)

	// Check for errors
	select {
	case err := <-errorChan:
		if err != nil {
			return 0, 0, 0, err
		}
	default:
	}

	processed, success, failed := stats.Get()
	return processed, success, failed, nil
}

// CreateSmsRecordDeliveryPartitions creates delivery partitions from SMS records (替代Java项目中的分区逻辑)
func (pm *PartitionManager) CreateSmsRecordDeliveryPartitions(records []*model.SmsRecordM, partitionCount int32) [][]*model.SmsRecordM {
	if len(records) == 0 || partitionCount <= 0 {
		return [][]*model.SmsRecordM{}
	}

	partitions := make([][]*model.SmsRecordM, partitionCount)
	partitionSize := len(records) / int(partitionCount)
	if partitionSize == 0 {
		partitionSize = 1
	}

	for i := 0; i < len(records); i++ {
		partitionIndex := i / partitionSize
		if partitionIndex >= int(partitionCount) {
			partitionIndex = int(partitionCount) - 1
		}
		partitions[partitionIndex] = append(partitions[partitionIndex], records[i])
	}

	// Remove empty partitions
	var result [][]*model.SmsRecordM
	for _, partition := range partitions {
		if len(partition) > 0 {
			result = append(result, partition)
		}
	}

	return result
}

// ProcessSmsRecordDeliveryPartitions processes SMS record delivery partitions concurrently (替代Java项目中的分区处理)
func (pm *PartitionManager) ProcessSmsRecordDeliveryPartitions(ctx context.Context, sm *model.SmsBatchM, partitions [][]*model.SmsRecordM, workerCount int) (int64, int64, int64, error) {
	if len(partitions) == 0 {
		return 0, 0, 0, nil
	}

	stats := NewStats()
	var wg sync.WaitGroup
	partitionChan := make(chan []*model.SmsRecordM, len(partitions))
	errorChan := make(chan error, len(partitions))

	// Start workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for partition := range partitionChan {
				partitionID := fmt.Sprintf("worker-%d-partition-%d", workerID, time.Now().UnixNano())
				processed, success, failed, err := pm.ProcessSmsRecordDeliveryPartition(ctx, sm, partitionID, partition)
				if err != nil {
					errorChan <- err
					return
				}
				stats.Add(int64(processed), int64(success), int64(failed))
				log.Debugw("Worker completed SMS record partition", "worker_id", workerID, "partition_size", len(partition))
			}
		}(i)
	}

	// Send work to workers
	go func() {
		defer close(partitionChan)
		for _, partition := range partitions {
			partitionChan <- partition
		}
	}()

	wg.Wait()
	close(errorChan)

	// Check for errors
	select {
	case err := <-errorChan:
		if err != nil {
			return 0, 0, 0, err
		}
	default:
	}

	processed, success, failed := stats.Get()
	return processed, success, failed, nil
}

// ProcessSmsRecordDeliveryPartition processes a single SMS record delivery partition (替代Java项目中的单分区处理)
func (pm *PartitionManager) ProcessSmsRecordDeliveryPartition(ctx context.Context, sm *model.SmsBatchM, partitionID string, partition []*model.SmsRecordM) (processed, success, failed int, err error) {
	log.Infow("Processing SMS record delivery partition", "partition_id", partitionID, "size", len(partition), "batch_id", sm.BatchID)

	processed = len(partition)
	success = 0
	failed = 0

	// Process each SMS record in the partition
	for _, record := range partition {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return processed, success, failed, ctx.Err()
		default:
		}

		// Simulate SMS delivery processing
		deliverySuccess := pm.simulateSmsDelivery(record)
		if deliverySuccess {
			// Mark as delivered
			record.Status = model.SmsRecordStatusDelivered
			record.DeliveredTime = &[]time.Time{time.Now()}[0]
			success++
		} else {
			// Mark as failed
			record.MarkAsFailed("delivery failed")
			failed++
		}
		record.UpdatedAt = time.Now()
	}

	// Simulate delivery time based on partition size
	deliveryTime := time.Duration(len(partition)) * 5 * time.Millisecond
	time.Sleep(deliveryTime)

	log.Infow("SMS record delivery partition completed",
		"partition_id", partitionID,
		"batch_id", sm.BatchID,
		"processed", processed,
		"success", success,
		"failed", failed)

	return processed, success, failed, nil
}

// simulateSmsDelivery simulates SMS delivery with realistic success rate
func (pm *PartitionManager) simulateSmsDelivery(record *model.SmsRecordM) bool {
	// Simulate different success rates based on phone number patterns
	// This mimics real-world scenarios where some numbers might be invalid
	if len(record.PhoneNumber) < 10 {
		return false // Invalid phone number
	}

	// Simulate 92% success rate
	successRate := 0.92
	return time.Now().UnixNano()%100 < int64(successRate*100)
}

// ProcessDeliveryPartition processes a single delivery partition
func (pm *PartitionManager) ProcessDeliveryPartition(ctx context.Context, sm interface{}, partitionID string, partition []string) (processed, success, failed int, err error) {
	log.Infow("Processing delivery partition", "partition_id", partitionID, "size", len(partition))

	// Simulate SMS delivery processing
	processed = len(partition)

	// Simulate delivery with some failures
	successRate := 0.92 // 92% success rate
	success = int(float64(processed) * successRate)
	failed = processed - success

	// Simulate delivery time
	deliveryTime := time.Duration(len(partition)) * 10 * time.Millisecond
	time.Sleep(deliveryTime)

	// 实现实际的短信投递逻辑
	// 调用短信供应商的API进行实际发送
	partitionTask := &model.SmsBatchPartitionTaskM{
		ID:              0,
		BatchPrimaryKey: "batch-" + partitionID, // 从sm中获取实际的BatchPrimaryKey
		PartitionKey:    partitionID,
		TaskCode:        fmt.Sprintf("task-%s", partitionID),
	}
	if err := pm.deliverSmsMessages(ctx, partitionTask); err != nil {
		log.Errorw("Failed to deliver SMS messages", "partition_key", partitionID, "error", err)
		return processed, success, failed, err
	}

	log.Infow("Delivery partition completed",
		"partition_id", partitionID,
		"processed", processed,
		"success", success,
		"failed", failed)

	return processed, success, failed, nil
}
