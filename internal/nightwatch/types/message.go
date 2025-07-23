package types

import "time"

// ProviderType represents the type of SMS provider
type ProviderType string

const (
	ProviderAliyun ProviderType = "aliyun"
	ProviderDummy  ProviderType = "dummy"
	ProviderWE     ProviderType = "we"
	ProviderXSXX   ProviderType = "xsxx"
)

// TemplateMsgRequest represents a template message request
type TemplateMsgRequest struct {
	RequestId    string   `json:"request_id"`
	PhoneNumber  string   `json:"phone_number"`
	Content      string   `json:"content"`
	TemplateCode string   `json:"template_code"`
	Providers    []string `json:"providers"`
	Params       map[string]string `json:"params,omitempty"`
}

// UplinkMsgRequest represents an uplink message request
type UplinkMsgRequest struct {
	RequestId   string    `json:"request_id"`
	PhoneNumber string    `json:"phone_number"`
	Content     string    `json:"content"`
	DestCode    string    `json:"dest_code"`
	SendTime    time.Time `json:"send_time"`
}

// SendResult represents the result of sending an SMS
type SendResult struct {
	BizId   string `json:"biz_id"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
}

// HTTPProviderConfig represents HTTP provider configuration
type HTTPProviderConfig struct {
	BaseURL     string            `json:"base_url"`
	Timeout     time.Duration     `json:"timeout"`
	RetryCount  int               `json:"retry_count"`
	Headers     map[string]string `json:"headers"`
	Credentials map[string]string `json:"credentials"`
}