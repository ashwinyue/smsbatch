package mqs

import (
	"context"
	"encoding/json"

	"github.com/ashwinyue/dcp/internal/nightwatch/types"
	"github.com/ashwinyue/dcp/internal/pkg/log"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

// UplinkMessageConsumer handles uplink message consumption
type UplinkMessageConsumer struct {
	ctx context.Context
}

// NewUplinkMessageConsumer creates a new uplink message consumer
func NewUplinkMessageConsumer(ctx context.Context) *UplinkMessageConsumer {
	return &UplinkMessageConsumer{
		ctx: ctx,
	}
}

// Consume processes a Kafka message
func (u *UplinkMessageConsumer) Consume(elem any) error {
	val := elem.(kafka.Message)

	var msg *types.UplinkMsgRequest
	err := json.Unmarshal(val.Value, &msg)
	if err != nil {
		log.Errorw("Failed to unmarshal message", "error", err, "value", string(val.Value))
		return err
	}

	log.Infow("Uplink message consumed", "message", msg)
	return u.handleSmsRequest(u.ctx, msg)
}

// handleSmsRequest processes the uplink SMS request
func (u *UplinkMessageConsumer) handleSmsRequest(ctx context.Context, msg *types.UplinkMsgRequest) error {
	// TODO: Add idempotent check
	// if !u.idt.Check(ctx, msg.RequestId) {
	//     log.Errorw("Idempotent token check failed", "error", "idempotent token is invalid")
	//     return errors.New("idempotent token is invalid")
	// }

	log.Infow("Checking for existing interaction records for mobile", "mobile", msg.PhoneNumber)

	// TODO: Add database integration
	// filter := make(map[string]any)
	// filter["mobile"] = msg.PhoneNumber
	// filter["content"] = msg.Content
	// filter["receive_time"] = msg.SendTime
	// count, _, _ := u.ds.Interactions().List(ctx, meta.WithFilter(filter))
	// if count > 0 {
	//     log.Infow("Interaction record already exists for mobile", "mobile", msg.PhoneNumber)
	// }

	// Create interaction record (simulated)
	interactionID := uuid.New().String()
	log.Infow("Creating new interaction record", "interaction_id", interactionID, "mobile", msg.PhoneNumber, "content", msg.Content)

	// TODO: Add database storage
	// var interactionM model.InteractionM
	// interactionM.InteractionID = interactionID
	// interactionM.Mobile = msg.PhoneNumber
	// interactionM.Content = msg.Content
	// interactionM.Param = msg.DestCode
	// interactionM.Provider = "AILIYUN"
	//
	// err := u.ds.Interactions().Create(ctx, &interactionM)
	// if err != nil {
	//     log.Errorw("Failed to create interaction record", "error", err)
	//     return err
	// }

	log.Infow("Interaction record created successfully", "interaction_id", interactionID)

	// TODO: Add specific interaction content processing
	return nil
}
