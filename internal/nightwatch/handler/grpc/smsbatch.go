// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

package grpc

import (
	"context"

	apiv1 "github.com/ashwinyue/dcp/pkg/api/nightwatch/v1"
)

// CreateSmsBatch 创建 SMS 批次.
func (h *Handler) CreateSmsBatch(ctx context.Context, rq *apiv1.CreateSmsBatchRequest) (*apiv1.CreateSmsBatchResponse, error) {
	return h.biz.SmsBatchV1().Create(ctx, rq)
}

// UpdateSmsBatch 更新 SMS 批次.
func (h *Handler) UpdateSmsBatch(ctx context.Context, rq *apiv1.UpdateSmsBatchRequest) (*apiv1.UpdateSmsBatchResponse, error) {
	return h.biz.SmsBatchV1().Update(ctx, rq)
}

// DeleteSmsBatch 删除 SMS 批次.
func (h *Handler) DeleteSmsBatch(ctx context.Context, rq *apiv1.DeleteSmsBatchRequest) (*apiv1.DeleteSmsBatchResponse, error) {
	return h.biz.SmsBatchV1().Delete(ctx, rq)
}

// GetSmsBatch 获取 SMS 批次详情.
func (h *Handler) GetSmsBatch(ctx context.Context, rq *apiv1.GetSmsBatchRequest) (*apiv1.GetSmsBatchResponse, error) {
	return h.biz.SmsBatchV1().Get(ctx, rq)
}

// ListSmsBatch 获取 SMS 批次列表.
func (h *Handler) ListSmsBatch(ctx context.Context, rq *apiv1.ListSmsBatchRequest) (*apiv1.ListSmsBatchResponse, error) {
	return h.biz.SmsBatchV1().List(ctx, rq)
}
