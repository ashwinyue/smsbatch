// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

package gin

import (
	"github.com/gin-gonic/gin"

	"github.com/onexstack/onexstack/pkg/core"

	"github.com/ashwinyue/dcp/internal/pkg/contextx"
	"github.com/ashwinyue/dcp/internal/pkg/errno"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// Authorizer 用于定义授权接口的实现.
type Authorizer interface {
	Authorize(subject, object, action string) (bool, error)
}

// AuthzMiddleware 是一个 Gin 中间件，用于进行请求授权.
func AuthzMiddleware(authorizer Authorizer) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject := contextx.UserID(c.Request.Context())
		object := c.Request.URL.Path
		action := c.Request.Method

		// 记录授权上下文信息
		log.Debugw("Build authorize context", "subject", subject, "object", object, "action", action)

		// 调用授权接口进行验证
		if allowed, err := authorizer.Authorize(subject, object, action); err != nil || !allowed {
			core.WriteResponse(c, nil, errno.ErrPermissionDenied.WithMessage(
				"access denied: subject=%s, object=%s, action=%s, reason=%v",
				subject,
				object,
				action,
				err,
			))
			c.Abort()
			return
		}

		c.Next() // 继续处理请求
	}
}
