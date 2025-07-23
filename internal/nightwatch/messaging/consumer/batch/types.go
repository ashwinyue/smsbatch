package batch

import (
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/messaging/sender"
)

// Message represents a Kafka message
type Message = sender.Message


// KafkaSenderConfig represents Kafka sender configuration
type KafkaSenderConfig = sender.KafkaSenderConfig


// BatchMessageRequest represents a batch message processing request
type BatchMessageRequest struct {
	RequestID    string                 `json:"request_id"`
	BatchID      string                 `json:"batch_id"`
	UserID       string                 `json:"user_id"`
	BatchType    string                 `json:"batch_type"`    // sms, email, push
	MessageType  string                 `json:"message_type"`  // template, marketing, notification
	ProviderType string                 `json:"provider_type"` // SMS provider type
	Recipients   []string               `json:"recipients"`
	Template     string                 `json:"template"`
	Params       map[string]interface{} `json:"params"`
	ScheduleTime *time.Time             `json:"schedule_time,omitempty"`
	Priority     int                    `json:"priority"` // 1-10, 10 is highest
	RetryCount   int                    `json:"retry_count"`
	MaxRetries   int                    `json:"max_retries"`
	Timeout      int64                  `json:"timeout"` // seconds
	CreatedAt    time.Time              `json:"created_at"`
}

// BatchStatusUpdate represents a batch status update event
type BatchStatusUpdate struct {
	RequestID    string                 `json:"request_id"`
	BatchID      string                 `json:"batch_id"`
	Status       string                 `json:"status"`
	CurrentPhase string                 `json:"current_phase"`
	Progress     float64                `json:"progress"`
	Processed    int64                  `json:"processed"`
	Success      int64                  `json:"success"`
	Failed       int64                  `json:"failed"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// BatchOperationCommand represents batch operation commands (pause, resume, abort)
type BatchOperationCommand struct {
	RequestID   string                 `json:"request_id"`
	BatchID     string                 `json:"batch_id"`
	Operation   string                 `json:"operation"`       // pause, resume, abort, retry
	Phase       string                 `json:"phase,omitempty"` // preparation, delivery
	Reason      string                 `json:"reason,omitempty"`
	OperatorID  string                 `json:"operator_id"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	RequestedAt time.Time              `json:"requested_at"`
}

// BatchOperationRequest represents a batch operation request (alias for BatchOperationCommand)
type BatchOperationRequest = BatchOperationCommand