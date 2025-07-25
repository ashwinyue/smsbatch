// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

package nightwatch

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	genericoptions "github.com/onexstack/onexstack/pkg/options"
	"github.com/onexstack/onexstack/pkg/watch"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/ashwinyue/dcp/internal/nightwatch/biz"
	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/pkg/validation"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher"
	_ "github.com/ashwinyue/dcp/internal/nightwatch/watcher/all"
	"github.com/ashwinyue/dcp/internal/pkg/client/minio/fake"
	dcplog "github.com/ashwinyue/dcp/internal/pkg/log"
	dcpserver "github.com/ashwinyue/dcp/internal/pkg/server"
)

const (
	// GRPCServerMode 定义 gRPC 服务模式.
	// 使用 gRPC 框架启动一个 gRPC 服务器.
	GRPCServerMode = "grpc"
	// GRPCGatewayServerMode 定义 gRPC + HTTP 服务模式.
	// 使用 gRPC 框架启动一个 gRPC 服务器 + HTTP 反向代理服务器.
	GRPCGatewayServerMode = "grpc-gateway"
	// GinServerMode 定义 Gin 服务模式.
	// 使用 Gin Web 框架启动一个 HTTP 服务器.
	GinServerMode = "gin"
)

// Config 配置结构体，用于存储应用相关的配置.
// 不用 viper.Get，是因为这种方式能更加清晰的知道应用提供了哪些配置项.
type Config struct {
	ServerMode        string
	EnableMemoryStore bool
	TLSOptions        *genericoptions.TLSOptions
	HTTPOptions       *genericoptions.HTTPOptions
	GRPCOptions       *genericoptions.GRPCOptions
	MySQLOptions      *genericoptions.MySQLOptions
	MongoOptions      *genericoptions.MongoOptions
	RedisOptions      *genericoptions.RedisOptions
	KafkaOptions      *genericoptions.KafkaOptions
	// Watcher related configurations
	WatchOptions          *watch.Options
	EnableWatcher         bool
	UserWatcherMaxWorkers int64
}

// UnionServer 定义一个联合服务器. 根据 ServerMode 决定要启动的服务器类型.
//
// 联合服务器分为以下 2 大类：
//  1. Gin 服务器：由 Gin 框架创建的标准的 REST 服务器。根据是否开启 TLS，
//     来判断启动 HTTP 或者 HTTPS；
//  2. GRPC 服务器：由 gRPC 框架创建的标准 RPC 服务器
//  3. HTTP 反向代理服务器：由 grpc-gateway 框架创建的 HTTP 反向代理服务器。
//     根据是否开启 TLS，来判断启动 HTTP 或者 HTTPS；
//
// HTTP 反向代理服务器依赖 gRPC 服务器，所以在开启 HTTP 反向代理服务器时，会先启动 gRPC 服务器.
type UnionServer struct {
	srv         dcpserver.Server
	watch       *watch.Watch
	db          *gorm.DB
	mongo       *MongoManager
	redisClient *redis.Client
	kafkaWriter *kafka.Writer
	kafkaReader *kafka.Reader
	config      *Config
}

// ServerConfig 包含服务器的核心依赖和配置.
type ServerConfig struct {
	cfg *Config
	biz biz.IBiz
	val *validation.Validator
}

