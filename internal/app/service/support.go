/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-support-bot/internal/app/clients/llm"
	"go-support-bot/internal/app/clients/telegram"
	"go-support-bot/internal/app/datastruct"
	"go-support-bot/internal/app/repository"
	"html"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

var ErrTopicNotFound = errors.New("topic not found")

const outOfHoursThrottleDuration = 30 * time.Minute

type SupportService struct {
	repo         repository.Repository
	bot          telegram.Telegram
	llm          llm.LLM
	supportGroup int64
	managerLang  string
	developerIDs []int64
}

func NewSupportService(
	repo repository.Repository,
	bot telegram.Telegram,
	llm llm.LLM,
	langCode string,
	groupID int64,
	developerIDs []int64,
) *SupportService {
	return &SupportService{
		repo:         repo,
		bot:          bot,
		supportGroup: groupID,
		llm:          llm,
		managerLang:  langCode,
		developerIDs: developerIDs,
	}
}

// Управление состоянием FSM через БД
func (s *SupportService) SetWaitingName(ctx context.Context, customerID int64) error {
	return s.repo.SetWaitingName(ctx, customerID)
}

func (s *SupportService) SaveName(ctx context.Context, customerID int64, name string) error {
	return s.repo.SaveName(ctx, customerID, name)
}

func (s *SupportService) GetSession(ctx context.Context, customerID int64) (*datastruct.SessionData, error) {
	return s.repo.GetSession(ctx, customerID)
}

func (s *SupportService) ClearSession(ctx context.Context, customerID int64) error {
	return s.repo.ClearSession(ctx, customerID)
}

// HasTopic проверяет, есть ли уже созданный топик для клиента
func (s *SupportService) HasTopic(ctx context.Context, customerID int64) bool {
	_, err := s.repo.GetCustomerTopic(ctx, customerID)
	return err == nil
}

