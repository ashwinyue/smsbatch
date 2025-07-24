// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

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

// LazyReadOptions 懒读取选项
type LazyReadOptions struct {
	BatchSize         int    `json:"batch_size"`          // 每批读取的记录数
	MaxBatches        int    `json:"max_batches"`         // 最大批次数，0表示无限制
	ContinuationToken string `json:"continuation_token"`  // 继续令牌，用于断点续读
	DelayMs           int    `json:"delay_ms"`            // 批次间延迟毫秒数
	StopOnError       bool   `json:"stop_on_error"`       // 遇到错误时是否停止
}

// LazyReadResult 懒读取结果
type LazyReadResult struct {
	TotalBatches      int    `json:"total_batches"`       // 总批次数
	TotalProcessed    int    `json:"total_processed"`     // 总处理记录数
	ContinuationToken string `json:"continuation_token"`  // 下次读取的继续令牌
	Completed         bool   `json:"completed"`           // 是否完全读取完成
	LastError         string `json:"last_error"`          // 最后的错误信息
}

// TableStorageStore 定义了Table Storage相关的存储操作接口
// 这个接口替代了原来service层的TableStorageService，将其移动到store层
type TableStorageStore interface {
	// ReadSmsRecords 从存储中读取SMS记录（替代Java中的Table Storage读取）
	ReadSmsRecords(ctx context.Context, tableStorageName string, batchSize int) ([]*model.SmsRecordM, error)
	// ReadSmsRecordsByBatch 根据批次ID读取SMS记录
	ReadSmsRecordsByBatch(ctx context.Context, batchID string, batchSize int) ([]*model.SmsRecordM, error)
	// ReadSmsRecordsInBatches 分批读取SMS记录（懒读取实现，直到所有数据读取完）
	ReadSmsRecordsInBatches(ctx context.Context, query *model.SmsRecordQuery, batchSize int, callback func([]*model.SmsRecordM) error) error
	// ReadSmsRecordsLazy 高级懒读取SMS记录，支持断点续读和精细控制
	ReadSmsRecordsLazy(ctx context.Context, query *model.SmsRecordQuery, options *LazyReadOptions, callback func([]*model.SmsRecordM) error) (*LazyReadResult, error)
	// CreateSmsRecords 创建SMS记录到存储中
	CreateSmsRecords(ctx context.Context, records []*model.SmsRecordM) error
	// UpdateSmsRecordStatus 更新SMS记录状态
	UpdateSmsRecordStatus(ctx context.Context, recordID int64, status string) error
	// GetSmsRecordStats 获取SMS记录统计信息
	GetSmsRecordStats(ctx context.Context, batchID string) (*SmsRecordStats, error)
}

// AzureTableConfig Azure Table Storage配置
type AzureTableConfig struct {
	ConnectionString string
	TableName        string
}

// tableStorageStore 是 TableStorageStore 的具体实现，使用Azure Table Storage
type tableStorageStore struct {
	client    *aztables.Client
	tableName string
}

// 确保 tableStorageStore 实现了 TableStorageStore 接口
var _ TableStorageStore = (*tableStorageStore)(nil)

// newTableStorageStore 创建一个新的 TableStorageStore 实例
func newTableStorageStore(config *AzureTableConfig) (TableStorageStore, error) {
	// 从连接字符串创建服务客户端
	serviceClient, err := aztables.NewServiceClientFromConnectionString(config.ConnectionString, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Tables service client: %w", err)
	}

	// 创建表客户端
	client := serviceClient.NewClient(config.TableName)

	// 确保表存在
	_, err = serviceClient.CreateTable(context.Background(), config.TableName, nil)
	if err != nil {
		// 如果表已存在，忽略错误
		log.Infow("Table creation result", "table_name", config.TableName, "error", err)
	}

	return &tableStorageStore{
		client:    client,
		tableName: config.TableName,
	}, nil
}

