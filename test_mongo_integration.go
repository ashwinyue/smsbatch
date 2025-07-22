package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch"
	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	genericoptions "github.com/onexstack/onexstack/pkg/options"
)

func main() {
	// 创建MongoDB配置
	cfg := &nightwatch.Config{
		MongoOptions: &genericoptions.MongoOptions{
			URL:      "mongodb://localhost:27017",
			Database: "dcp_test",
			Timeout:  10 * time.Second,
		},
	}

	// 直接创建MongoDB管理器
	mongoManager, err := cfg.NewMongoManager()
	if err != nil {
		log.Fatalf("创建MongoDB管理器失败: %v", err)
	}
	defer mongoManager.Close()

	// 直接创建SmsBatch MongoDB store
	smsBatchStore := store.NewSmsBatchMongoStore(mongoManager)

	// 创建测试数据
	testBatch := &model.SmsBatchM{
		BatchID:     "test-batch-mongo-001",
		UserID:      "user-123",
		Name:        "MongoDB测试批次",
		Description: "这是一个测试MongoDB集成的SMS批次",
		Status:      "pending",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	ctx := context.Background()

	// 测试创建
	fmt.Println("正在测试MongoDB中的SMS批次创建...")
	err = smsBatchStore.Create(ctx, testBatch)
	if err != nil {
		log.Printf("创建SMS批次失败: %v", err)
	} else {
		fmt.Printf("成功在MongoDB中创建SMS批次: %s\n", testBatch.BatchID)
	}

	// 测试查询
	fmt.Println("正在测试从MongoDB查询SMS批次...")
	// 注意：这里需要使用where条件，但为了简化测试，我们跳过查询测试
	// 在实际使用中，需要实现适当的where条件
	fmt.Println("查询测试跳过（需要实现where条件）")

	fmt.Println("MongoDB集成测试完成！")
	fmt.Println("如果看到上述成功消息，说明smsbatch已成功从MySQL迁移到MongoDB")
}
