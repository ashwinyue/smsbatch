package client

import (
	"time"

	"github.com/go-resty/resty/v2"
)

// HTTPClientOptions defines options for HTTP client
type HTTPClientOptions struct {
	UserAgent   string
	Debug       bool
	RetryCount  int
	Timeout     time.Duration
	ContentType string
}

// DefaultHTTPClientOptions returns default HTTP client options
func DefaultHTTPClientOptions() *HTTPClientOptions {
	return &HTTPClientOptions{
		UserAgent:   "smsbatch-client/1.0",
		Debug:       false,
		RetryCount:  3,
		Timeout:     30 * time.Second,
		ContentType: "application/json",
	}
}

// NewRequest creates a new HTTP request with default configuration
func NewRequest(opts ...*HTTPClientOptions) *resty.Request {
	options := DefaultHTTPClientOptions()
	if len(opts) > 0 && opts[0] != nil {
		options = opts[0]
	}

	client := resty.New()
	client.SetRetryCount(options.RetryCount)
	client.SetTimeout(options.Timeout)
	client.SetDebug(options.Debug)

	request := client.R()
	request.SetHeader("User-Agent", options.UserAgent)
	request.SetHeader("Content-Type", options.ContentType)

	return request
}

// NewClient creates a new HTTP client with configuration
func NewClient(opts ...*HTTPClientOptions) *resty.Client {
	options := DefaultHTTPClientOptions()
	if len(opts) > 0 && opts[0] != nil {
		options = opts[0]
	}

	client := resty.New()
	client.SetRetryCount(options.RetryCount)
	client.SetTimeout(options.Timeout)
	client.SetDebug(options.Debug)
	client.SetHeader("User-Agent", options.UserAgent)
	client.SetHeader("Content-Type", options.ContentType)

	return client
}