// smsRecordToEntity 将SmsRecordM转换为Azure Table实体
func (s *tableStorageStore) smsRecordToEntity(record *model.SmsRecordM) (aztables.EDMEntity, error) {
	// 生成分区键和行键
	partitionKey := record.GeneratePartitionKey()
	rowKey := record.GenerateRowKey()

	// 序列化自定义字段
	customFieldsJSON := ""
	if record.CustomFields != nil {
		customFieldsBytes, err := json.Marshal(record.CustomFields)
		if err != nil {
			return aztables.EDMEntity{}, fmt.Errorf("failed to marshal custom fields: %w", err)
		}
		customFieldsJSON = string(customFieldsBytes)
	}

	entity := aztables.EDMEntity{
		Entity: aztables.Entity{
			PartitionKey: partitionKey,
			RowKey:       rowKey,
		},
		Properties: map[string]any{
			"ID":               record.ID,
			"BatchID":          record.BatchID,
			"TableStorageName": record.TableStorageName,
			"PhoneNumber":      record.PhoneNumber,
			"Content":          record.Content,
			"TemplateID":       record.TemplateID,
			"TemplateParams":   record.TemplateParams,
			"ExtCode":          record.ExtCode,
			"ProviderType":     record.ProviderType,
			"Status":           record.Status,
			"Priority":         record.Priority,
			"ErrorMessage":     record.ErrorMessage,
			"RetryCount":       record.RetryCount,
			"MaxRetries":       record.MaxRetries,
			"CustomFields":     customFieldsJSON,
			"CreatedAt":        aztables.EDMDateTime(record.CreatedAt),
			"UpdatedAt":        aztables.EDMDateTime(record.UpdatedAt),
		},
	}

	// 添加可选的时间字段
	if record.ScheduleTime != nil {
		entity.Properties["ScheduleTime"] = aztables.EDMDateTime(*record.ScheduleTime)
	}
	if record.SentTime != nil {
		entity.Properties["SentTime"] = aztables.EDMDateTime(*record.SentTime)
	}
	if record.DeliveredTime != nil {
		entity.Properties["DeliveredTime"] = aztables.EDMDateTime(*record.DeliveredTime)
	}

	return entity, nil
}

// entityToSmsRecord 将Azure Table实体转换为SmsRecordM
func (s *tableStorageStore) entityToSmsRecord(entity []byte) (*model.SmsRecordM, error) {
	var edmEntity aztables.EDMEntity
	err := json.Unmarshal(entity, &edmEntity)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal entity: %w", err)
	}

	record := &model.SmsRecordM{
		PartitionKey: edmEntity.PartitionKey,
		RowKey:       edmEntity.RowKey,
	}

	// 转换属性
	if val, ok := edmEntity.Properties["ID"]; ok {
		if id, ok := val.(float64); ok {
			record.ID = int64(id)
		}
	}
	if val, ok := edmEntity.Properties["BatchID"]; ok {
		if batchID, ok := val.(string); ok {
			record.BatchID = batchID
		}
	}
	if val, ok := edmEntity.Properties["TableStorageName"]; ok {
		if tableName, ok := val.(string); ok {
			record.TableStorageName = tableName
		}
	}
	if val, ok := edmEntity.Properties["PhoneNumber"]; ok {
		if phoneNumber, ok := val.(string); ok {
			record.PhoneNumber = phoneNumber
		}
	}
	if val, ok := edmEntity.Properties["Content"]; ok {
		if content, ok := val.(string); ok {
			record.Content = content
		}
	}
	if val, ok := edmEntity.Properties["TemplateID"]; ok {
		if templateID, ok := val.(string); ok {
			record.TemplateID = templateID
		}
	}
	if val, ok := edmEntity.Properties["TemplateParams"]; ok {
		if templateParams, ok := val.(string); ok {
			record.TemplateParams = templateParams
		}
	}
	if val, ok := edmEntity.Properties["ExtCode"]; ok {
		if extCode, ok := val.(float64); ok {
			record.ExtCode = int(extCode)
		}
	}
	if val, ok := edmEntity.Properties["ProviderType"]; ok {
		if providerType, ok := val.(string); ok {
			record.ProviderType = providerType
		}
	}
	if val, ok := edmEntity.Properties["Status"]; ok {
		if status, ok := val.(string); ok {
			record.Status = status
		}
	}
	if val, ok := edmEntity.Properties["Priority"]; ok {
		if priority, ok := val.(float64); ok {
			record.Priority = int(priority)
		}
	}
	if val, ok := edmEntity.Properties["ErrorMessage"]; ok {
		if errorMessage, ok := val.(string); ok {
			record.ErrorMessage = errorMessage
		}
	}
	if val, ok := edmEntity.Properties["RetryCount"]; ok {
		if retryCount, ok := val.(float64); ok {
			record.RetryCount = int(retryCount)
		}
	}
	if val, ok := edmEntity.Properties["MaxRetries"]; ok {
		if maxRetries, ok := val.(float64); ok {
			record.MaxRetries = int(maxRetries)
		}
	}

	// 处理自定义字段
	if val, ok := edmEntity.Properties["CustomFields"]; ok {
		if customFieldsJSON, ok := val.(string); ok && customFieldsJSON != "" {
			err := json.Unmarshal([]byte(customFieldsJSON), &record.CustomFields)
			if err != nil {
				log.Warnw("Failed to unmarshal custom fields", "error", err)
			}
		}
	}

	// 处理时间字段
	if val, ok := edmEntity.Properties["CreatedAt"]; ok {
		if timeStr, ok := val.(string); ok {
			if parsedTime, err := time.Parse(time.RFC3339, timeStr); err == nil {
				record.CreatedAt = parsedTime
			}
		}
	}
	if val, ok := edmEntity.Properties["UpdatedAt"]; ok {
		if timeStr, ok := val.(string); ok {
			if parsedTime, err := time.Parse(time.RFC3339, timeStr); err == nil {
				record.UpdatedAt = parsedTime
			}
		}
	}
	if val, ok := edmEntity.Properties["ScheduleTime"]; ok {
		if timeStr, ok := val.(string); ok {
			if parsedTime, err := time.Parse(time.RFC3339, timeStr); err == nil {
				record.ScheduleTime = &parsedTime
			}
		}
	}
	if val, ok := edmEntity.Properties["SentTime"]; ok {
		if timeStr, ok := val.(string); ok {
			if parsedTime, err := time.Parse(time.RFC3339, timeStr); err == nil {
				record.SentTime = &parsedTime
			}
		}
	}
	if val, ok := edmEntity.Properties["DeliveredTime"]; ok {
		if timeStr, ok := val.(string); ok {
			if parsedTime, err := time.Parse(time.RFC3339, timeStr); err == nil {
				record.DeliveredTime = &parsedTime
			}
		}
	}

	return record, nil
}

