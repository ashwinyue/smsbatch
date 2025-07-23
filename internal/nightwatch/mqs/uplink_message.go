package mqs

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/nightwatch/types"
	"github.com/ashwinyue/dcp/internal/pkg/log"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

// UplinkMessageConsumer handles uplink message consumption
type UplinkMessageConsumer struct {
	ctx   context.Context
	store store.IStore
	idt   types.IdempotentChecker
}

// NewUplinkMessageConsumer creates a new uplink message consumer
func NewUplinkMessageConsumer(ctx context.Context, store store.Store, idt types.IdempotentChecker) *UplinkMessageConsumer {
	return &UplinkMessageConsumer{
		ctx:   ctx,
		store: store,
		idt:   idt,
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
	// 幂等性检查
	isValid, err := u.idt.Check(ctx, msg.RequestId)
	if err != nil {
		log.Errorw("Idempotent token check error", "request_id", msg.RequestId, "error", err)
		return err
	}
	if !isValid {
		log.Errorw("Idempotent token check failed", "request_id", msg.RequestId)
		return errors.New("idempotent token is invalid")
	}

	log.Infow("Checking for existing interaction records for mobile", "mobile", msg.PhoneNumber)

	// 检查是否已存在相同的交互记录 - 暂时跳过此检查
	// TODO: 实现 Interactions store 后启用此功能
	/*
		filter := make(map[string]any)
		filter["mobile"] = msg.PhoneNumber
		filter["content"] = msg.Content
		filter["receive_time"] = msg.SendTime
		count, _, err := u.store.Interactions().List(ctx, meta.WithFilter(filter))
		if err != nil {
			log.Errorw("Failed to check existing interactions", "error", err)
			return err
		}
		if count > 0 {
			log.Infow("Interaction record already exists for mobile", "mobile", msg.PhoneNumber)
			return nil // 已存在相同记录，直接返回
		}
	*/

	// Create interaction record (simulated)
	interactionID := uuid.New().String()
	log.Infow("Creating new interaction record", "interaction_id", interactionID, "mobile", msg.PhoneNumber, "content", msg.Content)

	// 创建交互记录
	var interactionM model.InteractionM
	interactionM.UserID = "system" // 默认用户ID
	interactionM.PhoneNumber = msg.PhoneNumber
	interactionM.Content = msg.Content
	interactionM.Type = "uplink"
	interactionM.Status = "received"
	interactionM.CreatedAt = time.Now()
	interactionM.UpdatedAt = time.Now()

	// TODO: 实现 Interactions store 后启用此功能
	/*
		err = u.store.Interactions().Create(ctx, &interactionM)
		if err != nil {
			log.Errorw("Failed to create interaction record", "error", err)
			return err
		}
	*/

	log.Infow("Interaction record created successfully", "interaction_id", interactionID)

	// 处理特定的交互内容
	err = u.processInteractionContent(ctx, &interactionM, msg)
	if err != nil {
		log.Errorw("Failed to process interaction content", "error", err, "interaction_id", interactionID)
		return err
	}

	return nil
}

// isUnsubscribeKeyword 检查是否为退订关键词
func (u *UplinkMessageConsumer) isUnsubscribeKeyword(content string) bool {
	unsubscribeKeywords := []string{"退订", "TD", "取消", "CANCEL", "STOP", "0000"}
	content = strings.ToUpper(strings.TrimSpace(content))
	for _, keyword := range unsubscribeKeywords {
		if strings.Contains(content, strings.ToUpper(keyword)) {
			return true
		}
	}
	return false
}

// isQueryKeyword 检查是否为查询关键词
func (u *UplinkMessageConsumer) isQueryKeyword(content string) bool {
	queryKeywords := []string{"查询", "CX", "QUERY", "余额", "积分", "状态"}
	content = strings.ToUpper(strings.TrimSpace(content))
	for _, keyword := range queryKeywords {
		if strings.Contains(content, strings.ToUpper(keyword)) {
			return true
		}
	}
	return false
}

// isComplaintKeyword 检查是否为投诉关键词
func (u *UplinkMessageConsumer) isComplaintKeyword(content string) bool {
	complaintKeywords := []string{"投诉", "TS", "COMPLAINT", "举报", "建议", "意见"}
	content = strings.ToUpper(strings.TrimSpace(content))
	for _, keyword := range complaintKeywords {
		if strings.Contains(content, strings.ToUpper(keyword)) {
			return true
		}
	}
	return false
}

// handleUnsubscribe 处理退订请求
func (u *UplinkMessageConsumer) handleUnsubscribe(ctx context.Context, mobile string) error {
	log.Infow("Processing unsubscribe request", "mobile", mobile)

	// 查找用户的订阅记录
	filter := make(map[string]any)
	filter["mobile"] = mobile
	filter["status"] = "active"

	// 这里应该调用实际的退订服务
	// 示例：更新用户订阅状态为已退订
	log.Infow("User unsubscribed successfully", "mobile", mobile)

	// 可以发送确认短信
	// u.sendConfirmationSMS(ctx, mobile, "您已成功退订，如需重新订阅请回复1")

	return nil
}

// handleQuery 处理查询请求
func (u *UplinkMessageConsumer) handleQuery(ctx context.Context, mobile string, content string) error {
	log.Infow("Processing query request", "mobile", mobile, "query", content)

	// 根据查询内容返回相应信息
	var response string
	if strings.Contains(strings.ToUpper(content), "余额") {
		response = "您的账户余额为100元"
	} else if strings.Contains(strings.ToUpper(content), "积分") {
		response = "您的积分余额为500分"
	} else {
		response = "感谢您的查询，如需帮助请联系客服"
	}

	// 发送回复短信
	log.Infow("Sending query response", "mobile", mobile, "response", response)
	// u.sendResponseSMS(ctx, mobile, response)

	return nil
}

// handleComplaint 处理投诉建议
func (u *UplinkMessageConsumer) handleComplaint(ctx context.Context, interaction *model.InteractionM, msg *types.UplinkMsgRequest) error {
	log.Infow("Processing complaint", "mobile", msg.PhoneNumber, "content", msg.Content)

	// 创建投诉记录
	complaintID := uuid.New().String()

	// 保存投诉记录到数据库
	// err := u.store.Complaints().Create(ctx, complaint)
	// if err != nil {
	//     log.Errorw("Failed to create complaint record", "error", err)
	//     return err
	// }

	log.Infow("Complaint record created", "complaint_id", complaintID, "mobile", msg.PhoneNumber)

	// 发送确认回复
	response := "您的投诉已收到，我们会在24小时内处理并回复您"
	log.Infow("Sending complaint confirmation", "mobile", msg.PhoneNumber, "response", response)
	// u.sendResponseSMS(ctx, msg.PhoneNumber, response)

	return nil
}

// processInteractionContent 处理交互内容的特定逻辑
func (u *UplinkMessageConsumer) processInteractionContent(ctx context.Context, interaction *model.InteractionM, msg *types.UplinkMsgRequest) error {
	// 根据消息内容进行特定处理
	log.Infow("Processing interaction content", "interaction_id", interaction.ID, "content", msg.Content)

	// 关键词匹配和自动回复逻辑
	if msg.Content != "" {
		// 处理退订请求
		if u.isUnsubscribeKeyword(msg.Content) {
			log.Infow("Unsubscribe request detected", "interaction_id", interaction.ID, "mobile", msg.PhoneNumber)
			// 处理退订逻辑
			if err := u.handleUnsubscribe(ctx, msg.PhoneNumber); err != nil {
				log.Errorw("Failed to handle unsubscribe", "error", err, "mobile", msg.PhoneNumber)
				return err
			}
		}

		// 处理查询请求
		if u.isQueryKeyword(msg.Content) {
			log.Infow("Query request detected", "interaction_id", interaction.ID, "mobile", msg.PhoneNumber)
			// 处理查询逻辑
			if err := u.handleQuery(ctx, msg.PhoneNumber, msg.Content); err != nil {
				log.Errorw("Failed to handle query", "error", err, "mobile", msg.PhoneNumber)
				return err
			}
		}

		// 处理投诉建议
		if u.isComplaintKeyword(msg.Content) {
			log.Infow("Complaint detected", "interaction_id", interaction.ID, "mobile", msg.PhoneNumber)
			// 处理投诉逻辑
			if err := u.handleComplaint(ctx, interaction, msg); err != nil {
				log.Errorw("Failed to handle complaint", "error", err, "mobile", msg.PhoneNumber)
				return err
			}
		}

		// 记录内容处理结果
		log.Infow("Content processed", "interaction_id", interaction.ID, "processed", true)
	}

	return nil
}
