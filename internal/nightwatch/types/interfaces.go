package types

import (
	"context"
)

// DataStore represents a data storage interface
type DataStore interface {
	// Add methods as needed for data storage operations
}

// IdempotentChecker represents an interface for checking idempotency
type IdempotentChecker interface {
	// Check checks if an operation with the given key has already been processed
	Check(ctx context.Context, key string) (bool, error)
	// Mark marks an operation with the given key as processed
	Mark(ctx context.Context, key string) error
}

// HistoryLogger represents an interface for logging history
type HistoryLogger interface {
	// Log logs a history entry
	Log(ctx context.Context, entry interface{}) error
	// Query queries history entries
	Query(ctx context.Context, filter interface{}) ([]interface{}, error)
	// WriteHistory writes a history entry with user ID, action, resource and details
	WriteHistory(ctx context.Context, userID, action, resource, details string) error
}

// SMSRequest represents an SMS request
type SMSRequest struct {
	ID          int64  `json:"id"`
	PhoneNumber string `json:"phone_number"`
	Content     string `json:"content"`
	ProviderType string `json:"provider_type"`
}