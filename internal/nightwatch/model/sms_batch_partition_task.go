package model

import (
	"time"

	"github.com/ashwinyue/dcp/internal/pkg/known/smsbatch"
)

// SmsBatchPartitionTaskM 短信批处理分区任务模型
// 对应Java项目中的SmsBatchPartitionTask
type SmsBatchPartitionTaskM struct {
	ID              int64      `gorm:"column:id;type:bigint(20) unsigned;primaryKey;autoIncrement:true;comment:主键 ID" json:"id"`
	BatchPrimaryKey string     `gorm:"column:batch_primary_key;type:varchar(100);not null;index:idx_batch_pk;comment:批次主键" json:"batch_primary_key"`
	PartitionKey    string     `gorm:"column:partition_key;type:varchar(100);not null;index:idx_partition_key;comment:分区键" json:"partition_key"`
	Status          string     `gorm:"column:status;type:varchar(32);not null;default:'INITIAL';comment:任务状态" json:"status"`
	TaskCode        string     `gorm:"column:task_code;type:varchar(100);not null;comment:任务代码" json:"task_code"`
	StartTime       *time.Time `gorm:"column:start_time;type:datetime;comment:开始时间" json:"start_time"`
	EndTime         *time.Time `gorm:"column:end_time;type:datetime;comment:结束时间" json:"end_time"`
	ProcessedCount  int64      `gorm:"column:processed_count;type:bigint(20);not null;default:0;comment:已处理数量" json:"processed_count"`
	SuccessCount    int64      `gorm:"column:success_count;type:bigint(20);not null;default:0;comment:成功数量" json:"success_count"`
	FailedCount     int64      `gorm:"column:failed_count;type:bigint(20);not null;default:0;comment:失败数量" json:"failed_count"`
	RetryCount      int32      `gorm:"column:retry_count;type:int(11);not null;default:0;comment:重试次数" json:"retry_count"`
	ErrorMessage    string     `gorm:"column:error_message;type:text;comment:错误信息" json:"error_message"`
	CreatedAt       time.Time  `gorm:"column:created_at;type:datetime;not null;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;type:datetime;not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"`
}

// TableName 表名
func (SmsBatchPartitionTaskM) TableName() string {
	return "nw_sms_batch_partition_task"
}

// 分区任务状态常量 - 使用统一的状态定义
// Partition task status constants - using unified status definitions
const (
	PartitionTaskStatusInitial            = smsbatch.PartitionTaskStatusInitial
	PartitionTaskStatusReady              = smsbatch.PartitionTaskStatusReady
	PartitionTaskStatusRunning            = smsbatch.PartitionTaskStatusRunning
	PartitionTaskStatusPaused             = smsbatch.PartitionTaskStatusPaused
	PartitionTaskStatusCanceled           = smsbatch.PartitionTaskStatusCanceled
	PartitionTaskStatusCompletedSucceeded = smsbatch.PartitionTaskStatusCompletedSucceeded
	PartitionTaskStatusCompletedFailed    = smsbatch.PartitionTaskStatusCompletedFailed
)

// IsCompleted 检查任务是否已完成
func (s *SmsBatchPartitionTaskM) IsCompleted() bool {
	return s.Status == PartitionTaskStatusCompletedSucceeded ||
		s.Status == PartitionTaskStatusCompletedFailed ||
		s.Status == PartitionTaskStatusCanceled
}

// IsRunning 检查任务是否正在运行
func (s *SmsBatchPartitionTaskM) IsRunning() bool {
	return s.Status == PartitionTaskStatusRunning
}

// IsPaused 检查任务是否已暂停
func (s *SmsBatchPartitionTaskM) IsPaused() bool {
	return s.Status == PartitionTaskStatusPaused
}

// CanStart 检查任务是否可以开始
func (s *SmsBatchPartitionTaskM) CanStart() bool {
	return s.Status == PartitionTaskStatusInitial ||
		s.Status == PartitionTaskStatusCompletedFailed ||
		s.Status == PartitionTaskStatusPaused ||
		s.Status == PartitionTaskStatusCanceled
}

// CanPause 检查任务是否可以暂停
func (s *SmsBatchPartitionTaskM) CanPause() bool {
	return s.Status == PartitionTaskStatusRunning || s.Status == PartitionTaskStatusReady
}

// CanResume 检查任务是否可以恢复
func (s *SmsBatchPartitionTaskM) CanResume() bool {
	return s.Status == PartitionTaskStatusPaused || s.Status == PartitionTaskStatusCanceled
}

// GetSuccessRate 获取成功率
func (s *SmsBatchPartitionTaskM) GetSuccessRate() float64 {
	if s.ProcessedCount == 0 {
		return 0.0
	}
	return float64(s.SuccessCount) / float64(s.ProcessedCount)
}

// GetFailureRate 获取失败率
func (s *SmsBatchPartitionTaskM) GetFailureRate() float64 {
	if s.ProcessedCount == 0 {
		return 0.0
	}
	return float64(s.FailedCount) / float64(s.ProcessedCount)
}

// UpdateProgress 更新进度
func (s *SmsBatchPartitionTaskM) UpdateProgress(processed, success, failed int64) {
	s.ProcessedCount = processed
	s.SuccessCount = success
	s.FailedCount = failed
	s.UpdatedAt = time.Now()
}

// MarkAsCompleted 标记为完成
func (s *SmsBatchPartitionTaskM) MarkAsCompleted(success bool) {
	if success {
		s.Status = PartitionTaskStatusCompletedSucceeded
	} else {
		s.Status = PartitionTaskStatusCompletedFailed
	}
	now := time.Now()
	s.EndTime = &now
	s.UpdatedAt = now
}

// MarkAsStarted 标记为开始
func (s *SmsBatchPartitionTaskM) MarkAsStarted() {
	s.Status = PartitionTaskStatusRunning
	now := time.Now()
	s.StartTime = &now
	s.UpdatedAt = now
}

// MarkAsPaused 标记为暂停
func (s *SmsBatchPartitionTaskM) MarkAsPaused() {
	s.Status = PartitionTaskStatusPaused
	s.UpdatedAt = time.Now()
}

// MarkAsResumed 标记为恢复
func (s *SmsBatchPartitionTaskM) MarkAsResumed() {
	s.Status = PartitionTaskStatusRunning
	s.UpdatedAt = time.Now()
}

// IncrementRetry 增加重试次数
func (s *SmsBatchPartitionTaskM) IncrementRetry() {
	s.RetryCount++
	s.UpdatedAt = time.Now()
}

// SetError 设置错误信息
func (s *SmsBatchPartitionTaskM) SetError(err error) {
	if err != nil {
		s.ErrorMessage = err.Error()
		s.UpdatedAt = time.Now()
	}
}
