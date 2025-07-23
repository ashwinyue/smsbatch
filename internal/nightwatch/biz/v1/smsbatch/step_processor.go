package smsbatch

import (
	"context"
	"fmt"
	"time"

	"github.com/onexstack/onexstack/pkg/log"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
)

// StepProcessor 步骤处理器接口
type StepProcessor interface {
	GetName() string
	GetType() string
	Execute(ctx context.Context, batch *model.SmsBatchM) error
	CanExecute(ctx context.Context, batch *model.SmsBatchM) bool
	GetDependencies() []string
	IsAsync() bool
	GetTimeout() time.Duration
	Validate(ctx context.Context, batch *model.SmsBatchM) error
}

// StepFactory 步骤工厂接口
type StepFactory interface {
	CreateStep(stepType string, config map[string]interface{}) (StepProcessor, error)
	RegisterStep(stepType string, creator StepCreator) error
	GetAvailableSteps() []string
	CreateStepChain(stepTypes []string, config map[string]interface{}) ([]StepProcessor, error)
	ValidateStepChain(steps []StepProcessor) error
}

// StepCreator 步骤创建器函数类型
type StepCreator func(config map[string]interface{}) (StepProcessor, error)

// stepFactory 步骤工厂实现
type stepFactory struct {
	stepCreators map[string]StepCreator
	store        store.IStore
}

// NewStepFactory 创建步骤工厂
func NewStepFactory(store store.IStore) StepFactory {
	factory := &stepFactory{
		stepCreators: make(map[string]StepCreator),
		store:        store,
	}

	// 注册内置步骤类型
	factory.registerBuiltinSteps()
	return factory
}

// CreateStep 创建步骤处理器
func (sf *stepFactory) CreateStep(stepType string, config map[string]interface{}) (StepProcessor, error) {
	creator, exists := sf.stepCreators[stepType]
	if !exists {
		return nil, fmt.Errorf("unknown step type: %s", stepType)
	}

	return creator(config)
}

// RegisterStep 注册步骤处理器
func (sf *stepFactory) RegisterStep(stepType string, creator StepCreator) error {
	if _, exists := sf.stepCreators[stepType]; exists {
		return fmt.Errorf("step type already registered: %s", stepType)
	}

	sf.stepCreators[stepType] = creator
	log.Infow("Step type registered", "step_type", stepType)
	return nil
}

// GetAvailableSteps 获取可用的步骤类型
func (sf *stepFactory) GetAvailableSteps() []string {
	steps := make([]string, 0, len(sf.stepCreators))
	for stepType := range sf.stepCreators {
		steps = append(steps, stepType)
	}
	return steps
}

// CreateStepChain 创建步骤链
func (sf *stepFactory) CreateStepChain(stepTypes []string, config map[string]interface{}) ([]StepProcessor, error) {
	steps := make([]StepProcessor, 0, len(stepTypes))

	for _, stepType := range stepTypes {
		step, err := sf.CreateStep(stepType, config)
		if err != nil {
			return nil, fmt.Errorf("failed to create step %s: %w", stepType, err)
		}
		steps = append(steps, step)
	}

	// 验证步骤链
	if err := sf.ValidateStepChain(steps); err != nil {
		return nil, fmt.Errorf("invalid step chain: %w", err)
	}

	return steps, nil
}

// ValidateStepChain 验证步骤链
func (sf *stepFactory) ValidateStepChain(steps []StepProcessor) error {
	// 检查循环依赖
	if err := sf.checkCircularDependencies(steps); err != nil {
		return err
	}

	// 检查依赖关系是否满足
	for _, step := range steps {
		dependencies := step.GetDependencies()
		for _, dep := range dependencies {
			if !sf.stepExistsInChain(dep, steps) {
				return fmt.Errorf("step %s depends on %s which is not in the chain", step.GetName(), dep)
			}
		}
	}

	return nil
}