// ReadSmsRecords 从存储中读取SMS记录（替代Java中的Table Storage读取）
func (s *tableStorageStore) ReadSmsRecords(ctx context.Context, tableStorageName string, batchSize int) ([]*model.SmsRecordM, error) {
	log.Infow("Reading SMS records from storage", "table_storage_name", tableStorageName, "batch_size", batchSize)

	// 构建过滤器
	filter := fmt.Sprintf("TableStorageName eq '%s'", tableStorageName)
	
	// 查询选项
	batchSizeInt32 := int32(batchSize)
	options := &aztables.ListEntitiesOptions{
		Filter: &filter,
		Top:    &batchSizeInt32,
	}
	
	// 查询实体
	pager := s.client.NewListEntitiesPager(options)
	var records []*model.SmsRecordM
	
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list entities: %w", err)
		}
		
		for _, entity := range page.Entities {
			record, err := s.entityToSmsRecord(entity)
			if err != nil {
				log.Warnw("Failed to convert entity to SMS record", "error", err)
				continue
			}
			records = append(records, record)
		}
		

	}

	log.Infow("Successfully read SMS records", "table_storage_name", tableStorageName, "count", len(records))
	return records, nil
}

// ReadSmsRecordsByBatch 根据批次ID读取SMS记录
func (s *tableStorageStore) ReadSmsRecordsByBatch(ctx context.Context, batchID string, batchSize int) ([]*model.SmsRecordM, error) {
	log.Infow("Reading SMS records by batch ID", "batch_id", batchID, "batch_size", batchSize)

	// 构建过滤器
	filter := fmt.Sprintf("BatchID eq '%s'", batchID)
	
	// 查询选项
	batchSizeInt32 := int32(batchSize)
	options := &aztables.ListEntitiesOptions{
		Filter: &filter,
		Top:    &batchSizeInt32,
	}
	
	// 查询实体
	pager := s.client.NewListEntitiesPager(options)
	var records []*model.SmsRecordM
	
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list entities: %w", err)
		}
		
		for _, entity := range page.Entities {
			record, err := s.entityToSmsRecord(entity)
			if err != nil {
				log.Warnw("Failed to convert entity to SMS record", "error", err)
				continue
			}
			records = append(records, record)
		}
		
		// 如果已达到批次大小，停止
		if len(records) >= batchSize {
			break
		}
	}

	log.Infow("Successfully read SMS records by batch", "batch_id", batchID, "count", len(records))
	return records, nil
}

