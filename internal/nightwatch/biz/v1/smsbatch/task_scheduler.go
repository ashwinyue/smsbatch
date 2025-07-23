package smsbatch

import (
	"context"
	"fmt"
	"time"

	"github.com/onexstack/onexstack/pkg/log"
	"github.com/onexstack/onexstack/pkg/store/where"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
)

// TaskScheduler 任务调度器，与cronjob集成管理批处理定时任务
type TaskScheduler struct {
	store           store.IStore
	smsBatchService ISmsBatchV1
	statisticsSync  *StatisticsSync
}

// ScheduledTask 调度任务信息
type ScheduledTask struct {
	TaskID       string                 `json:"task_id"`
	BatchID      string                 `json:"batch_id"`
	TaskType     string                 `json:"task_type"` // "batch_processing", "statistics_sync", "cleanup"
	ScheduleTime time.Time              `json:"schedule_time"`
	CronSpec     string                 `json:"cron_spec,omitempty"`
	IsRecurring  bool                   `json:"is_recurring"`
	Status       string                 `json:"status"` // "pending", "running", "completed", "failed", "cancelled"
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// NewTaskScheduler 创建任务调度器
func NewTaskScheduler(store store.IStore, smsBatchService ISmsBatchV1, statisticsSync *StatisticsSync) *TaskScheduler {
	return &TaskScheduler{
		store:           store,
		smsBatchService: smsBatchService,
		statisticsSync:  statisticsSync,
	}
}

// ScheduleBatchProcessing 调度批处理任务
func (ts *TaskScheduler) ScheduleBatchProcessing(ctx context.Context, batchID string, scheduleTime time.Time) (*ScheduledTask, error) {
	// 验证批次存在
	if err := ts.smsBatchService.ValidateBatch(ctx, batchID); err != nil {
		log.Errorw(err, "Failed to validate batch for scheduling", "batch_id", batchID)
		return nil, err
	}

	// 创建调度任务
	task := &ScheduledTask{
		TaskID:       fmt.Sprintf("batch_processing_%s_%d", batchID, time.Now().Unix()),
		BatchID:      batchID,
		TaskType:     "batch_processing",
		ScheduleTime: scheduleTime,
		IsRecurring:  false,
		Status:       "pending",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// 如果是立即执行
	if scheduleTime.Before(time.Now()) || scheduleTime.Equal(time.Now()) {
		return ts.executeTask(ctx, task)
	}

	// 否则创建cronjob任务
	return ts.createCronJobTask(ctx, task)
}

// ScheduleRecurringStatisticsSync 调度周期性统计同步任务
func (ts *TaskScheduler) ScheduleRecurringStatisticsSync(ctx context.Context, cronSpec string) (*ScheduledTask, error) {
	task := &ScheduledTask{
		TaskID:      fmt.Sprintf("statistics_sync_%d", time.Now().Unix()),
		TaskType:    "statistics_sync",
		CronSpec:    cronSpec,
		IsRecurring: true,
		Status:      "pending",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return ts.createCronJobTask(ctx, task)
}

// ScheduleRecurringCleanup 调度周期性清理任务
func (ts *TaskScheduler) ScheduleRecurringCleanup(ctx context.Context, cronSpec string) (*ScheduledTask, error) {
	task := &ScheduledTask{
		TaskID:      fmt.Sprintf("cleanup_%d", time.Now().Unix()),
		TaskType:    "cleanup",
		CronSpec:    cronSpec,
		IsRecurring: true,
		Status:      "pending",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return ts.createCronJobTask(ctx, task)
}

// CancelScheduledTask 取消调度任务
func (ts *TaskScheduler) CancelScheduledTask(ctx context.Context, taskID string) error {
	// 获取cronjob任务
	cronJob, err := ts.store.CronJob().Get(ctx, where.F("name", taskID))
	if err != nil {
		log.Errorw(err, "Failed to get cronjob for cancellation", "task_id", taskID)
		return err
	}

	// 更新任务状态为已取消
	cronJob.Suspend = 1 // 暂停任务
	if err := ts.store.CronJob().Update(ctx, cronJob); err != nil {
		log.Errorw(err, "Failed to cancel cronjob task", "task_id", taskID)
		return err
	}

	log.Infow("Scheduled task cancelled", "task_id", taskID)
	return nil
}

// GetScheduledTasks 获取调度任务列表
func (ts *TaskScheduler) GetScheduledTasks(ctx context.Context, taskType string) ([]*ScheduledTask, error) {
	// 构建查询条件
	whr := where.T(ctx)
	if taskType != "" {
		// 通过名称前缀过滤任务类型
		whr = whr.F("name", "LIKE", taskType+"%")
	}

	_, cronJobs, err := ts.store.CronJob().List(ctx, whr)
	if err != nil {
		log.Errorw(err, "Failed to list scheduled tasks", "task_type", taskType)
		return nil, err
	}

	// 转换为ScheduledTask格式
	tasks := make([]*ScheduledTask, 0, len(cronJobs))
	for _, cronJob := range cronJobs {
		task := ts.cronJobToScheduledTask(cronJob)
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// ExecutePendingTasks 执行待处理的任务
func (ts *TaskScheduler) ExecutePendingTasks(ctx context.Context) error {
	// 获取所有待执行的任务
	tasks, err := ts.GetScheduledTasks(ctx, "")
	if err != nil {
		return err
	}

	now := time.Now()
	executedCount := 0

	for _, task := range tasks {
		// 检查是否需要执行
		if task.Status == "pending" && !task.IsRecurring && task.ScheduleTime.Before(now) {
			if _, err := ts.executeTask(ctx, task); err != nil {
				log.Errorw(err, "Failed to execute scheduled task", "task_id", task.TaskID)
				continue
			}
			executedCount++
		}
	}

	log.Infow("Executed pending tasks", "executed_count", executedCount, "total_tasks", len(tasks))
	return nil
}

// createCronJobTask 创建cronjob任务
func (ts *TaskScheduler) createCronJobTask(ctx context.Context, task *ScheduledTask) (*ScheduledTask, error) {
	// 构建cronjob模型
	cronJobM := &model.CronJobM{
		Name:        task.TaskID,
		Description: fmt.Sprintf("Scheduled task: %s", task.TaskType),
		Schedule:    "",
		Suspend:     0, // 0表示不挂起
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 设置调度规范
	if task.IsRecurring {
		cronJobM.Schedule = task.CronSpec
	} else {
		// 对于一次性任务，创建一个在指定时间执行的cron表达式
		cronJobM.Schedule = ts.timeToOnceOnlyCron(task.ScheduleTime)
	}

	// 创建cronjob
	cronJobBiz := ts.store.CronJob()
	if err := cronJobBiz.Create(ctx, cronJobM); err != nil {
		log.Errorw(err, "Failed to create cronjob task", "task_id", task.TaskID)
		return nil, err
	}

	log.Infow("Scheduled task created", "task_id", task.TaskID, "task_type", task.TaskType, "is_recurring", task.IsRecurring)
	return task, nil
}

// executeTask 执行任务
func (ts *TaskScheduler) executeTask(ctx context.Context, task *ScheduledTask) (*ScheduledTask, error) {
	task.Status = "running"
	task.UpdatedAt = time.Now()

	var err error
	switch task.TaskType {
	case "batch_processing":
		err = ts.smsBatchService.StartProcessing(ctx, task.BatchID)
	case "statistics_sync":
		err = ts.statisticsSync.SyncAllActiveBatches(ctx)
	case "cleanup":
		err = ts.statisticsSync.CleanupExpiredCache(ctx)
	default:
		err = fmt.Errorf("unknown task type: %s", task.TaskType)
	}

	if err != nil {
		task.Status = "failed"
		log.Errorw(err, "Task execution failed", "task_id", task.TaskID, "task_type", task.TaskType)
	} else {
		task.Status = "completed"
		log.Infow("Task executed successfully", "task_id", task.TaskID, "task_type", task.TaskType)
	}

	task.UpdatedAt = time.Now()
	return task, err
}

// cronJobToScheduledTask 将CronJob转换为ScheduledTask
func (ts *TaskScheduler) cronJobToScheduledTask(cronJob *model.CronJobM) *ScheduledTask {
	task := &ScheduledTask{
		TaskID:      cronJob.Name,
		TaskType:    ts.extractTaskTypeFromName(cronJob.Name),
		CronSpec:    cronJob.Schedule,
		IsRecurring: true,       // cronjob默认为周期性任务
		CreatedAt:   time.Now(), // Use current time as placeholder
		UpdatedAt:   time.Now(), // Use current time as placeholder
	}

	// 根据suspend状态设置任务状态
	if cronJob.Suspend == 1 {
		task.Status = "cancelled"
	} else {
		task.Status = "pending"
	}

	// 从任务名称中提取BatchID（如果适用）
	if task.TaskType == "batch_processing" {
		// 假设任务名称格式为 "batch_processing_<batch_id>"
		if len(cronJob.Name) > len("batch_processing_") {
			task.BatchID = cronJob.Name[len("batch_processing_"):]
		}
	}

	return task
}

// extractTaskTypeFromName 从任务名称中提取任务类型
func (ts *TaskScheduler) extractTaskTypeFromName(name string) string {
	if len(name) == 0 {
		return "unknown"
	}

	// 根据名称前缀判断任务类型
	if len(name) >= len("batch_processing") && name[:len("batch_processing")] == "batch_processing" {
		return "batch_processing"
	}
	if len(name) >= len("statistics_sync") && name[:len("statistics_sync")] == "statistics_sync" {
		return "statistics_sync"
	}
	if len(name) >= len("cleanup") && name[:len("cleanup")] == "cleanup" {
		return "cleanup"
	}

	return "unknown"
}

// timeToOnceOnlyCron 将时间转换为一次性执行的cron表达式
func (ts *TaskScheduler) timeToOnceOnlyCron(t time.Time) string {
	// 格式: 秒 分 时 日 月 年
	return fmt.Sprintf("%d %d %d %d %d %d",
		t.Second(), t.Minute(), t.Hour(), t.Day(), int(t.Month()), t.Year())
}

// buildTaskCommand 构建任务执行命令
func (ts *TaskScheduler) buildTaskCommand(task *ScheduledTask) string {
	// 这里需要根据实际的watcher执行机制来构建命令
	// 例如，可能需要调用特定的watcher或者发送事件
	switch task.TaskType {
	case "batch_processing":
		return fmt.Sprintf("smsbatch-watcher --batch-id=%s --action=start", task.BatchID)
	case "statistics_sync":
		return "smsbatch-watcher --action=sync-statistics"
	case "cleanup":
		return "smsbatch-watcher --action=cleanup"
	default:
		return "echo 'Unknown task type'"
	}
}

// GetTaskMetrics 获取任务调度指标
func (ts *TaskScheduler) GetTaskMetrics(ctx context.Context) (map[string]interface{}, error) {
	allTasks, err := ts.GetScheduledTasks(ctx, "")
	if err != nil {
		return nil, err
	}

	metrics := map[string]interface{}{
		"total_tasks":     len(allTasks),
		"pending_tasks":   0,
		"running_tasks":   0,
		"completed_tasks": 0,
		"failed_tasks":    0,
		"cancelled_tasks": 0,
		"recurring_tasks": 0,
		"last_check_time": time.Now(),
	}

	for _, task := range allTasks {
		switch task.Status {
		case "pending":
			metrics["pending_tasks"] = metrics["pending_tasks"].(int) + 1
		case "running":
			metrics["running_tasks"] = metrics["running_tasks"].(int) + 1
		case "completed":
			metrics["completed_tasks"] = metrics["completed_tasks"].(int) + 1
		case "failed":
			metrics["failed_tasks"] = metrics["failed_tasks"].(int) + 1
		case "cancelled":
			metrics["cancelled_tasks"] = metrics["cancelled_tasks"].(int) + 1
		}

		if task.IsRecurring {
			metrics["recurring_tasks"] = metrics["recurring_tasks"].(int) + 1
		}
	}

	return metrics, nil
}
