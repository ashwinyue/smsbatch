package mqs

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/ashwinyue/dcp/internal/nightwatch/provider"
	"github.com/ashwinyue/dcp/internal/nightwatch/types"
	"github.com/ashwinyue/dcp/internal/pkg/log"
	"github.com/segmentio/kafka-go"
)

// CommonMessageConsumer handles common message consumption
type CommonMessageConsumer struct {
	ctx       context.Context
	providers *provider.ProviderFactory
}

// NewCommonMessageConsumer creates a new common message consumer
func NewCommonMessageConsumer(ctx context.Context, providers *provider.ProviderFactory) *CommonMessageConsumer {
	return &CommonMessageConsumer{
		ctx:       ctx,
		providers: providers,
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
	// TODO: Add idempotent check
	// if !c.idt.Check(ctx, msg.RequestId) {
	//     log.Errorw("Idempotent token check failed", "error", "idempotent token is invalid")
	//     return errors.New("idempotent token is invalid")
	// }

	// TODO: Add history record creation
	// historyM := model.HistoryM{
	//     Mobile:            msg.PhoneNumber,
	//     SendTime:          time.Now(),
	//     Content:           msg.Content,
	//     MessageTemplateID: msg.TemplateCode,
	// }

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

	// TODO: Add history writer integration
	// c.logger.WriterHistory(&historyM)

	return nil
}
