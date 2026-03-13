/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package service

import (
	"context"
	"errors"
	"fmt"
	"go-support-bot/internal/app/clients/llm"
	"go-support-bot/internal/app/clients/telegram"
	"go-support-bot/internal/app/datastruct"
	"go-support-bot/internal/app/repository"
	"html"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

var ErrTopicNotFound = errors.New("topic not found")

const outOfHoursThrottleDuration = 30 * time.Minute

type SessionData struct {
	WaitingName bool
	FullName    string
}

type SupportService struct {
	repo         *repository.SupportRepo
	bot          *telegram.Client
	llm          *llm.GeminiClient
	supportGroup int64

	// FSM (Машина состояний) в памяти: UserID -> bool (ждем ли мы имя)
	sessions           sync.Map
	outOfHoursThrottle sync.Map
	managerLang        string
}

func NewSupportService(repo *repository.SupportRepo, bot *telegram.Client, llm *llm.GeminiClient, langCode string, groupID int64) *SupportService {
	return &SupportService{
		repo:         repo,
		bot:          bot,
		supportGroup: groupID,
		llm:          llm,
		managerLang:  langCode,
	}
}

// Управление состоянием FSM
func (s *SupportService) SetWaitingName(customerID int64) {
	s.sessions.Store(customerID, SessionData{WaitingName: true})
}

func (s *SupportService) SaveName(customerID int64, name string) {
	s.sessions.Store(customerID, SessionData{WaitingName: false, FullName: name})
}

func (s *SupportService) GetSession(customerID int64) (SessionData, bool) {
	val, ok := s.sessions.Load(customerID)
	if !ok {
		return SessionData{}, false
	}
	return val.(SessionData), true
}

func (s *SupportService) ClearSession(customerID int64) {
	s.sessions.Delete(customerID)
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
		_, sendErr = s.bot.Bot.SendMessage(ctx, &telego.SendMessageParams{
			ChatID:          tu.ID(s.supportGroup),
			MessageThreadID: topic.TopicID,
			Text:            finalText,
			ParseMode:       telego.ModeHTML,
		})
	} else {
		_, sendErr = s.bot.Bot.CopyMessage(ctx, &telego.CopyMessageParams{
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
		newTopic, err := s.bot.Bot.CreateForumTopic(ctx, &telego.CreateForumTopicParams{
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
		category, _ := s.repo.GetCategoryByID(ctx, topic.CategoryID)
		safeCategoryName := "Неизвестно"
		if category != nil {
			safeCategoryName = html.EscapeString(category.Name)
		}

		_, _ = s.bot.Bot.SendMessage(ctx, &telego.SendMessageParams{
			ChatID:          tu.ID(s.supportGroup),
			MessageThreadID: topic.TopicID,
			Text:            fmt.Sprintf("🔄 <b>Обращение пересоздано</b> (старый топик был удален)\nВыбрана тема: %s\nКлиент: %s", safeCategoryName, html.EscapeString(fullName)),
			ParseMode:       telego.ModeHTML,
		})

		// Повторяем отправку самого сообщения в НОВЫЙ топик
		if msg.Text != "" {
			finalText := translatedText
			if translatedText != msg.Text {
				finalText = fmt.Sprintf("%s\n\n<i>Оригинал: %s</i>", translatedText, msg.Text)
			}
			_, sendErr = s.bot.Bot.SendMessage(ctx, &telego.SendMessageParams{
				ChatID:          tu.ID(s.supportGroup),
				MessageThreadID: topic.TopicID,
				Text:            finalText,
				ParseMode:       telego.ModeHTML,
			})
		} else {
			_, sendErr = s.bot.Bot.CopyMessage(ctx, &telego.CopyMessageParams{
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
	if !msg.IsTopicMessage {
		return nil
	}
	topicID := msg.MessageThreadID

	// Получаем ID студента по ID топика
	customerID, err := s.repo.GetCustomerID(ctx, topicID)
	if err != nil {
		return err
	}

	// Достаем язык клиента из базы
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
		_, err = s.bot.Bot.SendMessage(ctx, &telego.SendMessageParams{
			ChatID:    tu.ID(customerID),
			Text:      translatedText,
			ParseMode: telego.ModeHTML,
		})
	} else {
		_, err = s.bot.Bot.CopyMessage(ctx, &telego.CopyMessageParams{
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
		newTopic, err := s.bot.Bot.CreateForumTopic(ctx, &telego.CreateForumTopicParams{
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
		err = s.bot.Bot.ReopenForumTopic(ctx, &telego.ReopenForumTopicParams{
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

			newTopic, errCreate := s.bot.Bot.CreateForumTopic(ctx, &telego.CreateForumTopicParams{
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

	if _, err = s.bot.Bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID:    tu.ID(*category.ManagerID),
		Text:      fmt.Sprintf("🚨 Новое обращение!\n\n<b>Клиент:</b> %s\n<b>Тема:</b> %s\n\n👉 <a href=\"%s\">Перейти в топик</a>", safeFullName, safeCategoryName, topicLink),
		ParseMode: telego.ModeHTML,
	}); err != nil {
		log.Printf("failed to send message to manager %d: %v", *category.ManagerID, err)
	}

	if _, err = s.bot.Bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID:          tu.ID(s.supportGroup),
		MessageThreadID: topicID,
		Text:            fmt.Sprintf("🔄 <b>Обращение открыто</b>\nВыбрана тема: %s\nМенеджер: <a href=\"tg://user?id=%d\">Ассистент</a>", safeCategoryName, category.ManagerID),
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

	last, ok := s.outOfHoursThrottle.Load(customerID)
	if ok && time.Since(last.(time.Time)) < outOfHoursThrottleDuration {
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

	if _, err = s.bot.Bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID:    tu.ID(customerID),
		Text:      msgText,
		ParseMode: telego.ModeHTML,
	}); err != nil {
		log.Printf("failed to send out of hours message to customer %d: %v", customerID, err)
	}

	s.outOfHoursThrottle.Store(customerID, time.Now())
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
	if _, err = s.bot.Bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID:          tu.ID(s.supportGroup),
		MessageThreadID: topic.TopicID,
		Text:            "❌ <b>Обращение завершено клиентом.</b>\nТема закрыта для новых сообщений.",
		ParseMode:       telego.ModeHTML,
	}); err != nil {
		log.Printf("failed to notify manager about client topic closure for topic %d: %v", topic.TopicID, err)
	}

	// 3. Закрываем сам топик (ветку форума) в Телеграме
	err = s.bot.Bot.CloseForumTopic(ctx, &telego.CloseForumTopicParams{
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
