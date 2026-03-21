/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package repository

import (
	"context"
	"go-support-bot/internal/app/datastruct"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	ReplaceCategoriesTree(ctx context.Context, mainPrompt string, messagesJSON []byte, antiSpamJSON []byte, roots []*datastruct.CategoryNode) error
	GetSetting(ctx context.Context, key string) (string, error)
	GetMainPrompt(ctx context.Context) (string, error)
	GetAllCategoriesFull(ctx context.Context) ([]datastruct.Category, error)
	GetCategoriesByParent(ctx context.Context, parentID *int) ([]datastruct.Category, error)
	GetCategoryByID(ctx context.Context, id int) (*datastruct.Category, error)
	SaveRating(ctx context.Context, customerID int64, topicID int, score int) error

	GetCustomerTopic(ctx context.Context, customerID int64) (*datastruct.CustomerTopic, error)
	SaveTopic(ctx context.Context, customerID int64, topicID int, categoryID int, langCode string) error
	GetCustomerID(ctx context.Context, topicID int) (int64, error)
	SetTopicStatus(ctx context.Context, topicID int, isClosed bool) error
	UpdateCustomerLang(ctx context.Context, customerID int64, langCode string) error
	UpdateActiveManager(ctx context.Context, topicID int, managerID int64) error
	CheckUserBanned(ctx context.Context, customerID int64) (bool, error)
	SetUserBanned(ctx context.Context, customerID int64, isBanned bool) error

	GetSession(ctx context.Context, customerID int64) (*datastruct.SessionData, error)
	SetWaitingName(ctx context.Context, customerID int64) error
	SaveName(ctx context.Context, customerID int64, name string) error
	ClearSession(ctx context.Context, customerID int64) error
	UpdateThrottle(ctx context.Context, customerID int64) error

	GetCustomerProfiles(ctx context.Context, search string) ([]datastruct.CustomerProfile, error)
	CreateBroadcast(ctx context.Context, text string, customerIDs []int64) (int, error)
	GetBroadcasts(ctx context.Context) ([]datastruct.Broadcast, error)
	RetryBroadcast(ctx context.Context, broadcastID int) error
	GetPendingBroadcastTasks(ctx context.Context, limit int) ([]datastruct.BroadcastTask, error)
	UpdateBroadcastRecipientStatus(ctx context.Context, recipientID int, status string, errText *string) error
	MarkCustomerAsBlocked(ctx context.Context, customerID int64) error
	CheckAndCompleteBroadcast(ctx context.Context, broadcastID int) error
	SaveCustomer(ctx context.Context, id int64, fullName string, username string) error

	GetNPSStats(ctx context.Context, startDate, endDate *time.Time) (*datastruct.NPSStats, error)
}

type SupportRepo struct {
	db *pgxpool.Pool
}

func NewSupportRepo(db *pgxpool.Pool) *SupportRepo {
	return &SupportRepo{db: db}
}
