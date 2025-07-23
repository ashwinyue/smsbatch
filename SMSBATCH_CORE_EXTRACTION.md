# SMS Batch Core 模块提取指南

## 概述

本文档描述了将 `watcher/job/smsbatch/core` 模块提取为独立业务模块 `biz/v1/smsbatch` 的重构过程。这次重构旨在减少 `watcher` 模块的复杂性，将业务逻辑从基础设施层分离出来。

## 重构目标

### 1. 减少模块嵌套
- **原始路径**: `watcher/job/smsbatch/core`（4层嵌套）
- **新路径**: `biz/v1/smsbatch`（3层嵌套）
- **效果**: 减少一层嵌套，提高代码可读性

### 2. 业务逻辑分离
- 将短信批处理的核心业务逻辑从 `watcher` 基础设施层分离
- 使业务逻辑更加独立，便于测试和维护
- 符合领域驱动设计（DDD）的分层架构原则

### 3. 简化依赖关系
- 减少 `watcher` 模块对深层业务逻辑的依赖
- 提供清晰的业务接口和依赖注入配置

## 模块结构对比

### 原始结构
```
watcher/job/smsbatch/core/
├── core.go                    # Wire 依赖注入配置
├── fsm/                       # 状态机相关
│   ├── event_coordinator.go
│   ├── state_machine.go
│   └── validator.go
├── manager/                   # 管理器组件
│   ├── partition_manager.go
│   ├── state_manager.go
│   └── stats.go
├── processor/                 # 处理器组件
│   ├── delivery_processor.go
│   └── preparation_processor.go
└── publisher/                 # 事件发布器
    └── event_publisher.go
```

### 新结构
```
biz/v1/smsbatch/
├── core.go                    # 核心组件和依赖注入
├── processor.go               # 处理器（合并）
├── manager.go                 # 管理器（合并）
├── publisher.go               # 事件发布器
├── provider.go                # Wire Provider 配置
├── smsbatch.go                # 现有业务服务（保持不变）
└── service_integration.go     # 现有服务集成（保持不变）
```

## 文件迁移详情

### 1. 核心组件整合

#### `core.go`
- **来源**: `watcher/job/smsbatch/core/core.go`
- **变更**: 
  - 保留 Wire 依赖注入配置
  - 添加 `EventCoordinator`、`Validator`、`RateLimiter` 定义
  - 简化初始化逻辑

#### `processor.go`
- **来源**: 
  - `watcher/job/smsbatch/core/processor/preparation_processor.go`
  - `watcher/job/smsbatch/core/processor/delivery_processor.go`
- **变更**: 
  - 合并两个处理器到一个文件
  - 修复字段名称（`TotalRecords` → `Total`，`ProcessedRecords` → `Processed`）
  - 保持原有业务逻辑不变

#### `manager.go`
- **来源**: 
  - `watcher/job/smsbatch/core/manager/partition_manager.go`
  - `watcher/job/smsbatch/core/manager/state_manager.go`
  - `watcher/job/smsbatch/core/manager/stats.go`
- **变更**: 
  - 合并三个管理器到一个文件
  - 保持原有接口和功能不变

#### `publisher.go`
- **来源**: `watcher/job/smsbatch/core/publisher/event_publisher.go`
- **变更**: 
  - 直接复制，无重大变更
  - 保持原有事件发布功能

### 2. 新增文件

#### `provider.go`
- **目的**: 提供 Wire 依赖注入的 ProviderSet
- **内容**: 
  - `ProviderSet` 定义
  - 默认配置创建函数
  - 便捷的初始化函数

## 依赖注入更新

### Wire 配置变更

#### `wire.go` 更新
```go
// 新增导入
import (
    "github.com/ashwinyue/dcp/internal/nightwatch/biz/v1/smsbatch"
)

// 新增初始化函数
func InitializeSMSBatchCore(
    tableStorageService service.TableStorageService,
    storeInterface store.IStore,
) (*smsbatch.EventCoordinator, error) {
    wire.Build(
        smsbatch.ProviderSet,
        store.ProviderSet,
        service.ProviderSet,
    )
    return nil, nil
}
```

### ProviderSet 定义
```go
var ProviderSet = wire.NewSet(
    CoreProviderSet,
    New, // SMS batch service
    DefaultRateLimiterConfig,
)

var CoreProviderSet = wire.NewSet(
    NewPreparationProcessor,
    NewDeliveryProcessor,
    NewStateManager,
    NewEventPublisher,
    NewValidator,
    NewPartitionManager,
    NewRateLimiter,
    NewEventCoordinator,
)
```

## 使用方式变更

### 原始使用方式
```go
import "github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core"

// 通过 core 包初始化
eventCoordinator, err := core.InitializeEventCoordinator(
    tableStorageService,
    storeInterface,
    config,
)
```

### 新使用方式
```go
import "github.com/ashwinyue/dcp/internal/nightwatch/biz/v1/smsbatch"

// 通过 smsbatch 包初始化
eventCoordinator, err := smsbatch.InitializeEventCoordinator(
    tableStorageService,
    storeInterface,
    config,
)

// 或使用默认配置
eventCoordinator, err := smsbatch.NewEventCoordinatorWithDefaults(
    tableStorageService,
    storeInterface,
)
```

## 兼容性说明

### 1. 接口兼容性
- 所有公开接口保持不变
- 函数签名和行为保持一致
- 现有调用代码只需更新导入路径

### 2. 配置兼容性
- `RateLimiterConfig` 结构保持不变
- 默认配置值保持不变
- Wire 依赖注入配置保持兼容

### 3. 数据模型兼容性
- 使用相同的 `model.SmsBatchM` 结构
- 字段映射已修复（`TotalRecords` → `Total`）
- 数据库模式无变更

## 迁移步骤

### 1. 更新导入语句
```go
// 旧导入
import "github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core"

// 新导入
import "github.com/ashwinyue/dcp/internal/nightwatch/biz/v1/smsbatch"
```

### 2. 更新初始化代码
```go
// 旧方式
eventCoordinator, err := core.InitializeEventCoordinator(...)

// 新方式
eventCoordinator, err := smsbatch.InitializeEventCoordinator(...)
```

### 3. 更新 Wire 配置
- 在 `wire.go` 中添加新的 `InitializeSMSBatchCore` 函数
- 更新相关的依赖注入配置

### 4. 验证功能
- 运行单元测试确保功能正常
- 验证 SMS 批处理流程完整性
- 检查事件发布和状态管理

## 优势总结

### 1. 架构清晰
- 业务逻辑与基础设施分离
- 符合 DDD 分层架构原则
- 模块职责更加明确

### 2. 代码简化
- 减少嵌套层级
- 合并相关组件到单一文件
- 简化依赖关系

### 3. 维护性提升
- 业务逻辑集中管理
- 更容易进行单元测试
- 便于功能扩展和修改

### 4. 复用性增强
- 业务逻辑可独立使用
- 不依赖 `watcher` 基础设施
- 便于在其他模块中复用

## 后续优化建议

### 1. 进一步模块化
- 考虑将状态机逻辑独立为单独模块
- 抽象通用的批处理框架

### 2. 接口优化
- 定义更清晰的业务接口
- 减少对具体实现的依赖

### 3. 测试覆盖
- 增加业务逻辑的单元测试
- 添加集成测试用例

### 4. 文档完善
- 补充业务流程文档
- 添加 API 使用示例