// HandleCustomerMessage обрабатывает сообщение от студента (пересылает в готовый топик)
func (s *SupportService) HandleCustomerMessage(ctx context.Context, msg *telego.Message) error {
	customerID := msg.From.ID

	topic, err := s.repo.GetCustomerTopic(ctx, customerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTopicNotFound
		}
		return fmt.Errorf("db error: %w", err)
	}

	textToTranslate := msg.Text
	if textToTranslate == "" {
		textToTranslate = msg.Caption
	}

	translatedText := textToTranslate
	if textToTranslate != "" && !strings.HasPrefix(topic.LangCode, s.managerLang) {
		translatedText = s.llm.Translate(ctx, textToTranslate, s.managerLang)
	}

	var sendErr error

	// Пытаемся отправить сообщение в топик
	if msg.Text != "" {
		finalText := translatedText
		if translatedText != msg.Text {
			finalText = fmt.Sprintf("%s\n\n<i>Оригинал: %s</i>", translatedText, msg.Text)
		}
		_, sendErr = s.bot.GetBot().SendMessage(ctx, &telego.SendMessageParams{
			ChatID:          tu.ID(s.supportGroup),
			MessageThreadID: topic.TopicID,
			Text:            finalText,
			ParseMode:       telego.ModeHTML,
		})
	} else {
		_, sendErr = s.bot.GetBot().CopyMessage(ctx, &telego.CopyMessageParams{
			ChatID:          tu.ID(s.supportGroup),
			FromChatID:      tu.ID(customerID),
			MessageID:       msg.MessageID,
			MessageThreadID: topic.TopicID,
			Caption:         translatedText,
		})
	}

	// ========================================================
	// ПЕРЕХВАТ ОШИБКИ: Если топик удален администратором
	// ========================================================
	if sendErr != nil && (strings.Contains(sendErr.Error(), "message thread not found") || strings.Contains(sendErr.Error(), "thread not found")) {
		log.Printf("Topic %d not found for customer %d. Recreating...", topic.TopicID, customerID)

		fullName := msg.From.FirstName
		if msg.From.LastName != "" {
			fullName += " " + msg.From.LastName
		}
		topicName := fullName
		if msg.From.Username != "" {
			topicName = fmt.Sprintf("%s [@%s]", fullName, msg.From.Username)
		}

		// Создаем новый топик
		newTopic, err := s.bot.GetBot().CreateForumTopic(ctx, &telego.CreateForumTopicParams{
			ChatID: tu.ID(s.supportGroup),
			Name:   topicName,
		})
		if err != nil {
			return fmt.Errorf("failed to recreate topic: %w", err)
		}

		// Обновляем ID топика в базе данных
		topic.TopicID = newTopic.MessageThreadID
		if err = s.repo.SaveTopic(ctx, customerID, topic.TopicID, topic.CategoryID, topic.LangCode); err != nil {
			log.Printf("failed to save new topic ID %d for customer %d: %v", topic.TopicID, customerID, err)
		}

		// Уведомляем менеджеров
		msgs, _ := s.GetMessages(ctx) // <-- Добавить чтение настроек
		category, _ := s.repo.GetCategoryByID(ctx, topic.CategoryID)
		safeCategoryName := "Unknown"
		if category != nil {
			safeCategoryName = html.EscapeString(category.Name)
		}

		_, _ = s.bot.GetBot().SendMessage(ctx, &telego.SendMessageParams{
			ChatID:          tu.ID(s.supportGroup),
			MessageThreadID: topic.TopicID,
			Text:            fmt.Sprintf(msgs.NotifyTopicRecreated, safeCategoryName, html.EscapeString(fullName)),
			ParseMode:       telego.ModeHTML,
		})

		// Повторяем отправку самого сообщения в НОВЫЙ топик
		if msg.Text != "" {
			finalText := translatedText
			if translatedText != msg.Text {
				finalText = fmt.Sprintf("%s\n\n<i>Оригинал: %s</i>", translatedText, msg.Text)
			}
			_, sendErr = s.bot.GetBot().SendMessage(ctx, &telego.SendMessageParams{
				ChatID:          tu.ID(s.supportGroup),
				MessageThreadID: topic.TopicID,
				Text:            finalText,
				ParseMode:       telego.ModeHTML,
			})
		} else {
			_, sendErr = s.bot.GetBot().CopyMessage(ctx, &telego.CopyMessageParams{
				ChatID:          tu.ID(s.supportGroup),
				FromChatID:      tu.ID(customerID),
				MessageID:       msg.MessageID,
				MessageThreadID: topic.TopicID,
				Caption:         translatedText,
			})
		}
	}

	// Отбивка о часах работы
	if category, err := s.repo.GetCategoryByID(ctx, topic.CategoryID); err == nil {
		s.notifyOutOfHoursIfNeeded(ctx, msg.From.ID, category.WorkHours, category.Timezone, topic.LangCode)
	}
	return sendErr
}

// HandleManagerMessage обрабатывает сообщение от менеджера
func (s *SupportService) HandleManagerMessage(ctx context.Context, msg *telego.Message) error {
	if !msg.IsTopicMessage || msg.MessageThreadID == 0 {
		return nil
	}

	// 1. Ищем, какому клиенту принадлежит этот топик
	customerID, err := s.repo.GetCustomerID(ctx, msg.MessageThreadID)
	if err != nil {
		return err
	}

	// 2. Запоминаем, что именно этот менеджер ведет диалог!
	_ = s.repo.UpdateActiveManager(ctx, msg.MessageThreadID, msg.From.ID)

	topic, err := s.repo.GetCustomerTopic(ctx, customerID)
	if err != nil {
		return err
	}

	textToTranslate := msg.Text
	if textToTranslate == "" {
		textToTranslate = msg.Caption
	}

	// По умолчанию текст равен оригиналу
	translatedText := textToTranslate

	// Переводим на язык клиента ТОЛЬКО если он отличается от языка поддержки
	if textToTranslate != "" && !strings.HasPrefix(topic.LangCode, s.managerLang) {
		translatedText = s.llm.Translate(ctx, textToTranslate, topic.LangCode)
	}

	if msg.Text != "" {
		_, err = s.bot.GetBot().SendMessage(ctx, &telego.SendMessageParams{
			ChatID:    tu.ID(customerID),
			Text:      translatedText,
			ParseMode: telego.ModeHTML,
		})
	} else {
		_, err = s.bot.GetBot().CopyMessage(ctx, &telego.CopyMessageParams{
			ChatID:     tu.ID(customerID),
			FromChatID: tu.ID(s.supportGroup),
			MessageID:  msg.MessageID,
			Caption:    translatedText,
		})
	}

	return err
}

