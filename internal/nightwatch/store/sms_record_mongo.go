// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

package store

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// SmsRecordMongoStore 定义了 sms_record 模块在 MongoDB store 层所实现的方法.
// 这个接口替代了Java项目中的Table Storage功能
type SmsRecordMongoStore interface {
	// CreateBatch 批量创建SMS记录
	CreateBatch(ctx context.Context, records []*model.SmsRecordM) error
	// GetByBatchID 根据批次ID获取SMS记录
	GetByBatchID(ctx context.Context, batchID string, limit, offset int) ([]*model.SmsRecordM, int64, error)
	// GetByTableStorageName 根据Table Storage名称获取SMS记录（兼容Java项目）
	GetByTableStorageName(ctx context.Context, tableStorageName string, limit, offset int) ([]*model.SmsRecordM, int64, error)
	// Query 根据查询条件获取SMS记录
	Query(ctx context.Context, query *model.SmsRecordQuery) ([]*model.SmsRecordM, int64, error)
	// Update 更新SMS记录
	Update(ctx context.Context, record *model.SmsRecordM) error
	// UpdateBatch 批量更新SMS记录状态
	UpdateBatch(ctx context.Context, batchID string, status string) error
	// Delete 删除SMS记录
	Delete(ctx context.Context, id int64) error
	// DeleteByBatchID 根据批次ID删除SMS记录
	DeleteByBatchID(ctx context.Context, batchID string) error
	// GetPendingRecords 获取待处理的SMS记录
	GetPendingRecords(ctx context.Context, limit int) ([]*model.SmsRecordM, error)
	// GetFailedRecords 获取失败的SMS记录（可重试）
	GetFailedRecords(ctx context.Context, limit int) ([]*model.SmsRecordM, error)
	// CountByStatus 根据状态统计SMS记录数量
	CountByStatus(ctx context.Context, batchID string, status string) (int64, error)
}

// smsRecordMongoStore 是 SmsRecordMongoStore 的一个具体实现.
type smsRecordMongoStore struct {
	mongoManager MongoManager
}

// 确保 smsRecordMongoStore 实现了 SmsRecordMongoStore 接口.
var _ SmsRecordMongoStore = (*smsRecordMongoStore)(nil)

// NewSmsRecordMongoStore 创建一个 smsRecordMongoStore 实例.
func NewSmsRecordMongoStore(mongoManager MongoManager) SmsRecordMongoStore {
	return &smsRecordMongoStore{
		mongoManager: mongoManager,
	}
}

// CreateBatch 批量创建SMS记录到 MongoDB.
func (s *smsRecordMongoStore) CreateBatch(ctx context.Context, records []*model.SmsRecordM) error {
	if len(records) == 0 {
		return nil
	}

	// 设置创建时间和更新时间
	now := time.Now()
	documents := make([]interface{}, len(records))

	for i, record := range records {
		if record.ID == 0 {
			record.ID = time.Now().UnixNano() + int64(i) // 确保ID唯一
		}
		record.CreatedAt = now
		record.UpdatedAt = now

		// 生成分区键和行键
		record.GeneratePartitionKey()
		record.GenerateRowKey()

		// 转换为BSON文档
		doc, err := bson.Marshal(record)
		if err != nil {
			return fmt.Errorf("failed to marshal sms record %d: %w", i, err)
		}

		var bsonDoc bson.M
		if err := bson.Unmarshal(doc, &bsonDoc); err != nil {
			return fmt.Errorf("failed to unmarshal to bson.M for record %d: %w", i, err)
		}

		documents[i] = bsonDoc
	}

	collection := s.mongoManager.GetCollection("sms_records")
	_, err := collection.InsertMany(ctx, documents)
	if err != nil {
		return fmt.Errorf("failed to insert sms records batch: %w", err)
	}

	log.Infow("SMS records batch created in MongoDB", "count", len(records))
	return nil
}

// GetByBatchID 根据批次ID获取SMS记录.
func (s *smsRecordMongoStore) GetByBatchID(ctx context.Context, batchID string, limit, offset int) ([]*model.SmsRecordM, int64, error) {
	filter := bson.M{"batch_id": batchID}
	return s.queryRecords(ctx, filter, limit, offset)
}

// GetByTableStorageName 根据Table Storage名称获取SMS记录（兼容Java项目）.
func (s *smsRecordMongoStore) GetByTableStorageName(ctx context.Context, tableStorageName string, limit, offset int) ([]*model.SmsRecordM, int64, error) {
	filter := bson.M{"table_storage_name": tableStorageName}
	return s.queryRecords(ctx, filter, limit, offset)
}

// Query 根据查询条件获取SMS记录.
func (s *smsRecordMongoStore) Query(ctx context.Context, query *model.SmsRecordQuery) ([]*model.SmsRecordM, int64, error) {
	filter := s.buildQueryFilter(query)
	limit := query.Limit
	offset := query.Offset

	if limit <= 0 {
		limit = 100 // 默认限制
	}

	return s.queryRecords(ctx, filter, limit, offset)
}

// Update 更新SMS记录.
func (s *smsRecordMongoStore) Update(ctx context.Context, record *model.SmsRecordM) error {
	// 设置更新时间
	record.UpdatedAt = time.Now()

	// 转换为BSON文档
	doc, err := bson.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal sms record: %w", err)
	}

	var bsonDoc bson.M
	if err := bson.Unmarshal(doc, &bsonDoc); err != nil {
		return fmt.Errorf("failed to unmarshal to bson.M: %w", err)
	}

	// 使用ID作为查询条件
	filter := bson.M{"_id": record.ID}
	update := bson.M{"$set": bsonDoc}

	collection := s.mongoManager.GetCollection("sms_records")
	_, err = collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update sms record: %w", err)
	}

	log.Infow("SMS record updated in MongoDB", "id", record.ID, "batch_id", record.BatchID)
	return nil
}

