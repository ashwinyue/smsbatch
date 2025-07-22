# Message Batch Service Event Integration

本模块实现了将 `message-batch-service` 的批处理功能集成到 DCP 项目的事件系统中。

## 概述

该集成方案提供了一个基于事件驱动的架构，用于处理SMS批处理任务的生命周期管理。通过Kafka消息队列，实现了批处理任务的创建、状态更新、操作控制等功能的解耦和异步处理。

## 架构设计

### 核心组件

1. **BatchMessageConsumer** (`batch_message.go`)
   - 处理批处理相关的Kafka消息
   - 支持三种消息类型：
     - `batch_request`: 批处理创建请求
     - `batch_status_update`: 批处理状态更新
     - `batch_operation`: 批处理操作命令（暂停、恢复、中止、重试）

2. **BatchEventPublisher** (`batch_publisher.go`)
   - 发布批处理相关事件到Kafka
   - 提供便捷的方法发布各种批处理事件

3. **BatchEventService** (`batch_service.go`)
   - 协调消费者和发布者
   - 管理Kafka队列的生命周期
   - 提供健康检查和指标监控

4. **BatchEventIntegration** (`integration_example.go`)
   - 演示如何与现有SMS批处理系统集成
   - 提供集成示例和最佳实践

### 消息类型

#### 1. 批处理请求消息 (BatchMessageRequest)
```json
{
  "request_id": "req-12345",
  "batch_id": "batch-67890",
  "user_id": "user-123",
  "batch_type": "sms",
  "message_type": "template",
  "recipients": ["+1234567890", "+0987654321"],
  "template": "Hello {{name}}, your code is {{code}}",
  "params": {"name": "John", "code": "123456"},
  "priority": 5,
  "max_retries": 3,
  "timeout": 300,
  "created_at": "2024-01-01T00:00:00Z"
}
```

#### 2. 状态更新消息 (BatchStatusUpdateRequest)
```json
{
  "request_id": "req-12345",
  "batch_id": "batch-67890",
  "status": "delivery_running",
  "current_phase": "delivery",
  "progress": 0.75,
  "processed": 150,
  "success": 140,
  "failed": 10,
  "error_message": "",
  "updated_at": "2024-01-01T00:05:00Z"
}
```

#### 3. 操作命令消息 (BatchOperationRequest)
```json
{
  "request_id": "req-12345",
  "batch_id": "batch-67890",
  "operation": "pause",
  "phase": "delivery",
  "reason": "Manual pause by operator",
  "operator_id": "admin-123",
  "requested_at": "2024-01-01T00:03:00Z"
}
```

## 集成步骤

### 1. 配置Kafka

```go
// Kafka发送器配置
kafkaConfig := &message.KafkaSenderConfig{
    Brokers:      []string{"localhost:9092"},
    Topic:        "sms-batch-events",
    Compression:  "gzip",
    BatchSize:    100,
    BatchTimeout: time.Second,
    MaxAttempts:  3,
    Async:        false,
}

// Kafka消费者配置
consumerConfig := &queue.KafkaConfig{
    Brokers:       []string{"localhost:9092"},
    Topic:         "sms-batch-events",
    GroupID:       "sms-batch-event-processor",
    Processors:    8,
    Consumers:     4,
    ForceCommit:   true,
    QueueCapacity: 200,
    MinBytes:      1024,
    MaxBytes:      1024 * 1024, // 1MB
}
```

### 2. 创建批处理事件服务

```go
serviceConfig := &BatchEventServiceConfig{
    KafkaConfig:    kafkaConfig,
    ConsumerConfig: consumerConfig,
    Topic:          "sms-batch-events",
    ConsumerGroup:  "sms-batch-event-processor",
}

batchService, err := NewBatchEventService(ctx, serviceConfig)
if err != nil {
    return fmt.Errorf("failed to create batch event service: %w", err)
}

// 启动服务
err = batchService.Start()
if err != nil {
    return fmt.Errorf("failed to start batch service: %w", err)
}
```

### 3. 与现有SMS批处理系统集成

在现有的SMS批处理系统中，在关键事件点调用相应的事件发布方法：

```go
// 创建批处理时
func (s *SmsBatchService) CreateBatch(req *CreateBatchRequest) error {
    // 现有的批处理创建逻辑
    batch := s.createBatchInDatabase(req)
    
    // 发布批处理创建事件
    err := s.eventIntegration.OnSmsBatchCreated(
        batch.ID, 
        batch.UserID, 
        batch.Recipients, 
        batch.Template, 
        batch.Params,
    )
    if err != nil {
        log.Errorw("Failed to publish batch created event", "error", err)
    }
    
    return nil
}

// 状态变更时
func (s *SmsBatchService) UpdateBatchStatus(batchID, status, phase string, progress float64) error {
    // 现有的状态更新逻辑
    s.updateBatchStatusInDatabase(batchID, status, phase, progress)
    
    // 发布状态更新事件
    err := s.eventIntegration.OnSmsBatchStatusChanged(
        batchID, 
        status, 
        phase, 
        progress, 
        processed, 
        success, 
        failed, 
        errorMessage,
    )
    if err != nil {
        log.Errorw("Failed to publish status change event", "error", err)
    }
    
    return nil
}

// 暂停批处理时
func (s *SmsBatchService) PauseBatch(batchID, phase, reason, operatorID string) error {
    // 现有的暂停逻辑
    s.pauseBatchInDatabase(batchID, phase)
    
    // 发布暂停事件
    err := s.eventIntegration.OnSmsBatchPaused(batchID, phase, reason, operatorID)
    if err != nil {
        log.Errorw("Failed to publish batch paused event", "error", err)
    }
    
    return nil
}
```