// CloseTopic вызывается, когда менеджер закрывает топик в группе
func (s *SupportService) CloseTopic(ctx context.Context, topicID int) error {
	return s.repo.SetTopicStatus(ctx, topicID, true)
}

// CreateOrReopenTopic создает новый топик или переоткрывает старый
func (s *SupportService) CreateOrReopenTopic(ctx context.Context, customerID int64, username, fullName string, categoryID int, langCode string) error {
	category, err := s.repo.GetCategoryByID(ctx, categoryID)
	if err != nil {
		return err
	}

	topic, err := s.repo.GetCustomerTopic(ctx, customerID)
	var topicID int

	if err != nil {
		// Топика нет -> СОЗДАЕМ
		topicName := fullName
		if username != "" {
			topicName = fmt.Sprintf("%s [@%s]", fullName, username)
		}
		newTopic, err := s.bot.GetBot().CreateForumTopic(ctx, &telego.CreateForumTopicParams{
			ChatID: tu.ID(s.supportGroup),
			Name:   topicName,
		})
		if err != nil {
			return err
		}
		topicID = newTopic.MessageThreadID
	} else {
		// Топик есть -> Пытаемся ПЕРЕОТКРЫТЬ
		topicID = topic.TopicID
		err = s.bot.GetBot().ReopenForumTopic(ctx, &telego.ReopenForumTopicParams{
			ChatID:          tu.ID(s.supportGroup),
			MessageThreadID: topicID,
		})

		// ========================================================
		// ПЕРЕХВАТ ОШИБКИ: Если топик удален администратором
		// ========================================================
		if err != nil && (strings.Contains(err.Error(), "message thread not found") || strings.Contains(err.Error(), "thread not found")) {
			log.Printf("Topic %d not found for reopening. Recreating...", topicID)
			topicName := fullName
			if username != "" {
				topicName = fmt.Sprintf("%s [@%s]", fullName, username)
			}

			newTopic, errCreate := s.bot.GetBot().CreateForumTopic(ctx, &telego.CreateForumTopicParams{
				ChatID: tu.ID(s.supportGroup),
				Name:   topicName,
			})
			if errCreate != nil {
				return errCreate
			}
			topicID = newTopic.MessageThreadID
		} else if err != nil {
			log.Printf("failed to reopen topic %d: %v", topicID, err)
		}
	}

	if langCode == "" {
		langCode = "ru"
	}

	if err := s.repo.SaveTopic(ctx, customerID, topicID, categoryID, langCode); err != nil {
		return err
	}

	groupIDStr := fmt.Sprintf("%d", s.supportGroup)
	cleanGroupID := strings.TrimPrefix(groupIDStr, "-100")
	topicLink := fmt.Sprintf("https://t.me/c/%s/%d", cleanGroupID, topicID)

	safeFullName := html.EscapeString(fullName)
	safeCategoryName := html.EscapeString(category.Name)

	msgs, _ := s.GetMessages(ctx)
	if _, err = s.bot.GetBot().SendMessage(ctx, &telego.SendMessageParams{
		ChatID:    tu.ID(*category.ManagerID),
		Text:      fmt.Sprintf(msgs.NotifyManagerNew, safeFullName, safeCategoryName, topicLink),
		ParseMode: telego.ModeHTML,
	}); err != nil {
		log.Printf("failed to send message to manager %d: %v", *category.ManagerID, err)
	}

	if _, err = s.bot.GetBot().SendMessage(ctx, &telego.SendMessageParams{
		ChatID:          tu.ID(s.supportGroup),
		MessageThreadID: topicID,
		Text:            fmt.Sprintf(msgs.NotifyTopicCreated, safeCategoryName, category.ManagerID),
		ParseMode:       telego.ModeHTML,
	}); err != nil {
		log.Printf("failed to send message to topic %d: %v", topicID, err)
	}

	s.notifyOutOfHoursIfNeeded(ctx, customerID, category.WorkHours, category.Timezone, langCode)
	return nil
}

