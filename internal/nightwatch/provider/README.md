# SMS Provider Integration

本模块实现了短信供应商接口的调用逻辑，支持多种短信供应商的HTTP API调用。

## 功能特性

- 支持多种短信供应商（阿里云、WE、XSXX等）
- 基于 `resty` 库的HTTP客户端实现
- 支持供应商故障转移（failover）
- 可配置的重试机制和超时设置
- 统一的Provider接口设计
- 支持Dummy Provider用于测试

## 架构设计

### 核心组件

1. **Provider Interface** (`provider.go`)
   - 定义统一的短信供应商接口
   - 包含 `Type()` 和 `Send()` 方法

2. **ProviderFactory** (`provider.go`)
   - 管理所有注册的短信供应商
   - 提供供应商实例获取功能

3. **HTTPProvider** (`http_provider.go`)
   - 基于HTTP API的通用短信供应商实现
   - 使用 `resty` 库进行HTTP调用
   - 支持不同供应商的请求格式适配

4. **DummyProvider** (`dummy_provider.go`)
   - 用于测试的模拟供应商
   - 不发送真实短信，仅记录日志

5. **HTTP Client** (`../client/client.go`)
   - 基于 `resty` 的HTTP客户端封装
   - 支持重试、超时、调试等配置

## 使用方法

### 1. 初始化Provider Factory

```go
import (
    "github.com/ashwinyue/dcp/internal/nightwatch/provider"
)

// 使用默认配置初始化
factory := provider.InitializeProviders()

// 或使用自定义配置
configs := map[types.ProviderType]*types.HTTPProviderConfig{
    types.ProviderAliyun: {
        BaseURL:    "https://dysmsapi.aliyuncs.com",
        Timeout:    30 * time.Second,
        RetryCount: 3,
        Headers: map[string]string{
            "Content-Type": "application/json",
        },
        Credentials: map[string]string{
            "access_key_id":     "your_key",
            "access_key_secret": "your_secret",
        },
    },
}
factory := provider.InitializeProvidersWithConfig(configs)
```

### 2. 发送短信

```go
// 创建消息消费者
consumer := mqs.NewCommonMessageConsumer(ctx, factory)

// 处理短信请求
msg := &types.TemplateMsgRequest{
    RequestId:    "req-123",
    PhoneNumber:  "13800138000",
    Content:      "您的验证码是：1234",
    TemplateCode: "SMS_001",
    Providers:    []string{"aliyun", "we", "dummy"},
    Params: map[string]string{
        "code": "1234",
    },
}

err := consumer.Consume(kafkaMessage)
```

### 3. 配置文件

参考 `configs/sms_providers.yaml` 配置不同的短信供应商：

```yaml
sms:
  providers:
    aliyun:
      enabled: true
      type: "http"
      base_url: "https://dysmsapi.aliyuncs.com"
      timeout: "30s"
      retry_count: 3
      credentials:
        access_key_id: "your_key"
        access_key_secret: "your_secret"
```

## 支持的供应商

### 1. 阿里云短信服务
- Provider Type: `aliyun`
- 支持模板短信发送
- 需要配置 AccessKey 和 SecretKey

### 2. WE短信平台
- Provider Type: `we`
- 支持HTTP API调用
- 需要配置 API Key 和 Secret

### 3. XSXX短信平台
- Provider Type: `xsxx`
- 支持RESTful API
- 需要配置用户名和密码

### 4. Dummy Provider
- Provider Type: `dummy`
- 仅用于测试，不发送真实短信
- 无需配置

## 故障转移机制

系统支持多供应商故障转移：

1. 按照 `Providers` 数组中的顺序依次尝试
2. 如果某个供应商失败，自动切换到下一个
3. 记录每次尝试的结果和错误信息
4. 只要有一个供应商成功，即认为发送成功

## 扩展新供应商

要添加新的短信供应商：

1. 实现 `Provider` 接口
2. 在 `factory.go` 中注册新供应商
3. 在 `types/message.go` 中添加新的 `ProviderType`
4. 更新配置文件模板

示例：

```go
type NewProvider struct {
    providerType types.ProviderType
    config       *types.HTTPProviderConfig
}

func (p *NewProvider) Type() types.ProviderType {
    return p.providerType
}

func (p *NewProvider) Send(ctx context.Context, request *types.TemplateMsgRequest) (*types.SendResult, error) {
    // 实现具体的发送逻辑
    return &types.SendResult{}, nil
}
```

## 监控和日志

系统提供详细的日志记录：

- 供应商选择过程
- HTTP请求和响应详情
- 错误信息和重试次数
- 发送结果统计

建议在生产环境中配置适当的日志级别和监控告警。