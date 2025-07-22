// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

package http

import (
	"github.com/gin-gonic/gin"
	"github.com/onexstack/onexstack/pkg/core"
)

// CreateSmsBatch 创建 SMS 批次.
func (h *Handler) CreateSmsBatch(c *gin.Context) {
	core.HandleJSONRequest(c, h.biz.SmsBatchV1().Create, h.val.ValidateCreateSmsBatchRequest)
}

// UpdateSmsBatch 更新 SMS 批次.
func (h *Handler) UpdateSmsBatch(c *gin.Context) {
	core.HandleJSONRequest(c, h.biz.SmsBatchV1().Update, h.val.ValidateUpdateSmsBatchRequest)
}

// DeleteSmsBatch 删除 SMS 批次.
func (h *Handler) DeleteSmsBatch(c *gin.Context) {
	core.HandleJSONRequest(c, h.biz.SmsBatchV1().Delete, h.val.ValidateDeleteSmsBatchRequest)
}

// GetSmsBatch 获取 SMS 批次详情.
func (h *Handler) GetSmsBatch(c *gin.Context) {
	core.HandleUriRequest(c, h.biz.SmsBatchV1().Get, h.val.ValidateGetSmsBatchRequest)
}

// ListSmsBatch 获取 SMS 批次列表.
func (h *Handler) ListSmsBatch(c *gin.Context) {
	core.HandleQueryRequest(c, h.biz.SmsBatchV1().List, h.val.ValidateListSmsBatchRequest)
}