// isWorkingHours проверяет, входит ли текущее время в интервал (например, "09:00-18:00")
func isWorkingHours(workHours *string, tz *string) bool {
	if workHours == nil || *workHours == "" {
		return true // 24/7
	}
	parts := strings.Split(*workHours, "-")
	if len(parts) != 2 {
		return true
	}

	// По умолчанию UTC
	location := time.UTC
	if tz != nil && *tz != "" {
		if loc, err := time.LoadLocation(*tz); err == nil {
			location = loc
		} else {
			log.Printf("invalid timezone %s: %v", *tz, err)
		}
	}

	// Берем время именно в нужном часовом поясе
	now := time.Now().In(location).Format("15:04")
	start := strings.TrimSpace(parts[0])
	end := strings.TrimSpace(parts[1])

	if start <= end {
		return now >= start && now <= end
	}
	return now >= start || now <= end
}

// notifyOutOfHoursIfNeeded отправляет отбивку, если сейчас нерабочее время (не чаще 1 раза в 30 минут)
func (s *SupportService) notifyOutOfHoursIfNeeded(ctx context.Context, customerID int64, workHours *string, timezone *string, langCode string) {
	if isWorkingHours(workHours, timezone) {
		return
	}

	session, _ := s.repo.GetSession(ctx, customerID)
	if session != nil && session.LastThrottle != nil && time.Since(*session.LastThrottle) < outOfHoursThrottleDuration {
		return
	}

	msgs, err := s.GetMessages(ctx)
	if err != nil {
		log.Printf("failed to get messages: %v", err)
		return
	}

	tz := "UTC"
	if timezone != nil && *timezone != "" {
		tz = *timezone
	}
	hoursWithZone := fmt.Sprintf("%s (%s)", *workHours, tz)

	// Подставляем объединенную строку в динамический текст
	msgText := fmt.Sprintf(msgs.OutOfHours, hoursWithZone)

	// ПЕРЕВОДИМ ТЕКСТ, ЕСЛИ ЯЗЫК КЛИЕНТА ОТЛИЧАЕТСЯ ОТ БАЗОВОГО
	if langCode != "" && !strings.HasPrefix(langCode, s.managerLang) {
		msgText = s.llm.Translate(ctx, msgText, langCode)
	}

	if _, err = s.bot.GetBot().SendMessage(ctx, &telego.SendMessageParams{
		ChatID:    tu.ID(customerID),
		Text:      msgText,
		ParseMode: telego.ModeHTML,
	}); err != nil {
		log.Printf("failed to send out of hours message to customer %d: %v", customerID, err)
	} else {
		_ = s.repo.UpdateThrottle(ctx, customerID) // Обновляем таймер только при успешной отправке
	}
}

func (s *SupportService) GetCustomerTopic(ctx context.Context, id int64) (*datastruct.CustomerTopic, error) {
	return s.repo.GetCustomerTopic(ctx, id)
}

func (s *SupportService) IsCustomer() th.Predicate {
	return s.bot.IsCustomer(s.supportGroup)
}

func (s *SupportService) ClearCacheBotClient() {
	s.bot.ClearCache()
}

func (s *SupportService) IsManager(ctx context.Context, userID int64) bool {
	return s.bot.IsManager(ctx, s.supportGroup, userID)
}

