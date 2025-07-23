package provider

import (
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/types"
)

// InitializeProviders initializes and registers all SMS providers
func InitializeProviders() *ProviderFactory {
	factory := NewProviderFactory()

	// Register dummy providers for testing
	factory.Register(NewDummyProvider(types.ProviderDummy))
	factory.Register(NewDummyProvider(types.ProviderAliyun))
	factory.Register(NewDummyProvider(types.ProviderWE))
	factory.Register(NewDummyProvider(types.ProviderXSXX))

	// Register HTTP providers with default configurations
	// These can be replaced with actual configurations from config files
	factory.Register(NewHTTPProvider(types.ProviderAliyun, &types.HTTPProviderConfig{
		BaseURL:    "https://dysmsapi.aliyuncs.com",
		Timeout:    30 * time.Second,
		RetryCount: 3,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Credentials: map[string]string{
			"access_key_id":     "your_access_key_id",
			"access_key_secret": "your_access_key_secret",
			"sign_name":         "your_sign_name",
		},
	}))

	factory.Register(NewHTTPProvider(types.ProviderWE, &types.HTTPProviderConfig{
		BaseURL:    "https://api.we.com/sms",
		Timeout:    30 * time.Second,
		RetryCount: 3,
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer your_token",
		},
		Credentials: map[string]string{
			"api_key":    "your_api_key",
			"api_secret": "your_api_secret",
		},
	}))

	factory.Register(NewHTTPProvider(types.ProviderXSXX, &types.HTTPProviderConfig{
		BaseURL:    "https://api.xsxx.com/v1/sms",
		Timeout:    30 * time.Second,
		RetryCount: 3,
		Headers: map[string]string{
			"Content-Type": "application/json",
			"X-API-Key":    "your_api_key",
		},
		Credentials: map[string]string{
			"username": "your_username",
			"password": "your_password",
		},
	}))

	return factory
}

// InitializeProvidersWithConfig initializes providers with custom configurations
func InitializeProvidersWithConfig(configs map[types.ProviderType]*types.HTTPProviderConfig) *ProviderFactory {
	factory := NewProviderFactory()

	// Always register dummy provider for fallback
	factory.Register(NewDummyProvider(types.ProviderDummy))

	// Register HTTP providers with provided configurations
	for providerType, config := range configs {
		if config != nil {
			factory.Register(NewHTTPProvider(providerType, config))
		} else {
			// Fallback to dummy provider if config is nil
			factory.Register(NewDummyProvider(providerType))
		}
	}

	return factory
}