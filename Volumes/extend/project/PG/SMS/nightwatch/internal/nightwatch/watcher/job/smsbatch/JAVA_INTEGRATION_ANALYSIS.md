# Java SMS Batch Service 功能分析与集成方案

## Java项目核心架构分析

### 1. 领域模型 (Domain Model)

#### SmsBatch 核心实体
- **主要属性**:
  - `batchId`: 短信批次号
  - `tableName`: Table Storage名字
  - `campaignId`: 运营活动ID
  - `campaignName`: 运营活动名称
  - `marketingProgramId`: 市场营销程序ID
  - `taskId`: 任务号
  - `contentId`: 短信模板内容ID
  - `content`: 短信模板内容
  - `contentSignature`: 短信模板签名
  - `url`: 用户可点击的长链接
  - `autoTrigger`: 自动触发标志
  - `scheduleTime`: 调度时间
  - `extCode`: 短信扩展码
  - `taskCode`: 任务码(用于判断队列消息是否同一批次)
  - `providerType`: 提供商类型
  - `currentStep`: 当前步骤(PREPARATION/DELIVERY)
  - `deliveryNum`: 统计返回DELIVRD短信计数

- **核心方法**:
  - `generateId()`: 生成唯一ID
  - `canStart()`: 检查是否可以开始
  - `canChangeProvider()`: 检查是否可以更换提供商
  - `entryProcess()`: 进入处理流程
  - `prepareExecute()`: 执行准备步骤
  - `deliveryExecute()`: 执行发送步骤
  - `pauseStep()`: 暂停步骤

### 2. 状态机模式 (State Machine Pattern)

#### AbstractStep 抽象步骤
- **状态枚举**: INITIAL, READY, RUNNING, PAUSED, CANCELED, COMPLETED_SUCCEEDED, COMPLETED_FAILED
- **核心方法**:
  - `canStart()`: 检查是否可以开始
  - `canPause()`: 检查是否可以暂停
  - `canResume()`: 检查是否可以恢复
  - `canReady()`: 检查是否可以就绪
  - `ready()`: 设置为就绪状态
  - `execute()`: 执行步骤
  - `pause()`: 暂停步骤
  - `resume()`: 恢复步骤

#### SmsPreparationStep 准备步骤
- **功能**: 从Table Storage读取短信记录，组装成SmsDeliveryPack
- **核心逻辑**:
  - 分批读取SmsRecord
  - 生成短链接码
  - 异步保存SmsDeliveryPack
  - 统计处理进度
  - 心跳检测

#### SmsDeliveryStep 发送步骤
- **功能**: 将批次分区发送到消息队列
- **核心逻辑**:
  - 生成128个分区任务
  - 发送SmsBatchPartitionTask到消息队列
  - 支持重试机制

### 3. 应用服务层 (Application Service)

#### SmsBatchService 核心业务服务
- **主要功能**:
  - `entryProcess()`: 批次进入处理流程
  - `disPatchSteps()`: 定时调度步骤执行
  - `pause()`: 暂停发送
  - `createSmsBatch()`: 创建短信批次
  - `listSmsBatch()`: 查询批次列表

#### JobHandleService 定时任务服务
- **定时任务**:
  - `dispatchBatchJobHandle()`: 调度批次作业
  - `msgCallbackStatisticJobHandle()`: 消息回调统计
  - `statusTrackingJobHandle()`: 状态跟踪
  - `cosmosRUController()`: Cosmos RU控制

### 4. 基础设施层 (Infrastructure)

#### 数据存储
- **SmsBatchRepository**: 批次数据仓库
- **SmsDeliveryPackBatchRepository**: 发送包数据仓库
- **TableStorageTemplate**: Table Storage模板

#### 消息队列
- **MessageQueueSender**: 消息队列发送器
- **SmsBatchPartitionTask**: 分区任务消息

#### 统计与监控
- **StatisticsSyncer**: 统计同步器
- **PrepareStatusTracker**: 准备状态跟踪器
- **DeliveryStatusTracker**: 发送状态跟踪器

## Go项目集成方案

### 1. 需要新增的核心组件

#### 1.1 领域模型增强
```go
// SmsBatch 结构体需要添加的字段
type SmsBatch struct {
    // 现有字段...
    
    // 新增字段
    MarketingProgramID string    `json:"marketing_program_id"`
    ContentSignature   string    `json:"content_signature"`
    URL               string    `json:"url"`
    AutoTrigger       bool      `json:"auto_trigger"`
    ExtCode           int       `json:"ext_code"`
    TaskCode          string    `json:"task_code"`
    DeliveryNum       int       `json:"delivery_num"`
    ContainsURL       *bool     `json:"contains_url,omitempty"`
}
```