// registerBuiltinSteps 注册内置步骤类型
func (sf *stepFactory) registerBuiltinSteps() {
	// 注册准备步骤
	sf.stepCreators["preparation"] = sf.createPreparationStep
	// 注册交付步骤
	sf.stepCreators["delivery"] = sf.createDeliveryStep
	// 注册验证步骤
	sf.stepCreators["validation"] = sf.createValidationStep
	// 注册分区步骤
	sf.stepCreators["partition"] = sf.createPartitionStep
	// 注册统计步骤
	sf.stepCreators["statistics"] = sf.createStatisticsStep
	// 注册清理步骤
	sf.stepCreators["cleanup"] = sf.createCleanupStep
}

// checkCircularDependencies 检查循环依赖
func (sf *stepFactory) checkCircularDependencies(steps []StepProcessor) error {
	// 构建依赖图
	dependencyGraph := make(map[string][]string)
	for _, step := range steps {
		dependencyGraph[step.GetName()] = step.GetDependencies()
	}

	// 使用DFS检查循环依赖
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for stepName := range dependencyGraph {
		if !visited[stepName] {
			if sf.hasCycleDFS(stepName, dependencyGraph, visited, recStack) {
				return fmt.Errorf("circular dependency detected involving step: %s", stepName)
			}
		}
	}

	return nil
}

// hasCycleDFS DFS检查循环依赖
func (sf *stepFactory) hasCycleDFS(stepName string, graph map[string][]string, visited, recStack map[string]bool) bool {
	visited[stepName] = true
	recStack[stepName] = true

	for _, dep := range graph[stepName] {
		if !visited[dep] {
			if sf.hasCycleDFS(dep, graph, visited, recStack) {
				return true
			}
		} else if recStack[dep] {
			return true
		}
	}

	recStack[stepName] = false
	return false
}

// stepExistsInChain 检查步骤是否存在于链中
func (sf *stepFactory) stepExistsInChain(stepName string, steps []StepProcessor) bool {
	for _, step := range steps {
		if step.GetName() == stepName {
			return true
		}
	}
	return false
}

// 内置步骤创建器实现

// createPreparationStep 创建准备步骤
func (sf *stepFactory) createPreparationStep(config map[string]interface{}) (StepProcessor, error) {
	return &preparationStep{
		baseStep: baseStep{
			name:     "preparation",
			stepType: "preparation",
			store:    sf.store,
			config:   config,
			timeout:  30 * time.Second,
		},
	}, nil
}

// createDeliveryStep 创建交付步骤
func (sf *stepFactory) createDeliveryStep(config map[string]interface{}) (StepProcessor, error) {
	return &deliveryStep{
		baseStep: baseStep{
			name:         "delivery",
			stepType:     "delivery",
			store:        sf.store,
			config:       config,
			dependencies: []string{"preparation"},
			timeout:      60 * time.Second,
		},
	}, nil
}

// createValidationStep 创建验证步骤
func (sf *stepFactory) createValidationStep(config map[string]interface{}) (StepProcessor, error) {
	return &validationStep{
		baseStep: baseStep{
			name:     "validation",
			stepType: "validation",
			store:    sf.store,
			config:   config,
			timeout:  15 * time.Second,
		},
	}, nil
}

// createPartitionStep 创建分区步骤
func (sf *stepFactory) createPartitionStep(config map[string]interface{}) (StepProcessor, error) {
	return &partitionStep{
		baseStep: baseStep{
			name:         "partition",
			stepType:     "partition",
			store:        sf.store,
			config:       config,
			dependencies: []string{"validation"},
			timeout:      30 * time.Second,
		},
	}, nil
}

// createStatisticsStep 创建统计步骤
func (sf *stepFactory) createStatisticsStep(config map[string]interface{}) (StepProcessor, error) {
	return &statisticsStep{
		baseStep: baseStep{
			name:     "statistics",
			stepType: "statistics",
			store:    sf.store,
			config:   config,
			timeout:  10 * time.Second,
		},
	}, nil
}

// createCleanupStep 创建清理步骤
func (sf *stepFactory) createCleanupStep(config map[string]interface{}) (StepProcessor, error) {
	return &cleanupStep{
		baseStep: baseStep{
			name:         "cleanup",
			stepType:     "cleanup",
			store:        sf.store,
			config:       config,
			dependencies: []string{"delivery"},
			timeout:      20 * time.Second,
		},
	}, nil
}

