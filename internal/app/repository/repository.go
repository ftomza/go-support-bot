/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package repository

import (
	"context"
	"go-support-bot/internal/app/datastruct"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	ReplaceCategoriesTree(ctx context.Context, mainPrompt string, messagesJSON []byte, roots []*datastruct.CategoryNode) error
	GetSetting(ctx context.Context, key string) (string, error)
	GetMainPrompt(ctx context.Context) (string, error)
	GetAllCategoriesFull(ctx context.Context) ([]datastruct.Category, error)
	GetCategoriesByParent(ctx context.Context, parentID *int) ([]datastruct.Category, error)
	GetCategoryByID(ctx context.Context, id int) (*datastruct.Category, error)

	GetCustomerTopic(ctx context.Context, customerID int64) (*datastruct.CustomerTopic, error)
	SaveTopic(ctx context.Context, customerID int64, topicID int, categoryID int, langCode string) error
	GetCustomerID(ctx context.Context, topicID int) (int64, error)
	SetTopicStatus(ctx context.Context, topicID int, isClosed bool) error
	UpdateCustomerLang(ctx context.Context, customerID int64, langCode string) error

	GetSession(ctx context.Context, customerID int64) (*datastruct.SessionData, error)
	SetWaitingName(ctx context.Context, customerID int64) error
	SaveName(ctx context.Context, customerID int64, name string) error
	ClearSession(ctx context.Context, customerID int64) error
	UpdateThrottle(ctx context.Context, customerID int64) error
}

type SupportRepo struct {
	db *pgxpool.Pool
}

func NewSupportRepo(db *pgxpool.Pool) *SupportRepo {
	return &SupportRepo{db: db}
}
