package smsbatch

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/looplab/fsm"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/mqs"
	"github.com/ashwinyue/dcp/internal/nightwatch/pkg/path"
	fakeminio "github.com/ashwinyue/dcp/internal/pkg/client/minio/fake"
	"github.com/ashwinyue/dcp/internal/pkg/log"
	jobconditionsutil "github.com/ashwinyue/dcp/internal/pkg/util/jobconditions"
	v1 "github.com/ashwinyue/dcp/pkg/api/nightwatch/v1"
)

// Stats provides thread-safe statistics tracking for SMS batch processing
type Stats struct {
	processed int64
	success   int64
	failed    int64
}

// NewStats creates a new Stats instance
func NewStats() *Stats {
	return &Stats{}
}

// Add atomically adds values to the statistics
func (s *Stats) Add(processed, success, failed int64) {
	atomic.AddInt64(&s.processed, processed)
	atomic.AddInt64(&s.success, success)
	atomic.AddInt64(&s.failed, failed)
}

// Get returns current statistics values
func (s *Stats) Get() (processed, success, failed int64) {
	return atomic.LoadInt64(&s.processed), atomic.LoadInt64(&s.success), atomic.LoadInt64(&s.failed)
}

// Reset resets all statistics to zero
func (s *Stats) Reset() {
	atomic.StoreInt64(&s.processed, 0)
	atomic.StoreInt64(&s.success, 0)
	atomic.StoreInt64(&s.failed, 0)
}

// processDeliveryPartitions processes delivery partitions concurrently
func (sm *StateMachine) processDeliveryPartitions(ctx context.Context, partitions [][]string, workerCount int) (int64, int64, int64, error) {
	if len(partitions) == 0 {
		return 0, 0, 0, nil
	}

	stats := NewStats()
	var wg sync.WaitGroup
	partitionChan := make(chan []string, len(partitions))
	errorChan := make(chan error, len(partitions))

	// Start workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for partition := range partitionChan {
				partitionID := fmt.Sprintf("worker-%d-partition-%d", workerID, time.Now().UnixNano())
				processed, success, failed, err := sm.processDeliveryPartition(ctx, partitionID, partition)
				if err != nil {
					errorChan <- err
					return
				}
				stats.Add(int64(processed), int64(success), int64(failed))
				log.Debugw("Worker completed partition", "worker_id", workerID, "partition_size", len(partition))
			}
		}(i)
	}

	// Send work to workers
	go func() {
		defer close(partitionChan)
		for _, partition := range partitions {
			partitionChan <- partition
		}
	}()

	wg.Wait()
	close(errorChan)

	// Check for errors
	select {
	case err := <-errorChan:
		if err != nil {
			return 0, 0, 0, err
		}
	default:
	}

	processed, success, failed := stats.Get()
	return processed, success, failed, nil
}

// PreparationExecute handles the SMS preparation phase.
func (sm *StateMachine) PreparationExecute(ctx context.Context, event *fsm.Event) error {
	// Set default SMS batch params.
	SetDefaultSmsBatchParams(sm.SmsBatch)

	// Skip the preparation if the operation has already been performed (idempotency check)
	if ShouldSkipOnIdempotency(sm.SmsBatch, event.FSM.Current()) {
		return nil
	}

	// Initialize SMS batch results if they are not already set
	if sm.SmsBatch.Results == nil {
		sm.SmsBatch.Results = &model.SmsBatchResults{}
	}

	// Initialize SMS batch phase stats
	if sm.SmsBatch.Results.PreparationStats == nil {
		sm.SmsBatch.Results.PreparationStats = &model.SmsBatchPhaseStats{}
	}

	// Validate batch parameters before processing
	if err := sm.validateBatchParams(); err != nil {
		return fmt.Errorf("batch validation failed: %w", err)
	}

	// Start preparation phase
	startTime := time.Now()
	sm.SmsBatch.Results.PreparationStats.StartTime = &startTime
	sm.SmsBatch.Results.CurrentPhase = "preparation"

	// 1. Read SMS parameters from table storage
	log.Infow("Reading SMS parameters from storage", "batch_id", sm.SmsBatch.BatchID)
	data, err := sm.Watcher.Minio.Read(ctx, fakeminio.FakeObjectName)
	if err != nil {
		return fmt.Errorf("failed to read SMS parameters: %w", err)
	}

	// 2. Process SMS data in batches (similar to Java pack size logic)
	batchSize := 1000 // Default batch size
	totalProcessed := int64(0)
	totalSuccess := int64(0)
	totalFailed := int64(0)

	// Process data in chunks
	for i := 0; i < len(data); i += batchSize {
		end := i + batchSize
		if end > len(data) {
			end = len(data)
		}

		// Check if preparation should be paused
		if sm.shouldPausePreparation(ctx) {
			log.Infow("Preparation paused", "batch_id", sm.SmsBatch.BatchID)
			return nil
		}

		// Process current batch
		batchData := data[i:end]
		processedCount, successCount, failedCount := sm.processSmsPreparationBatch(ctx, batchData)

		totalProcessed += int64(processedCount)
		totalSuccess += int64(successCount)
		totalFailed += int64(failedCount)

		// Update progress
		progress := float64(end) / float64(len(data)) * 100.0
		sm.SmsBatch.Results.ProgressPercent = progress

		// Publish progress update
		if sm.Watcher.EventPublisher != nil {
			if err := sm.Watcher.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
				RequestID:    fmt.Sprintf("%s-prep-progress-%d", sm.SmsBatch.BatchID, time.Now().Unix()),
				BatchID:      sm.SmsBatch.BatchID,
				Status:       "preparation_running",
				CurrentPhase: "preparation",
				Progress:     progress,
				Processed:    totalProcessed,
				Success:      totalSuccess,
				Failed:       totalFailed,
				UpdatedAt:    time.Now(),
			}); err != nil {
				log.Errorw("Failed to publish progress update", "batch_id", sm.SmsBatch.BatchID, "error", err)
			}
		}
	}

	// 3. Pack and save SMS data
	packedDataPath := path.Job.Path(sm.SmsBatch.BatchID, "sms_packed_data")
	if err := sm.Watcher.Minio.Write(ctx, packedDataPath, data); err != nil {
		return fmt.Errorf("failed to save packed data: %w", err)
	}

	// 4. Update final statistics
	now := time.Now()
	sm.SmsBatch.Results.PreparationStats = &model.SmsBatchPhaseStats{
		Processed: totalProcessed,
		Success:   totalSuccess,
		Failed:    totalFailed,
		StartTime: &startTime,
		EndTime:   &now,
	}
	sm.SmsBatch.Results.ProcessedMessages = totalProcessed
	sm.SmsBatch.Results.SuccessMessages = totalSuccess
	sm.SmsBatch.Results.FailedMessages = totalFailed
	sm.SmsBatch.Results.ProgressPercent = 100.0

	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	// Publish preparation completed event
	if sm.Watcher.EventPublisher != nil {
		if err := sm.Watcher.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
			RequestID:    fmt.Sprintf("%s-prep-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:      sm.SmsBatch.BatchID,
			Status:       event.FSM.Current(),
			CurrentPhase: "preparation",
			Progress:     100.0,
			Processed:    totalProcessed,
			Success:      totalSuccess,
			Failed:       totalFailed,
			UpdatedAt:    now,
		}); err != nil {
			log.Errorw("Failed to publish preparation completed event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	log.Infow("SMS batch preparation completed successfully",
		"batch_id", sm.SmsBatch.BatchID,
		"processed", totalProcessed,
		"success", totalSuccess,
		"failed", totalFailed)

	return nil
}