// UpdateBatch 批量更新SMS记录状态.
func (s *smsRecordMongoStore) UpdateBatch(ctx context.Context, batchID string, status string) error {
	filter := bson.M{"batch_id": batchID}
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	}

	collection := s.mongoManager.GetCollection("sms_records")
	result, err := collection.UpdateMany(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update sms records batch: %w", err)
	}

	log.Infow("SMS records batch updated in MongoDB", "batch_id", batchID, "status", status, "modified_count", result.ModifiedCount)
	return nil
}

// Delete 删除SMS记录.
func (s *smsRecordMongoStore) Delete(ctx context.Context, id int64) error {
	filter := bson.M{"_id": id}

	collection := s.mongoManager.GetCollection("sms_records")
	_, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete sms record: %w", err)
	}

	return nil
}

// DeleteByBatchID 根据批次ID删除SMS记录.
func (s *smsRecordMongoStore) DeleteByBatchID(ctx context.Context, batchID string) error {
	filter := bson.M{"batch_id": batchID}

	collection := s.mongoManager.GetCollection("sms_records")
	result, err := collection.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete sms records by batch_id: %w", err)
	}

	log.Infow("SMS records deleted by batch_id", "batch_id", batchID, "deleted_count", result.DeletedCount)
	return nil
}

// GetPendingRecords 获取待处理的SMS记录.
func (s *smsRecordMongoStore) GetPendingRecords(ctx context.Context, limit int) ([]*model.SmsRecordM, error) {
	filter := bson.M{"status": model.SmsRecordStatusPending}
	records, _, err := s.queryRecords(ctx, filter, limit, 0)
	return records, err
}

// GetFailedRecords 获取失败的SMS记录（可重试）.
func (s *smsRecordMongoStore) GetFailedRecords(ctx context.Context, limit int) ([]*model.SmsRecordM, error) {
	filter := bson.M{
		"status": model.SmsRecordStatusFailed,
		"$expr": bson.M{
			"$lt": []interface{}{"$retry_count", "$max_retries"},
		},
	}
	records, _, err := s.queryRecords(ctx, filter, limit, 0)
	return records, err
}

// CountByStatus 根据状态统计SMS记录数量.
func (s *smsRecordMongoStore) CountByStatus(ctx context.Context, batchID string, status string) (int64, error) {
	filter := bson.M{
		"batch_id": batchID,
		"status":   status,
	}

	collection := s.mongoManager.GetCollection("sms_records")
	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to count sms records: %w", err)
	}

	return count, nil
}

// queryRecords 通用查询方法.
func (s *smsRecordMongoStore) queryRecords(ctx context.Context, filter bson.M, limit, offset int) ([]*model.SmsRecordM, int64, error) {
	collection := s.mongoManager.GetCollection("sms_records")

	// 获取总数
	totalCount, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count documents: %w", err)
	}

	// 构建查询选项
	findOptions := options.Find()
	if limit > 0 {
		findOptions.SetLimit(int64(limit))
	}
	if offset > 0 {
		findOptions.SetSkip(int64(offset))
	}
	// 按创建时间排序
	findOptions.SetSort(bson.D{{"created_at", 1}})

	// 执行查询
	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find documents: %w", err)
	}
	defer cursor.Close(ctx)

	// 解析结果
	var results []*model.SmsRecordM
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return nil, 0, fmt.Errorf("failed to decode document: %w", err)
		}

		smsRecord, err := s.unmarshalFromBSON(doc)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal document: %w", err)
		}

		results = append(results, smsRecord)
	}

	if err := cursor.Err(); err != nil {
		return nil, 0, fmt.Errorf("cursor error: %w", err)
	}

	return results, totalCount, nil
}

// buildQueryFilter 构建查询过滤器.
func (s *smsRecordMongoStore) buildQueryFilter(query *model.SmsRecordQuery) bson.M {
	filter := bson.M{}

	if query.BatchID != "" {
		filter["batch_id"] = query.BatchID
	}
	if query.TableStorageName != "" {
		filter["table_storage_name"] = query.TableStorageName
	}
	if query.Status != "" {
		filter["status"] = query.Status
	}
	if query.ProviderType != "" {
		filter["provider_type"] = query.ProviderType
	}
	if query.PhoneNumber != "" {
		filter["phone_number"] = query.PhoneNumber
	}
	if query.PartitionKey != "" {
		filter["partition_key"] = query.PartitionKey
	}

	// 时间范围查询
	if query.CreatedAfter != nil || query.CreatedBefore != nil {
		timeFilter := bson.M{}
		if query.CreatedAfter != nil {
			timeFilter["$gte"] = *query.CreatedAfter
		}
		if query.CreatedBefore != nil {
			timeFilter["$lte"] = *query.CreatedBefore
		}
		filter["created_at"] = timeFilter
	}

	return filter
}

// unmarshalFromBSON 从BSON文档转换为SmsRecordM结构.
func (s *smsRecordMongoStore) unmarshalFromBSON(doc bson.M) (*model.SmsRecordM, error) {
	bsonDoc, err := bson.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal bson document: %w", err)
	}

	var smsRecord model.SmsRecordM
	if err := bson.Unmarshal(bsonDoc, &smsRecord); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to SmsRecordM: %w", err)
	}

	return &smsRecord, nil
}