## 与现有系统的集成点

### 1. SMS批处理状态机集成

在现有的 `fsm.go` 和 `event.go` 中的状态转换回调中添加事件发布：

```go
// 在 event.go 的状态转换回调中
func (sm *StateMachine) onPreparationReady(ctx context.Context, e *fsm.Event) {
    // 现有逻辑...
    
    // 发布状态变更事件
    sm.eventIntegration.OnSmsBatchStatusChanged(
        batchID, 
        "preparation_ready", 
        "preparation", 
        0.0, 
        0, 0, 0, "",
    )
}
```

### 2. Watcher集成

在现有的 `watcher.go` 中集成事件发布：

```go
// 在批处理开始处理时
func (w *Watcher) processBatch(batch *SmsBatchM) {
    // 现有处理逻辑...
    
    // 发布处理开始事件
    w.eventIntegration.OnSmsBatchStatusChanged(
        batch.ID, 
        "processing", 
        batch.CurrentPhase, 
        batch.Progress, 
        batch.Processed, 
        batch.Success, 
        batch.Failed, 
        batch.ErrorMessage,
    )
}
```

### 3. API层集成

在gRPC API处理中集成事件发布：

```go
// 在 API 更新方法中
func (s *SmsBatchService) UpdateSmsBatch(ctx context.Context, req *UpdateSmsBatchRequest) (*SmsBatchResponse, error) {
    // 现有更新逻辑...
    
    // 如果是暂停操作
    if req.Suspend {
        err := s.eventIntegration.OnSmsBatchPaused(
            req.BatchId, 
            "", 
            "API suspend request", 
            req.OperatorId,
        )
        if err != nil {
            log.Errorw("Failed to publish pause event", "error", err)
        }
    }
    
    return response, nil
}
```

## 监控和健康检查

### 健康检查

```go
// 检查服务健康状态
if !batchService.IsHealthy() {
    log.Errorw("Batch event service is unhealthy")
}
```

### 指标监控

```go
// 获取服务指标
metrics := batchService.GetMetrics()
log.Infow("Batch service metrics", "metrics", metrics)
```

## 配置示例

### 环境变量配置

```bash
# Kafka配置
KAFKA_BROKERS=localhost:9092
KAFKA_TOPIC=sms-batch-events
KAFKA_GROUP_ID=sms-batch-event-processor

# 性能配置
KAFKA_BATCH_SIZE=100
KAFKA_BATCH_TIMEOUT=1s
KAFKA_MAX_ATTEMPTS=3
KAFKA_PROCESSORS=8
KAFKA_CONSUMERS=4
```

### 配置文件示例 (YAML)

```yaml
batch_events:
  kafka:
    brokers:
      - localhost:9092
    topic: sms-batch-events
    compression: gzip
    batch_size: 100
    batch_timeout: 1s
    max_attempts: 3
    async: false
  
  consumer:
    brokers:
      - localhost:9092
    topic: sms-batch-events
    group_id: sms-batch-event-processor
    processors: 8
    consumers: 4
    force_commit: true
    queue_capacity: 200
    min_bytes: 1024
    max_bytes: 1048576
```

## 最佳实践

### 1. 错误处理

- 事件发布失败不应影响主业务流程
- 使用异步发布避免阻塞主流程
- 实现重试机制处理临时故障

### 2. 性能优化

- 合理配置批处理大小和超时时间
- 根据负载调整消费者和处理器数量
- 使用压缩减少网络传输开销

### 3. 监控告警

- 监控消息积压情况
- 监控处理延迟和错误率
- 设置关键指标的告警阈值

### 4. 数据一致性

- 确保事件发布的幂等性
- 处理重复消息的情况
- 实现事件溯源和审计日志

## 故障排查

### 常见问题

1. **Kafka连接失败**
   - 检查Kafka服务是否正常运行
   - 验证网络连接和防火墙设置
   - 确认Kafka配置参数正确

2. **消息积压**
   - 增加消费者数量
   - 优化消息处理逻辑
   - 检查下游服务性能

3. **重复消息处理**
   - 实现消息去重逻辑
   - 检查消费者提交策略
   - 确保处理逻辑的幂等性

### 日志分析

关键日志关键字：
- `batch_request`: 批处理请求相关
- `batch_status_update`: 状态更新相关
- `batch_operation`: 操作命令相关
- `Failed to publish`: 事件发布失败
- `consume message failed`: 消息消费失败

## 扩展功能

### 1. 事件存储

可以扩展实现事件存储功能，用于审计和回放：

```go
type EventStore interface {
    Store(event *BatchEvent) error
    Replay(batchID string) ([]*BatchEvent, error)
}
```

### 2. 事件通知

可以扩展实现多种通知方式：

```go
type NotificationService interface {
    NotifyStatusChange(batchID, status string) error
    NotifyCompletion(batchID string, result *BatchResult) error
}
```

### 3. 指标收集

可以集成Prometheus等监控系统：

```go
type MetricsCollector interface {
    IncrementMessageCount(messageType string)
    RecordProcessingLatency(duration time.Duration)
    RecordErrorCount(errorType string)
}
```

## 总结

本集成方案提供了一个完整的事件驱动架构，用于管理SMS批处理任务的生命周期。通过Kafka消息队列实现了系统的解耦和扩展性，同时保持了与现有系统的兼容性。该方案支持水平扩展、故障恢复和监控告警，为大规模SMS批处理提供了可靠的基础设施。