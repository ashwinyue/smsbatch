# SMS Batch Business Logic

本目录包含短信批处理的业务逻辑实现，与现有的 `cronjob` 和 `fsm` (状态机) 架构深度集成。

## 架构概述

### 核心组件

1. **smsbatch.go** - 基础CRUD操作和高级批处理管理
2. **statistics_sync.go** - 统计信息同步到Redis缓存
3. **task_scheduler.go** - 任务调度器，与cronjob集成
4. **step_processor.go** - 步骤处理器工厂，管理批处理流程步骤
5. **service_integration.go** - 服务集成器，统一管理所有组件

### 与现有架构的集成

#### 1. 与 CronJob 集成

- **位置**: `/internal/nightwatch/biz/v1/cronjob/`
- **集成方式**: `TaskScheduler` 使用现有的 `CronJobBiz` 接口创建和管理定时任务
- **功能**:
  - 调度批处理任务
  - 周期性统计同步
  - 定期清理任务

```go
// 示例：调度批处理任务
taskScheduler := NewTaskScheduler(store, smsBatchService, statisticsSync)
task, err := taskScheduler.ScheduleBatchProcessing(ctx, batchID, scheduleTime)
```

#### 2. 与 FSM (状态机) 集成

- **位置**: `/internal/nightwatch/biz/v1/smsbatch/`
- **集成方式**: 直接使用内置的 `StateMachine` 来驱动批处理状态转换
- **状态流转**:
  - `StartProcessing` → 触发 `InitialExecute`
  - `PauseBatch` → 调用 `PreparationPause` 或 `DeliveryPause`
  - `ResumeBatch` → 调用 `PreparationResume` 或 `DeliveryResume`

```go
// 示例：启动批处理
stateMachine := NewStateMachine(smsBatch, nil, nil)
err := stateMachine.InitialExecute(ctx, nil)
```

#### 3. 与 Watcher 集成

- **位置**: `/internal/nightwatch/watcher/job/smsbatch/`
- **集成方式**: 业务逻辑层为 Watcher 提供高级管理接口
- **协作模式**:
  - Watcher 负责监控和触发
  - 业务逻辑层负责状态管理和流程控制

## 主要功能

### 1. 批处理管理 (ISmsBatchV1)

#### 基础CRUD操作
- `Create` - 创建批处理
- `Update` - 更新批处理
- `Delete` - 删除批处理
- `Get` - 获取批处理
- `List` - 列出批处理

#### 高级管理功能
- `StartProcessing` - 启动批处理流程
- `PauseBatch` - 暂停批处理
- `ResumeBatch` - 恢复批处理
- `RetryBatch` - 重试批处理
- `AbortBatch` - 中止批处理
- `GetBatchStatus` - 获取批处理状态
- `GetBatchProgress` - 获取批处理进度
- `ValidateBatch` - 验证批处理配置

### 2. 统计同步 (StatisticsSync)

- **实时同步**: 将批处理统计信息同步到Redis
- **缓存管理**: 提供缓存失效和清理功能
- **性能优化**: 减少数据库查询，提高响应速度

```go
// 示例：同步统计信息
statisticsSync := NewStatisticsSync(store, redisClient)
err := statisticsSync.SyncBatchStatistics(ctx, batchID)
```

### 3. 任务调度 (TaskScheduler)

- **一次性任务**: 调度特定时间执行的批处理
- **周期性任务**: 统计同步、清理等定期任务
- **任务管理**: 取消、查询调度任务

```go
// 示例：调度周期性统计同步
task, err := taskScheduler.ScheduleRecurringStatisticsSync(ctx, "0 */5 * * * *")
```

### 4. 步骤处理 (StepFactory)

- **步骤管理**: 创建和管理处理步骤
- **依赖检查**: 验证步骤依赖关系
- **流程控制**: 支持同步和异步执行

#### 内置步骤类型
- `preparation` - 准备步骤
- `delivery` - 交付步骤
- `validation` - 验证步骤
- `partition` - 分区步骤
- `statistics` - 统计步骤
- `cleanup` - 清理步骤

