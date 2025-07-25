// Code generated by Wire. DO NOT EDIT.

//go:generate go run -mod=mod github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package nightwatch

import (
	"github.com/ashwinyue/dcp/internal/nightwatch/biz"
	"github.com/ashwinyue/dcp/internal/nightwatch/messaging"
	"github.com/ashwinyue/dcp/internal/nightwatch/pkg/validation"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/pkg/server"
)

import (
	_ "github.com/ashwinyue/dcp/internal/nightwatch/watcher/all"
)

// Injectors from wire.go:

func InitializeWebServer(config *Config) (server.Server, error) {
	string2 := config.ServerMode
	iStore, err := ProvideStoreWithMongo(config)
	if err != nil {
		return nil, err
	}
	bizBiz := biz.NewBiz(iStore)
	dataStore := ProvideValidationDataStore(iStore)
	validator := validation.New(dataStore)
	serverConfig := &ServerConfig{
		cfg: config,
		biz: bizBiz,
		val: validator,
	}
	serverServer, err := NewWebServer(string2, serverConfig)
	if err != nil {
		return nil, err
	}
	return serverServer, nil
}

// InitializeMessagingService 初始化统一消息服务
func InitializeMessagingService(store2 store.IStore) (*messaging.MessagingService, error) {
	messagingService, err := messaging.NewUnifiedMessagingServiceWithDefaults(store2)
	if err != nil {
		return nil, err
	}
	return messagingService, nil
}

// wire.go:

// ProvideValidationDataStore provides a DataStore implementation for validation
func ProvideValidationDataStore(store2 store.IStore) validation.DataStore {
	return store2
}
