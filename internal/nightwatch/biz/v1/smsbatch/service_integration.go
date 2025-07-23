package smsbatch

import (
	"context"
	"fmt"
	"time"

	"github.com/onexstack/onexstack/pkg/log"
	"github.com/onexstack/onexstack/pkg/store/where"
	"github.com/redis/go-redis/v9"

	"github.com/ashwinyue/dcp/internal/nightwatch/store"
)

// SmsBatchServiceIntegration 短信批处理服务集成器
// 整合所有业务逻辑组件，提供统一的服务接口
type SmsBatchServiceIntegration struct {
	smsBatchService ISmsBatchV1
	statisticsSync  *StatisticsSync
	taskScheduler   *TaskScheduler
	stepFactory     StepFactory
	store           store.IStore
	redisClient     *redis.Client
}

// ServiceConfig 服务配置
type ServiceConfig struct {
	RedisClient           *redis.Client
	StatisticsSyncEnabled bool
	TaskSchedulerEnabled  bool
	DefaultCronSpecs      map[string]string
}

// NewSmsBatchServiceIntegration 创建短信批处理服务集成器
func NewSmsBatchServiceIntegration(store store.IStore, config *ServiceConfig) *SmsBatchServiceIntegration {
	// 创建基础服务
	smsBatchService := New(store)

	// 创建统计同步器
	var statisticsSync *StatisticsSync
	if config.StatisticsSyncEnabled && config.RedisClient != nil {
		statisticsSync = NewStatisticsSync(store, config.RedisClient)
	}

	// 创建步骤工厂
	stepFactory := NewStepFactory(store)

	// 创建任务调度器
	var taskScheduler *TaskScheduler
	if config.TaskSchedulerEnabled {
		taskScheduler = NewTaskScheduler(store, smsBatchService, statisticsSync)
	}

	integration := &SmsBatchServiceIntegration{
		smsBatchService: smsBatchService,
		statisticsSync:  statisticsSync,
		taskScheduler:   taskScheduler,
		stepFactory:     stepFactory,
		store:           store,
		redisClient:     config.RedisClient,
	}

	// 初始化默认定时任务
	if config.TaskSchedulerEnabled && config.DefaultCronSpecs != nil {
		integration.initializeDefaultTasks(context.Background(), config.DefaultCronSpecs)
	}

	return integration
}

// GetSmsBatchService 获取短信批处理服务
func (si *SmsBatchServiceIntegration) GetSmsBatchService() ISmsBatchV1 {
	return si.smsBatchService
}

// GetStatisticsSync 获取统计同步器
func (si *SmsBatchServiceIntegration) GetStatisticsSync() *StatisticsSync {
	return si.statisticsSync
}

// GetTaskScheduler 获取任务调度器
func (si *SmsBatchServiceIntegration) GetTaskScheduler() *TaskScheduler {
	return si.taskScheduler
}

// GetStepFactory 获取步骤工厂
func (si *SmsBatchServiceIntegration) GetStepFactory() StepFactory {
	return si.stepFactory
}

// ProcessBatchWithSteps 使用步骤链处理批次
func (si *SmsBatchServiceIntegration) ProcessBatchWithSteps(ctx context.Context, batchID string, stepTypes []string) error {
	// 获取批次信息
	batch, err := si.store.SmsBatch().Get(ctx, where.F("batch_id", batchID))
	if err != nil {
		log.Errorw(err, "Failed to get batch for step processing", "batch_id", batchID)
		return err
	}

	// 创建步骤链
	steps, err := si.stepFactory.CreateStepChain(stepTypes, nil)
	if err != nil {
		log.Errorw(err, "Failed to create step chain", "batch_id", batchID, "step_types", stepTypes)
		return err
	}

	// 执行步骤链
	for _, step := range steps {
		log.Infow("Executing step", "batch_id", batchID, "step_name", step.GetName(), "step_type", step.GetType())

		// 检查是否可以执行
		if !step.CanExecute(ctx, batch) {
			log.Warnw("Step cannot be executed, skipping", "batch_id", batchID, "step_name", step.GetName())
			continue
		}

		// 验证步骤
		if err := step.Validate(ctx, batch); err != nil {
			log.Errorw(err, "Step validation failed", "batch_id", batchID, "step_name", step.GetName())
			return fmt.Errorf("step %s validation failed: %w", step.GetName(), err)
		}

		// 执行步骤
		if step.IsAsync() {
			// 异步执行
			go func(s StepProcessor) {
				ctx, cancel := context.WithTimeout(context.Background(), s.GetTimeout())
				defer cancel()

				if err := s.Execute(ctx, batch); err != nil {
					log.Errorw(err, "Async step execution failed", "batch_id", batchID, "step_name", s.GetName())
				}
			}(step)
		} else {
			// 同步执行
			ctx, cancel := context.WithTimeout(ctx, step.GetTimeout())
			if err := step.Execute(ctx, batch); err != nil {
				cancel()
				log.Errorw(err, "Step execution failed", "batch_id", batchID, "step_name", step.GetName())
				return fmt.Errorf("step %s execution failed: %w", step.GetName(), err)
			}
			cancel()
		}

		// 重新获取批次信息（可能被步骤更新）
		batch, err = si.store.SmsBatch().Get(ctx, where.F("batch_id", batchID))
		if err != nil {
			log.Errorw(err, "Failed to refresh batch after step execution", "batch_id", batchID, "step_name", step.GetName())
			return err
		}
	}

	log.Infow("Batch processing with steps completed", "batch_id", batchID, "steps_count", len(steps))
	return nil
}

