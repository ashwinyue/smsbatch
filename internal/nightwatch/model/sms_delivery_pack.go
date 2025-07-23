package model

import (
	"time"

	"github.com/ashwinyue/dcp/internal/pkg/known/smsbatch"
)

// SmsDeliveryPackM 短信投递包模型
// 对应Java项目中的SmsDeliveryPack
type SmsDeliveryPackM struct {
	ID             int64      `gorm:"column:id;type:bigint(20) unsigned;primaryKey;autoIncrement:true;comment:主键 ID" json:"id"`
	PackID         string     `gorm:"column:pack_id;type:varchar(100);not null;uniqueIndex:idx_pack_id;comment:投递包ID" json:"pack_id"`
	BatchID        string     `gorm:"column:batch_id;type:varchar(100);not null;index:idx_batch_id;comment:批次ID" json:"batch_id"`
	PartitionKey   string     `gorm:"column:partition_key;type:varchar(100);not null;index:idx_partition_key;comment:分区键" json:"partition_key"`
	Status         string     `gorm:"column:status;type:varchar(32);not null;default:'INITIAL';comment:投递包状态" json:"status"`
	PackSize       int32      `gorm:"column:pack_size;type:int(11);not null;default:0;comment:包大小" json:"pack_size"`
	ProcessedCount int32      `gorm:"column:processed_count;type:int(11);not null;default:0;comment:已处理数量" json:"processed_count"`
	SuccessCount   int32      `gorm:"column:success_count;type:int(11);not null;default:0;comment:成功数量" json:"success_count"`
	FailedCount    int32      `gorm:"column:failed_count;type:int(11);not null;default:0;comment:失败数量" json:"failed_count"`
	RetryCount     int32      `gorm:"column:retry_count;type:int(11);not null;default:0;comment:重试次数" json:"retry_count"`
	MaxRetries     int32      `gorm:"column:max_retries;type:int(11);not null;default:3;comment:最大重试次数" json:"max_retries"`
	ProviderType   string     `gorm:"column:provider_type;type:varchar(32);comment:提供商类型" json:"provider_type"`
	MessageType    string     `gorm:"column:message_type;type:varchar(32);comment:消息类型" json:"message_type"`
	Priority       int32      `gorm:"column:priority;type:int(11);not null;default:0;comment:优先级" json:"priority"`
	ScheduledTime  *time.Time `gorm:"column:scheduled_time;type:datetime;comment:调度时间" json:"scheduled_time"`
	StartTime      *time.Time `gorm:"column:start_time;type:datetime;comment:开始时间" json:"start_time"`
	EndTime        *time.Time `gorm:"column:end_time;type:datetime;comment:结束时间" json:"end_time"`
	LastRetryTime  *time.Time `gorm:"column:last_retry_time;type:datetime;comment:最后重试时间" json:"last_retry_time"`
	ErrorMessage   string     `gorm:"column:error_message;type:text;comment:错误信息" json:"error_message"`
	DeliveryData   string     `gorm:"column:delivery_data;type:longtext;comment:投递数据(JSON)" json:"delivery_data"`
	Metadata       string     `gorm:"column:metadata;type:text;comment:元数据(JSON)" json:"metadata"`
	CreatedAt      time.Time  `gorm:"column:created_at;type:datetime;not null;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;type:datetime;not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"`
}

// TableName 表名
func (SmsDeliveryPackM) TableName() string {
	return "nw_sms_delivery_pack"
}

// 投递包状态常量 - 使用统一的状态定义
// Delivery pack status constants - using unified status definitions
const (
	DeliveryPackStatusInitial            = smsbatch.DeliveryPackStatusInitial
	DeliveryPackStatusReady              = smsbatch.DeliveryPackStatusReady
	DeliveryPackStatusRunning            = smsbatch.DeliveryPackStatusRunning
	DeliveryPackStatusPaused             = smsbatch.DeliveryPackStatusPaused
	DeliveryPackStatusCanceled           = smsbatch.DeliveryPackStatusCanceled
	DeliveryPackStatusCompletedSucceeded = smsbatch.DeliveryPackStatusCompletedSucceeded
	DeliveryPackStatusCompletedFailed    = smsbatch.DeliveryPackStatusCompletedFailed
	DeliveryPackStatusRetrying           = smsbatch.DeliveryPackStatusRetrying
)

// IsCompleted 检查投递包是否已完成
func (s *SmsDeliveryPackM) IsCompleted() bool {
	return s.Status == DeliveryPackStatusCompletedSucceeded ||
		s.Status == DeliveryPackStatusCompletedFailed ||
		s.Status == DeliveryPackStatusCanceled
}

// IsRunning 检查投递包是否正在运行
func (s *SmsDeliveryPackM) IsRunning() bool {
	return s.Status == DeliveryPackStatusRunning
}