// 内置步骤实现

// baseStep 基础步骤实现
type baseStep struct {
	name         string
	stepType     string
	store        store.IStore
	config       map[string]interface{}
	dependencies []string
	isAsync      bool
	timeout      time.Duration
}

func (bs *baseStep) GetName() string {
	return bs.name
}

func (bs *baseStep) GetType() string {
	return bs.stepType
}

func (bs *baseStep) GetDependencies() []string {
	return bs.dependencies
}

func (bs *baseStep) IsAsync() bool {
	return bs.isAsync
}

func (bs *baseStep) GetTimeout() time.Duration {
	return bs.timeout
}

func (bs *baseStep) CanExecute(ctx context.Context, batch *model.SmsBatchM) bool {
	return true // 默认实现，子类可以重写
}

func (bs *baseStep) Validate(ctx context.Context, batch *model.SmsBatchM) error {
	return nil // 默认实现，子类可以重写
}

// preparationStep 准备步骤
type preparationStep struct {
	baseStep
}

func (ps *preparationStep) Execute(ctx context.Context, batch *model.SmsBatchM) error {
	log.Infow("Executing preparation step", "batch_id", batch.BatchID)

	// 这里可以集成现有的preparation processor
	// 例如：调用现有的preparation逻辑

	// 更新批次状态
	batch.Status = "preparation"
	batch.UpdatedAt = time.Now()

	return ps.store.SmsBatch().Update(ctx, batch)
}

// deliveryStep 交付步骤
type deliveryStep struct {
	baseStep
}

func (ds *deliveryStep) Execute(ctx context.Context, batch *model.SmsBatchM) error {
	log.Infow("Executing delivery step", "batch_id", batch.BatchID)

	// 这里可以集成现有的delivery processor
	// 例如：调用现有的delivery逻辑

	// 更新批次状态
	batch.Status = "delivery"
	batch.UpdatedAt = time.Now()

	return ds.store.SmsBatch().Update(ctx, batch)
}

// validationStep 验证步骤
type validationStep struct {
	baseStep
}

func (vs *validationStep) Execute(ctx context.Context, batch *model.SmsBatchM) error {
	log.Infow("Executing validation step", "batch_id", batch.BatchID)

	// 验证批次配置
	if batch.Content == "" {
		return fmt.Errorf("batch content is empty")
	}
	if batch.TableStorageName == "" {
		return fmt.Errorf("table storage name is empty")
	}

	// 更新验证结果
	batch.Status = "validated"
	batch.UpdatedAt = time.Now()

	return vs.store.SmsBatch().Update(ctx, batch)
}

// partitionStep 分区步骤
type partitionStep struct {
	baseStep
}

func (ps *partitionStep) Execute(ctx context.Context, batch *model.SmsBatchM) error {
	log.Infow("Executing partition step", "batch_id", batch.BatchID)

	// 这里可以实现数据分区逻辑
	// 例如：根据数据量和配置将数据分成多个分区

	// 更新分区信息
	batch.Status = "partitioned"
	batch.UpdatedAt = time.Now()

	return ps.store.SmsBatch().Update(ctx, batch)
}

// statisticsStep 统计步骤
type statisticsStep struct {
	baseStep
}

func (ss *statisticsStep) Execute(ctx context.Context, batch *model.SmsBatchM) error {
	log.Infow("Executing statistics step", "batch_id", batch.BatchID)

	// 初始化统计信息
	batch.Status = "statistics"
	batch.UpdatedAt = time.Now()

	return ss.store.SmsBatch().Update(ctx, batch)
}

// cleanupStep 清理步骤
type cleanupStep struct {
	baseStep
}

func (cs *cleanupStep) Execute(ctx context.Context, batch *model.SmsBatchM) error {
	log.Infow("Executing cleanup step", "batch_id", batch.BatchID)

	// 这里可以实现清理逻辑
	// 例如：清理临时文件、释放资源等

	// 更新清理状态
	batch.Status = "completed"
	batch.UpdatedAt = time.Now()

	return cs.store.SmsBatch().Update(ctx, batch)
}
