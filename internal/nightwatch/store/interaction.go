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

// InteractionStore 定义了 interaction 模块在 store 层所实现的方法.
type InteractionStore interface {
	Create(ctx context.Context, interaction *model.InteractionM) error
	Get(ctx context.Context, where where.Where) (*model.InteractionM, error)
	Update(ctx context.Context, interaction *model.InteractionM) error
	List(ctx context.Context, where where.Where) (count int64, ret []*model.InteractionM, err error)
	Delete(ctx context.Context, where where.Where) error
	// GetByPhoneNumber 根据手机号获取交互记录
	GetByPhoneNumber(ctx context.Context, phoneNumber string) (*model.InteractionM, error)
	// CheckExistsByPhoneNumber 检查手机号是否存在交互记录
	CheckExistsByPhoneNumber(ctx context.Context, phoneNumber string) (bool, error)
}

// interactionStore 是 InteractionStore 的一个具体实现.
type interactionStore struct {
	store *datastore
}

// 确保 interactionStore 实现了 InteractionStore 接口.
var _ InteractionStore = (*interactionStore)(nil)

// newInteractionStore 创建一个 interactionStore 实例.
func newInteractionStore(store *datastore) *interactionStore {
	return &interactionStore{store: store}
}

// Create 插入一条 interaction 记录.
func (i *interactionStore) Create(ctx context.Context, interaction *model.InteractionM) error {
	return i.store.DB(ctx).Create(&interaction).Error
}

// Get 根据 where 条件，查询指定的 interaction 数据库记录.
func (i *interactionStore) Get(ctx context.Context, where where.Where) (*model.InteractionM, error) {
	var interaction model.InteractionM
	if err := i.store.DB(ctx, where).First(&interaction).Error; err != nil {
		return nil, err
	}

	return &interaction, nil
}

// Update 更新一条 interaction 数据库记录.
func (i *interactionStore) Update(ctx context.Context, interaction *model.InteractionM) error {
	return i.store.DB(ctx).Save(interaction).Error
}

// List 根据 where 条件，分页查询 interaction 数据库记录.
func (i *interactionStore) List(ctx context.Context, where where.Where) (count int64, ret []*model.InteractionM, err error) {
	db := i.store.DB(ctx, where)

	if err := db.Model(&model.InteractionM{}).Count(&count).Error; err != nil {
		return 0, nil, err
	}

	if err := db.Find(&ret).Error; err != nil {
		return 0, nil, err
	}

	return count, ret, nil
}

// Delete 根据 where 条件，删除 interaction 数据库记录.
func (i *interactionStore) Delete(ctx context.Context, where where.Where) error {
	return i.store.DB(ctx, where).Delete(&model.InteractionM{}).Error
}

// GetByPhoneNumber 根据手机号获取交互记录.
func (i *interactionStore) GetByPhoneNumber(ctx context.Context, phoneNumber string) (*model.InteractionM, error) {
	var interaction model.InteractionM
	if err := i.store.DB(ctx).Where("phone_number = ?", phoneNumber).First(&interaction).Error; err != nil {
		return nil, err
	}

	return &interaction, nil
}

// CheckExistsByPhoneNumber 检查手机号是否存在交互记录.
func (i *interactionStore) CheckExistsByPhoneNumber(ctx context.Context, phoneNumber string) (bool, error) {
	var count int64
	if err := i.store.DB(ctx).Model(&model.InteractionM{}).Where("phone_number = ?", phoneNumber).Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}