// NewUnionServer 根据配置创建联合服务器.
func (cfg *Config) NewUnionServer() (*UnionServer, error) {

	dcplog.Infow("Initializing federation server", "server-mode", cfg.ServerMode, "enable-memory-store", cfg.EnableMemoryStore, "enable-watcher", cfg.EnableWatcher)

	// 创建数据库连接
	db, err := cfg.NewDB()
	if err != nil {
		return nil, err
	}

	// 创建MongoDB连接
	mongoManager, err := cfg.NewMongoManager()
	if err != nil {
		return nil, fmt.Errorf("初始化MongoDB失败: %w", err)
	}

	// 创建MongoDB索引
	if err := mongoManager.CreateIndexes(); err != nil {
		return nil, fmt.Errorf("创建MongoDB索引失败: %w", err)
	}

	// 创建Redis客户端
	redisClient, err := cfg.NewRedisClient()
	if err != nil {
		return nil, fmt.Errorf("初始化Redis失败: %w", err)
	}

	// 创建Kafka Writer
	kafkaWriter, err := cfg.NewKafkaWriter()
	if err != nil {
		return nil, fmt.Errorf("初始化Kafka Writer失败: %w", err)
	}

	// 创建Kafka Reader
	kafkaReader, err := cfg.NewKafkaReader()
	if err != nil {
		return nil, fmt.Errorf("初始化Kafka Reader失败: %w", err)
	}

	// 创建服务配置，这些配置可用来创建服务器
	srv, err := InitializeWebServer(cfg)
	if err != nil {
		return nil, err
	}

	var watchIns *watch.Watch
	if cfg.EnableWatcher {
		// 创建watcher配置，传入已初始化的组件
		watcherConfig, err := cfg.CreateWatcherConfig(db, mongoManager)
		if err != nil {
			return nil, err
		}

		// 初始化watcher
		initialize := watcher.NewInitializer(watcherConfig)
		opts := []watch.Option{
			watch.WithInitialize(initialize),
		}

		watchIns, err = watch.NewWatch(cfg.WatchOptions, db, opts...)
		if err != nil {
			return nil, err
		}
	}

	return &UnionServer{
		srv:         srv,
		watch:       watchIns,
		db:          db,
		mongo:       mongoManager,
		redisClient: redisClient,
		kafkaWriter: kafkaWriter,
		kafkaReader: kafkaReader,
		config:      cfg,
	}, nil
}

// NewDB 创建数据库连接
func (cfg *Config) NewDB() (*gorm.DB, error) {
	// 使用 SQLite 数据库
	db, err := gorm.Open(sqlite.Open("nightwatch.db"), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	// 自动迁移数据库模式
	err = db.AutoMigrate(
		&model.CronJobM{},
		&model.JobM{},
		&model.PostM{},
		&model.SmsBatchM{},
		&model.SmsRecordM{},
		&model.SmsBatchPartitionTaskM{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

// ProvideDB 提供数据库实例给 Wire
func ProvideDB(cfg *Config) (*gorm.DB, error) {
	return cfg.NewDB()
}

// NewRedisClient 创建Redis客户端
func (cfg *Config) NewRedisClient() (*redis.Client, error) {
	if cfg.RedisOptions == nil {
		return nil, fmt.Errorf("Redis配置为空")
	}
	return cfg.RedisOptions.NewClient()
}

// NewKafkaWriter 创建Kafka生产者
func (cfg *Config) NewKafkaWriter() (*kafka.Writer, error) {
	if cfg.KafkaOptions == nil {
		return nil, fmt.Errorf("Kafka配置为空")
	}

	// 创建Kafka Writer配置
	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.KafkaOptions.Brokers...),
		Topic:        cfg.KafkaOptions.Topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequiredAcks(cfg.KafkaOptions.WriterOptions.RequiredAcks),
		MaxAttempts:  cfg.KafkaOptions.WriterOptions.MaxAttempts,
		Async:        cfg.KafkaOptions.WriterOptions.Async,
		BatchSize:    cfg.KafkaOptions.WriterOptions.BatchSize,
		BatchTimeout: cfg.KafkaOptions.WriterOptions.BatchTimeout,
		BatchBytes:   int64(cfg.KafkaOptions.WriterOptions.BatchBytes),
	}

	return writer, nil
}

// NewKafkaReader 创建Kafka消费者
func (cfg *Config) NewKafkaReader() (*kafka.Reader, error) {
	if cfg.KafkaOptions == nil {
		return nil, fmt.Errorf("Kafka配置为空")
	}

	// 创建Kafka Reader配置
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:           cfg.KafkaOptions.Brokers,
		Topic:             cfg.KafkaOptions.Topic,
		GroupID:           cfg.KafkaOptions.ReaderOptions.GroupID,
		Partition:         cfg.KafkaOptions.ReaderOptions.Partition,
		QueueCapacity:     cfg.KafkaOptions.ReaderOptions.QueueCapacity,
		MinBytes:          cfg.KafkaOptions.ReaderOptions.MinBytes,
		MaxBytes:          cfg.KafkaOptions.ReaderOptions.MaxBytes,
		MaxWait:           cfg.KafkaOptions.ReaderOptions.MaxWait,
		ReadBatchTimeout:  cfg.KafkaOptions.ReaderOptions.ReadBatchTimeout,
		HeartbeatInterval: cfg.KafkaOptions.ReaderOptions.HeartbeatInterval,
		CommitInterval:    cfg.KafkaOptions.ReaderOptions.CommitInterval,
		RebalanceTimeout:  cfg.KafkaOptions.ReaderOptions.RebalanceTimeout,
		StartOffset:       cfg.KafkaOptions.ReaderOptions.StartOffset,
		MaxAttempts:       cfg.KafkaOptions.ReaderOptions.MaxAttempts,
	})

	return reader, nil
}