// ReadSmsRecordsInBatches 分批读取SMS记录（懒读取实现，直到所有数据读取完）
// 参考message-batch-service项目的懒读取方式，实现真正的流式处理
func (s *tableStorageStore) ReadSmsRecordsInBatches(ctx context.Context, query *model.SmsRecordQuery, batchSize int, callback func([]*model.SmsRecordM) error) error {
	log.Infow("Starting lazy loading SMS records in batches", "batch_size", batchSize)

	// 构建过滤器
	var filters []string
	
	if query.TableStorageName != "" {
		filters = append(filters, fmt.Sprintf("TableStorageName eq '%s'", query.TableStorageName))
	}
	if query.BatchID != "" {
		filters = append(filters, fmt.Sprintf("BatchID eq '%s'", query.BatchID))
	}
	if query.Status != "" {
		filters = append(filters, fmt.Sprintf("Status eq '%s'", query.Status))
	}
	if query.ProviderType != "" {
		filters = append(filters, fmt.Sprintf("ProviderType eq '%s'", query.ProviderType))
	}
	if query.PartitionKey != "" {
		filters = append(filters, fmt.Sprintf("PartitionKey eq '%s'", query.PartitionKey))
	}
	if query.PhoneNumber != "" {
		filters = append(filters, fmt.Sprintf("PhoneNumber eq '%s'", query.PhoneNumber))
	}
	
	var filter string
	if len(filters) > 0 {
		filter = strings.Join(filters, " and ")
	}
	
	// 查询选项 - 使用较小的批次大小进行懒读取
	batchSizeInt32 := int32(batchSize)
	options := &aztables.ListEntitiesOptions{
		Top: &batchSizeInt32,
	}
	if filter != "" {
		options.Filter = &filter
	}
	
	// 创建分页器进行懒读取
	pager := s.client.NewListEntitiesPager(options)
	totalProcessed := 0
	batchCount := 0
	
	// 懒读取循环 - 只有在需要时才读取下一页
	for pager.More() {
		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			log.Infow("Context cancelled, stopping lazy loading", "total_processed", totalProcessed)
			return ctx.Err()
		default:
		}

		// 懒读取下一页数据
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to lazy load next page: %w", err)
		}
		
		// 如果当前页没有数据，继续下一页
		if len(page.Entities) == 0 {
			log.Debugw("Empty page encountered, continuing lazy loading")
			continue
		}
		
		// 转换实体为SMS记录
		var records []*model.SmsRecordM
		for _, entity := range page.Entities {
			record, err := s.entityToSmsRecord(entity)
			if err != nil {
				log.Warnw("Failed to convert entity to SMS record during lazy loading", "error", err)
				continue
			}
			records = append(records, record)
		}
		
		// 只有当有有效记录时才调用回调
		if len(records) > 0 {
			batchCount++
			
			// 调用回调处理当前批次
			if err := callback(records); err != nil {
				log.Errorw("Callback failed during lazy loading", "batch_count", batchCount, "error", err)
				return fmt.Errorf("failed to process SMS records batch %d: %w", batchCount, err)
			}
			
			totalProcessed += len(records)
			
			log.Infow("Lazy loaded and processed SMS records batch",
				"batch_number", batchCount,
				"batch_size", len(records),
				"total_processed", totalProcessed,
				"has_more", pager.More())
		}

		// 添加小延迟，避免过度占用资源，实现真正的懒读取
		time.Sleep(50 * time.Millisecond)
	}

	log.Infow("Completed lazy loading all SMS records",
		"total_batches", batchCount,
		"total_processed", totalProcessed)

	return nil
}

