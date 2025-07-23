// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

//go:build wireinject
// +build wireinject

package nightwatch

import (
	"github.com/google/wire"

	"github.com/ashwinyue/dcp/internal/nightwatch/biz"
	"github.com/ashwinyue/dcp/internal/nightwatch/messaging"
	"github.com/ashwinyue/dcp/internal/nightwatch/pkg/validation"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/pkg/server"
)

// ProvideValidationDataStore provides a DataStore implementation for validation
func ProvideValidationDataStore(store store.IStore) validation.DataStore {
	return store
}

func InitializeWebServer(*Config) (server.Server, error) {
	wire.Build(
		wire.NewSet(NewWebServer, wire.FieldsOf(new(*Config), "ServerMode")),
		wire.Struct(new(ServerConfig), "*"), // * 表示注入全部字段
		biz.ProviderSet,
		ProvideStoreWithMongo,      // 提供带有MongoDB支持的Store实例
		ProvideValidationDataStore, // 提供验证器数据存储
		validation.ProviderSet,
	)
	return nil, nil
}

// InitializeMessagingService 初始化统一消息服务
func InitializeMessagingService(
	store store.IStore,
) (*messaging.UnifiedMessagingService, error) {
	wire.Build(
		messaging.ProviderSet,
	)
	return nil, nil
}
