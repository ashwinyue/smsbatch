// Copyright 2024 孔令飞 <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file. The original repo for
// this file is https://github.com/onexstack/miniblog. The professional
// version of this repository is https://github.com/onexstack/onex.

package store

import (
	"context"

	"github.com/onexstack/onexstack/pkg/store/where"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
)

// SmsBatchStore 定义了 sms_batch 模块在 store 层所实现的方法.
type SmsBatchStore interface {
	Create(ctx context.Context, smsBatch *model.SmsBatchM) error
	Get(ctx context.Context, where where.Where) (*model.SmsBatchM, error)
	Update(ctx context.Context, smsBatch *model.SmsBatchM) error
	List(ctx context.Context, where where.Where) (count int64, ret []*model.SmsBatchM, err error)
	Delete(ctx context.Context, where where.Where) error
}

// smsBatchStore 是 SmsBatchStore 的一个具体实现.
type smsBatchStore struct {
	store *datastore
}

// 确保 smsBatchStore 实现了 SmsBatchStore 接口.
var _ SmsBatchStore = (*smsBatchStore)(nil)

// newSmsBatchStore 创建一个 smsBatchStore 实例.
func newSmsBatchStore(store *datastore) *smsBatchStore {
	return &smsBatchStore{store: store}
}

// Create 插入一条 sms_batch 记录.
func (s *smsBatchStore) Create(ctx context.Context, smsBatch *model.SmsBatchM) error {
	return s.store.DB(ctx).Create(&smsBatch).Error
}

// Get 根据 where 条件，查询指定的 sms_batch 数据库记录.
func (s *smsBatchStore) Get(ctx context.Context, where where.Where) (*model.SmsBatchM, error) {
	var smsBatch model.SmsBatchM
	if err := s.store.DB(ctx, where).First(&smsBatch).Error; err != nil {
		return nil, err
	}

	return &smsBatch, nil
}

// Update 更新一条 sms_batch 数据库记录.
func (s *smsBatchStore) Update(ctx context.Context, smsBatch *model.SmsBatchM) error {
	return s.store.DB(ctx).Save(smsBatch).Error
}

// List 根据 where 条件，分页查询 sms_batch 数据库记录.
func (s *smsBatchStore) List(ctx context.Context, where where.Where) (count int64, ret []*model.SmsBatchM, err error) {
	db := s.store.DB(ctx, where)

	if err := db.Model(&model.SmsBatchM{}).Count(&count).Error; err != nil {
		return 0, nil, err
	}

	if err := db.Find(&ret).Error; err != nil {
		return 0, nil, err
	}

	return count, ret, nil
}

// Delete 根据 where 条件，删除 sms_batch 数据库记录.
func (s *smsBatchStore) Delete(ctx context.Context, where where.Where) error {
	return s.store.DB(ctx, where).Delete(&model.SmsBatchM{}).Error
}