// DeliveryExecute handles the SMS delivery phase.
func (sm *StateMachine) DeliveryExecute(ctx context.Context, event *fsm.Event) error {
	if ShouldSkipOnIdempotency(sm.SmsBatch, event.FSM.Current()) {
		return nil
	}

	log.Infow("Starting SMS batch delivery", "batch_id", sm.SmsBatch.BatchID)

	// Initialize SMS batch results if they are not already set
	if sm.SmsBatch.Results == nil {
		sm.SmsBatch.Results = &model.SmsBatchResults{}
	}

	// Initialize delivery statistics
	startTime := time.Now()
	sm.SmsBatch.Results.CurrentPhase = "delivery"
	if sm.SmsBatch.Results.DeliveryStats == nil {
		sm.SmsBatch.Results.DeliveryStats = &model.SmsBatchPhaseStats{}
	}
	sm.SmsBatch.Results.DeliveryStats.StartTime = &startTime

	// Read packed data from Minio
	packedDataPath := path.Job.Path(sm.SmsBatch.BatchID, "sms_packed_data")
	data, err := sm.Watcher.Minio.Read(ctx, packedDataPath)
	if err != nil {
		return fmt.Errorf("failed to read packed data: %w", err)
	}

	log.Infow("Processing SMS delivery with partitioning", "batch_id", sm.SmsBatch.BatchID, "total_data", len(data))

	// Get partition configuration from params
	partitionCount := int32(4) // Default partition count
	if sm.SmsBatch.Params != nil && sm.SmsBatch.Params.PartitionCount > 0 {
		partitionCount = sm.SmsBatch.Params.PartitionCount
	}

	// Create delivery partitions (similar to Java's SmsBatchPartitionTask)
	partitions := sm.createDeliveryPartitions(data, partitionCount)
	totalProcessed := int64(0)
	totalSuccess := int64(0)
	totalFailed := int64(0)

	// Initialize partition statuses
	sm.SmsBatch.Results.PartitionStatuses = make([]*model.SmsBatchPartitionStatus, len(partitions))

	// Process partitions concurrently using processDeliveryPartitions
	workerCount := 4 // Default worker count
	if sm.SmsBatch.Params != nil && sm.SmsBatch.Params.PartitionCount > 0 {
		workerCount = int(sm.SmsBatch.Params.PartitionCount)
	}

	processed, success, failed, err := sm.processDeliveryPartitions(ctx, partitions, workerCount)
	if err != nil {
		log.Errorw("Failed to process delivery partitions", "error", err, "batch_id", sm.SmsBatch.BatchID)
		return err
	}

	totalProcessed = processed
	totalSuccess = success
	totalFailed = failed

	// Update progress to 100% after all partitions are processed
	sm.SmsBatch.Results.ProgressPercent = 100.0

	// Publish final progress update
	if sm.Watcher.EventPublisher != nil {
		if err := sm.Watcher.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
			RequestID:    fmt.Sprintf("%s-delivery-progress-final-%d", sm.SmsBatch.BatchID, time.Now().Unix()),
			BatchID:      sm.SmsBatch.BatchID,
			Status:       "delivery_running",
			CurrentPhase: "delivery",
			Progress:     100.0,
			Processed:    totalProcessed,
			Success:      totalSuccess,
			Failed:       totalFailed,
			UpdatedAt:    time.Now(),
		}); err != nil {
			log.Errorw("Failed to publish final progress update", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	// Update final delivery statistics
	endTime := time.Now()
	sm.SmsBatch.Results.DeliveryStats.Processed = totalProcessed
	sm.SmsBatch.Results.DeliveryStats.Success = totalSuccess
	sm.SmsBatch.Results.DeliveryStats.Failed = totalFailed
	sm.SmsBatch.Results.DeliveryStats.EndTime = &endTime
	sm.SmsBatch.Results.DeliveryStats.DurationSeconds = int64(endTime.Sub(startTime).Seconds())

	// Update overall batch statistics
	sm.SmsBatch.Results.ProcessedMessages = totalProcessed
	sm.SmsBatch.Results.SuccessMessages = totalSuccess
	sm.SmsBatch.Results.FailedMessages = totalFailed
	sm.SmsBatch.Results.ProgressPercent = 100.0

	// Update conditions
	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	// Publish final status update
	if sm.Watcher.EventPublisher != nil {
		if err := sm.Watcher.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
			RequestID:    fmt.Sprintf("%s-delivery-completed-%d", sm.SmsBatch.BatchID, time.Now().Unix()),
			BatchID:      sm.SmsBatch.BatchID,
			Status:       "delivery_completed",
			CurrentPhase: "delivery",
			Progress:     100.0,
			Processed:    totalProcessed,
			Success:      totalSuccess,
			Failed:       totalFailed,
			UpdatedAt:    endTime,
		}); err != nil {
			log.Errorw("Failed to publish final delivery status update", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	log.Infow("SMS batch delivery completed successfully",
		"batch_id", sm.SmsBatch.BatchID,
		"processed", totalProcessed,
		"success", totalSuccess,
		"failed", totalFailed,
		"partitions", len(partitions))

	return nil
}

// PreparationPause handles pausing the preparation phase.
func (sm *StateMachine) PreparationPause(ctx context.Context, event *fsm.Event) error {
	log.Infow("Pausing SMS preparation", "batch_id", sm.SmsBatch.BatchID)

	// Save current state and progress
	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentState = "paused"
	}

	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	// Publish preparation pause event
	if sm.Watcher.EventPublisher != nil {
		now := time.Now()
		if err := sm.Watcher.EventPublisher.PublishBatchOperation(ctx, &mqs.BatchOperationRequest{
			RequestID:   fmt.Sprintf("%s-pause-prep-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:     sm.SmsBatch.BatchID,
			Operation:   "pause",
			Phase:       "preparation",
			Reason:      "Manual pause operation",
			OperatorID:  "system",
			RequestedAt: now,
		}); err != nil {
			log.Errorw("Failed to publish preparation pause event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	return nil
}

// DeliveryPause handles pausing the delivery phase.
func (sm *StateMachine) DeliveryPause(ctx context.Context, event *fsm.Event) error {
	log.Infow("Pausing SMS delivery", "batch_id", sm.SmsBatch.BatchID)

	// Save current state and progress
	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentState = "paused"
	}

	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	// Publish delivery pause event
	if sm.Watcher.EventPublisher != nil {
		now := time.Now()
		if err := sm.Watcher.EventPublisher.PublishBatchOperation(ctx, &mqs.BatchOperationRequest{
			RequestID:   fmt.Sprintf("%s-pause-deliv-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:     sm.SmsBatch.BatchID,
			Operation:   "pause",
			Phase:       "delivery",
			Reason:      "Manual pause operation",
			OperatorID:  "system",
			RequestedAt: now,
		}); err != nil {
			log.Errorw("Failed to publish delivery pause event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	return nil
}

// PreparationResume handles resuming the preparation phase.
func (sm *StateMachine) PreparationResume(ctx context.Context, event *fsm.Event) error {
	log.Infow("Resuming SMS preparation", "batch_id", sm.SmsBatch.BatchID)

	// Clear paused state
	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentState = "running"
	}

	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	// Publish preparation resume event
	if sm.Watcher.EventPublisher != nil {
		now := time.Now()
		if err := sm.Watcher.EventPublisher.PublishBatchOperation(ctx, &mqs.BatchOperationRequest{
			RequestID:   fmt.Sprintf("%s-resume-prep-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:     sm.SmsBatch.BatchID,
			Operation:   "resume",
			Phase:       "preparation",
			Reason:      "Manual resume operation",
			OperatorID:  "system",
			RequestedAt: now,
		}); err != nil {
			log.Errorw("Failed to publish preparation resume event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	return nil
}

// DeliveryResume handles resuming the delivery phase.
func (sm *StateMachine) DeliveryResume(ctx context.Context, event *fsm.Event) error {
	log.Infow("Resuming SMS delivery", "batch_id", sm.SmsBatch.BatchID)

	// Clear paused state
	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentState = "running"
	}

	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	// Publish delivery resume event
	if sm.Watcher.EventPublisher != nil {
		now := time.Now()
		if err := sm.Watcher.EventPublisher.PublishBatchOperation(ctx, &mqs.BatchOperationRequest{
			RequestID:   fmt.Sprintf("%s-resume-deliv-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:     sm.SmsBatch.BatchID,
			Operation:   "resume",
			Phase:       "delivery",
			Reason:      "Manual resume operation",
			OperatorID:  "system",
			RequestedAt: now,
		}); err != nil {
			log.Errorw("Failed to publish delivery resume event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	return nil
}

// InitialExecute handles the initial state of the SMS batch.
func (sm *StateMachine) InitialExecute(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch initialized", "batch_id", sm.SmsBatch.BatchID)

	// Set default SMS batch params
	SetDefaultSmsBatchParams(sm.SmsBatch)

	// Initialize SMS batch results
	if sm.SmsBatch.Results == nil {
		sm.SmsBatch.Results = &model.SmsBatchResults{}
	}

	sm.SmsBatch.Results.CurrentPhase = "initialization"
	sm.SmsBatch.Results.CurrentState = event.FSM.Current()

	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	return nil
}

// PreparationReady handles the preparation ready state.
func (sm *StateMachine) PreparationReady(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch preparation ready", "batch_id", sm.SmsBatch.BatchID)

	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentPhase = "preparation"
		sm.SmsBatch.Results.CurrentState = event.FSM.Current()
	}

	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	return nil
}

// PreparationStart handles the start of preparation phase.
func (sm *StateMachine) PreparationStart(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch preparation started", "batch_id", sm.SmsBatch.BatchID)

	// Apply rate limiting for preparation
	if sm.Watcher.Limiter.Preparation != nil {
		sm.Watcher.Limiter.Preparation.Take()
	}

	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentPhase = "preparation"
		sm.SmsBatch.Results.CurrentState = event.FSM.Current()
	}

	// Initialize preparation stats
	if sm.SmsBatch.Results != nil && sm.SmsBatch.Results.PreparationStats == nil {
		now := time.Now()
		sm.SmsBatch.Results.PreparationStats = &model.SmsBatchPhaseStats{
			StartTime: &now,
		}
	}

	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	return nil
}

// DeliveryReady handles the delivery ready state.
func (sm *StateMachine) DeliveryReady(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch delivery ready", "batch_id", sm.SmsBatch.BatchID)

	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentPhase = "delivery"
		sm.SmsBatch.Results.CurrentState = event.FSM.Current()
	}

	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	return nil
}

// DeliveryStart handles the start of delivery phase.
func (sm *StateMachine) DeliveryStart(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch delivery started", "batch_id", sm.SmsBatch.BatchID)

	// Apply rate limiting for delivery
	if sm.Watcher.Limiter.Delivery != nil {
		sm.Watcher.Limiter.Delivery.Take()
	}

	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentPhase = "delivery"
		sm.SmsBatch.Results.CurrentState = event.FSM.Current()
	}

	// Initialize delivery stats
	if sm.SmsBatch.Results != nil && sm.SmsBatch.Results.DeliveryStats == nil {
		now := time.Now()
		sm.SmsBatch.Results.DeliveryStats = &model.SmsBatchPhaseStats{
			StartTime: &now,
		}
	}

	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	return nil
}

// BatchSucceeded handles the successful completion of the SMS batch.
func (sm *StateMachine) BatchSucceeded(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch succeeded", "batch_id", sm.SmsBatch.BatchID)

	now := time.Now()
	sm.SmsBatch.EndedAt = now

	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentPhase = "completed"
		sm.SmsBatch.Results.CurrentState = event.FSM.Current()
		sm.SmsBatch.Results.ProgressPercent = 100.0
	}

	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	// Publish batch success event
	if sm.Watcher.EventPublisher != nil {
		if err := sm.Watcher.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
			RequestID:    fmt.Sprintf("%s-success-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:      sm.SmsBatch.BatchID,
			Status:       SmsBatchSucceeded,
			CurrentPhase: "completed",
			Progress:     100.0,
			Processed:    sm.SmsBatch.Results.ProcessedMessages,
			Success:      sm.SmsBatch.Results.SuccessMessages,
			Failed:       sm.SmsBatch.Results.FailedMessages,
			UpdatedAt:    now,
		}); err != nil {
			log.Errorw("Failed to publish batch success event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	return nil
}

// BatchFailed handles the failure of the SMS batch.
func (sm *StateMachine) BatchFailed(ctx context.Context, event *fsm.Event) error {
	log.Errorw("SMS batch failed", "batch_id", sm.SmsBatch.BatchID, "error", event.Err)

	now := time.Now()
	sm.SmsBatch.EndedAt = now

	errorMessage := ""
	if event.Err != nil {
		errorMessage = event.Err.Error()
	}

	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentPhase = "failed"
		sm.SmsBatch.Results.CurrentState = event.FSM.Current()
		sm.SmsBatch.Results.ErrorMessage = errorMessage
	}

	cond := jobconditionsutil.FalseCondition(event.FSM.Current(), errorMessage)
	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, cond)

	// Publish batch failure event
	if sm.Watcher.EventPublisher != nil {
		if err := sm.Watcher.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
			RequestID:    fmt.Sprintf("%s-failed-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:      sm.SmsBatch.BatchID,
			Status:       SmsBatchFailed,
			CurrentPhase: "failed",
			Progress:     0.0,
			Processed:    sm.SmsBatch.Results.ProcessedMessages,
			Success:      sm.SmsBatch.Results.SuccessMessages,
			Failed:       sm.SmsBatch.Results.FailedMessages,
			ErrorMessage: errorMessage,
			UpdatedAt:    now,
		}); err != nil {
			log.Errorw("Failed to publish batch failure event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	return nil
}

// validateBatchParams validates SMS batch parameters
func (sm *StateMachine) validateBatchParams() error {
	if sm.SmsBatch.BatchID == "" {
		return fmt.Errorf("batch ID is required")
	}
	if sm.SmsBatch.TableStorageName == "" {
		return fmt.Errorf("table storage name is required")
	}
	// Add more validation as needed
	return nil
}

// shouldPausePreparation checks if preparation should be paused
func (sm *StateMachine) shouldPausePreparation(ctx context.Context) bool {
	// Check if delivery phase is running (similar to Java logic)
	if sm.SmsBatch.CurrentPhase == "delivery" {
		return true
	}
	// Check if batch is paused
	if sm.SmsBatch.Status == "paused" {
		return true
	}
	return false
}

// processSmsPreparationBatch processes a batch of SMS data
func (sm *StateMachine) processSmsPreparationBatch(ctx context.Context, batchData []string) (processed, success, failed int) {
	// Simulate processing logic
	processed = len(batchData)
	success = int(float64(processed) * 0.95) // 95% success rate
	failed = processed - success

	// Add small delay to simulate processing
	time.Sleep(100 * time.Millisecond)

	return processed, success, failed
}

// createDeliveryPartitions creates delivery partitions from data
func (sm *StateMachine) createDeliveryPartitions(data []string, partitionCount int32) [][]string {
	if len(data) == 0 || partitionCount <= 0 {
		return [][]string{}
	}

	partitions := make([][]string, partitionCount)
	partitionSize := len(data) / int(partitionCount)
	if partitionSize == 0 {
		partitionSize = 1
	}

	for i := 0; i < len(data); i++ {
		partitionIndex := i / partitionSize
		if partitionIndex >= int(partitionCount) {
			partitionIndex = int(partitionCount) - 1
		}
		partitions[partitionIndex] = append(partitions[partitionIndex], data[i])
	}

	// Remove empty partitions
	var result [][]string
	for _, partition := range partitions {
		if len(partition) > 0 {
			result = append(result, partition)
		}
	}

	return result
}

// processDeliveryPartition processes a single delivery partition
func (sm *StateMachine) processDeliveryPartition(ctx context.Context, partitionID string, partition []string) (processed, success, failed int, err error) {
	log.Infow("Processing delivery partition", "partition_id", partitionID, "size", len(partition))

	// Simulate SMS delivery processing
	processed = len(partition)

	// Simulate delivery with some failures
	successRate := 0.92 // 92% success rate
	success = int(float64(processed) * successRate)
	failed = processed - success

	// Simulate delivery time
	deliveryTime := time.Duration(len(partition)) * 10 * time.Millisecond
	time.Sleep(deliveryTime)

	// TODO: Implement actual SMS delivery logic
	// - Send SMS messages to provider
	// - Handle delivery responses
	// - Update delivery status
	// - Handle retries for failed messages

	log.Infow("Delivery partition completed",
		"partition_id", partitionID,
		"processed", processed,
		"success", success,
		"failed", failed)

	return processed, success, failed, nil
}

// BatchAborted handles the abortion of the SMS batch.
func (sm *StateMachine) BatchAborted(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch aborted", "batch_id", sm.SmsBatch.BatchID)

	// Get abort reason from event args
	abortReason := "Batch processing aborted"
	operatorID := "system"
	if event.Args != nil && len(event.Args) > 0 {
		if reason, ok := event.Args[0].(string); ok {
			abortReason = reason
		}
		if len(event.Args) > 1 {
			if operator, ok := event.Args[1].(string); ok {
				operatorID = operator
			}
		}
	}

	now := time.Now()
	sm.SmsBatch.EndedAt = now
	sm.SmsBatch.Status = "aborted"

	// Update results with abort information
	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentPhase = "aborted"
		sm.SmsBatch.Results.CurrentState = event.FSM.Current()

		// Finalize any ongoing phase statistics
		if sm.SmsBatch.Results.PreparationStats != nil && sm.SmsBatch.Results.PreparationStats.EndTime == nil {
			sm.SmsBatch.Results.PreparationStats.EndTime = &now
		}
		if sm.SmsBatch.Results.DeliveryStats != nil && sm.SmsBatch.Results.DeliveryStats.EndTime == nil {
			sm.SmsBatch.Results.DeliveryStats.EndTime = &now
			sm.SmsBatch.Results.DeliveryStats.DurationSeconds = int64(now.Sub(*sm.SmsBatch.Results.DeliveryStats.StartTime).Seconds())
		}

		// Log abort statistics
		log.Infow("Batch abort statistics",
			"batch_id", sm.SmsBatch.BatchID,
			"processed", sm.SmsBatch.Results.ProcessedMessages,
			"success", sm.SmsBatch.Results.SuccessMessages,
			"failed", sm.SmsBatch.Results.FailedMessages,
			"progress", sm.SmsBatch.Results.ProgressPercent)
	}

	// Set abort condition
	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	// Publish batch abort events
	if sm.Watcher.EventPublisher != nil {
		// Publish abort operation event
		if err := sm.Watcher.EventPublisher.PublishBatchOperation(ctx, &mqs.BatchOperationRequest{
			RequestID:   fmt.Sprintf("%s-abort-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:     sm.SmsBatch.BatchID,
			Operation:   "abort",
			Phase:       sm.SmsBatch.Results.CurrentPhase,
			Reason:      abortReason,
			OperatorID:  operatorID,
			RequestedAt: now,
		}); err != nil {
			log.Errorw("Failed to publish batch abort operation event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}

		// Publish status update for abort
		if err := sm.Watcher.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
			RequestID:    fmt.Sprintf("%s-abort-status-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:      sm.SmsBatch.BatchID,
			Status:       "aborted",
			CurrentPhase: sm.SmsBatch.Results.CurrentPhase,
			Progress:     sm.SmsBatch.Results.ProgressPercent,
			Processed:    sm.SmsBatch.Results.ProcessedMessages,
			Success:      sm.SmsBatch.Results.SuccessMessages,
			Failed:       sm.SmsBatch.Results.FailedMessages,
			UpdatedAt:    now,
		}); err != nil {
			log.Errorw("Failed to publish batch abort status update", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	log.Infow("SMS batch successfully aborted",
		"batch_id", sm.SmsBatch.BatchID,
		"final_status", sm.SmsBatch.Status,
		"abort_reason", abortReason)

	return nil
}

// BatchPaused handles the pausing of the SMS batch.
func (sm *StateMachine) BatchPaused(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch paused", "batch_id", sm.SmsBatch.BatchID)

	// Update batch status
	sm.SmsBatch.Status = "paused"

	// Update current phase if results exist
	if sm.SmsBatch.Results != nil {
		// Save the current phase before pausing
		if sm.SmsBatch.Results.CurrentPhase == "" {
			sm.SmsBatch.Results.CurrentPhase = "preparation" // Default to preparation if not set
		}

		// Update progress and status
		if sm.SmsBatch.Results.CurrentPhase == "preparation" && sm.SmsBatch.Results.PreparationStats != nil {
			// Sync preparation statistics before pausing
			log.Infow("Syncing preparation statistics before pause", "batch_id", sm.SmsBatch.BatchID)
			// In Java, this would call prepareSyncer.syncStatistics()
		}

		if sm.SmsBatch.Results.CurrentPhase == "delivery" && sm.SmsBatch.Results.DeliveryStats != nil {
			// Sync delivery statistics before pausing
			log.Infow("Syncing delivery statistics before pause", "batch_id", sm.SmsBatch.BatchID)
			// In Java, this would call deliverySyncer.syncStatistics()
		}
	}

	// Set pause condition
	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	// Publish pause event with detailed information
	if sm.Watcher.EventPublisher != nil {
		pauseReason := "User requested pause"
		operatorID := "system"
		if event.Args != nil && len(event.Args) > 0 {
			if reason, ok := event.Args[0].(string); ok {
				pauseReason = reason
			}
			if len(event.Args) > 1 {
				if operator, ok := event.Args[1].(string); ok {
					operatorID = operator
				}
			}
		}

		currentPhase := "preparation"
		if sm.SmsBatch.Results != nil && sm.SmsBatch.Results.CurrentPhase != "" {
			currentPhase = sm.SmsBatch.Results.CurrentPhase
		}

		if err := sm.Watcher.EventPublisher.PublishSmsBatchPaused(ctx, sm.SmsBatch.BatchID, currentPhase, pauseReason, operatorID); err != nil {
			log.Errorw("Failed to publish SMS batch paused event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}

		// Also publish status update
		if err := sm.Watcher.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
			RequestID:    fmt.Sprintf("%s-paused-%d", sm.SmsBatch.BatchID, time.Now().Unix()),
			BatchID:      sm.SmsBatch.BatchID,
			Status:       "paused",
			CurrentPhase: sm.SmsBatch.Results.CurrentPhase,
			Progress:     sm.SmsBatch.Results.ProgressPercent,
			Processed:    sm.SmsBatch.Results.ProcessedMessages,
			Success:      sm.SmsBatch.Results.SuccessMessages,
			Failed:       sm.SmsBatch.Results.FailedMessages,
			UpdatedAt:    time.Now(),
		}); err != nil {
			log.Errorw("Failed to publish batch status update for pause", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	log.Infow("SMS batch successfully paused",
		"batch_id", sm.SmsBatch.BatchID,
		"current_phase", sm.SmsBatch.Results.CurrentPhase,
		"progress", sm.SmsBatch.Results.ProgressPercent)

	return nil
}

// BatchResumed handles the resuming of the SMS batch.
func (sm *StateMachine) BatchResumed(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch resumed", "batch_id", sm.SmsBatch.BatchID)

	// Clear paused state
	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentState = "running"
	}

	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	// Publish resume event
	if sm.Watcher.EventPublisher != nil {
		now := time.Now()
		resumeReason := "User requested resume"
		if event.Args != nil && len(event.Args) > 0 {
			if reason, ok := event.Args[0].(string); ok {
				resumeReason = reason
			}
		}

		if err := sm.Watcher.EventPublisher.PublishBatchOperation(ctx, &mqs.BatchOperationRequest{
			RequestID:   fmt.Sprintf("%s-resume-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:     sm.SmsBatch.BatchID,
			Operation:   "resume",
			Phase:       sm.SmsBatch.Results.CurrentPhase,
			Reason:      resumeReason,
			OperatorID:  "system",
			RequestedAt: now,
		}); err != nil {
			log.Errorw("Failed to publish batch resume event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}

		// Also publish status update
		if err := sm.Watcher.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
			RequestID:    fmt.Sprintf("%s-resumed-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:      sm.SmsBatch.BatchID,
			Status:       "running",
			CurrentPhase: sm.SmsBatch.Results.CurrentPhase,
			Progress:     sm.SmsBatch.Results.ProgressPercent,
			Processed:    sm.SmsBatch.Results.ProcessedMessages,
			Success:      sm.SmsBatch.Results.SuccessMessages,
			Failed:       sm.SmsBatch.Results.FailedMessages,
			UpdatedAt:    now,
		}); err != nil {
			log.Errorw("Failed to publish batch status update for resume", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	log.Infow("SMS batch successfully resumed",
		"batch_id", sm.SmsBatch.BatchID,
		"current_phase", sm.SmsBatch.Results.CurrentPhase,
		"progress", sm.SmsBatch.Results.ProgressPercent)

	return nil
}

// BatchRetry handles the retry of the SMS batch.
func (sm *StateMachine) BatchRetry(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch retry initiated", "batch_id", sm.SmsBatch.BatchID)

	// Reset batch status and error conditions
	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentState = "retrying"
		sm.SmsBatch.Results.ErrorMessage = ""

		// Increment retry count
		if sm.SmsBatch.Results.RetryCount == 0 {
			sm.SmsBatch.Results.RetryCount = 1
		} else {
			sm.SmsBatch.Results.RetryCount++
		}

		// Reset progress for retry
		sm.SmsBatch.Results.ProgressPercent = 0.0

		// Reset phase statistics for retry
		if sm.SmsBatch.Results.CurrentPhase == "preparation" {
			sm.SmsBatch.Results.PreparationStats = &model.SmsBatchPhaseStats{
				StartTime: &[]time.Time{time.Now()}[0],
			}
		} else if sm.SmsBatch.Results.CurrentPhase == "delivery" {
			sm.SmsBatch.Results.DeliveryStats = &model.SmsBatchPhaseStats{
				StartTime: &[]time.Time{time.Now()}[0],
			}
		}
	}

	// Clear failed conditions and set retry condition
	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	// Publish retry event
	if sm.Watcher.EventPublisher != nil {
		now := time.Now()
		retryReason := "Automatic retry after failure"
		if event.Args != nil && len(event.Args) > 0 {
			if reason, ok := event.Args[0].(string); ok {
				retryReason = reason
			}
		}

		if err := sm.Watcher.EventPublisher.PublishBatchOperation(ctx, &mqs.BatchOperationRequest{
			RequestID:   fmt.Sprintf("%s-retry-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:     sm.SmsBatch.BatchID,
			Operation:   "retry",
			Phase:       sm.SmsBatch.Results.CurrentPhase,
			Reason:      retryReason,
			OperatorID:  "system",
			RequestedAt: now,
		}); err != nil {
			log.Errorw("Failed to publish batch retry event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}

		// Also publish status update
		if err := sm.Watcher.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
			RequestID:    fmt.Sprintf("%s-retry-status-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:      sm.SmsBatch.BatchID,
			Status:       "retrying",
			CurrentPhase: sm.SmsBatch.Results.CurrentPhase,
			Progress:     0.0,
			Processed:    0,
			Success:      0,
			Failed:       0,
			UpdatedAt:    now,
		}); err != nil {
			log.Errorw("Failed to publish batch status update for retry", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	log.Infow("SMS batch retry initiated successfully",
		"batch_id", sm.SmsBatch.BatchID,
		"retry_count", sm.SmsBatch.Results.RetryCount,
		"current_phase", sm.SmsBatch.Results.CurrentPhase)

	return nil
}

// BatchCompleted handles the completion of the SMS batch.
func (sm *StateMachine) BatchCompleted(ctx context.Context, event *fsm.Event) error {
	log.Infow("SMS batch completed", "batch_id", sm.SmsBatch.BatchID)

	// Update batch status and final timestamps
	sm.SmsBatch.Status = "completed"
	now := time.Now()
	sm.SmsBatch.EndedAt = now

	// Update final results
	if sm.SmsBatch.Results != nil {
		sm.SmsBatch.Results.CurrentPhase = "completed"
		sm.SmsBatch.Results.CurrentState = "completed"
		sm.SmsBatch.Results.ProgressPercent = 100.0

		// Calculate total duration
		if !sm.SmsBatch.StartedAt.IsZero() {
			duration := now.Sub(sm.SmsBatch.StartedAt)
			log.Infow("Batch completion duration", "batch_id", sm.SmsBatch.BatchID, "duration", duration)
		}

		// Finalize phase statistics
		if sm.SmsBatch.Results.PreparationStats != nil && sm.SmsBatch.Results.PreparationStats.EndTime == nil {
			sm.SmsBatch.Results.PreparationStats.EndTime = &now
		}
		if sm.SmsBatch.Results.DeliveryStats != nil && sm.SmsBatch.Results.DeliveryStats.EndTime == nil {
			sm.SmsBatch.Results.DeliveryStats.EndTime = &now
			sm.SmsBatch.Results.DeliveryStats.DurationSeconds = int64(now.Sub(*sm.SmsBatch.Results.DeliveryStats.StartTime).Seconds())
		}

		// Calculate success rate
		if sm.SmsBatch.Results.ProcessedMessages > 0 {
			successRate := float64(sm.SmsBatch.Results.SuccessMessages) / float64(sm.SmsBatch.Results.ProcessedMessages) * 100.0
			log.Infow("Batch completion statistics",
				"batch_id", sm.SmsBatch.BatchID,
				"total_processed", sm.SmsBatch.Results.ProcessedMessages,
				"success_count", sm.SmsBatch.Results.SuccessMessages,
				"failed_count", sm.SmsBatch.Results.FailedMessages,
				"success_rate", fmt.Sprintf("%.2f%%", successRate))
		}
	}

	// Set completion condition
	sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, jobconditionsutil.TrueCondition(event.FSM.Current()))

	// Publish completion event
	if sm.Watcher.EventPublisher != nil {
		// Publish final status update
		if err := sm.Watcher.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
			RequestID:    fmt.Sprintf("%s-completed-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:      sm.SmsBatch.BatchID,
			Status:       "completed",
			CurrentPhase: "completed",
			Progress:     100.0,
			Processed:    sm.SmsBatch.Results.ProcessedMessages,
			Success:      sm.SmsBatch.Results.SuccessMessages,
			Failed:       sm.SmsBatch.Results.FailedMessages,
			UpdatedAt:    now,
		}); err != nil {
			log.Errorw("Failed to publish batch completion status update", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}

		// Publish completion operation event
		if err := sm.Watcher.EventPublisher.PublishBatchOperation(ctx, &mqs.BatchOperationRequest{
			RequestID:   fmt.Sprintf("%s-completion-%d", sm.SmsBatch.BatchID, now.Unix()),
			BatchID:     sm.SmsBatch.BatchID,
			Operation:   "complete",
			Phase:       "completed",
			Reason:      "Batch processing completed successfully",
			OperatorID:  "system",
			RequestedAt: now,
		}); err != nil {
			log.Errorw("Failed to publish batch completion operation event", "batch_id", sm.SmsBatch.BatchID, "error", err)
		}
	}

	log.Infow("SMS batch completed successfully",
		"batch_id", sm.SmsBatch.BatchID,
		"final_status", sm.SmsBatch.Status,
		"total_processed", sm.SmsBatch.Results.ProcessedMessages,
		"success_count", sm.SmsBatch.Results.SuccessMessages,
		"failed_count", sm.SmsBatch.Results.FailedMessages)

	return nil
}

// EnterState handles the state transition of the state machine
// and updates the SmsBatch's status and conditions based on the event.
func (sm *StateMachine) EnterState(ctx context.Context, event *fsm.Event) error {
	sm.SmsBatch.Status = event.FSM.Current()
	now := time.Now()

	// Record the start time of the SMS batch
	if sm.SmsBatch.Status == SmsBatchPreparationReady {
		sm.SmsBatch.StartedAt = now
	}

	// Unified handling logic for SMS batch failure
	if event.Err != nil || isSmsBatchTimeout(sm.SmsBatch) {
		sm.SmsBatch.Status = SmsBatchFailed
		sm.SmsBatch.EndedAt = now

		var cond *v1.JobCondition
		errorMessage := ""
		if isSmsBatchTimeout(sm.SmsBatch) {
			log.Infow("SMS batch task timeout")
			errorMessage = "SMS batch task exceeded timeout seconds"
			cond = jobconditionsutil.FalseCondition(event.FSM.Current(), errorMessage)
		} else {
			errorMessage = event.Err.Error()
			cond = jobconditionsutil.FalseCondition(event.FSM.Current(), errorMessage)
		}

		sm.SmsBatch.Conditions = jobconditionsutil.Set(sm.SmsBatch.Conditions, cond)

		// Publish batch failure event
		if sm.Watcher.EventPublisher != nil {
			if err := sm.Watcher.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
				RequestID:    fmt.Sprintf("%s-failed-%d", sm.SmsBatch.BatchID, now.Unix()),
				BatchID:      sm.SmsBatch.BatchID,
				Status:       SmsBatchFailed,
				CurrentPhase: sm.SmsBatch.Results.CurrentPhase,
				Progress:     0.0,
				Processed:    sm.SmsBatch.Results.ProcessedMessages,
				Success:      sm.SmsBatch.Results.SuccessMessages,
				Failed:       sm.SmsBatch.Results.FailedMessages,
				ErrorMessage: errorMessage,
				UpdatedAt:    now,
			}); err != nil {
				log.Errorw("Failed to publish batch failure event", "batch_id", sm.SmsBatch.BatchID, "error", err)
			}
		}
	}

	// Record completion time for final states
	if sm.SmsBatch.Status == SmsBatchSucceeded || sm.SmsBatch.Status == SmsBatchFailed || sm.SmsBatch.Status == SmsBatchAborted {
		sm.SmsBatch.EndedAt = now

		// Publish batch completion event for successful batches
		if sm.SmsBatch.Status == SmsBatchSucceeded && sm.Watcher.EventPublisher != nil {
			if err := sm.Watcher.EventPublisher.PublishBatchStatusUpdate(ctx, &mqs.BatchStatusUpdateRequest{
				RequestID:    fmt.Sprintf("%s-success-%d", sm.SmsBatch.BatchID, now.Unix()),
				BatchID:      sm.SmsBatch.BatchID,
				Status:       SmsBatchSucceeded,
				CurrentPhase: "completed",
				Progress:     100.0,
				Processed:    sm.SmsBatch.Results.ProcessedMessages,
				Success:      sm.SmsBatch.Results.SuccessMessages,
				Failed:       sm.SmsBatch.Results.FailedMessages,
				UpdatedAt:    now,
			}); err != nil {
				log.Errorw("Failed to publish batch success event", "batch_id", sm.SmsBatch.BatchID, "error", err)
			}
		}

		// Publish batch abort event
		if sm.SmsBatch.Status == SmsBatchAborted && sm.Watcher.EventPublisher != nil {
			if err := sm.Watcher.EventPublisher.PublishBatchOperation(ctx, &mqs.BatchOperationRequest{
				RequestID:   fmt.Sprintf("%s-abort-%d", sm.SmsBatch.BatchID, now.Unix()),
				BatchID:     sm.SmsBatch.BatchID,
				Operation:   "abort",
				Reason:      "Batch processing aborted",
				OperatorID:  "system",
				RequestedAt: now,
			}); err != nil {
				log.Errorw("Failed to publish batch abort event", "batch_id", sm.SmsBatch.BatchID, "error", err)
			}
		}
	}

	if err := sm.Watcher.Store.SmsBatch().Update(ctx, sm.SmsBatch); err != nil {
		return err
	}

	return nil
}