// ReadSmsRecordsLazy 高级懒读取SMS记录，支持断点续读和精细控制
// 参考message-batch-service项目的高级懒读取实现
func (s *tableStorageStore) ReadSmsRecordsLazy(ctx context.Context, query *model.SmsRecordQuery, options *LazyReadOptions, callback func([]*model.SmsRecordM) error) (*LazyReadResult, error) {
	log.Infow("Starting advanced lazy loading SMS records", 
		"batch_size", options.BatchSize, 
		"max_batches", options.MaxBatches,
		"continuation_token", options.ContinuationToken)

	// 设置默认值
	if options.BatchSize <= 0 {
		options.BatchSize = 100
	}
	if options.DelayMs < 0 {
		options.DelayMs = 50
	}

	// 构建过滤器
	var filters []string
	
	if query.TableStorageName != "" {
		filters = append(filters, fmt.Sprintf("TableStorageName eq '%s'", query.TableStorageName))
	}
	if query.BatchID != "" {
		filters = append(filters, fmt.Sprintf("BatchID eq '%s'", query.BatchID))
	}
	if query.Status != "" {
		filters = append(filters, fmt.Sprintf("Status eq '%s'", query.Status))
	}
	if query.ProviderType != "" {
		filters = append(filters, fmt.Sprintf("ProviderType eq '%s'", query.ProviderType))
	}
	if query.PartitionKey != "" {
		filters = append(filters, fmt.Sprintf("PartitionKey eq '%s'", query.PartitionKey))
	}
	if query.PhoneNumber != "" {
		filters = append(filters, fmt.Sprintf("PhoneNumber eq '%s'", query.PhoneNumber))
	}
	
	var filter string
	if len(filters) > 0 {
		filter = strings.Join(filters, " and ")
	}
	
	// 查询选项
	batchSizeInt32 := int32(options.BatchSize)
	listOptions := &aztables.ListEntitiesOptions{
		Top: &batchSizeInt32,
	}
	if filter != "" {
		listOptions.Filter = &filter
	}
	
	// 如果有继续令牌，设置起始位置
	if options.ContinuationToken != "" {
		// Azure Table Storage 使用 NextPartitionKey 和 NextRowKey 进行分页
		// 这里简化处理，实际项目中可能需要解析 continuation token
		log.Debugw("Resuming from continuation token", "token", options.ContinuationToken)
	}
	
	// 创建分页器
	pager := s.client.NewListEntitiesPager(listOptions)
	totalProcessed := 0
	batchCount := 0
	var lastError string
	completed := false
	newContinuationToken := ""
	
	// 高级懒读取循环
	for pager.More() {
		// 检查是否达到最大批次限制
		if options.MaxBatches > 0 && batchCount >= options.MaxBatches {
			log.Infow("Reached maximum batch limit", "max_batches", options.MaxBatches)
			break
		}
		
		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			log.Infow("Context cancelled during advanced lazy loading", "total_processed", totalProcessed)
			lastError = ctx.Err().Error()
			return &LazyReadResult{
				TotalBatches:      batchCount,
				TotalProcessed:    totalProcessed,
				ContinuationToken: newContinuationToken,
				Completed:         false,
				LastError:         lastError,
			}, ctx.Err()
		default:
		}

		// 懒读取下一页数据
		page, err := pager.NextPage(ctx)
		if err != nil {
			lastError = fmt.Sprintf("failed to lazy load next page: %v", err)
			log.Errorw("Failed to load next page during advanced lazy loading", "error", err)
			
			if options.StopOnError {
				return &LazyReadResult{
					TotalBatches:      batchCount,
					TotalProcessed:    totalProcessed,
					ContinuationToken: newContinuationToken,
					Completed:         false,
					LastError:         lastError,
				}, fmt.Errorf(lastError)
			}
			continue
		}
		
		// 如果当前页没有数据，继续下一页
		if len(page.Entities) == 0 {
			log.Debugw("Empty page encountered during advanced lazy loading")
			continue
		}
		
		// 转换实体为SMS记录
		var records []*model.SmsRecordM
		for _, entity := range page.Entities {
			record, err := s.entityToSmsRecord(entity)
			if err != nil {
				log.Warnw("Failed to convert entity during advanced lazy loading", "error", err)
				if !options.StopOnError {
					continue
				}
				lastError = fmt.Sprintf("entity conversion failed: %v", err)
				return &LazyReadResult{
					TotalBatches:      batchCount,
					TotalProcessed:    totalProcessed,
					ContinuationToken: newContinuationToken,
					Completed:         false,
					LastError:         lastError,
				}, fmt.Errorf(lastError)
			}
			records = append(records, record)
		}
		
		// 只有当有有效记录时才调用回调
		if len(records) > 0 {
			batchCount++
			
			// 调用回调处理当前批次
			if err := callback(records); err != nil {
				lastError = fmt.Sprintf("callback failed for batch %d: %v", batchCount, err)
				log.Errorw("Callback failed during advanced lazy loading", "batch_count", batchCount, "error", err)
				
				if options.StopOnError {
					return &LazyReadResult{
						TotalBatches:      batchCount,
						TotalProcessed:    totalProcessed,
						ContinuationToken: newContinuationToken,
						Completed:         false,
						LastError:         lastError,
					}, fmt.Errorf(lastError)
				}
			}
			
			totalProcessed += len(records)
			
			// 生成继续令牌（简化实现）
			if len(records) > 0 {
				lastRecord := records[len(records)-1]
				newContinuationToken = fmt.Sprintf("%s_%s", lastRecord.PartitionKey, lastRecord.RowKey)
			}
			
			log.Infow("Advanced lazy loaded and processed SMS records batch",
				"batch_number", batchCount,
				"batch_size", len(records),
				"total_processed", totalProcessed,
				"has_more", pager.More(),
				"continuation_token", newContinuationToken)
		}

		// 添加延迟，实现真正的懒读取
		if options.DelayMs > 0 {
			time.Sleep(time.Duration(options.DelayMs) * time.Millisecond)
		}
	}
	
	// 检查是否完全读取完成
	completed = !pager.More() && (options.MaxBatches == 0 || batchCount < options.MaxBatches)
	
	result := &LazyReadResult{
		TotalBatches:      batchCount,
		TotalProcessed:    totalProcessed,
		ContinuationToken: newContinuationToken,
		Completed:         completed,
		LastError:         lastError,
	}

	log.Infow("Completed advanced lazy loading SMS records",
		"total_batches", result.TotalBatches,
		"total_processed", result.TotalProcessed,
		"completed", result.Completed,
		"continuation_token", result.ContinuationToken)

	return result, nil
}

