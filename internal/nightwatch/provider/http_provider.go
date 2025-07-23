package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/client"
	"github.com/ashwinyue/dcp/internal/nightwatch/types"
	"github.com/ashwinyue/dcp/internal/pkg/log"
	"github.com/go-resty/resty/v2"
)

// HTTPProvider implements Provider interface for HTTP-based SMS providers
type HTTPProvider struct {
	providerType types.ProviderType
	config       *types.HTTPProviderConfig
	client       *resty.Client
}

// NewHTTPProvider creates a new HTTP provider
func NewHTTPProvider(providerType types.ProviderType, config *types.HTTPProviderConfig) *HTTPProvider {
	clientOpts := &client.HTTPClientOptions{
		UserAgent:   fmt.Sprintf("smsbatch-%s-client/1.0", string(providerType)),
		Debug:       false,
		RetryCount:  config.RetryCount,
		Timeout:     config.Timeout,
		ContentType: "application/json",
	}

	httpClient := client.NewClient(clientOpts)

	// Set custom headers if provided
	for key, value := range config.Headers {
		httpClient.SetHeader(key, value)
	}

	return &HTTPProvider{
		providerType: providerType,
		config:       config,
		client:       httpClient,
	}
}

// Type returns the provider type
func (h *HTTPProvider) Type() types.ProviderType {
	return h.providerType
}

// Send sends SMS via HTTP API
func (h *HTTPProvider) Send(ctx context.Context, request *types.TemplateMsgRequest) (*types.SendResult, error) {
	log.Infow("Sending SMS via HTTP provider", 
		"provider", h.providerType, 
		"phone", request.PhoneNumber,
		"template", request.TemplateCode)

	// Prepare request payload based on provider type
	payload := h.preparePayload(request)

	// Make HTTP request
	resp, err := h.client.R().
		SetContext(ctx).
		SetBody(payload).
		SetResult(&types.SendResult{}).
		Post(h.config.BaseURL + "/send")

	if err != nil {
		log.Errorw("Failed to send HTTP request", 
			"provider", h.providerType, 
			"error", err)
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	if resp.StatusCode() >= 400 {
		log.Errorw("HTTP request returned error status", 
			"provider", h.providerType, 
			"status_code", resp.StatusCode(),
			"response", string(resp.Body()))
		return nil, fmt.Errorf("HTTP request failed with status %d", resp.StatusCode())
	}

	// Parse response
	result := resp.Result().(*types.SendResult)
	if result == nil {
		// Fallback: create result from response body
		result = &types.SendResult{
			BizId:   fmt.Sprintf("%s-%d", string(h.providerType), time.Now().Unix()),
			Code:    "200",
			Message: "SMS sent successfully",
			Success: true,
		}
	}

	log.Infow("SMS sent successfully via HTTP provider", 
		"provider", h.providerType, 
		"biz_id", result.BizId,
		"code", result.Code)

	return result, nil
}

// preparePayload prepares the request payload based on provider type
func (h *HTTPProvider) preparePayload(request *types.TemplateMsgRequest) map[string]interface{} {
	switch h.providerType {
	case types.ProviderAliyun:
		return map[string]interface{}{
			"PhoneNumbers":  request.PhoneNumber,
			"SignName":      h.config.Credentials["sign_name"],
			"TemplateCode":  request.TemplateCode,
			"TemplateParam": request.Params,
		}
	case types.ProviderWE:
		return map[string]interface{}{
			"mobile":       request.PhoneNumber,
			"content":      request.Content,
			"template_id":  request.TemplateCode,
			"params":       request.Params,
		}
	case types.ProviderXSXX:
		return map[string]interface{}{
			"phone":        request.PhoneNumber,
			"message":      request.Content,
			"template":     request.TemplateCode,
			"variables":    request.Params,
		}
	default:
		// Generic payload format
		return map[string]interface{}{
			"phone_number":  request.PhoneNumber,
			"content":       request.Content,
			"template_code": request.TemplateCode,
			"params":        request.Params,
			"request_id":    request.RequestId,
		}
	}
}