// IsPaused 检查投递包是否已暂停
func (s *SmsDeliveryPackM) IsPaused() bool {
	return s.Status == DeliveryPackStatusPaused
}

// IsRetrying 检查投递包是否正在重试
func (s *SmsDeliveryPackM) IsRetrying() bool {
	return s.Status == DeliveryPackStatusRetrying
}

// CanStart 检查投递包是否可以开始
func (s *SmsDeliveryPackM) CanStart() bool {
	return s.Status == DeliveryPackStatusInitial ||
		s.Status == DeliveryPackStatusReady ||
		s.Status == DeliveryPackStatusCompletedFailed ||
		s.Status == DeliveryPackStatusPaused ||
		s.Status == DeliveryPackStatusCanceled
}

// CanRetry 检查投递包是否可以重试
func (s *SmsDeliveryPackM) CanRetry() bool {
	return s.Status == DeliveryPackStatusCompletedFailed && s.RetryCount < s.MaxRetries
}

// CanPause 检查投递包是否可以暂停
func (s *SmsDeliveryPackM) CanPause() bool {
	return s.Status == DeliveryPackStatusRunning || s.Status == DeliveryPackStatusReady
}

// CanResume 检查投递包是否可以恢复
func (s *SmsDeliveryPackM) CanResume() bool {
	return s.Status == DeliveryPackStatusPaused || s.Status == DeliveryPackStatusCanceled
}

// GetSuccessRate 获取成功率
func (s *SmsDeliveryPackM) GetSuccessRate() float64 {
	if s.ProcessedCount == 0 {
		return 0.0
	}
	return float64(s.SuccessCount) / float64(s.ProcessedCount)
}

// GetFailureRate 获取失败率
func (s *SmsDeliveryPackM) GetFailureRate() float64 {
	if s.ProcessedCount == 0 {
		return 0.0
	}
	return float64(s.FailedCount) / float64(s.ProcessedCount)
}

// GetProgress 获取进度百分比
func (s *SmsDeliveryPackM) GetProgress() float64 {
	if s.PackSize == 0 {
		return 0.0
	}
	return float64(s.ProcessedCount) / float64(s.PackSize) * 100.0
}

// UpdateProgress 更新进度
func (s *SmsDeliveryPackM) UpdateProgress(processed, success, failed int32) {
	s.ProcessedCount = processed
	s.SuccessCount = success
	s.FailedCount = failed
	s.UpdatedAt = time.Now()
}

// MarkAsStarted 标记为开始
func (s *SmsDeliveryPackM) MarkAsStarted() {
	s.Status = DeliveryPackStatusRunning
	now := time.Now()
	s.StartTime = &now
	s.UpdatedAt = now
}

// MarkAsCompleted 标记为完成
func (s *SmsDeliveryPackM) MarkAsCompleted(success bool) {
	if success {
		s.Status = DeliveryPackStatusCompletedSucceeded
	} else {
		s.Status = DeliveryPackStatusCompletedFailed
	}
	now := time.Now()
	s.EndTime = &now
	s.UpdatedAt = now
}

// MarkAsPaused 标记为暂停
func (s *SmsDeliveryPackM) MarkAsPaused() {
	s.Status = DeliveryPackStatusPaused
	s.UpdatedAt = time.Now()
}

// MarkAsResumed 标记为恢复
func (s *SmsDeliveryPackM) MarkAsResumed() {
	s.Status = DeliveryPackStatusRunning
	s.UpdatedAt = time.Now()
}

// MarkAsRetrying 标记为重试
func (s *SmsDeliveryPackM) MarkAsRetrying() {
	s.Status = DeliveryPackStatusRetrying
	s.RetryCount++
	now := time.Now()
	s.LastRetryTime = &now
	s.UpdatedAt = now
}

// SetError 设置错误信息
func (s *SmsDeliveryPackM) SetError(err error) {
	if err != nil {
		s.ErrorMessage = err.Error()
		s.UpdatedAt = time.Now()
	}
}

// IsScheduled 检查是否已调度
func (s *SmsDeliveryPackM) IsScheduled() bool {
	return s.ScheduledTime != nil && time.Now().After(*s.ScheduledTime)
}

// ShouldRetry 检查是否应该重试
func (s *SmsDeliveryPackM) ShouldRetry() bool {
	if !s.CanRetry() {
		return false
	}

	// 如果没有最后重试时间，可以重试
	if s.LastRetryTime == nil {
		return true
	}

	// 检查重试间隔（指数退避）
	retryDelay := time.Duration(s.RetryCount*s.RetryCount) * time.Minute
	return time.Now().After(s.LastRetryTime.Add(retryDelay))
}
