package validator

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
)

// ValidationError represents a validation error with field information
type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid  bool               `json:"valid"`
	Errors []*ValidationError `json:"errors,omitempty"`
}

// AddError adds a validation error
func (r *ValidationResult) AddError(field, message string, value interface{}) {
	r.Valid = false
	r.Errors = append(r.Errors, &ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	})
}

// HasErrors returns true if there are validation errors
func (r *ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// GetErrorMessages returns all error messages
func (r *ValidationResult) GetErrorMessages() []string {
	messages := make([]string, len(r.Errors))
	for i, err := range r.Errors {
		messages[i] = err.Error()
	}
	return messages
}

// Validator provides validation functionality without storage dependencies
type Validator struct {
	// Configuration for validation rules
	phoneRegex    *regexp.Regexp
	emailRegex    *regexp.Regexp
	maxBatchSize  int
	maxMessageLen int
	maxRetryCount int
}

// NewValidator creates a new validator instance
func NewValidator() *Validator {
	return &Validator{
		phoneRegex:    regexp.MustCompile(`^1[3-9]\d{9}$`), // 中国手机号格式
		emailRegex:    regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`),
		maxBatchSize:  10000,
		maxMessageLen: 1000,
		maxRetryCount: 3,
	}
}

// ValidateCronJob validates a CronJob model
func (v *Validator) ValidateCronJob(cronJob *model.CronJobM) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if cronJob == nil {
		result.AddError("cronJob", "cronJob cannot be nil", nil)
		return result
	}

	// Validate CronJobID
	if strings.TrimSpace(cronJob.CronJobID) == "" {
		result.AddError("cronJobID", "cronJob ID is required", cronJob.CronJobID)
	}

	// Validate Name
	if strings.TrimSpace(cronJob.Name) == "" {
		result.AddError("name", "cronJob name is required", cronJob.Name)
	} else if len(cronJob.Name) > 100 {
		result.AddError("name", "cronJob name cannot exceed 100 characters", cronJob.Name)
	}

	// Validate Schedule (cron expression)
	if strings.TrimSpace(cronJob.Schedule) == "" {
		result.AddError("schedule", "schedule is required", cronJob.Schedule)
	} else if !v.isValidCronExpression(cronJob.Schedule) {
		result.AddError("schedule", "invalid cron expression", cronJob.Schedule)
	}

	// Validate UserID
	if strings.TrimSpace(cronJob.UserID) == "" {
		result.AddError("userID", "userID is required", cronJob.UserID)
	}

	// Validate SmsBatchTemplate if present
	if cronJob.SmsBatchTemplate != nil {
		templateResult := v.ValidateSmsBatch(cronJob.SmsBatchTemplate)
		if templateResult.HasErrors() {
			for _, err := range templateResult.Errors {
				result.AddError(fmt.Sprintf("smsBatchTemplate.%s", err.Field), err.Message, err.Value)
			}
		}
	}

	return result
}

// ValidateSmsBatch validates an SMS batch model
func (v *Validator) ValidateSmsBatch(smsBatch *model.SmsBatchM) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if smsBatch == nil {
		result.AddError("smsBatch", "smsBatch cannot be nil", nil)
		return result
	}

	// Validate BatchID
	if strings.TrimSpace(smsBatch.BatchID) == "" {
		result.AddError("batchID", "batch ID is required", smsBatch.BatchID)
	}

	// Validate Name
	if strings.TrimSpace(smsBatch.Name) == "" {
		result.AddError("name", "batch name is required", smsBatch.Name)
	} else if len(smsBatch.Name) > 200 {
		result.AddError("name", "batch name cannot exceed 200 characters", smsBatch.Name)
	}

	// Validate UserID
	if strings.TrimSpace(smsBatch.UserID) == "" {
		result.AddError("userID", "userID is required", smsBatch.UserID)
	}

	// Validate TableStorageName
	if strings.TrimSpace(smsBatch.TableStorageName) == "" {
		result.AddError("tableStorageName", "table storage name is required", smsBatch.TableStorageName)
	}

	// Validate Message content
	if strings.TrimSpace(smsBatch.Content) == "" {
		result.AddError("content", "message content is required", smsBatch.Content)
	} else if len(smsBatch.Content) > v.maxMessageLen {
		result.AddError("content", fmt.Sprintf("message cannot exceed %d characters", v.maxMessageLen), smsBatch.Content)
	}

	// Validate Status
	if !v.isValidSmsBatchStatus(smsBatch.Status) {
		result.AddError("status", "invalid SMS batch status", smsBatch.Status)
	}

	// Validate retry count (using preparation and delivery retries)
	if smsBatch.PreparationRetries < 0 || int(smsBatch.PreparationRetries) > v.maxRetryCount {
		result.AddError("preparationRetries", fmt.Sprintf("preparation retry count must be between 0 and %d", v.maxRetryCount), smsBatch.PreparationRetries)
	}
	if smsBatch.DeliveryRetries < 0 || int(smsBatch.DeliveryRetries) > v.maxRetryCount {
		result.AddError("deliveryRetries", fmt.Sprintf("delivery retry count must be between 0 and %d", v.maxRetryCount), smsBatch.DeliveryRetries)
	}

	// Validate scheduled time if present
	if smsBatch.ScheduleTime != nil && !smsBatch.ScheduleTime.IsZero() && smsBatch.ScheduleTime.Before(time.Now()) {
		result.AddError("scheduleTime", "scheduled time cannot be in the past", smsBatch.ScheduleTime)
	}

	return result
}

// ValidateInteraction validates an interaction model
func (v *Validator) ValidateInteraction(interaction *model.InteractionM) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if interaction == nil {
		result.AddError("interaction", "interaction cannot be nil", nil)
		return result
	}

	// Validate PhoneNumber
	if strings.TrimSpace(interaction.PhoneNumber) == "" {
		result.AddError("phoneNumber", "phone number is required", interaction.PhoneNumber)
	} else if !v.phoneRegex.MatchString(interaction.PhoneNumber) {
		result.AddError("phoneNumber", "invalid phone number format", interaction.PhoneNumber)
	}

	// Validate UserID
	if strings.TrimSpace(interaction.UserID) == "" {
		result.AddError("userID", "userID is required", interaction.UserID)
	}

	// Validate interaction type
	if strings.TrimSpace(interaction.Type) == "" {
		result.AddError("type", "interaction type is required", interaction.Type)
	}

	return result
}

// ValidatePhoneNumber validates a phone number format
func (v *Validator) ValidatePhoneNumber(phoneNumber string) bool {
	return v.phoneRegex.MatchString(strings.TrimSpace(phoneNumber))
}

// ValidateEmail validates an email format
func (v *Validator) ValidateEmail(email string) bool {
	return v.emailRegex.MatchString(strings.TrimSpace(email))
}

// ValidateBatchSize validates batch size
func (v *Validator) ValidateBatchSize(size int) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if size <= 0 {
		result.AddError("batchSize", "batch size must be positive", size)
	} else if size > v.maxBatchSize {
		result.AddError("batchSize", fmt.Sprintf("batch size cannot exceed %d", v.maxBatchSize), size)
	}

	return result
}

// isValidCronExpression validates cron expression format
func (v *Validator) isValidCronExpression(expr string) bool {
	// 简单的cron表达式验证，支持标准5字段和6字段格式
	expr = strings.TrimSpace(expr)

	// 支持特殊表达式
	specialExpressions := []string{
		"@yearly", "@annually", "@monthly", "@weekly", "@daily", "@midnight", "@hourly",
	}
	for _, special := range specialExpressions {
		if expr == special {
			return true
		}
	}

	// 支持 @every 格式
	if strings.HasPrefix(expr, "@every ") {
		duration := strings.TrimPrefix(expr, "@every ")
		_, err := time.ParseDuration(duration)
		return err == nil
	}

	// 验证标准cron表达式格式
	fields := strings.Fields(expr)
	if len(fields) != 5 && len(fields) != 6 {
		return false
	}

	// 这里可以添加更详细的字段验证逻辑
	return true
}

// isValidSmsBatchStatus validates SMS batch status
func (v *Validator) isValidSmsBatchStatus(status string) bool {
	validStatuses := []string{
		"pending", "processing", "completed", "failed", "cancelled", "paused",
	}

	for _, validStatus := range validStatuses {
		if status == validStatus {
			return true
		}
	}
	return false
}

// SetMaxBatchSize sets the maximum batch size
func (v *Validator) SetMaxBatchSize(size int) {
	v.maxBatchSize = size
}

// SetMaxMessageLength sets the maximum message length
func (v *Validator) SetMaxMessageLength(length int) {
	v.maxMessageLen = length
}

// SetMaxRetryCount sets the maximum retry count
func (v *Validator) SetMaxRetryCount(count int) {
	v.maxRetryCount = count
}

// GetMaxBatchSize returns the maximum batch size
func (v *Validator) GetMaxBatchSize() int {
	return v.maxBatchSize
}

// GetMaxMessageLength returns the maximum message length
func (v *Validator) GetMaxMessageLength() int {
	return v.maxMessageLen
}

// GetMaxRetryCount returns the maximum retry count
func (v *Validator) GetMaxRetryCount() int {
	return v.maxRetryCount
}