// ScheduleBatchWithSteps 调度带步骤的批处理
func (si *SmsBatchServiceIntegration) ScheduleBatchWithSteps(ctx context.Context, batchID string, stepTypes []string, scheduleTime time.Time) error {
	if si.taskScheduler == nil {
		return fmt.Errorf("task scheduler is not enabled")
	}

	// 验证批次和步骤
	if err := si.smsBatchService.ValidateBatch(ctx, batchID); err != nil {
		return fmt.Errorf("batch validation failed: %w", err)
	}

	// 验证步骤链
	steps, err := si.stepFactory.CreateStepChain(stepTypes, nil)
	if err != nil {
		return fmt.Errorf("step chain validation failed: %w", err)
	}

	log.Infow("Scheduling batch with steps", "batch_id", batchID, "step_types", stepTypes, "schedule_time", scheduleTime, "steps_count", len(steps))

	// 调度批处理任务
	_, err = si.taskScheduler.ScheduleBatchProcessing(ctx, batchID, scheduleTime)
	return err
}

// GetBatchStatusWithProgress 获取批次状态和进度
func (si *SmsBatchServiceIntegration) GetBatchStatusWithProgress(ctx context.Context, batchID string) (*BatchStatusWithProgress, error) {
	// 获取状态
	status, err := si.smsBatchService.GetBatchStatus(ctx, batchID)
	if err != nil {
		return nil, err
	}

	// 获取进度
	progress, err := si.smsBatchService.GetBatchProgress(ctx, batchID)
	if err != nil {
		return nil, err
	}

	// 尝试从缓存获取统计信息
	var cachedStats *BatchStatistics
	if si.statisticsSync != nil {
		cachedStats, _ = si.statisticsSync.GetBatchStatisticsFromCache(ctx, batchID)
	}

	return &BatchStatusWithProgress{
		Status:      status,
		Progress:    progress,
		CachedStats: cachedStats,
		LastUpdated: time.Now(),
	}, nil
}

// BatchStatusWithProgress 批次状态和进度信息
type BatchStatusWithProgress struct {
	Status      *BatchStatus     `json:"status"`
	Progress    *BatchProgress   `json:"progress"`
	CachedStats *BatchStatistics `json:"cached_stats,omitempty"`
	LastUpdated time.Time        `json:"last_updated"`
}

// SyncBatchStatistics 同步批次统计信息
func (si *SmsBatchServiceIntegration) SyncBatchStatistics(ctx context.Context, batchID string) error {
	if si.statisticsSync == nil {
		return fmt.Errorf("statistics sync is not enabled")
	}

	return si.statisticsSync.SyncBatchStatistics(ctx, batchID)
}

// GetServiceMetrics 获取服务指标
func (si *SmsBatchServiceIntegration) GetServiceMetrics(ctx context.Context) (map[string]interface{}, error) {
	metrics := map[string]interface{}{
		"service_name":    "smsbatch_integration",
		"last_check_time": time.Now(),
	}

	// 获取统计同步指标
	if si.statisticsSync != nil {
		syncMetrics, err := si.statisticsSync.GetSyncMetrics(ctx)
		if err == nil {
			metrics["statistics_sync"] = syncMetrics
		}
	}

	// 获取任务调度指标
	if si.taskScheduler != nil {
		taskMetrics, err := si.taskScheduler.GetTaskMetrics(ctx)
		if err == nil {
			metrics["task_scheduler"] = taskMetrics
		}
	}

	// 获取步骤工厂指标
	availableSteps := si.stepFactory.GetAvailableSteps()
	metrics["step_factory"] = map[string]interface{}{
		"available_steps_count": len(availableSteps),
		"available_steps":       availableSteps,
	}

	return metrics, nil
}

// initializeDefaultTasks 初始化默认定时任务
func (si *SmsBatchServiceIntegration) initializeDefaultTasks(ctx context.Context, cronSpecs map[string]string) {
	if si.taskScheduler == nil {
		return
	}

	// 初始化统计同步任务
	if syncSpec, exists := cronSpecs["statistics_sync"]; exists && si.statisticsSync != nil {
		if _, err := si.taskScheduler.ScheduleRecurringStatisticsSync(ctx, syncSpec); err != nil {
			log.Errorw(err, "Failed to schedule recurring statistics sync task")
		} else {
			log.Infow("Scheduled recurring statistics sync task", "cron_spec", syncSpec)
		}
	}

	// 初始化清理任务
	if cleanupSpec, exists := cronSpecs["cleanup"]; exists {
		if _, err := si.taskScheduler.ScheduleRecurringCleanup(ctx, cleanupSpec); err != nil {
			log.Errorw(err, "Failed to schedule recurring cleanup task")
		} else {
			log.Infow("Scheduled recurring cleanup task", "cron_spec", cleanupSpec)
		}
	}

	log.Infow("Default tasks initialization completed", "cron_specs", cronSpecs)
}

// Shutdown 关闭服务
func (si *SmsBatchServiceIntegration) Shutdown(ctx context.Context) error {
	log.Infow("Shutting down SMS batch service integration")

	// 这里可以添加清理逻辑
	// 例如：停止定时任务、关闭连接等

	log.Infow("SMS batch service integration shutdown completed")
	return nil
}

// HealthCheck 健康检查
func (si *SmsBatchServiceIntegration) HealthCheck(ctx context.Context) error {
	// 检查存储连接
	if si.store == nil {
		return fmt.Errorf("store is not available")
	}

	// 检查Redis连接（如果启用）
	if si.redisClient != nil {
		if err := si.redisClient.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("redis connection failed: %w", err)
		}
	}

	return nil
}