// CreateSmsRecords 创建SMS记录到存储中
func (s *tableStorageStore) CreateSmsRecords(ctx context.Context, records []*model.SmsRecordM) error {
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

	// Azure Table Storage 支持批量操作，但有限制
	// 同一分区键的实体可以在一个事务中批量操作，最多100个实体
	
	// 按分区键分组
	partitionGroups := make(map[string][]*model.SmsRecordM)
	for _, record := range records {
		partitionKey := record.GeneratePartitionKey()
		partitionGroups[partitionKey] = append(partitionGroups[partitionKey], record)
	}
	
	// 为每个分区组执行批量操作
	for partitionKey, partitionRecords := range partitionGroups {
		// 分批处理，每批最多100个
		for i := 0; i < len(partitionRecords); i += 100 {
			end := i + 100
			if end > len(partitionRecords) {
				end = len(partitionRecords)
			}
			batch := partitionRecords[i:end]
			
			// 创建批量事务
			transactionActions := make([]aztables.TransactionAction, 0, len(batch))
			
			for _, record := range batch {
				entity, err := s.smsRecordToEntity(record)
				if err != nil {
					return fmt.Errorf("failed to convert record to entity: %w", err)
				}
				
					// 将实体转换为字节数组
				entityBytes, err := json.Marshal(entity)
				if err != nil {
					return fmt.Errorf("failed to marshal entity: %w", err)
				}
				
				transactionActions = append(transactionActions, aztables.TransactionAction{
					ActionType: aztables.TransactionTypeAdd,
					Entity:     entityBytes,
				})
			}
			
			// 执行批量事务
			_, err := s.client.SubmitTransaction(ctx, transactionActions, nil)
			if err != nil {
				// 如果批量操作失败，尝试单个插入
				log.Warnw("Batch insert failed, falling back to individual inserts", "partition", partitionKey, "error", err)
				
				for _, record := range batch {
					entity, err := s.smsRecordToEntity(record)
					if err != nil {
						return fmt.Errorf("failed to convert record to entity: %w", err)
					}
					
						entityBytes, err := json.Marshal(entity)
					if err != nil {
						return fmt.Errorf("failed to marshal entity: %w", err)
					}
					
					_, err = s.client.AddEntity(ctx, entityBytes, nil)
					if err != nil {
						return fmt.Errorf("failed to add entity: %w", err)
					}
				}
			}
		}
	}

	log.Infow("Successfully created SMS records", "count", len(records))
	return nil
}

