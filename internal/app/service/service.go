/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package service

import (
	"context"
	"go-support-bot/internal/app/datastruct"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

type Service interface {
	ExportConfig(ctx context.Context) (*YamlConfig, error)
	ImportConfig(ctx context.Context, cfg *YamlConfig) error
	LoadCategoriesFromYAML(ctx context.Context, data []byte) error
	ExportCategoriesToYAML(ctx context.Context) ([]byte, error)
	GetMessages(ctx context.Context) (YamlMessages, error)
	GetMainPrompt(ctx context.Context) (string, error)
	GetCategoriesKeyboard(ctx context.Context, parentID *int) (*telego.InlineKeyboardMarkup, error)
	GetCategoryByID(ctx context.Context, id int) (*datastruct.Category, error)

	GetManagersList(ctx context.Context) (string, error)
	GetManagersData(ctx context.Context) ([]ManagerData, error)

	SetWaitingName(ctx context.Context, customerID int64) error
	SaveName(ctx context.Context, customerID int64, name string) error
	GetSession(ctx context.Context, customerID int64) (*datastruct.SessionData, error)
	ClearSession(ctx context.Context, customerID int64) error
	HasTopic(ctx context.Context, customerID int64) bool
	HandleCustomerMessage(ctx context.Context, msg *telego.Message) error
	HandleManagerMessage(ctx context.Context, msg *telego.Message) error
	CloseTopic(ctx context.Context, topicID int) error
	CreateOrReopenTopic(ctx context.Context, customerID int64, username, fullName string, categoryID int, langCode string) error
	GetCustomerTopic(ctx context.Context, id int64) (*datastruct.CustomerTopic, error)
	IsCustomer() th.Predicate
	ClearCacheBotClient()
	IsManager(ctx context.Context, userID int64) bool
	ToggleTestMode(userID int64) bool
	GetCustomerID(ctx context.Context, topicID int) (int64, error)
	CloseTopicByClient(ctx context.Context, customerID int64) error
	SetCustomerLangByTopic(ctx context.Context, topicID int, langCode string) error
}
