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

	// По умолчанию текст равен оригиналу
	translatedText := textToTranslate

	// Вызываем Gemini ТОЛЬКО если языки отличаются
	// Используем HasPrefix, чтобы "ru-RU" совпало с "ru"
	if textToTranslate != "" && !strings.HasPrefix(topic.LangCode, s.managerLang) {
		translatedText = s.llm.Translate(ctx, textToTranslate, s.managerLang)
	}

	// Если это просто текст
	if msg.Text != "" {
		finalText := translatedText

		// Добавляем оригинал, только если реально был сделан перевод
		if translatedText != msg.Text {
			finalText = fmt.Sprintf("%s\n\n<i>Оригинал: %s</i>", translatedText, msg.Text)
		}

		_, err = s.bot.Bot.SendMessage(ctx, &telego.SendMessageParams{
			ChatID:          tu.ID(s.supportGroup),
			MessageThreadID: topic.TopicID,
			Text:            finalText,
			ParseMode:       telego.ModeHTML,
		})
	} else {
		// Если это картинка/файл, копируем медиа, но подменяем caption
		_, err = s.bot.Bot.CopyMessage(ctx, &telego.CopyMessageParams{
			ChatID:          tu.ID(s.supportGroup),
			FromChatID:      tu.ID(customerID),
			MessageID:       msg.MessageID,
			MessageThreadID: topic.TopicID,
			Caption:         translatedText, // Заменяем оригинальную подпись на перевод
		})
	}

	if category, err := s.repo.GetCategoryByID(ctx, topic.CategoryID); err == nil {
		s.notifyOutOfHoursIfNeeded(ctx, msg.From.ID, category.WorkHours)
	}
	return err
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
		// Топик уже есть -> ПЕРЕОТКРЫВАЕМ
		topicID = topic.TopicID

		// Используем метод Telegram API для открытия ветки
		if err = s.bot.Bot.ReopenForumTopic(ctx, &telego.ReopenForumTopicParams{
			ChatID:          tu.ID(s.supportGroup),
			MessageThreadID: topicID,
		}); err != nil {
			log.Printf("failed to reopen topic %d: %v", topicID, err)
		}
	}

	if langCode == "" {
		langCode = "ru" // по умолчанию
	}

	// =================================================================
	// СОХРАНЯЕМ В БАЗУ (Общий шаг для обоих сценариев)
	// UPSERT запрос обновит category_id на новую и поставит is_closed = false
	// =================================================================
	if err := s.repo.SaveTopic(ctx, customerID, topicID, categoryID, langCode); err != nil {
		return err
	}

	// 1. Уведомляем менеджера в личку
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

	// 2. Отправляем системное сообщение в САМ ТОПИК
	if _, err = s.bot.Bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID:          tu.ID(s.supportGroup),
		MessageThreadID: topicID,
		Text:            fmt.Sprintf("🔄 <b>Обращение открыто</b>\nВыбрана тема: %s\nМенеджер: <a href=\"tg://user?id=%d\">Ассистент</a>", safeCategoryName, category.ManagerID),
		ParseMode:       telego.ModeHTML,
	}); err != nil {
		log.Printf("failed to send message to topic %d: %v", topicID, err)
	}

	// 3. Проверяем рабочие часы и при необходимости шлем автоответ
	s.notifyOutOfHoursIfNeeded(ctx, customerID, category.WorkHours)

	return nil
}

// isWorkingHours проверяет, входит ли текущее время в интервал (например, "09:00-18:00")
func isWorkingHours(workHours *string) bool {
	if workHours == nil || *workHours == "" {
		return true // Если график не указан, работаем 24/7
	}
	parts := strings.Split(*workHours, "-")
	if len(parts) != 2 {
		return true // Защита от кривого конфига
	}

	start := strings.TrimSpace(parts[0])
	end := strings.TrimSpace(parts[1])
	now := time.Now().Format("15:04") // Текущее время в формате ЧЧ:ММ

	if start <= end {
		return now >= start && now <= end
	}
	// Поддержка ночных смен (например, "22:00-06:00")
	return now >= start || now <= end
}

// notifyOutOfHoursIfNeeded отправляет отбивку, если сейчас нерабочее время (не чаще 1 раза в 30 минут)
func (s *SupportService) notifyOutOfHoursIfNeeded(ctx context.Context, customerID int64, workHours *string) {
	if isWorkingHours(workHours) {
		return
	}

	last, ok := s.outOfHoursThrottle.Load(customerID)
	if ok && time.Since(last.(time.Time)) < outOfHoursThrottleDuration {
		return
	}

	msgs, err := s.GetMessages(ctx) // <--- Берем конфиг из БД
	if err != nil {
		log.Printf("failed to get messages: %v", err)
		return
	}

	// Подставляем часы работы в динамический текст
	msgText := fmt.Sprintf(msgs.OutOfHours, *workHours)

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
