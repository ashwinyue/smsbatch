package manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// PartitionManager handles SMS batch partition creation and processing
type PartitionManager struct{}

// NewPartitionManager creates a new PartitionManager instance
func NewPartitionManager() *PartitionManager {
	return &PartitionManager{}
}

// CreateDeliveryPartitions creates delivery partitions from data
func (pm *PartitionManager) CreateDeliveryPartitions(data []string, partitionCount int32) [][]string {
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

// ProcessDeliveryPartitions processes delivery partitions concurrently
func (pm *PartitionManager) ProcessDeliveryPartitions(ctx context.Context, sm interface{}, partitions [][]string, workerCount int) (int64, int64, int64, error) {
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
				processed, success, failed, err := pm.ProcessDeliveryPartition(ctx, sm, partitionID, partition)
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

// CreateSmsRecordDeliveryPartitions creates delivery partitions from SMS records (替代Java项目中的分区逻辑)
func (pm *PartitionManager) CreateSmsRecordDeliveryPartitions(records []*model.SmsRecordM, partitionCount int32) [][]*model.SmsRecordM {
	if len(records) == 0 || partitionCount <= 0 {
		return [][]*model.SmsRecordM{}
	}

	partitions := make([][]*model.SmsRecordM, partitionCount)
	partitionSize := len(records) / int(partitionCount)
	if partitionSize == 0 {
		partitionSize = 1
	}

	for i := 0; i < len(records); i++ {
		partitionIndex := i / partitionSize
		if partitionIndex >= int(partitionCount) {
			partitionIndex = int(partitionCount) - 1
		}
		partitions[partitionIndex] = append(partitions[partitionIndex], records[i])
	}

	// Remove empty partitions
	var result [][]*model.SmsRecordM
	for _, partition := range partitions {
		if len(partition) > 0 {
			result = append(result, partition)
		}
	}

	return result
}

// ProcessSmsRecordDeliveryPartitions processes SMS record delivery partitions concurrently (替代Java项目中的分区处理)
func (pm *PartitionManager) ProcessSmsRecordDeliveryPartitions(ctx context.Context, sm *model.SmsBatchM, partitions [][]*model.SmsRecordM, workerCount int) (int64, int64, int64, error) {
	if len(partitions) == 0 {
		return 0, 0, 0, nil
	}

	stats := NewStats()
	var wg sync.WaitGroup
	partitionChan := make(chan []*model.SmsRecordM, len(partitions))
	errorChan := make(chan error, len(partitions))

	// Start workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for partition := range partitionChan {
				partitionID := fmt.Sprintf("worker-%d-partition-%d", workerID, time.Now().UnixNano())
				processed, success, failed, err := pm.ProcessSmsRecordDeliveryPartition(ctx, sm, partitionID, partition)
				if err != nil {
					errorChan <- err
					return
				}
				stats.Add(int64(processed), int64(success), int64(failed))
				log.Debugw("Worker completed SMS record partition", "worker_id", workerID, "partition_size", len(partition))
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

// ProcessSmsRecordDeliveryPartition processes a single SMS record delivery partition (替代Java项目中的单分区处理)
func (pm *PartitionManager) ProcessSmsRecordDeliveryPartition(ctx context.Context, sm *model.SmsBatchM, partitionID string, partition []*model.SmsRecordM) (processed, success, failed int, err error) {
	log.Infow("Processing SMS record delivery partition", "partition_id", partitionID, "size", len(partition), "batch_id", sm.BatchID)

	processed = len(partition)
	success = 0
	failed = 0

	// Process each SMS record in the partition
	for _, record := range partition {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return processed, success, failed, ctx.Err()
		default:
		}

		// Simulate SMS delivery processing
		deliverySuccess := pm.simulateSmsDelivery(record)
		if deliverySuccess {
			// Mark as delivered
			record.Status = model.SmsRecordStatusDelivered
			record.DeliveredTime = &[]time.Time{time.Now()}[0]
			success++
		} else {
			// Mark as failed
			record.MarkAsFailed("delivery failed")
			failed++
		}
		record.UpdatedAt = time.Now()
	}

	// Simulate delivery time based on partition size
	deliveryTime := time.Duration(len(partition)) * 5 * time.Millisecond
	time.Sleep(deliveryTime)

	log.Infow("SMS record delivery partition completed",
		"partition_id", partitionID,
		"batch_id", sm.BatchID,
		"processed", processed,
		"success", success,
		"failed", failed)

	return processed, success, failed, nil
}

// simulateSmsDelivery simulates SMS delivery with realistic success rate
func (pm *PartitionManager) simulateSmsDelivery(record *model.SmsRecordM) bool {
	// Simulate different success rates based on phone number patterns
	// This mimics real-world scenarios where some numbers might be invalid
	if len(record.PhoneNumber) < 10 {
		return false // Invalid phone number
	}

	// Simulate 92% success rate
	successRate := 0.92
	return time.Now().UnixNano()%100 < int64(successRate*100)
}

// ProcessDeliveryPartition processes a single delivery partition
func (pm *PartitionManager) ProcessDeliveryPartition(ctx context.Context, sm interface{}, partitionID string, partition []string) (processed, success, failed int, err error) {
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
