package mqs

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/ashwinyue/dcp/internal/pkg/log"
	"github.com/segmentio/kafka-go"
)

// TemplateMsgRequest represents a template message request
type TemplateMsgRequest struct {
	RequestId    string   `json:"request_id"`
	PhoneNumber  string   `json:"phone_number"`
	Content      string   `json:"content"`
	TemplateCode string   `json:"template_code"`
	Providers    []string `json:"providers"`
}

// UplinkMsgRequest represents an uplink message request
type UplinkMsgRequest struct {
	RequestId   string    `json:"request_id"`
	PhoneNumber string    `json:"phone_number"`
	Content     string    `json:"content"`
	DestCode    string    `json:"dest_code"`
	SendTime    time.Time `json:"send_time"`
}

// CommonMessageConsumer handles common message consumption
type CommonMessageConsumer struct {
	ctx context.Context
}

// NewCommonMessageConsumer creates a new common message consumer
func NewCommonMessageConsumer(ctx context.Context) *CommonMessageConsumer {
	return &CommonMessageConsumer{
		ctx: ctx,
	}
}

// Consume processes a Kafka message
func (c *CommonMessageConsumer) Consume(elem any) error {
	val := elem.(kafka.Message)
	var msg *TemplateMsgRequest
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
func (c *CommonMessageConsumer) handleSmsRequest(ctx context.Context, msg *TemplateMsgRequest) error {
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

	for _, provider := range msg.Providers {
		log.Infow("Attempting to use provider", "provider", provider)
		// TODO: Add provider factory integration
		// providerIns, err := c.providers.GetSMSTemplateProvider(types.ProviderType(provider))
		// if err != nil {
		//     continue
		// }
		// ret, err := providerIns.Send(ctx, msg)

		// Simulate successful processing for now
		log.Infow("SMS sent successfully (simulated)", "provider", provider, "phone", msg.PhoneNumber)
		successful = true
		break
	}

	if !successful {
		return errors.New("failed to send SMS with all providers")
	}

	// TODO: Add history writer integration
	// c.logger.WriterHistory(&historyM)

	return nil
}
