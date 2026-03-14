/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package telegram

import (
	"context"
	"fmt"
	"sync"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

type Bot interface {
	GetChatMember(ctx context.Context, params *telego.GetChatMemberParams) (telego.ChatMember, error)
	GetChatAdministrators(ctx context.Context, params *telego.GetChatAdministratorsParams) ([]telego.ChatMember, error)
	SendMessage(ctx context.Context, params *telego.SendMessageParams) (*telego.Message, error)
	SendPhoto(ctx context.Context, params *telego.SendPhotoParams) (*telego.Message, error)
	DeleteMessage(ctx context.Context, params *telego.DeleteMessageParams) error
	AnswerCallbackQuery(ctx context.Context, params *telego.AnswerCallbackQueryParams) error
	GetFile(ctx context.Context, params *telego.GetFileParams) (*telego.File, error)
	CloseForumTopic(ctx context.Context, params *telego.CloseForumTopicParams) error
	CreateForumTopic(ctx context.Context, params *telego.CreateForumTopicParams) (*telego.ForumTopic, error)
	ReopenForumTopic(ctx context.Context, params *telego.ReopenForumTopicParams) error
	CopyMessage(ctx context.Context, params *telego.CopyMessageParams) (*telego.MessageID, error)
	Token() string
}

type Telegram interface {
	IsCustomer(supportGroup int64) th.Predicate
	IsManager(ctx context.Context, supportGroup int64, userID int64) bool
	ToggleTestMode(userID int64) bool
	ClearCache()
	GetChatAdministrators(ctx context.Context, chatID int64) ([]telego.ChatMember, error)
	GetBot() Bot
}

type Client struct {
	Bot Bot

	roles *roleCache
}

type roleCache struct {
	sync.RWMutex
	isManager map[int64]bool
	testMode  map[int64]bool
}

func NewTelegramBot(token string) (*Client, error) {
	bot, err := telego.NewBot(
		token,
		telego.WithDefaultDebugLogger(),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating telegram bot: %v", err)
	}

	return &Client{
		Bot: bot,
		roles: &roleCache{
			isManager: make(map[int64]bool),
			testMode:  make(map[int64]bool),
		},
	}, nil
}

func (c *Client) GetBot() Bot {
	return c.Bot
}

func (c *Client) IsCustomer(supportGroup int64) th.Predicate {
	return func(ctx context.Context, update telego.Update) bool {
		var userID int64

		// Достаем ID пользователя в зависимости от типа апдейта
		if update.Message != nil {
			userID = update.Message.From.ID
		} else if update.CallbackQuery != nil {
			userID = update.CallbackQuery.From.ID
		} else {
			// Если это ни сообщение, ни коллбэк (например, редактирование сообщения) — игнорируем
			return false
		}

		// 1. Проверяем кэш (быстрое чтение)
		c.roles.RLock()
		isMgr, exists := c.roles.isManager[userID]
		isTestMode, _ := c.roles.testMode[userID]
		c.roles.RUnlock()

		if isTestMode {
			return true
		}

		if exists {
			return !isMgr // Если он менеджер, возвращаем false (он не студент)
		}

		// 2. Если в кэше нет, делаем запрос к Telegram API
		member, err := c.Bot.GetChatMember(ctx, &telego.GetChatMemberParams{
			ChatID: tu.ID(supportGroup),
			UserID: userID,
		})

		isMgrNow := false
		if err == nil && member != nil {
			status := member.MemberStatus()
			if status == telego.MemberStatusMember ||
				status == telego.MemberStatusAdministrator ||
				status == telego.MemberStatusCreator {
				isMgrNow = true
			}
		}

		// 3. Записываем результат в кэш
		c.roles.Lock()
		c.roles.isManager[userID] = isMgrNow
		c.roles.Unlock()

		return !isMgrNow
	}
}

func (c *Client) IsManager(ctx context.Context, supportGroup int64, userID int64) bool {
	c.roles.RLock()
	isMgr, exists := c.roles.isManager[userID]
	c.roles.RUnlock()

	if exists {
		return isMgr
	}

	member, err := c.Bot.GetChatMember(ctx, &telego.GetChatMemberParams{
		ChatID: tu.ID(supportGroup),
		UserID: userID,
	})

	isMgrNow := false
	if err == nil && member != nil {
		status := member.MemberStatus()
		if status == telego.MemberStatusMember ||
			status == telego.MemberStatusAdministrator ||
			status == telego.MemberStatusCreator {
			isMgrNow = true
		}
	}

	c.roles.Lock()
	c.roles.isManager[userID] = isMgrNow
	c.roles.Unlock()

	return isMgrNow
}

func (c *Client) ToggleTestMode(userID int64) bool {
	c.roles.Lock()
	defer c.roles.Unlock()

	newState := !c.roles.testMode[userID]
	c.roles.testMode[userID] = newState
	return newState
}

func (c *Client) ClearCache() {
	c.roles.Lock()
	c.roles.isManager = map[int64]bool{}
	c.roles.testMode = map[int64]bool{} // Сбрасываем и режим тестов на всякий случай
	c.roles.Unlock()
}

func (c *Client) GetChatAdministrators(ctx context.Context, chatID int64) ([]telego.ChatMember, error) {
	return c.Bot.GetChatAdministrators(ctx, &telego.GetChatAdministratorsParams{
		ChatID: tu.ID(chatID),
	})
}

func IsPrivateChat() th.Predicate {
	return func(ctx context.Context, update telego.Update) bool {
		if update.Message != nil {
			return update.Message.Chat.Type == telego.ChatTypePrivate
		}
		return false
	}
}

func IsGroupChat() th.Predicate {
	return func(ctx context.Context, update telego.Update) bool {
		if update.Message != nil {
			return update.Message.Chat.Type == telego.ChatTypeGroup ||
				update.Message.Chat.Type == telego.ChatTypeSupergroup
		}
		return false
	}
}

func IsTopicClosed() th.Predicate {
	return func(ctx context.Context, update telego.Update) bool {
		return update.Message != nil && update.Message.ForumTopicClosed != nil
	}
}
