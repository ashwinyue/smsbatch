package mqs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/provider"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/nightwatch/types"
	"github.com/ashwinyue/dcp/internal/pkg/log"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

// CommonMessageConsumer handles common message consumption
type CommonMessageConsumer struct {
	ctx       context.Context
	providers *provider.ProviderFactory
	store     types.DataStore
	idt       types.IdempotentChecker
	logger    types.HistoryLogger
}

// CommonMessageHandler provides shared functionality for message processing
type CommonMessageHandler struct {
	ctx               context.Context
	store             store.Store
	idempotentChecker types.IdempotentChecker
	historyLogger     types.HistoryLogger
}

// NewCommonMessageConsumer creates a new common message consumer
func NewCommonMessageConsumer(ctx context.Context, providers *provider.ProviderFactory, store types.DataStore, idt types.IdempotentChecker, logger types.HistoryLogger) *CommonMessageConsumer {
	return &CommonMessageConsumer{
		ctx:       ctx,
		providers: providers,
		store:     store,
		idt:       idt,
		logger:    logger,
	}
}

// NewCommonMessageHandler creates a new common message handler
func NewCommonMessageHandler(ctx context.Context, store store.Store, idempotentChecker types.IdempotentChecker, historyLogger types.HistoryLogger) *CommonMessageHandler {
	return &CommonMessageHandler{
		ctx:               ctx,
		store:             store,
		idempotentChecker: idempotentChecker,
		historyLogger:     historyLogger,
	}
}

// Consume processes a Kafka message
func (c *CommonMessageConsumer) Consume(elem any) error {
	val := elem.(kafka.Message)
	var msg *types.TemplateMsgRequest
	err := json.Unmarshal(val.Value, &msg)
	if err != nil {
		log.Errorw("Failed to unmarshal message value", "value", string(val.Value), "error", err)
		return err
	}
	log.Infow("Successfully unmarshalled message", "message", msg)

	if err := c.handleSmsRequest(c.ctx, msg); err != nil {
		log.Errorw("Error handling SMS request", "error", err)
		return err
	}

	log.Infow("SMS request handled successfully")
	return nil
}

// handleSmsRequest processes the SMS request
func (c *CommonMessageConsumer) handleSmsRequest(ctx context.Context, msg *types.TemplateMsgRequest) error {
	// 幂等性检查
	isValid, err := c.idt.Check(ctx, msg.RequestId)
	if err != nil {
		log.Errorw("Idempotent token check error", "request_id", msg.RequestId, "error", err)
		return err
	}
	if !isValid {
		log.Errorw("Idempotent token check failed", "request_id", msg.RequestId)
		return errors.New("idempotent token is invalid")
	}

	// 创建历史记录
	historyM := model.HistoryM{
		UserID:    "system", // 默认用户ID
		Action:    "send_sms",
		Resource:  msg.PhoneNumber,
		Details:   fmt.Sprintf("Template: %s, Content: %s", msg.TemplateCode, msg.Content),
		CreatedAt: time.Now(),
	}

	successful := false
	var lastError error

	for _, providerName := range msg.Providers {
		log.Infow("Attempting to use provider", "provider", providerName)

		// Get provider instance from factory
		providerIns, err := c.providers.GetSMSTemplateProvider(types.ProviderType(providerName))
		if err != nil {
			log.Errorw("Failed to get provider", "provider", providerName, "error", err)
			lastError = err
			continue
		}

		// Send SMS using the provider
		ret, err := providerIns.Send(ctx, msg)
		if err != nil {
			log.Errorw("Failed to send SMS", "provider", providerName, "error", err)
			lastError = err
			continue
		}

		log.Infow("SMS sent successfully",
			"provider", providerName,
			"phone", msg.PhoneNumber,
			"biz_id", ret.BizId,
			"code", ret.Code)

		successful = true
		break
	}

	if !successful {
		if lastError != nil {
			return lastError
		}
		return errors.New("failed to send SMS with all providers")
	}

	// 写入历史记录
	if err := c.logger.WriteHistory(ctx, historyM.UserID, historyM.Action, historyM.Resource, historyM.Details); err != nil {
		log.Errorw("Failed to write history record", "error", err, "phone", msg.PhoneNumber)
		// 历史记录写入失败不影响主流程
	}

	return nil
}
