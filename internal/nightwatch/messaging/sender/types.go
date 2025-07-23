package sender

import (
	"time"
)

// Message represents a unified message structure
type Message struct {
	Key       string                 `json:"key,omitempty"`
	Value     interface{}            `json:"value"`
	Headers   map[string]string      `json:"headers,omitempty"`
	Partition int                    `json:"partition,omitempty"`
	Timestamp time.Time              `json:"timestamp,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// BatchMessage represents batch processing messages
type BatchMessage struct {
	BatchID     string                 `json:"batch_id"`
	UserID      string                 `json:"user_id"`
	BatchType   string                 `json:"batch_type"`
	MessageType string                 `json:"message_type"`
	Recipients  []string               `json:"recipients"`
	Template    string                 `json:"template"`
	Params      map[string]interface{} `json:"params"`
	Priority    int                    `json:"priority"`
	MaxRetries  int                    `json:"max_retries"`
	Timeout     int                    `json:"timeout"`
	CreatedAt   time.Time              `json:"created_at"`
}

// BatchStatusUpdate represents batch status change events
type BatchStatusUpdate struct {
	BatchID      string    `json:"batch_id"`
	Status       string    `json:"status"`
	CurrentPhase string    `json:"current_phase"`
	Progress     float64   `json:"progress"`
	Processed    int64     `json:"processed"`
	Success      int64     `json:"success"`
	Failed       int64     `json:"failed"`
	ErrorMessage string    `json:"error_message,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}