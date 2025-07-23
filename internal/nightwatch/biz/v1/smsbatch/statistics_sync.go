package smsbatch

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/onexstack/onexstack/pkg/log"
	"github.com/onexstack/onexstack/pkg/store/where"
	"github.com/redis/go-redis/v9"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
)

// StatisticsSync 统计同步器，用于将批处理统计信息同步到Redis
type StatisticsSync struct {
	store       store.IStore
	redisClient *redis.Client
	keyPrefix   string
	ttl         time.Duration
}

// BatchStatistics 批处理统计信息
type BatchStatistics struct {
	BatchID          string    `json:"batch_id"`
	TotalRecords     int64     `json:"total_records"`
	ProcessedRecords int64     `json:"processed_records"`
	SuccessRecords   int64     `json:"success_records"`
	FailedRecords    int64     `json:"failed_records"`
	ThroughputPerSec float64   `json:"throughput_per_sec"`
	LastUpdated      time.Time `json:"last_updated"`
	Status           string    `json:"status"`
	Phase            string    `json:"phase"`
}

// NewStatisticsSync 创建统计同步器
func NewStatisticsSync(store store.IStore, redisClient *redis.Client) *StatisticsSync {
	return &StatisticsSync{
		store:       store,
		redisClient: redisClient,
		keyPrefix:   "smsbatch:stats:",
		ttl:         24 * time.Hour, // 统计信息缓存24小时
	}
}

// SyncBatchStatistics 同步单个批处理的统计信息到Redis
func (s *StatisticsSync) SyncBatchStatistics(ctx context.Context, batchID string) error {
	// 获取批次信息
	smsBatch, err := s.store.SmsBatch().Get(ctx, where.F("batch_id", batchID))
	if err != nil {
		log.Errorw(err, "Failed to get SMS batch for statistics sync", "batch_id", batchID)
		return err
	}

	// 构建统计信息
	stats := &BatchStatistics{
		BatchID:     smsBatch.BatchID,
		Status:      smsBatch.Status,
		LastUpdated: time.Now(),
	}

	// 从Results中提取统计数据
	if smsBatch.Results != nil {
		stats.TotalRecords = smsBatch.Results.TotalMessages
		stats.ProcessedRecords = smsBatch.Results.ProcessedMessages
		stats.SuccessRecords = smsBatch.Results.SuccessMessages
		stats.FailedRecords = smsBatch.Results.FailedMessages
		stats.Phase = smsBatch.Results.CurrentPhase
		// 计算吞吐量（简单估算）
		if smsBatch.Results.ProcessedMessages > 0 {
			duration := time.Since(smsBatch.CreatedAt).Seconds()
			if duration > 0 {
				stats.ThroughputPerSec = float64(smsBatch.Results.ProcessedMessages) / duration
			}
		}
	}

	// 序列化统计信息
	statsJSON, err := json.Marshal(stats)
	if err != nil {
		log.Errorw(err, "Failed to marshal batch statistics", "batch_id", batchID)
		return err
	}

	// 存储到Redis
	key := s.keyPrefix + batchID
	if err := s.redisClient.Set(ctx, key, statsJSON, s.ttl).Err(); err != nil {
		log.Errorw(err, "Failed to store batch statistics to Redis", "batch_id", batchID, "key", key)
		return err
	}

	log.Debugw("Batch statistics synced to Redis", "batch_id", batchID, "key", key)
	return nil
}

// SyncAllActiveBatches 同步所有活跃批处理的统计信息
func (s *StatisticsSync) SyncAllActiveBatches(ctx context.Context) error {
	// 查询所有活跃的批次（非终止状态）
	activeBatches, err := s.getActiveBatches(ctx)
	if err != nil {
		log.Errorw(err, "Failed to get active batches for statistics sync")
		return err
	}

	log.Infow("Starting statistics sync for active batches", "count", len(activeBatches))

	// 同步每个活跃批次的统计信息
	for _, batch := range activeBatches {
		if err := s.SyncBatchStatistics(ctx, batch.BatchID); err != nil {
			log.Errorw(err, "Failed to sync statistics for batch", "batch_id", batch.BatchID)
			// 继续处理其他批次，不因单个失败而中断
			continue
		}
	}

	log.Infow("Completed statistics sync for active batches", "count", len(activeBatches))
	return nil
}

// GetBatchStatisticsFromCache 从Redis缓存获取批处理统计信息
func (s *StatisticsSync) GetBatchStatisticsFromCache(ctx context.Context, batchID string) (*BatchStatistics, error) {
	key := s.keyPrefix + batchID
	statsJSON, err := s.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("batch statistics not found in cache: %s", batchID)
		}
		log.Errorw(err, "Failed to get batch statistics from Redis", "batch_id", batchID, "key", key)
		return nil, err
	}

	var stats BatchStatistics
	if err := json.Unmarshal([]byte(statsJSON), &stats); err != nil {
		log.Errorw(err, "Failed to unmarshal batch statistics", "batch_id", batchID)
		return nil, err
	}

	return &stats, nil
}

// InvalidateBatchCache 使批处理缓存失效
func (s *StatisticsSync) InvalidateBatchCache(ctx context.Context, batchID string) error {
	key := s.keyPrefix + batchID
	if err := s.redisClient.Del(ctx, key).Err(); err != nil {
		log.Errorw(err, "Failed to invalidate batch cache", "batch_id", batchID, "key", key)
		return err
	}

	log.Debugw("Batch cache invalidated", "batch_id", batchID, "key", key)
	return nil
}

// CleanupExpiredCache 清理过期的缓存数据
func (s *StatisticsSync) CleanupExpiredCache(ctx context.Context) error {
	// 获取所有统计缓存键
	pattern := s.keyPrefix + "*"
	keys, err := s.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		log.Errorw(err, "Failed to get cache keys for cleanup", "pattern", pattern)
		return err
	}

	if len(keys) == 0 {
		log.Debugw("No cache keys found for cleanup", "pattern", pattern)
		return nil
	}

	cleanedCount := 0
	for _, key := range keys {
		// 检查键是否存在（可能已过期）
		exists, err := s.redisClient.Exists(ctx, key).Result()
		if err != nil {
			log.Errorw(err, "Failed to check key existence", "key", key)
			continue
		}

		if exists == 0 {
			cleanedCount++
		}
	}

	log.Infow("Cache cleanup completed", "total_keys", len(keys), "cleaned_count", cleanedCount)
	return nil
}

// getActiveBatches 获取所有活跃的批次
func (s *StatisticsSync) getActiveBatches(ctx context.Context) ([]*model.SmsBatchM, error) {
	// 查询非终止状态的批次
	whr := where.T(ctx).F("status", []string{"pending", "running", "paused", "preparation", "delivery"})

	_, batches, err := s.store.SmsBatch().List(ctx, whr)
	if err != nil {
		return nil, err
	}

	return batches, nil
}

// GetSyncMetrics 获取同步指标
func (s *StatisticsSync) GetSyncMetrics(ctx context.Context) (map[string]interface{}, error) {
	// 获取缓存中的键数量
	pattern := s.keyPrefix + "*"
	keys, err := s.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	// 获取活跃批次数量
	activeBatches, err := s.getActiveBatches(ctx)
	if err != nil {
		return nil, err
	}

	metrics := map[string]interface{}{
		"cached_batches_count": len(keys),
		"active_batches_count": len(activeBatches),
		"cache_key_prefix":     s.keyPrefix,
		"cache_ttl_hours":      s.ttl.Hours(),
		"last_check_time":      time.Now(),
	}

	return metrics, nil
}
