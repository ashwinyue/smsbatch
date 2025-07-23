package model

import (
	"time"
)

// SmsRecordM represents an SMS record stored in MongoDB
// This replaces the Table Storage functionality from Java project
type SmsRecordM struct {
	ID               int64                  `bson:"_id,omitempty" json:"id"`
	BatchID          string                 `bson:"batch_id" json:"batch_id"`                                 // SMS批次ID
	TableStorageName string                 `bson:"table_storage_name" json:"table_storage_name"`             // 原Table Storage名称
	PhoneNumber      string                 `bson:"phone_number" json:"phone_number"`                         // 手机号码
	Content          string                 `bson:"content" json:"content"`                                   // 短信内容
	TemplateID       string                 `bson:"template_id" json:"template_id"`                           // 模板ID
	TemplateParams   string                 `bson:"template_params" json:"template_params"`                   // 模板参数(JSON格式)
	ExtCode          int                    `bson:"ext_code" json:"ext_code"`                                 // 扩展码
	ProviderType     string                 `bson:"provider_type" json:"provider_type"`                       // 提供商类型
	Status           string                 `bson:"status" json:"status"`                                     // 状态: pending, processing, sent, failed
	Priority         int                    `bson:"priority" json:"priority"`                                 // 优先级
	ScheduleTime     *time.Time             `bson:"schedule_time,omitempty" json:"schedule_time,omitempty"`   // 调度时间
	SentTime         *time.Time             `bson:"sent_time,omitempty" json:"sent_time,omitempty"`           // 发送时间
	DeliveredTime    *time.Time             `bson:"delivered_time,omitempty" json:"delivered_time,omitempty"` // 送达时间
	ErrorMessage     string                 `bson:"error_message,omitempty" json:"error_message,omitempty"`   // 错误信息
	RetryCount       int                    `bson:"retry_count" json:"retry_count"`                           // 重试次数
	MaxRetries       int                    `bson:"max_retries" json:"max_retries"`                           // 最大重试次数
	PartitionKey     string                 `bson:"partition_key" json:"partition_key"`                       // 分区键
	RowKey           string                 `bson:"row_key" json:"row_key"`                                   // 行键
	CustomFields     map[string]interface{} `bson:"custom_fields,omitempty" json:"custom_fields,omitempty"`   // 自定义字段
	CreatedAt        time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt        time.Time              `bson:"updated_at" json:"updated_at"`
}

// SmsRecordBatch represents a batch of SMS records for processing
type SmsRecordBatch struct {
	BatchID    string        `json:"batch_id"`
	Records    []*SmsRecordM `json:"records"`
	TotalCount int           `json:"total_count"`
	Processed  int           `json:"processed"`
	Success    int           `json:"success"`
	Failed     int           `json:"failed"`
	CreatedAt  time.Time     `json:"created_at"`
}

// SmsRecordQuery represents query parameters for SMS records
type SmsRecordQuery struct {
	ID               int64      `json:"id,omitempty"`
	BatchID          string     `json:"batch_id,omitempty"`
	TableStorageName string     `json:"table_storage_name,omitempty"`
	Status           string     `json:"status,omitempty"`
	ProviderType     string     `json:"provider_type,omitempty"`
	PhoneNumber      string     `json:"phone_number,omitempty"`
	PartitionKey     string     `json:"partition_key,omitempty"`
	CreatedAfter     *time.Time `json:"created_after,omitempty"`
	CreatedBefore    *time.Time `json:"created_before,omitempty"`
	Limit            int        `json:"limit,omitempty"`
	Offset           int        `json:"offset,omitempty"`
}

// SMS Record Status Constants
const (
	SmsRecordStatusPending    = "pending"
	SmsRecordStatusProcessing = "processing"
	SmsRecordStatusSent       = "sent"
	SmsRecordStatusDelivered  = "delivered"
	SmsRecordStatusFailed     = "failed"
	SmsRecordStatusCanceled   = "canceled"
)

// SMS Record Priority Constants
const (
	SmsRecordPriorityLow    = 1
	SmsRecordPriorityNormal = 5
	SmsRecordPriorityHigh   = 10
	SmsRecordPriorityUrgent = 15
)

// IsValidStatus checks if the status is valid
func (sr *SmsRecordM) IsValidStatus() bool {
	validStatuses := []string{
		SmsRecordStatusPending,
		SmsRecordStatusProcessing,
		SmsRecordStatusSent,
		SmsRecordStatusDelivered,
		SmsRecordStatusFailed,
		SmsRecordStatusCanceled,
	}

	for _, status := range validStatuses {
		if sr.Status == status {
			return true
		}
	}
	return false
}

// CanRetry checks if the SMS record can be retried
func (sr *SmsRecordM) CanRetry() bool {
	return sr.Status == SmsRecordStatusFailed && sr.RetryCount < sr.MaxRetries
}

// MarkAsSent marks the SMS record as sent
func (sr *SmsRecordM) MarkAsSent() {
	now := time.Now()
	sr.Status = SmsRecordStatusSent
	sr.SentTime = &now
	sr.UpdatedAt = now
}

// MarkAsDelivered marks the SMS record as delivered
func (sr *SmsRecordM) MarkAsDelivered() {
	now := time.Now()
	sr.Status = SmsRecordStatusDelivered
	sr.DeliveredTime = &now
	sr.UpdatedAt = now
}

// MarkAsFailed marks the SMS record as failed
func (sr *SmsRecordM) MarkAsFailed(errorMsg string) {
	sr.Status = SmsRecordStatusFailed
	sr.ErrorMessage = errorMsg
	sr.RetryCount++
	sr.UpdatedAt = time.Now()
}

// GeneratePartitionKey generates a partition key for the SMS record
func (sr *SmsRecordM) GeneratePartitionKey() string {
	if sr.PartitionKey != "" {
		return sr.PartitionKey
	}

	// Use batch_id as partition key by default
	sr.PartitionKey = sr.BatchID
	return sr.PartitionKey
}

// GenerateRowKey generates a row key for the SMS record
func (sr *SmsRecordM) GenerateRowKey() string {
	if sr.RowKey != "" {
		return sr.RowKey
	}

	// Use phone_number + timestamp as row key
	sr.RowKey = sr.PhoneNumber + "_" + sr.CreatedAt.Format("20060102150405")
	return sr.RowKey
}