```go
// 示例：创建步骤链
stepFactory := NewStepFactory(store)
steps, err := stepFactory.CreateStepChain([]string{"validation", "preparation", "delivery"}, nil)
```

### 5. 服务集成 (SmsBatchServiceIntegration)

统一管理所有组件，提供完整的业务服务接口。

```go
// 示例：创建集成服务
config := &ServiceConfig{
    RedisClient:           redisClient,
    StatisticsSyncEnabled: true,
    TaskSchedulerEnabled:  true,
    DefaultCronSpecs: map[string]string{
        "statistics_sync": "0 */5 * * * *",  // 每5分钟同步统计
        "cleanup":         "0 0 2 * * *",    // 每天凌晨2点清理
    },
}

integration := NewSmsBatchServiceIntegration(store, config)
```

## 使用示例

### 1. 创建并启动批处理

```go
// 创建批处理
req := &apiv1.CreateSmsBatchRequest{
    SmsBatch: &apiv1.SmsBatch{
        Name:             "测试批次",
        TableStorageName: "sms_data",
        Content:          "Hello World",
        MessageType:      "SMS",
    },
}

resp, err := smsBatchService.Create(ctx, req)
if err != nil {
    return err
}

// 启动处理
err = smsBatchService.StartProcessing(ctx, resp.BatchID)
```

### 2. 使用步骤链处理

```go
// 使用预定义步骤链处理批次
stepTypes := []string{"validation", "preparation", "delivery", "statistics"}
err := integration.ProcessBatchWithSteps(ctx, batchID, stepTypes)
```

### 3. 调度批处理

```go
// 调度1小时后执行的批处理
scheduleTime := time.Now().Add(time.Hour)
err := integration.ScheduleBatchWithSteps(ctx, batchID, stepTypes, scheduleTime)
```

### 4. 监控批处理状态

```go
// 获取详细状态和进度
statusWithProgress, err := integration.GetBatchStatusWithProgress(ctx, batchID)
if err != nil {
    return err
}

fmt.Printf("状态: %s, 进度: %.2f%%\n", 
    statusWithProgress.Status.Status, 
    statusWithProgress.Progress.ProgressPercent)
```

## 配置说明

### 环境变量

- `REDIS_ADDR` - Redis服务器地址
- `STATISTICS_SYNC_ENABLED` - 是否启用统计同步
- `TASK_SCHEDULER_ENABLED` - 是否启用任务调度

### 默认配置

```go
defaultConfig := &ServiceConfig{
    StatisticsSyncEnabled: true,
    TaskSchedulerEnabled:  true,
    DefaultCronSpecs: map[string]string{
        "statistics_sync": "0 */5 * * * *",  // 每5分钟
        "cleanup":         "0 0 2 * * *",    // 每天凌晨2点
    },
}
```

## 扩展指南

### 1. 添加自定义步骤

```go
// 实现StepProcessor接口
type customStep struct {
    baseStep
}

func (cs *customStep) Execute(ctx context.Context, batch *store.SmsBatchM) error {
    // 自定义逻辑
    return nil
}

// 注册步骤
stepFactory.RegisterStep("custom", func(config map[string]interface{}) (StepProcessor, error) {
    return &customStep{}, nil
})
```

### 2. 自定义任务类型

扩展 `TaskScheduler` 的 `executeTask` 方法以支持新的任务类型。

### 3. 集成外部服务

通过 `ServiceConfig` 添加外部服务配置，在 `SmsBatchServiceIntegration` 中集成。

## 注意事项

1. **状态一致性**: 确保状态机状态与业务逻辑状态保持一致
2. **错误处理**: 所有操作都应该有适当的错误处理和日志记录
3. **性能考虑**: 大批量处理时注意内存和数据库连接管理
4. **监控**: 使用 `GetServiceMetrics` 监控服务健康状态
5. **优雅关闭**: 使用 `Shutdown` 方法确保资源正确释放

## 与现有代码的兼容性

- **完全兼容**: 不修改现有的watcher、fsm和cronjob代码
- **增强功能**: 在现有基础上添加高级管理功能
- **可选集成**: 可以选择性启用各个组件
- **渐进迁移**: 支持逐步从现有实现迁移到新的业务逻辑层