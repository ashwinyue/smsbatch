// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

package conversion

import (
	"github.com/onexstack/onexstack/pkg/core"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	apiv1 "github.com/ashwinyue/dcp/pkg/api/nightwatch/v1"
)

// SmsBatchMToSmsBatchV1 converts a SmsBatchM object from the internal model
// to a SmsBatch object in the v1 API format.
func SmsBatchMToSmsBatchV1(smsBatchModel *model.SmsBatchM) *apiv1.SmsBatch {
	var smsBatch apiv1.SmsBatch
	_ = core.CopyWithConverters(&smsBatch, smsBatchModel)
	return &smsBatch
}

// SmsBatchV1ToSmsBatchM converts a SmsBatch object from the v1 API format
// to a SmsBatchM object in the internal model.
func SmsBatchV1ToSmsBatchM(smsBatch *apiv1.SmsBatch) *model.SmsBatchM {
	var smsBatchModel model.SmsBatchM
	_ = core.CopyWithConverters(&smsBatchModel, smsBatch)
	return &smsBatchModel
}
