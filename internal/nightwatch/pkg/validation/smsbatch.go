// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

package validation

import (
	"context"
	"fmt"

	"github.com/ashwinyue/dcp/internal/pkg/errno"
	apiv1 "github.com/ashwinyue/dcp/pkg/api/nightwatch/v1"
)

// ValidateCreateSmsBatchRequest 验证创建 SMS 批次请求.
func (v *Validator) ValidateCreateSmsBatchRequest(ctx context.Context, req *apiv1.CreateSmsBatchRequest) error {
	if req.SmsBatch == nil {
		return errno.ErrInvalidArgument.WithMessage("smsBatch is required")
	}

	// 验证名称
	if req.SmsBatch.GetName() == "" {
		return errno.ErrInvalidArgument.WithMessage("name is required")
	}
	if !lengthRegex.MatchString(req.SmsBatch.GetName()) {
		return errno.ErrInvalidArgument.WithMessage("name length must be between 3 and 50 characters")
	}
	if !validRegex.MatchString(req.SmsBatch.GetName()) {
		return errno.ErrInvalidArgument.WithMessage("name can only contain letters, numbers, underscores and hyphens")
	}

	// 验证用户ID
	if req.SmsBatch.GetUserID() == "" {
		return errno.ErrInvalidArgument.WithMessage("userID is required")
	}

	// 验证描述长度
	if req.SmsBatch.GetDescription() != "" && !descriptionLen.MatchString(req.SmsBatch.GetDescription()) {
		return errno.ErrInvalidArgument.WithMessage("description length must not exceed 500 characters")
	}

	// 验证内容
	if req.SmsBatch.GetContent() == "" {
		return errno.ErrInvalidArgument.WithMessage("content is required")
	}

	return nil
}

// ValidateUpdateSmsBatchRequest 验证更新 SMS 批次请求.
func (v *Validator) ValidateUpdateSmsBatchRequest(ctx context.Context, req *apiv1.UpdateSmsBatchRequest) error {
	if req.GetBatchID() == "" {
		return errno.ErrInvalidArgument.WithMessage("batchID is required")
	}

	// 验证名称（如果提供）
	if req.GetName() != "" {
		if !lengthRegex.MatchString(req.GetName()) {
			return errno.ErrInvalidArgument.WithMessage("name length must be between 3 and 50 characters")
		}
		if !validRegex.MatchString(req.GetName()) {
			return errno.ErrInvalidArgument.WithMessage("name can only contain letters, numbers, underscores and hyphens")
		}
	}

	// 验证描述长度（如果提供）
	if req.GetDescription() != "" && !descriptionLen.MatchString(req.GetDescription()) {
		return errno.ErrInvalidArgument.WithMessage("description length must not exceed 500 characters")
	}

	return nil
}

// ValidateDeleteSmsBatchRequest 验证删除 SMS 批次请求.
func (v *Validator) ValidateDeleteSmsBatchRequest(ctx context.Context, req *apiv1.DeleteSmsBatchRequest) error {
	if len(req.GetBatchIDs()) == 0 {
		return errno.ErrInvalidArgument.WithMessage("batchIDs is required")
	}

	// 验证每个ID不为空
	for i, id := range req.GetBatchIDs() {
		if id == "" {
			return errno.ErrInvalidArgument.WithMessage(fmt.Sprintf("batchIDs[%d] cannot be empty", i))
		}
	}

	return nil
}

// ValidateGetSmsBatchRequest 验证获取 SMS 批次请求.
func (v *Validator) ValidateGetSmsBatchRequest(ctx context.Context, req *apiv1.GetSmsBatchRequest) error {
	if req.GetBatchID() == "" {
		return errno.ErrInvalidArgument.WithMessage("batchID is required")
	}

	return nil
}

// ValidateListSmsBatchRequest 验证列表 SMS 批次请求.
func (v *Validator) ValidateListSmsBatchRequest(ctx context.Context, req *apiv1.ListSmsBatchRequest) error {
	if req.GetLimit() <= 0 {
		return errno.ErrInvalidArgument.WithMessage("limit must be greater than 0")
	}

	if req.GetLimit() > 1000 {
		return errno.ErrInvalidArgument.WithMessage("limit must not exceed 1000")
	}

	if req.GetOffset() < 0 {
		return errno.ErrInvalidArgument.WithMessage("offset must be non-negative")
	}

	return nil
}