func (s *SupportService) ToggleTestMode(userID int64) bool {
	return s.bot.ToggleTestMode(userID)
}

// GetCustomerID проксирует вызов к репозиторию для получения ID клиента по ID топика
func (s *SupportService) GetCustomerID(ctx context.Context, topicID int) (int64, error) {
	return s.repo.GetCustomerID(ctx, topicID)
}

// CloseTopicByClient закрывает топик по инициативе клиента
func (s *SupportService) CloseTopicByClient(ctx context.Context, customerID int64) error {
	topic, err := s.repo.GetCustomerTopic(ctx, customerID)
	if err != nil {
		return err
	}

	// Если уже закрыт, ничего не делаем
	if topic.IsClosed {
		return nil
	}

	// 1. Закрываем в БД
	err = s.repo.SetTopicStatus(ctx, topic.TopicID, true)
	if err != nil {
		return err
	}

	// 2. Отправляем уведомление менеджеру прямо в топик
	msgs, _ := s.GetMessages(ctx)
	if _, err = s.bot.GetBot().SendMessage(ctx, &telego.SendMessageParams{
		ChatID:          tu.ID(s.supportGroup),
		MessageThreadID: topic.TopicID,
		Text:            msgs.NotifyTopicClosedClient,
		ParseMode:       telego.ModeHTML,
	}); err != nil {
		log.Printf("failed to notify manager about client topic closure for topic %d: %v", topic.TopicID, err)
	}

	// 3. Закрываем сам топик (ветку форума) в Телеграме
	err = s.bot.GetBot().CloseForumTopic(ctx, &telego.CloseForumTopicParams{
		ChatID:          tu.ID(s.supportGroup),
		MessageThreadID: topic.TopicID,
	})

	return err
}

// SetCustomerLangByTopic находит клиента по топику и меняет ему язык
func (s *SupportService) SetCustomerLangByTopic(ctx context.Context, topicID int, langCode string) error {
	customerID, err := s.repo.GetCustomerID(ctx, topicID)
	if err != nil {
		return err
	}
	return s.repo.UpdateCustomerLang(ctx, customerID, langCode)
}

// GetRatingKeyboard генерирует инлайн-кнопки с оценками от 1 до 5
func (s *SupportService) GetRatingKeyboard(topicID int) *telego.InlineKeyboardMarkup {
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{
				{Text: "1 ⭐️", CallbackData: fmt.Sprintf("rate_1_%d", topicID)},
				{Text: "2 ⭐️", CallbackData: fmt.Sprintf("rate_2_%d", topicID)},
				{Text: "3 ⭐️", CallbackData: fmt.Sprintf("rate_3_%d", topicID)},
				{Text: "4 ⭐️", CallbackData: fmt.Sprintf("rate_4_%d", topicID)},
				{Text: "5 ⭐️", CallbackData: fmt.Sprintf("rate_5_%d", topicID)},
			},
		},
	}
}

func (s *SupportService) SaveRating(ctx context.Context, customerID int64, topicID int, score int) error {
	return s.repo.SaveRating(ctx, customerID, topicID, score)
}

func (s *SupportService) CheckUserBanned(ctx context.Context, customerID int64) (bool, error) {
	return s.repo.CheckUserBanned(ctx, customerID)
}

func (s *SupportService) SetUserBanned(ctx context.Context, customerID int64, isBanned bool) error {
	return s.repo.SetUserBanned(ctx, customerID, isBanned)
}

func (s *SupportService) GetAntiSpam(ctx context.Context) (AntiSpamConfig, error) {
	res := AntiSpamConfig{}

	settings, err := s.repo.GetSetting(ctx, "antispam")
	if err != nil {
		return res, fmt.Errorf("failed to get antispam settings: %v", err)
	}

	if settings == "" {
		return GetDefaultAntiSpam(), nil
	}

	err = json.Unmarshal([]byte(settings), &res)
	if err != nil {
		return res, fmt.Errorf("failed to unmarshal antispam settings: %v", err)
	}
	return res, nil
}
