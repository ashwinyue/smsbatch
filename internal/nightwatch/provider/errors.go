package provider

import "errors"

// Provider related errors
var (
	ErrProviderNotFound = errors.New("provider not found")
	ErrInvalidRequest   = errors.New("invalid request")
	ErrSendFailed      = errors.New("failed to send SMS")
)