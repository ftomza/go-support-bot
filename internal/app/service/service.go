/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package service

import (
	"context"
	"go-support-bot/internal/app/datastruct"
	"log"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
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
	SaveCustomer(ctx context.Context, customerID int64, fullName, username string) error

	GetRatingKeyboard(topicID int) *telego.InlineKeyboardMarkup
	SaveRating(ctx context.Context, customerID int64, topicID int, score int) error

	GetCustomerProfiles(ctx context.Context, search string) ([]datastruct.CustomerProfile, error)
	CreateBroadcast(ctx context.Context, text string, customerIDs []int64) (int, error)
	GetBroadcasts(ctx context.Context) ([]datastruct.Broadcast, error)
	RetryBroadcast(ctx context.Context, broadcastID int) error
	StartBroadcastWorker(ctx context.Context)
}

func (s *SupportService) NotifyDevelopers(ctx context.Context, text string) {
	for _, devID := range s.developerIDs {
		_, err := s.bot.GetBot().SendMessage(ctx, tu.Message(
			tu.ID(devID),
			text,
		).WithParseMode(telego.ModeHTML))

		if err != nil {
			log.Printf("Не удалось отправить алерт разработчику %d: %v", devID, err)
		}
	}
}