// UpdateSmsRecordStatus 更新SMS记录状态
func (s *tableStorageStore) UpdateSmsRecordStatus(ctx context.Context, recordID int64, status string) error {
	log.Infow("Updating SMS record status", "record_id", recordID, "status", status)

	// 注意：Azure Table Storage需要PartitionKey和RowKey来更新实体
	// 这里需要根据recordID生成或查找对应的PartitionKey和RowKey
	// 暂时使用recordID作为RowKey，需要根据实际业务逻辑调整
	partitionKey := strconv.FormatInt(recordID/1000, 10) // 简单的分区策略
	rowKey := strconv.FormatInt(recordID, 10)

	// 构建更新的属性
	updateProperties := map[string]any{
		"Status":    status,
		"UpdatedAt": aztables.EDMDateTime(time.Now()),
	}
	
	// 创建更新实体
	updateEntity := aztables.EDMEntity{
		Entity: aztables.Entity{
			PartitionKey: partitionKey,
			RowKey:       rowKey,
		},
		Properties: updateProperties,
	}
	
	// 将实体转换为字节数组
	entityBytes, err := json.Marshal(updateEntity)
	if err != nil {
		return fmt.Errorf("failed to marshal update entity: %w", err)
	}
	
	// 执行更新操作（合并模式）
	_, err = s.client.UpdateEntity(ctx, entityBytes, &aztables.UpdateEntityOptions{
		UpdateMode: aztables.UpdateModeMerge,
	})
	if err != nil {
		return fmt.Errorf("failed to update entity status: %w", err)
	}

	log.Infow("Successfully updated SMS record status", "record_id", recordID, "status", status)
	return nil
}

// GetSmsRecordStats 获取SMS记录统计信息
func (s *tableStorageStore) GetSmsRecordStats(ctx context.Context, batchID string) (*SmsRecordStats, error) {
	log.Infow("Getting SMS record statistics", "batch_id", batchID)

	// Azure Table Storage 不支持聚合查询，需要遍历所有记录进行统计
	filter := fmt.Sprintf("BatchID eq '%s'", batchID)
	
	options := &aztables.ListEntitiesOptions{
		Filter: &filter,
	}
	
	stats := &SmsRecordStats{
		BatchID: batchID,
	}
	
	pager := s.client.NewListEntitiesPager(options)
	
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list entities for stats: %w", err)
		}
		
		for _, entity := range page.Entities {
			var edmEntity aztables.EDMEntity
			err := json.Unmarshal(entity, &edmEntity)
			if err != nil {
				log.Warnw("Failed to unmarshal entity for stats", "error", err)
				continue
			}
			
			stats.TotalCount++
			
			if statusVal, ok := edmEntity.Properties["Status"]; ok {
				if status, ok := statusVal.(string); ok {
					switch status {
					case model.SmsRecordStatusPending:
						stats.PendingCount++
					case model.SmsRecordStatusProcessing:
						stats.ProcessingCount++
					case model.SmsRecordStatusSent:
						stats.SentCount++
					case model.SmsRecordStatusDelivered:
						stats.DeliveredCount++
					case model.SmsRecordStatusFailed:
						stats.FailedCount++
					case model.SmsRecordStatusCanceled:
						stats.CanceledCount++
					}
				}
			}
		}
	}

	log.Infow("Successfully retrieved SMS record statistics",
		"batch_id", batchID,
		"total_count", stats.TotalCount,
		"pending_count", stats.PendingCount,
		"sent_count", stats.SentCount,
		"failed_count", stats.FailedCount)

	return stats, nil
}