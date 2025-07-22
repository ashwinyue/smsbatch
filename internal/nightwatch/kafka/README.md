# Nightwatch Kafka Consumer Demo

这个包实现了一个基于 monster 项目中 Kafka 消费者的 demo，展示了如何在 nightwatch 服务中使用 Kafka 消费者功能。

## 功能特性

- 基于 `github.com/ashwinyue/dcp/pkg/streams/connector/kafka` 的 Kafka 消费者实现
- 支持结构化消息（JSON）和原始文本消息处理
- 优雅的关闭机制
- 详细的日志记录
- 命令行接口集成

## 使用方法

### 1. 启动 Kafka 消费者 Demo

```bash
# 使用默认配置
./nightwatch kafka consumer

# 自定义配置
./nightwatch kafka consumer \
  --brokers localhost:9092,localhost:9093 \
  --topic my-topic \
  --group-id my-consumer-group
```

### 2. 生成测试消息

```bash
# 生成默认数量的测试消息
./nightwatch kafka producer

# 自定义配置
./nightwatch kafka producer \
  --brokers localhost:9092 \
  --topic my-topic \
  --count 20
```

## 命令行参数

### Consumer 命令

- `--brokers, -b`: Kafka broker 地址列表（默认: `[localhost:9092]`）
- `--topic, -t`: 要消费的 Kafka topic（默认: `nightwatch-demo`）
- `--group-id, -g`: Kafka 消费者组 ID（默认: `nightwatch-consumer-group`）

### Producer 命令

- `--brokers, -b`: Kafka broker 地址列表（默认: `[localhost:9092]`）
- `--topic, -t`: 要生产消息的 Kafka topic（默认: `nightwatch-demo`）
- `--count, -n`: 要生成的测试消息数量（默认: `10`）

## 消息格式

### DemoMessage 结构

```go
type DemoMessage struct {
    ID        string    `json:"id"`
    Content   string    `json:"content"`
    Timestamp time.Time `json:"timestamp"`
    Source    string    `json:"source"`
}
```

### 示例 JSON 消息

```json
{
  "id": "demo-msg-1",
  "content": "This is demo message number 1",
  "timestamp": "2024-01-01T12:00:00Z",
  "source": "nightwatch-demo"
}
```

## 架构设计

### 核心组件

1. **ConsumerDemo**: 主要的消费者 demo 类
   - 管理 Kafka 消费者生命周期
   - 处理消息路由和解析
   - 提供优雅关闭机制

2. **KafkaSource**: 来自 streams 包的 Kafka 源连接器
   - 处理底层 Kafka 连接
   - 提供消息流接口
   - 管理消费者组和分区

### 消息处理流程

1. 接收来自 Kafka 的原始消息
2. 尝试解析为结构化的 `DemoMessage`
3. 如果解析失败，作为原始文本消息处理
4. 记录详细的处理日志
5. 执行业务逻辑（当前为演示日志记录）

## 扩展指南

### 添加自定义消息处理逻辑

在 `handleDemoMessage` 和 `handleRawMessage` 方法中添加你的业务逻辑：

```go
func (cd *ConsumerDemo) handleDemoMessage(demoMsg DemoMessage, kafkaMsg kafka.Message) {
    // 记录消息
    klog.InfoS("Processing demo message", "id", demoMsg.ID)
    
    // 添加你的业务逻辑
    // 例如：保存到数据库、触发工作流等
    if err := cd.saveToDatabase(demoMsg); err != nil {
        klog.ErrorS(err, "Failed to save message to database")
    }
}
```

### 集成到现有服务

可以将 `ConsumerDemo` 集成到 nightwatch 的主服务中：

```go
// 在 server.go 中
func (s *UnionServer) startKafkaConsumer() {
    consumer, err := kafka.NewConsumerDemo(
        s.kafkaConfig.Brokers,
        s.kafkaConfig.Topic,
        s.kafkaConfig.GroupID,
    )
    if err != nil {
        klog.ErrorS(err, "Failed to create Kafka consumer")
        return
    }
    
    consumer.Start()
    s.kafkaConsumer = consumer
}
```

## 故障排除

### 常见问题

1. **连接失败**: 检查 Kafka broker 地址和网络连接
2. **消费者组冲突**: 使用不同的 `group-id`
3. **Topic 不存在**: 确保 Kafka topic 已创建
4. **权限问题**: 检查 Kafka ACL 配置

### 调试日志

增加日志详细程度：

```bash
./nightwatch kafka consumer --log.level=debug
```

## 与 Monster 项目集成

此实现集成了 monster 项目的模式：

### 队列实现 (`/root/workspace/dcp/internal/nightwatch/queue/kafka.go`)
- 基于 `monster/pkg/queue/kafka.go`
- 实现 `ConsumeHandler` 接口用于消息处理
- 提供 `KQueue` 管理消费者和生产者
- 支持可配置的处理器和消费者
- 包含使用 `WaitGroupWrapper` 的优雅关闭

### 消息消费者 (`/root/workspace/dcp/internal/nightwatch/mqs/`)
- `CommonMessageConsumer` - 处理模板短信消息
- `UplinkMessageConsumer` - 处理上行短信消息
- 基于 `monster/internal/sms/mqs/` 模式
- 支持幂等消息处理
- 包含结构化日志和错误处理

### 集成演示

运行完整的集成演示：

```bash
# 使用默认设置运行集成演示
./bin/nightwatch kafka integration

# 使用自定义配置运行
./bin/nightwatch kafka integration \
  --brokers localhost:9092,kafka-2:9092 \
  --topic sms-messages \
  --group-id sms-consumer-group
```

集成演示将：
1. 启动 Kafka 消费者进行消息处理
2. 生产模板短信消息
3. 生产上行短信消息
4. 演示消息消费和处理
5. 显示结构化日志输出

### 配置

查看 `config_example.yaml` 了解全面的配置选项，包括：
- Kafka broker 设置
- TLS 和 SASL 认证
- 生产者/消费者调优参数
- 短信处理配置
- 监控和日志设置

## 依赖项

- `github.com/segmentio/kafka-go`: Kafka 客户端库
- `github.com/ashwinyue/dcp/pkg/streams/connector/kafka`: 内部 Kafka 连接器
- `k8s.io/klog/v2`: 日志库
- `github.com/spf13/cobra`: 命令行框架
- `github.com/google/uuid`: 消息 ID 的 UUID 生成

## 许可证

本项目使用 MIT 许可证。详见 LICENSE 文件。