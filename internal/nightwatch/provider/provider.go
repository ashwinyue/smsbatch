package provider

import (
	"context"
	"github.com/ashwinyue/dcp/internal/nightwatch/types"
)

// Provider defines the interface for SMS providers
type Provider interface {
	Type() types.ProviderType
	Send(ctx context.Context, request *types.TemplateMsgRequest) (*types.SendResult, error)
}

// ProviderFactory manages SMS providers
type ProviderFactory struct {
	providers map[types.ProviderType]Provider
}

// NewProviderFactory creates a new provider factory
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{
		providers: make(map[types.ProviderType]Provider),
	}
}

// Register registers a provider with the factory
func (f *ProviderFactory) Register(provider Provider) {
	f.providers[provider.Type()] = provider
}

// GetSMSTemplateProvider retrieves an SMS template provider based on the given provider type
func (f *ProviderFactory) GetSMSTemplateProvider(providerType types.ProviderType) (Provider, error) {
	provider, exists := f.providers[providerType]
	if !exists {
		return nil, ErrProviderNotFound
	}
	return provider, nil
}

// GetAllProviders returns all registered providers
func (f *ProviderFactory) GetAllProviders() map[types.ProviderType]Provider {
	return f.providers
}