// ProvideStoreWithMongo 提供带有MongoDB支持的Store实例给Wire
func ProvideStoreWithMongo(cfg *Config) (store.IStore, error) {
	// 创建传统数据库连接
	db, err := cfg.NewDB()
	if err != nil {
		return nil, fmt.Errorf("创建数据库连接失败: %w", err)
	}

	// 创建MongoDB连接
	mongoManager, err := cfg.NewMongoManager()
	if err != nil {
		return nil, fmt.Errorf("创建MongoDB连接失败: %w", err)
	}

	// 创建带有MongoDB支持的store实例
	return store.NewStoreWithMongo(db, mongoManager), nil
}

// CreateWatcherConfig used to create configuration used by all watcher.
func (cfg *Config) CreateWatcherConfig(db *gorm.DB, mongoManager *MongoManager) (*watcher.AggregateConfig, error) {
	// 创建带有MongoDB支持的store实例
	storeClient := store.NewStoreWithMongo(db, mongoManager)

	// 创建 MinIO 客户端 (使用 fake 实现)
	minioClient, err := fake.NewFakeMinioClient("default-bucket")
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	return &watcher.AggregateConfig{
		Store:                 storeClient,
		DB:                    db,
		Minio:                 minioClient,
		UserWatcherMaxWorkers: cfg.UserWatcherMaxWorkers,
	}, nil
}

// Run 运行应用.
func (s *UnionServer) Run() error {
	go s.srv.RunOrDie()

	// 启动watcher服务
	if s.watch != nil {
		dcplog.Infow("Starting watcher service")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go s.watch.Start(ctx.Done())
	}

	// 创建一个 os.Signal 类型的 channel，用于接收系统信号
	quit := make(chan os.Signal, 1)
	// 当执行 kill 命令时（不带参数），默认会发送 syscall.SIGTERM 信号
	// 使用 kill -2 命令会发送 syscall.SIGINT 信号（例如按 CTRL+C 触发）
	// 使用 kill -9 命令会发送 syscall.SIGKILL 信号，但 SIGKILL 信号无法被捕获，因此无需监听和处理
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	// 阻塞程序，等待从 quit channel 中接收到信号
	<-quit

	dcplog.Infow("Shutting down server ...")

	// 优雅关闭服务
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if s.watch != nil {
		dcplog.Infow("Stopping watcher service")
		s.watch.Stop()
	}

	if s.srv != nil {
		dcplog.Infow("Stopping server")
		s.srv.GracefulStop(ctx)
	}

	// 关闭Redis客户端
	if s.redisClient != nil {
		if err := s.redisClient.Close(); err != nil {
			dcplog.Errorw("关闭Redis客户端失败", "error", err)
		}
	}

	// 关闭Kafka Writer
	if s.kafkaWriter != nil {
		if err := s.kafkaWriter.Close(); err != nil {
			dcplog.Errorw("关闭Kafka Writer失败", "error", err)
		}
	}

	// 关闭Kafka Reader
	if s.kafkaReader != nil {
		if err := s.kafkaReader.Close(); err != nil {
			dcplog.Errorw("关闭Kafka Reader失败", "error", err)
		}
	}

	// 关闭数据库连接
	if s.db != nil {
		if sqlDB, err := s.db.DB(); err == nil {
			sqlDB.Close()
		}
	}

	dcplog.Infow("Server exiting")

	return nil
}

// NewWebServer 根据服务器模式创建对应的服务器实例
func NewWebServer(serverMode string, serverConfig *ServerConfig) (dcpserver.Server, error) {
	// 根据服务模式创建对应的服务实例
	switch serverMode {
	case GinServerMode:
		return serverConfig.NewGinServer(), nil
	default:
		return serverConfig.NewGRPCServerOr()
	}
}
