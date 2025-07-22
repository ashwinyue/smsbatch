package smsbatch

import (
	"context"
	"fmt"

	"github.com/onexstack/onexstack/pkg/log"
	"github.com/onexstack/onexstack/pkg/store/where"

	"github.com/ashwinyue/dcp/internal/nightwatch/pkg/conversion"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	apiv1 "github.com/ashwinyue/dcp/pkg/api/nightwatch/v1"
)

type ISmsBatchV1 interface {
	Create(ctx context.Context, rq *apiv1.CreateSmsBatchRequest) (*apiv1.CreateSmsBatchResponse, error)
	Update(ctx context.Context, rq *apiv1.UpdateSmsBatchRequest) (*apiv1.UpdateSmsBatchResponse, error)
	Delete(ctx context.Context, rq *apiv1.DeleteSmsBatchRequest) (*apiv1.DeleteSmsBatchResponse, error)
	Get(ctx context.Context, rq *apiv1.GetSmsBatchRequest) (*apiv1.GetSmsBatchResponse, error)
	List(ctx context.Context, rq *apiv1.ListSmsBatchRequest) (*apiv1.ListSmsBatchResponse, error)
}

type smsBatchV1 struct {
	store store.IStore
}

func New(store store.IStore) ISmsBatchV1 {
	return &smsBatchV1{
		store: store,
	}
}

func (s *smsBatchV1) Create(ctx context.Context, rq *apiv1.CreateSmsBatchRequest) (*apiv1.CreateSmsBatchResponse, error) {
	smsBatchM := conversion.SmsBatchV1ToSmsBatchM(rq.SmsBatch)

	// 设置默认值
	if smsBatchM.Watcher == "" {
		smsBatchM.Watcher = "smsbatch"
	}

	if err := s.store.SmsBatch().Create(ctx, smsBatchM); err != nil {
		log.Errorw(err, "Failed to create SMS batch", "name", smsBatchM.Name)
		return nil, err
	}

	log.Infow("SMS batch created successfully", "batch_id", smsBatchM.BatchID, "name", smsBatchM.Name)

	return &apiv1.CreateSmsBatchResponse{
		BatchID: smsBatchM.BatchID,
	}, nil
}

func (s *smsBatchV1) Update(ctx context.Context, rq *apiv1.UpdateSmsBatchRequest) (*apiv1.UpdateSmsBatchResponse, error) {
	// 获取现有的SMS批次
	existingBatch, err := s.store.SmsBatch().Get(ctx, where.F("batch_id", rq.BatchID))
	if err != nil {
		log.Errorw(err, "Failed to get SMS batch for update", "batch_id", rq.BatchID)
		return nil, err
	}

	// 更新字段
	if rq.Name != nil {
		existingBatch.Name = *rq.Name
	}
	if rq.Description != nil {
		existingBatch.Description = *rq.Description
	}
	if rq.CampaignID != nil {
		existingBatch.CampaignID = *rq.CampaignID
	}
	if rq.TaskID != nil {
		existingBatch.TaskID = *rq.TaskID
	}
	if rq.TableStorageName != nil {
		existingBatch.TableStorageName = *rq.TableStorageName
	}
	if rq.ContentID != nil {
		existingBatch.ContentID = *rq.ContentID
	}
	if rq.Content != nil {
		existingBatch.Content = *rq.Content
	}
	if rq.ContentSignature != nil {
		existingBatch.ContentSignature = *rq.ContentSignature
	}
	if rq.Url != nil {
		existingBatch.URL = *rq.Url
	}
	if rq.CombineMemberIDWithURL != nil {
		existingBatch.CombineMemberIDWithURL = *rq.CombineMemberIDWithURL
	}
	if rq.AutoTrigger != nil {
		existingBatch.AutoTrigger = *rq.AutoTrigger
	}
	if rq.ScheduleTime != nil {
		scheduleTime := rq.ScheduleTime.AsTime()
		existingBatch.ScheduleTime = &scheduleTime
	}
	if rq.ExtCode != nil {
		existingBatch.ExtCode = int32(len(*rq.ExtCode)) // Convert string to int32
	}
	if rq.TaskCode != nil {
		existingBatch.TaskCode = *rq.TaskCode
	}
	if rq.ProviderType != nil {
		existingBatch.ProviderType = *rq.ProviderType
	}
	if rq.MessageType != nil {
		existingBatch.MessageType = *rq.MessageType
	}
	if rq.MessageCategory != nil {
		existingBatch.MessageCategory = *rq.MessageCategory
	}
	if rq.Region != nil {
		existingBatch.Region = *rq.Region
	}
	if rq.Source != nil {
		existingBatch.Source = *rq.Source
	}
	if rq.Watcher != nil {
		existingBatch.Watcher = *rq.Watcher
	}
	if rq.Suspend != nil {
		if *rq.Suspend {
			existingBatch.Suspend = 1
		} else {
			existingBatch.Suspend = 0
		}
	}

	if err := s.store.SmsBatch().Update(ctx, existingBatch); err != nil {
		log.Errorw(err, "Failed to update SMS batch", "batch_id", rq.BatchID)
		return nil, err
	}

	log.Infow("SMS batch updated successfully", "batch_id", rq.BatchID)

	return &apiv1.UpdateSmsBatchResponse{}, nil
}

func (s *smsBatchV1) Delete(ctx context.Context, rq *apiv1.DeleteSmsBatchRequest) (*apiv1.DeleteSmsBatchResponse, error) {
	for _, batchID := range rq.BatchIDs {
		if err := s.store.SmsBatch().Delete(ctx, where.F("batch_id", batchID)); err != nil {
			log.Errorw(err, "Failed to delete SMS batch", "batch_id", batchID)
			return nil, fmt.Errorf("failed to delete SMS batch %s: %w", batchID, err)
		}
		log.Infow("SMS batch deleted successfully", "batch_id", batchID)
	}

	return &apiv1.DeleteSmsBatchResponse{}, nil
}

func (s *smsBatchV1) Get(ctx context.Context, rq *apiv1.GetSmsBatchRequest) (*apiv1.GetSmsBatchResponse, error) {
	smsBatchM, err := s.store.SmsBatch().Get(ctx, where.F("batch_id", rq.BatchID))
	if err != nil {
		log.Errorw(err, "Failed to get SMS batch", "batch_id", rq.BatchID)
		return nil, err
	}

	smsBatch := conversion.SmsBatchMToSmsBatchV1(smsBatchM)

	return &apiv1.GetSmsBatchResponse{
		SmsBatch: smsBatch,
	}, nil
}

func (s *smsBatchV1) List(ctx context.Context, rq *apiv1.ListSmsBatchRequest) (*apiv1.ListSmsBatchResponse, error) {
	whr := where.T(ctx).P(int(rq.Offset), int(rq.Limit))
	total, smsBatchMs, err := s.store.SmsBatch().List(ctx, whr)
	if err != nil {
		log.Errorw(err, "Failed to list SMS batches")
		return nil, err
	}

	smsBatches := make([]*apiv1.SmsBatch, 0, len(smsBatchMs))
	for _, smsBatchM := range smsBatchMs {
		smsBatch := conversion.SmsBatchMToSmsBatchV1(smsBatchM)
		smsBatches = append(smsBatches, smsBatch)
	}

	return &apiv1.ListSmsBatchResponse{
		Total:      total,
		SmsBatches: smsBatches,
	}, nil
}