#### 1.2 状态机增强
```go
// 新增状态
const (
    StatusReady              = "READY"
    StatusPaused             = "PAUSED"
    StatusCanceled           = "CANCELED"
    StatusCompletedSucceeded = "COMPLETED_SUCCEEDED"
    StatusCompletedFailed    = "COMPLETED_FAILED"
)

// 新增方法
func (sm *SmsBatch) CanStart() bool
func (sm *SmsBatch) CanPause() bool
func (sm *SmsBatch) CanResume() bool
func (sm *SmsBatch) CanChangeProvider() bool
func (sm *SmsBatch) GenerateID() error
func (sm *SmsBatch) GetContainsURL() bool
```

#### 1.3 分区任务处理
```go
type SmsBatchPartitionTask struct {
    BatchPrimaryKey string `json:"batch_primary_key"`
    PartitionKey    string `json:"partition_key"`
    Status          string `json:"status"`
    TaskCode        string `json:"task_code"`
}

type PartitionProcessor struct {
    partitionNum int
    mqsSender    MQSSender
}

func (pp *PartitionProcessor) GeneratePartitionTasks(batch *SmsBatch) []*SmsBatchPartitionTask
func (pp *PartitionProcessor) SendPartitionTasks(tasks []*SmsBatchPartitionTask) error
```

#### 1.4 统计与监控
```go
type StatisticsSyncer struct {
    redis  *redis.Client
    logger *zap.Logger
}

func (ss *StatisticsSyncer) IncreasePrepareBatchNum(num int)
func (ss *StatisticsSyncer) IncreaseDeliveryBatchNum(num int)
func (ss *StatisticsSyncer) HasRunningPrepareStep() bool
func (ss *StatisticsSyncer) HasRunningDeliveryStep() bool
func (ss *StatisticsSyncer) ClearAllKeys(batchID string)
```

#### 1.5 状态跟踪器
```go
type PrepareStatusTracker struct {
    repository SmsBatchRepository
    logger     *zap.Logger
}

func (pst *PrepareStatusTracker) Track()
func (pst *PrepareStatusTracker) HeartBeat(batch *SmsBatch)

type DeliveryStatusTracker struct {
    repository SmsBatchRepository
    logger     *zap.Logger
}

func (dst *DeliveryStatusTracker) Track()
```

### 2. 现有组件增强

#### 2.1 PreparationProcessor 增强
- 添加分批处理逻辑
- 添加异步保存机制
- 添加心跳检测
- 添加统计同步

#### 2.2 DeliveryProcessor 增强
- 添加分区任务生成
- 添加消息队列发送
- 添加重试机制

#### 2.3 StateMachine 增强
- 添加更多状态转换
- 添加状态检查方法
- 添加步骤调度逻辑

### 3. 新增服务组件

#### 3.1 JobScheduler 定时任务调度器
```go
type JobScheduler struct {
    smsBatchService *SmsBatchService
    statisticsSyncer *StatisticsSyncer
    statusTrackers   []StatusTracker
}

func (js *JobScheduler) DispatchBatchJobs()
func (js *JobScheduler) MessageCallbackStatistic()
func (js *JobScheduler) StatusTracking()
```

#### 3.2 SmsBatchService 业务服务
```go
type SmsBatchService struct {
    repository      SmsBatchRepository
    stepFactory     StepFactory
    statisticsSyncer *StatisticsSyncer
}

func (sbs *SmsBatchService) EntryProcess(batchID string, providerType ProviderType) error
func (sbs *SmsBatchService) DispatchSteps()
func (sbs *SmsBatchService) Pause(batchID string) error
func (sbs *SmsBatchService) CreateSmsBatch(batch *SmsBatch) (*SmsBatch, error)
```

### 4. 集成优先级

#### 第一阶段: 核心状态机增强
1. 扩展SmsBatch结构体
2. 增强状态转换逻辑
3. 添加状态检查方法

#### 第二阶段: 分区处理机制
1. 实现分区任务生成
2. 集成消息队列发送
3. 添加重试机制

#### 第三阶段: 统计与监控
1. 实现统计同步器
2. 添加状态跟踪器
3. 集成心跳检测

#### 第四阶段: 定时任务调度
1. 实现作业调度器
2. 添加定时任务
3. 集成监控告警

### 5. 技术考虑

#### 5.1 并发安全
- 使用sync.RWMutex保护共享状态
- 使用atomic操作更新计数器
- 使用channel进行goroutine通信

#### 5.2 错误处理
- 统一错误码定义
- 分层错误处理
- 错误重试机制

#### 5.3 性能优化
- 批量数据处理
- 异步任务执行
- 连接池管理

#### 5.4 可观测性
- 结构化日志
- 指标监控
- 链路追踪

这个集成方案将Java项目的核心功能和架构模式移植到Go项目中，保持了原有的业务逻辑和设计模式，同时利用Go语言的特性进行了适当的优化。