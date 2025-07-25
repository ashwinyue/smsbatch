// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

package store

//go:generate mockgen -destination mock_store.go -package store github.com/ashwinyue/dcp/internal/apiserver/store IStore,UserStore,PostStore,ConcretePostStore

import (
	"context"
	"sync"

	"github.com/google/wire"
	"github.com/onexstack/onexstack/pkg/store/where"
	"go.mongodb.org/mongo-driver/mongo"
	"gorm.io/gorm"

	"github.com/ashwinyue/dcp/internal/nightwatch/pkg/validation"
)

// ProviderSet 是一个 Wire 的 Provider 集合，用于声明依赖注入的规则.
// 包含 NewStore 构造函数，用于生成 datastore 实例.
// wire.Bind 用于将接口 IStore 与具体实现 *datastore 绑定，
// 从而在依赖 IStore 的地方，能够自动注入 *datastore 实例.
var ProviderSet = wire.NewSet(NewStore, wire.Bind(new(IStore), new(*datastore)))

var (
	once sync.Once
	// S 全局变量，方便其它包直接调用已初始化好的 datastore 实例.
	S *datastore
)

// Store is an alias for IStore to maintain compatibility
type Store = IStore

// IStore 定义了 store 层需要实现的方法.
type IStore interface {
	// DB 返回 Store 层的 *gorm.DB 实例，在少数场景下会被用到.
	DB(ctx context.Context, wheres ...where.Where) *gorm.DB
	TX(ctx context.Context, fn func(ctx context.Context) error) error

	CronJob() CronJobStore
	Job() JobStore
	SmsBatch() SmsBatchStore
	// SmsRecord 返回SMS记录存储接口，替代Java项目中的Table Storage功能
	SmsRecord() SmsRecordMongoStore
	// TableStorage 返回Table Storage存储接口，替代原来service层的TableStorageService
	TableStorage() TableStorageStore
	Post() PostStore
	// ConcretePost ConcretePosts 是一个示例 store 实现，用来演示在 Go 中如何直接与 DB 交互.
	ConcretePost() ConcretePostStore
	// Interaction 返回交互记录存储接口
	Interaction() InteractionStore
}

// transactionKey 用于在 context.Context 中存储事务上下文的键.
type transactionKey struct{}

// MongoManager 接口定义MongoDB管理器的基本方法
type MongoManager interface {
	GetCollection(name string) *mongo.Collection
	Close() error
}

// datastore 是 IStore 的具体实现.
type datastore struct {
	core *gorm.DB

	// MongoDB管理器
	mongoManager MongoManager

	// MongoDB集合
	documentCollection *mongo.Collection

	// 可以根据需要添加其他数据库实例
	// fake *gorm.DB
}

// 确保 datastore 实现了 IStore 接口.
var _ IStore = (*datastore)(nil)

// 确保 datastore 实现了 validation.DataStore 接口.
var _ validation.DataStore = (*datastore)(nil)

// NewStore 创建一个 IStore 类型的实例.
func NewStore(db *gorm.DB) *datastore {
	// 确保 S 只被初始化一次
	once.Do(func() {
		S = &datastore{core: db}
	})

	return S
}

// NewStoreWithMongo 创建一个带有MongoDB支持的IStore类型实例.
func NewStoreWithMongo(db *gorm.DB, mongoManager MongoManager) *datastore {
	// 确保 S 只被初始化一次
	once.Do(func() {
		S = &datastore{
			core:         db,
			mongoManager: mongoManager,
		}
	})

	return S
}

// NewStoreWithMongoCollection 创建一个带有MongoDB集合支持的IStore类型实例.
func NewStoreWithMongoCollection(db *gorm.DB, documentCollection *mongo.Collection) *datastore {
	// 确保 S 只被初始化一次
	once.Do(func() {
		S = &datastore{
			core:               db,
			documentCollection: documentCollection,
		}
	})

	return S
}

// DB 根据传入的条件（wheres）对数据库实例进行筛选.
// 如果未传入任何条件，则返回上下文中的数据库实例（事务实例或核心数据库实例）.
func (store *datastore) DB(ctx context.Context, wheres ...where.Where) *gorm.DB {
	db := store.core
	// 从上下文中提取事务实例
	if tx, ok := ctx.Value(transactionKey{}).(*gorm.DB); ok {
		db = tx
	}

	// 遍历所有传入的条件并逐一叠加到数据库查询对象上
	for _, whr := range wheres {
		db = whr.Where(db)
	}
	return db
}

// TX 返回一个新的事务实例.
// nolint: fatcontext
func (store *datastore) TX(ctx context.Context, fn func(ctx context.Context) error) error {
	return store.core.WithContext(ctx).Transaction(
		func(tx *gorm.DB) error {
			ctx = context.WithValue(ctx, transactionKey{}, tx)
			return fn(ctx)
		},
	)
}

// CronJob 返回一个实现了 CronJobStore 接口的实例.
func (store *datastore) CronJob() CronJobStore {
	return newCronJobStore(store)
}

// Job 返回一个实现了 JobStore 接口的实例.
func (store *datastore) Job() JobStore {
	return newJobStore(store)
}

// SmsBatch 返回一个实现了 SmsBatchStore 接口的实例.
func (store *datastore) SmsBatch() SmsBatchStore {
	// 如果有MongoDB管理器，使用MongoDB实现
	if store.mongoManager != nil {
		return newSmsBatchMongoStore(store.mongoManager)
	}
	// 否则使用MySQL实现
	return newSmsBatchStore(store)
}

// Post 返回一个实现了 PostStore 接口的实例.
func (store *datastore) Post() PostStore {
	return newPostStore(store)
}

// SmsRecord 返回一个实现了 SmsRecordMongoStore 接口的实例.
// 这个方法替代了Java项目中的Table Storage功能
func (store *datastore) SmsRecord() SmsRecordMongoStore {
	// SMS记录必须使用MongoDB存储
	if store.mongoManager == nil {
		panic("MongoDB manager is required for SMS record storage")
	}
	return NewSmsRecordMongoStore(store.mongoManager)
}

// ConcretePost 返回一个实现了 ConcretePostStore 接口的实例.
func (store *datastore) ConcretePost() ConcretePostStore {
	return newConcretePostStore(store)
}

// TableStorage 返回一个实现了 TableStorageStore 接口的实例.
func (store *datastore) TableStorage() TableStorageStore {
	// 使用Azure Table Storage实现
	// 这里需要从配置中获取Azure连接字符串和表名
	// 暂时使用默认配置，实际使用时应该从配置文件或环境变量中获取
	config := &AzureTableConfig{
		ConnectionString: "DefaultEndpointsProtocol=https;AccountName=your_account;AccountKey=your_key;EndpointSuffix=core.windows.net",
		TableName:        "smsrecords",
	}
	
	tableStore, err := newTableStorageStore(config)
	if err != nil {
		panic("Failed to create Azure Table Storage store: " + err.Error())
	}
	
	return tableStore
}

// Interaction 返回一个实现了 InteractionStore 接口的实例.
func (store *datastore) Interaction() InteractionStore {
	return newInteractionStore(store)
}
