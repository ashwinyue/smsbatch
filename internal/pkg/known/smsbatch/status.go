package smsbatch

// SMS Batch Status Constants
// 短信批处理状态常量
const (
	// Batch Level States - 批次级别状态
	BatchStatusInitial            = "INITIAL"
	BatchStatusReady              = "READY"
	BatchStatusRunning            = "RUNNING"
	BatchStatusPaused             = "PAUSED"
	BatchStatusCanceled           = "CANCELED"
	BatchStatusCompletedSucceeded = "COMPLETED_SUCCEEDED"
	BatchStatusCompletedFailed    = "COMPLETED_FAILED"
	BatchStatusAborted            = "ABORTED"
	BatchStatusFailed             = "FAILED"
	BatchStatusRetrying           = "RETRYING"

	// Batch Phase States - 批次阶段状态
	BatchPhaseInitial              = "sms_batch_initial"
	BatchPhasePreparationReady     = "sms_batch_preparation_ready"
	BatchPhasePreparationRunning   = "sms_batch_preparation_running"
	BatchPhasePreparationCompleted = "sms_batch_preparation_completed"
	BatchPhasePreparationPaused    = "sms_batch_preparation_paused"
	BatchPhaseDeliveryReady        = "sms_batch_delivery_ready"
	BatchPhaseDeliveryRunning      = "sms_batch_delivery_running"
	BatchPhaseDeliveryCompleted    = "sms_batch_delivery_completed"
	BatchPhaseDeliveryPaused       = "sms_batch_delivery_paused"
	BatchPhaseSucceeded            = "sms_batch_succeeded"
	BatchPhaseFailed               = "sms_batch_failed"
	BatchPhaseAborted              = "sms_batch_aborted"

	// Delivery Pack States - 投递包状态
	DeliveryPackStatusInitial            = "INITIAL"
	DeliveryPackStatusReady              = "READY"
	DeliveryPackStatusRunning            = "RUNNING"
	DeliveryPackStatusPaused             = "PAUSED"
	DeliveryPackStatusCanceled           = "CANCELED"
	DeliveryPackStatusCompletedSucceeded = "COMPLETED_SUCCEEDED"
	DeliveryPackStatusCompletedFailed    = "COMPLETED_FAILED"
	DeliveryPackStatusRetrying           = "RETRYING"

	// Partition Task States - 分区任务状态
	PartitionTaskStatusInitial            = "INITIAL"
	PartitionTaskStatusReady              = "READY"
	PartitionTaskStatusRunning            = "RUNNING"
	PartitionTaskStatusPaused             = "PAUSED"
	PartitionTaskStatusCanceled           = "CANCELED"
	PartitionTaskStatusCompletedSucceeded = "COMPLETED_SUCCEEDED"
	PartitionTaskStatusCompletedFailed    = "COMPLETED_FAILED"
)

// SMS Batch Event Constants
// 短信批处理事件常量
const (
	// Batch Events - 批次事件
	EventPausePreparation  = "pause_preparation"
	EventResumePreparation = "resume_preparation"
	EventPauseDelivery     = "pause_delivery"
	EventResumeDelivery    = "resume_delivery"
	EventRetryPreparation  = "retry_preparation"
	EventRetryDelivery     = "retry_delivery"
	EventPreparationFailed = "preparation_failed"
	EventDeliveryFailed    = "delivery_failed"
	EventAbort             = "abort"
)

// SMS Batch Configuration Constants
// 短信批处理配置常量
const (
	// Job Configuration - 任务配置
	JobScope            = "sms_batch"
	WatcherName         = "sms_batch_watcher"
	DefaultTimeout      = 3600 // 1 hour in seconds
	PreparationQPS      = 10
	DeliveryQPS         = 20
	MaxWorkers          = 5
	NonSuspended        = false
	IdempotentExecution = "idempotent"

	// Default Values - 默认值
	DefaultBatchSize      = 1000
	DefaultPartitionCount = 4
	DefaultMaxRetries     = 3
	DefaultPackSize       = 1000
)

// IsValidBatchStatus checks if the given status is a valid batch status
// 检查给定状态是否为有效的批次状态
func IsValidBatchStatus(status string) bool {
	validStatuses := []string{
		BatchStatusInitial,
		BatchStatusReady,
		BatchStatusRunning,
		BatchStatusPaused,
		BatchStatusCanceled,
		BatchStatusCompletedSucceeded,
		BatchStatusCompletedFailed,
		BatchStatusAborted,
	}
	for _, validStatus := range validStatuses {
		if status == validStatus {
			return true
		}
	}
	return false
}

// IsValidDeliveryPackStatus checks if the given status is a valid delivery pack status
// 检查给定状态是否为有效的投递包状态
func IsValidDeliveryPackStatus(status string) bool {
	validStatuses := []string{
		DeliveryPackStatusInitial,
		DeliveryPackStatusReady,
		DeliveryPackStatusRunning,
		DeliveryPackStatusPaused,
		DeliveryPackStatusCanceled,
		DeliveryPackStatusCompletedSucceeded,
		DeliveryPackStatusCompletedFailed,
		DeliveryPackStatusRetrying,
	}
	for _, validStatus := range validStatuses {
		if status == validStatus {
			return true
		}
	}
	return false
}

// IsValidPartitionTaskStatus checks if the given status is a valid partition task status
// 检查给定状态是否为有效的分区任务状态
func IsValidPartitionTaskStatus(status string) bool {
	validStatuses := []string{
		PartitionTaskStatusInitial,
		PartitionTaskStatusReady,
		PartitionTaskStatusRunning,
		PartitionTaskStatusPaused,
		PartitionTaskStatusCanceled,
		PartitionTaskStatusCompletedSucceeded,
		PartitionTaskStatusCompletedFailed,
	}
	for _, validStatus := range validStatuses {
		if status == validStatus {
			return true
		}
	}
	return false
}

// IsCompletedStatus checks if the given status represents a completed state
// 检查给定状态是否表示已完成状态
func IsCompletedStatus(status string) bool {
	return status == BatchStatusCompletedSucceeded ||
		status == BatchStatusCompletedFailed ||
		status == BatchStatusCanceled ||
		status == BatchStatusAborted
}

// IsRunningStatus checks if the given status represents a running state
// 检查给定状态是否表示运行中状态
func IsRunningStatus(status string) bool {
	return status == BatchStatusRunning
}

// IsPausedStatus checks if the given status represents a paused state
// 检查给定状态是否表示暂停状态
func IsPausedStatus(status string) bool {
	return status == BatchStatusPaused
}
