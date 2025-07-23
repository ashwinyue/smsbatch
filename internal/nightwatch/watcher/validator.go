package watcher

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/robfig/cron/v3"
)

// ValidationError 验证错误
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationResult 验证结果
type ValidationResult struct {
	IsValid bool               `json:"is_valid"`
	Errors  []*ValidationError `json:"errors"`
}

// AddError 添加验证错误
func (vr *ValidationResult) AddError(field, message string) {
	vr.IsValid = false
	vr.Errors = append(vr.Errors, &ValidationError{
		Field:   field,
		Message: message,
	})
}

// GetErrorMessages 获取所有错误消息
func (vr *ValidationResult) GetErrorMessages() []string {
	messages := make([]string, len(vr.Errors))
	for i, err := range vr.Errors {
		messages[i] = fmt.Sprintf("%s: %s", err.Field, err.Message)
	}
	return messages
}

// Validator 验证器
type Validator struct{}

// NewValidator 创建新的验证器
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateCronJob 验证CronJob数据
func (v *Validator) ValidateCronJob(cronJob *model.CronJobM) *ValidationResult {
	result := &ValidationResult{IsValid: true}

	// 验证必填字段
	if cronJob.CronJobID == "" {
		result.AddError("cronjob_id", "CronJob ID不能为空")
	}

	if cronJob.UserID == "" {
		result.AddError("user_id", "用户ID不能为空")
	}

	if cronJob.Name == "" {
		result.AddError("name", "CronJob名称不能为空")
	}

	if cronJob.Schedule == "" {
		result.AddError("schedule", "调度表达式不能为空")
	} else if !v.isValidCronExpression(cronJob.Schedule) {
		result.AddError("schedule", "调度表达式格式无效")
	}

	// 验证并发策略
	if cronJob.ConcurrencyPolicy < 1 || cronJob.ConcurrencyPolicy > 3 {
		result.AddError("concurrency_policy", "并发策略必须在1-3之间")
	}

	// 验证历史限制
	if cronJob.SuccessHistoryLimit < 0 {
		result.AddError("success_history_limit", "成功历史限制不能为负数")
	}

	if cronJob.FailedHistoryLimit < 0 {
		result.AddError("failed_history_limit", "失败历史限制不能为负数")
	}

	return result
}

// ValidateSmsBatch 验证SmsBatch数据
func (v *Validator) ValidateSmsBatch(smsBatch *model.SmsBatchM) *ValidationResult {
	result := &ValidationResult{IsValid: true}

	// 验证必填字段
	if smsBatch.BatchID == "" {
		result.AddError("batch_id", "批次ID不能为空")
	}

	if smsBatch.UserID == "" {
		result.AddError("user_id", "用户ID不能为空")
	}

	if smsBatch.Name == "" {
		result.AddError("name", "批次名称不能为空")
	}

	if smsBatch.Content == "" {
		result.AddError("content", "短信内容不能为空")
	}

	// 验证状态
	if !v.isValidSmsBatchStatus(smsBatch.Status) {
		result.AddError("status", "无效的批次状态")
	}

	// 验证优先级
	if smsBatch.Priority < 0 {
		result.AddError("priority", "优先级不能为负数")
	}

	// 验证调度时间
	if smsBatch.ScheduleTime != nil && smsBatch.ScheduleTime.Before(time.Now()) {
		result.AddError("schedule_time", "调度时间不能早于当前时间")
	}

	return result
}

// ValidateInteraction 验证Interaction数据
func (v *Validator) ValidateInteraction(interaction *model.InteractionM) *ValidationResult {
	result := &ValidationResult{IsValid: true}

	// 验证必填字段
	if interaction.UserID == "" {
		result.AddError("user_id", "用户ID不能为空")
	}

	if interaction.PhoneNumber == "" {
		result.AddError("phone_number", "手机号码不能为空")
	} else if !v.isValidPhoneNumber(interaction.PhoneNumber) {
		result.AddError("phone_number", "手机号码格式无效")
	}

	if interaction.Content == "" {
		result.AddError("content", "交互内容不能为空")
	}

	if interaction.Type == "" {
		result.AddError("type", "交互类型不能为空")
	}

	return result
}

// 辅助验证函数

// isValidCronExpression 验证cron表达式
func (v *Validator) isValidCronExpression(expr string) bool {
	_, err := cron.ParseStandard(expr)
	return err == nil
}

// isValidSmsBatchStatus 验证SMS批次状态
func (v *Validator) isValidSmsBatchStatus(status string) bool {
	validStatuses := []string{
		"Pending", "Running", "Completed", "Failed", "Cancelled",
		"Paused", "Preparing", "Delivering", "Succeeded", "Aborted",
	}
	for _, validStatus := range validStatuses {
		if status == validStatus {
			return true
		}
	}
	return false
}

// isValidPhoneNumber 验证手机号码格式
func (v *Validator) isValidPhoneNumber(phone string) bool {
	// 简单的手机号码验证，支持国际格式
	pattern := `^\+?[1-9]\d{1,14}$`
	regex := regexp.MustCompile(pattern)
	return regex.MatchString(strings.ReplaceAll(phone, " ", ""))
}

// ValidateGeneric 通用验证方法
func (v *Validator) ValidateGeneric(data interface{}) *ValidationResult {
	result := &ValidationResult{IsValid: true}

	switch obj := data.(type) {
	case *model.CronJobM:
		return v.ValidateCronJob(obj)
	case *model.SmsBatchM:
		return v.ValidateSmsBatch(obj)
	case *model.InteractionM:
		return v.ValidateInteraction(obj)
	default:
		result.AddError("type", "不支持的数据类型")
	}

	return result
}
