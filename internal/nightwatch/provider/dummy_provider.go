package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/types"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// DummyProvider implements Provider interface for testing purposes
type DummyProvider struct {
	providerType types.ProviderType
}

// NewDummyProvider creates a new dummy provider
func NewDummyProvider(providerType types.ProviderType) *DummyProvider {
	return &DummyProvider{
		providerType: providerType,
	}
}

// Type returns the provider type
func (d *DummyProvider) Type() types.ProviderType {
	return d.providerType
}

// Send simulates sending SMS without actually sending
func (d *DummyProvider) Send(ctx context.Context, request *types.TemplateMsgRequest) (*types.SendResult, error) {
	log.Infow("Sending SMS via dummy provider (simulation)", 
		"provider", d.providerType, 
		"phone", request.PhoneNumber,
		"template", request.TemplateCode,
		"content", request.Content)

	// Simulate processing time
	time.Sleep(100 * time.Millisecond)

	// Create successful result
	result := &types.SendResult{
		BizId:   fmt.Sprintf("dummy-%s-%d", string(d.providerType), time.Now().Unix()),
		Code:    "200",
		Message: "SMS sent successfully (dummy)",
		Success: true,
	}

	log.Infow("SMS sent successfully via dummy provider", 
		"provider", d.providerType, 
		"biz_id", result.BizId)

	return result, nil
}