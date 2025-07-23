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
	"github.com/ashwinyue/dcp/internal/nightwatch/pkg/validation"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/fsm"
	"github.com/ashwinyue/dcp/internal/nightwatch/service"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/pkg/server"
)

func InitializeWebServer(*Config) (server.Server, error) {
	wire.Build(
		wire.NewSet(NewWebServer, wire.FieldsOf(new(*Config), "ServerMode")),
		wire.Struct(new(ServerConfig), "*"), // * 表示注入全部字段
		biz.ProviderSet,
		ProvideStoreWithMongo, // 提供带有MongoDB支持的Store实例
		validation.ProviderSet,
	)
	return nil, nil
}

// ProvideDefaultRateLimiterConfig provides default rate limiter configuration
func ProvideDefaultRateLimiterConfig() *core.RateLimiterConfig {
	return core.DefaultRateLimiterConfig()
}

// InitializeSMSBatchCore 初始化SMS批处理核心组件
// 使用core包中已有的InitializeEventCoordinator函数
func InitializeSMSBatchCore(
	tableStorageService service.TableStorageService,
	storeInterface store.IStore,
) (*fsm.EventCoordinator, error) {
	// 使用默认配置
	config := ProvideDefaultRateLimiterConfig()
	return core.InitializeEventCoordinator(tableStorageService, storeInterface, config)
}
