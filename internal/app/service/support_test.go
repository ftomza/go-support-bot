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
	"strings"
	"testing"
	"time"

	clientsMocks "go-support-bot/internal/app/clients/mocks"
	"go-support-bot/internal/app/datastruct"
	repoMocks "go-support-bot/internal/app/repository/mocks"

	"github.com/mymmrac/telego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSupportService_HandleManagerMessage(t *testing.T) {
	// =====================================================================
	// Сценарий 1: Успешная обработка сообщения от менеджера (с переводом)
	// =====================================================================
	t.Run("Success with translation", func(t *testing.T) {
		// 1. Arrange (Подготовка)
		mockRepo := repoMocks.NewMockRepository(t)
		mockLLM := clientsMocks.NewMockLLM(t)
		mockBot := clientsMocks.NewMockBot(t)
		mockTg := clientsMocks.NewMockTelegram(t)

		// Настраиваем, чтобы mockTg возвращал наш mockBot
		mockTg.EXPECT().GetBot().Return(mockBot)

		managerLang := "ru"
		supportGroupID := int64(-100123456)
		svc := NewSupportService(mockRepo, mockTg, mockLLM, managerLang, supportGroupID)

		ctx := context.Background()
		topicID := 42
		customerID := int64(999)
		originalText := "Привет, мы решаем вашу проблему"
		translatedText := "Hello, we are solving your problem"

		// Входящее сообщение от менеджера (внутри топика 42)
		msg := &telego.Message{
			IsTopicMessage:  true,
			MessageThreadID: topicID,
			Text:            originalText,
			Chat: telego.Chat{
				ID: supportGroupID,
			},
		}

		// Прописываем ожидания (что должен вызвать наш сервис)
		mockRepo.EXPECT().GetCustomerID(ctx, topicID).Return(customerID, nil)
		mockRepo.EXPECT().GetCustomerTopic(ctx, customerID).Return(&datastruct.CustomerTopic{
			TopicID:  topicID,
			LangCode: "en", // Язык клиента отличается от языка менеджера
		}, nil)

		// Ожидаем вызов переводчика LLM
		mockLLM.EXPECT().Translate(ctx, originalText, "en").Return(translatedText)

		// Ожидаем отправку финального переведенного сообщения клиенту
		mockBot.EXPECT().SendMessage(ctx, mock.MatchedBy(func(params *telego.SendMessageParams) bool {
			return params.ChatID.ID == customerID && params.Text == translatedText
		})).Return(&telego.Message{}, nil)

		// 2. Act (Выполнение)
		err := svc.HandleManagerMessage(ctx, msg)

		// 3. Assert (Проверка)
		assert.NoError(t, err)
	})

	// =====================================================================
	// Сценарий 2: Игнорирование сообщений вне топика (в "General")
	// =====================================================================
	t.Run("Ignore non-topic message", func(t *testing.T) {
		// 1. Arrange
		mockRepo := repoMocks.NewMockRepository(t)
		mockLLM := clientsMocks.NewMockLLM(t)
		mockTg := clientsMocks.NewMockTelegram(t)
		// Нам не нужно мокать Bot, так как до него не должно дойти

		svc := NewSupportService(mockRepo, mockTg, mockLLM, "ru", -100123456)

		msg := &telego.Message{
			IsTopicMessage: false, // Сообщение в общий чат
			Text:           "Просто сообщение админов между собой",
		}

		// 2. Act
		err := svc.HandleManagerMessage(context.Background(), msg)

		// 3. Assert
		assert.NoError(t, err)
		// Если бы сервис попытался вызвать базу или бота, тест бы упал,
		// так как мы не прописали EXPECT() для этих моков.
	})
}

func TestSupportService_HandleCustomerMessage(t *testing.T) {
	// =====================================================================
	// Сценарий 1: Успешная пересылка сообщения от клиента (без перевода)
	// =====================================================================
	t.Run("Success simple text message", func(t *testing.T) {
		mockRepo := repoMocks.NewMockRepository(t)
		mockLLM := clientsMocks.NewMockLLM(t)
		mockBot := clientsMocks.NewMockBot(t)
		mockTg := clientsMocks.NewMockTelegram(t)

		mockTg.EXPECT().GetBot().Return(mockBot)

		supportGroupID := int64(-100123456)
		managerLang := "ru"
		svc := NewSupportService(mockRepo, mockTg, mockLLM, managerLang, supportGroupID)

		ctx := context.Background()
		customerID := int64(111)
		topicID := 42
		categoryID := 1

		msg := &telego.Message{
			From: &telego.User{ID: customerID, FirstName: "Ivan"},
			Text: "У меня не работает кнопка",
		}

		// 1. Бот проверяет топик
		mockRepo.EXPECT().GetCustomerTopic(ctx, customerID).Return(&datastruct.CustomerTopic{
			TopicID:    topicID,
			CategoryID: categoryID,
			LangCode:   "ru", // Совпадает с языком поддержки -> перевод не нужен
		}, nil)

		// 2. Бот отправляет сообщение в группу
		mockBot.EXPECT().SendMessage(ctx, mock.MatchedBy(func(params *telego.SendMessageParams) bool {
			return params.ChatID.ID == supportGroupID && params.MessageThreadID == topicID && params.Text == msg.Text
		})).Return(&telego.Message{}, nil)

		// 3. Бот проверяет рабочие часы (отбивка)
		mockRepo.EXPECT().GetCategoryByID(ctx, categoryID).Return(&datastruct.Category{
			ID:        categoryID,
			WorkHours: nil, // 24/7, отбивка не нужна
		}, nil)

		err := svc.HandleCustomerMessage(ctx, msg)
		assert.NoError(t, err)
	})

	// =====================================================================
	// Сценарий 2: Восстановление удаленного топика!
	// =====================================================================
	t.Run("Recreate deleted topic", func(t *testing.T) {
		mockRepo := repoMocks.NewMockRepository(t)
		mockLLM := clientsMocks.NewMockLLM(t)
		mockBot := clientsMocks.NewMockBot(t)
		mockTg := clientsMocks.NewMockTelegram(t)

		mockTg.EXPECT().GetBot().Return(mockBot)

		supportGroupID := int64(-100123456)
		svc := NewSupportService(mockRepo, mockTg, mockLLM, "ru", supportGroupID)

		ctx := context.Background()
		customerID := int64(222)
		oldTopicID := 42
		newTopicID := 43
		categoryID := 2

		msg := &telego.Message{
			From: &telego.User{ID: customerID, FirstName: "Anna", Username: "anna_test"},
			Text: "Помогите",
		}

		// 1. Берем старый топик
		mockRepo.EXPECT().GetCustomerTopic(ctx, customerID).Return(&datastruct.CustomerTopic{
			TopicID:    oldTopicID,
			CategoryID: categoryID,
			LangCode:   "ru",
		}, nil)

		// 2. Пытаемся отправить сообщение, но получаем ошибку от Telegram
		expectedErr := errors.New("Bad Request: message thread not found")
		mockBot.EXPECT().SendMessage(ctx, mock.MatchedBy(func(params *telego.SendMessageParams) bool {
			return params.MessageThreadID == oldTopicID
		})).Return(nil, expectedErr) // <-- Имитируем удаление!

		mockRepo.EXPECT().GetSetting(ctx, "messages").Return("", nil)

		// 3. БОТ ДОЛЖЕН НАЧАТЬ ПЕРЕСОЗДАНИЕ:
		// Создает новый топик
		mockBot.EXPECT().CreateForumTopic(ctx, mock.MatchedBy(func(params *telego.CreateForumTopicParams) bool {
			return params.ChatID.ID == supportGroupID && params.Name == "Anna [@anna_test]"
		})).Return(&telego.ForumTopic{MessageThreadID: newTopicID}, nil)

		// Сохраняет в базу
		mockRepo.EXPECT().SaveTopic(ctx, customerID, newTopicID, categoryID, "ru").Return(nil)

		// Берет категорию для уведомления
		mockRepo.EXPECT().GetCategoryByID(ctx, categoryID).Return(&datastruct.Category{Name: "Тех. вопросы"}, nil)

		// Отправляет системное сообщение в новый топик о том, что он пересоздан
		mockBot.EXPECT().SendMessage(ctx, mock.MatchedBy(func(params *telego.SendMessageParams) bool {
			return params.MessageThreadID == newTopicID && strings.Contains(params.Text, "Обращение пересоздано")
		})).Return(&telego.Message{}, nil)

		// Отправляет само сообщение клиента в НОВЫЙ топик
		mockBot.EXPECT().SendMessage(ctx, mock.MatchedBy(func(params *telego.SendMessageParams) bool {
			return params.MessageThreadID == newTopicID && params.Text == msg.Text
		})).Return(&telego.Message{}, nil)

		// Проверяет рабочие часы для нового топика
		mockRepo.EXPECT().GetCategoryByID(ctx, categoryID).Return(&datastruct.Category{WorkHours: nil}, nil)

		// Запускаем!
		err := svc.HandleCustomerMessage(ctx, msg)

		// Несмотря на то, что старый топик был удален, функция должна отработать без ошибок
		// и незаметно для клиента доставить сообщение.
		assert.NoError(t, err)
	})
}

func TestSupportService_CloseTopicByClient(t *testing.T) {
	// =====================================================================
	// Сценарий 1: Успешное закрытие активного топика
	// =====================================================================
	t.Run("Success closing active topic", func(t *testing.T) {
		mockRepo := repoMocks.NewMockRepository(t)
		mockBot := clientsMocks.NewMockBot(t)
		mockTg := clientsMocks.NewMockTelegram(t)

		// Нам не нужен LLM для этого метода, поэтому можем оставить nil или сделать мок
		// Но лучше сделать мок на всякий случай, хотя EXPECT() мы для него не пишем
		mockTg.EXPECT().GetBot().Return(mockBot)

		supportGroupID := int64(-100123456)
		svc := NewSupportService(mockRepo, mockTg, nil, "ru", supportGroupID)

		ctx := context.Background()
		customerID := int64(333)
		topicID := 42

		// 1. Бот проверяет статус топика
		mockRepo.EXPECT().GetCustomerTopic(ctx, customerID).Return(&datastruct.CustomerTopic{
			TopicID:  topicID,
			IsClosed: false, // Топик сейчас открыт
		}, nil)

		// 2. Обновляет статус в БД
		mockRepo.EXPECT().SetTopicStatus(ctx, topicID, true).Return(nil)

		// 3. Берет тексты для уведомления менеджеров
		mockRepo.EXPECT().GetSetting(ctx, "messages").Return("", nil)

		// 4. Отправляет уведомление в группу
		mockBot.EXPECT().SendMessage(ctx, mock.MatchedBy(func(params *telego.SendMessageParams) bool {
			return params.MessageThreadID == topicID && params.ChatID.ID == supportGroupID
		})).Return(&telego.Message{}, nil)

		// 5. Физически закрывает топик (ветку) в Телеграме
		mockBot.EXPECT().CloseForumTopic(ctx, mock.MatchedBy(func(params *telego.CloseForumTopicParams) bool {
			return params.MessageThreadID == topicID && params.ChatID.ID == supportGroupID
		})).Return(nil)

		// Запускаем
		err := svc.CloseTopicByClient(ctx, customerID)
		assert.NoError(t, err)
	})

	// =====================================================================
	// Сценарий 2: Попытка закрыть уже закрытый топик (Ранний выход)
	// =====================================================================
	t.Run("Already closed", func(t *testing.T) {
		mockRepo := repoMocks.NewMockRepository(t)
		// Мы даже не создаем мок бота, потому что логика не должна дойти до работы с Telegram API

		svc := NewSupportService(mockRepo, nil, nil, "ru", 0)

		ctx := context.Background()
		customerID := int64(333)

		// 1. Бот проверяет статус топика
		mockRepo.EXPECT().GetCustomerTopic(ctx, customerID).Return(&datastruct.CustomerTopic{
			IsClosed: true, // Топик УЖЕ закрыт
		}, nil)

		// Запускаем
		err := svc.CloseTopicByClient(ctx, customerID)

		// Ожидаем, что вернется nil, и никакие другие методы (БД или Телеграм) не будут вызваны
		assert.NoError(t, err)
	})
}

func TestSupportService_CreateOrReopenTopic(t *testing.T) {
	// =====================================================================
	// Сценарий 1: Клиент пишет впервые (Создание нового топика)
	// =====================================================================
	t.Run("Create brand new topic", func(t *testing.T) {
		mockRepo := repoMocks.NewMockRepository(t)
		mockBot := clientsMocks.NewMockBot(t)
		mockTg := clientsMocks.NewMockTelegram(t)
		mockTg.EXPECT().GetBot().Return(mockBot)

		supportGroupID := int64(-100123456)
		svc := NewSupportService(mockRepo, mockTg, nil, "ru", supportGroupID)

		ctx := context.Background()
		customerID := int64(123)
		categoryID := 1
		managerID := int64(999)
		newTopicID := 42

		// 0. Берет тексты для уведомления менеджеров
		mockRepo.EXPECT().GetSetting(ctx, "messages").Return("", nil)

		// 1. Получаем категорию (без рабочих часов, чтобы не усложнять тест проверкой времени)
		mockRepo.EXPECT().GetCategoryByID(ctx, categoryID).Return(&datastruct.Category{
			ID:        categoryID,
			Name:      "Финансы",
			ManagerID: &managerID,
			WorkHours: nil,
		}, nil)

		// 2. Проверяем наличие топика (его нет)
		mockRepo.EXPECT().GetCustomerTopic(ctx, customerID).Return(nil, errors.New("not found"))

		// 3. Создаем топик в Телеграме
		mockBot.EXPECT().CreateForumTopic(ctx, mock.MatchedBy(func(params *telego.CreateForumTopicParams) bool {
			return params.ChatID.ID == supportGroupID && params.Name == "Ivan [@ivan_test]"
		})).Return(&telego.ForumTopic{MessageThreadID: newTopicID}, nil)

		// 4. Сохраняем топик в БД
		mockRepo.EXPECT().SaveTopic(ctx, customerID, newTopicID, categoryID, "ru").Return(nil)

		// 5. Отправляем уведомление менеджеру в ЛС
		mockBot.EXPECT().SendMessage(ctx, mock.MatchedBy(func(params *telego.SendMessageParams) bool {
			return params.ChatID.ID == managerID && strings.Contains(params.Text, "Новое обращение")
		})).Return(&telego.Message{}, nil)

		// 6. Отправляем приветственное сообщение в сам топик
		mockBot.EXPECT().SendMessage(ctx, mock.MatchedBy(func(params *telego.SendMessageParams) bool {
			return params.MessageThreadID == newTopicID && params.ChatID.ID == supportGroupID
		})).Return(&telego.Message{}, nil)

		// Выполняем
		err := svc.CreateOrReopenTopic(ctx, customerID, "ivan_test", "Ivan", categoryID, "ru")
		assert.NoError(t, err)
	})

	// =====================================================================
	// Сценарий 2: Переоткрытие удаленного топика
	// =====================================================================
	t.Run("Recreate deleted topic on reopen", func(t *testing.T) {
		mockRepo := repoMocks.NewMockRepository(t)
		mockBot := clientsMocks.NewMockBot(t)
		mockTg := clientsMocks.NewMockTelegram(t)
		mockTg.EXPECT().GetBot().Return(mockBot)

		supportGroupID := int64(-100123456)
		svc := NewSupportService(mockRepo, mockTg, nil, "ru", supportGroupID)

		ctx := context.Background()
		customerID := int64(123)
		categoryID := 1
		managerID := int64(999)
		oldTopicID := 42
		newTopicID := 43

		mockRepo.EXPECT().GetCategoryByID(ctx, categoryID).Return(&datastruct.Category{
			ID:        categoryID,
			Name:      "Финансы",
			ManagerID: &managerID,
			WorkHours: nil,
		}, nil)

		// 0. Берет тексты для уведомления менеджеров
		mockRepo.EXPECT().GetSetting(ctx, "messages").Return("", nil)

		// 1. Топик ЕСТЬ в базе
		mockRepo.EXPECT().GetCustomerTopic(ctx, customerID).Return(&datastruct.CustomerTopic{
			TopicID: oldTopicID,
		}, nil)

		// 2. Пытаемся его переоткрыть, но получаем ошибку (удален админом)
		mockBot.EXPECT().ReopenForumTopic(ctx, mock.MatchedBy(func(params *telego.ReopenForumTopicParams) bool {
			return params.MessageThreadID == oldTopicID
		})).Return(errors.New("Bad Request: message thread not found"))

		// 3. Бот должен перехватить ошибку и СОЗДАТЬ новый топик
		mockBot.EXPECT().CreateForumTopic(ctx, mock.MatchedBy(func(params *telego.CreateForumTopicParams) bool {
			return params.ChatID.ID == supportGroupID
		})).Return(&telego.ForumTopic{MessageThreadID: newTopicID}, nil)

		// 4. Сохраняем НОВЫЙ ID в БД
		mockRepo.EXPECT().SaveTopic(ctx, customerID, newTopicID, categoryID, "ru").Return(nil)

		// 5. Уведомление менеджеру
		mockBot.EXPECT().SendMessage(ctx, mock.MatchedBy(func(params *telego.SendMessageParams) bool {
			return params.ChatID.ID == managerID && strings.Contains(params.Text, fmt.Sprintf("/%d", newTopicID))
		})).Return(&telego.Message{}, nil)

		// 6. Уведомление в топик
		mockBot.EXPECT().SendMessage(ctx, mock.MatchedBy(func(params *telego.SendMessageParams) bool {
			return params.MessageThreadID == newTopicID
		})).Return(&telego.Message{}, nil)

		err := svc.CreateOrReopenTopic(ctx, customerID, "ivan_test", "Ivan", categoryID, "ru")
		assert.NoError(t, err)
	})
}

func TestSupportService_notifyOutOfHoursIfNeeded(t *testing.T) {
	// =====================================================================
	// Сценарий 1: Рабочие часы 24/7 (Отбивка не нужна)
	// =====================================================================
	t.Run("24/7 no notification", func(t *testing.T) {
		mockRepo := repoMocks.NewMockRepository(t)
		// Бота и LLM не мокаем, так как до них не должно дойти
		svc := NewSupportService(mockRepo, nil, nil, "ru", 0)

		// Передаем nil в workHours, что означает 24/7
		svc.notifyOutOfHoursIfNeeded(context.Background(), 123, nil, nil, "ru")

		// Если бы метод пошел дальше, он бы упал из-за отсутствия моков БД/Бота
	})

	// =====================================================================
	// Сценарий 2: Нерабочие часы, но сработал троттлинг (уже писали недавно)
	// =====================================================================
	t.Run("Throttled notification", func(t *testing.T) {
		mockRepo := repoMocks.NewMockRepository(t)
		svc := NewSupportService(mockRepo, nil, nil, "ru", 0)

		ctx := context.Background()
		customerID := int64(123)
		workHours := "00:00-00:01" // Заведомо нерабочее время для теста
		tz := "UTC"

		// Имитируем, что последняя отбивка была всего 5 минут назад
		recentTime := time.Now().Add(-5 * time.Minute)
		mockRepo.EXPECT().GetSession(ctx, customerID).Return(&datastruct.SessionData{
			LastThrottle: &recentTime,
		}, nil)

		svc.notifyOutOfHoursIfNeeded(ctx, customerID, &workHours, &tz, "ru")
		// Опять же, до отправки сообщения дойти не должно
	})

	// =====================================================================
	// Сценарий 3: Успешная отправка отбивки с переводом на английский
	// =====================================================================
	t.Run("Send notification with translation", func(t *testing.T) {
		mockRepo := repoMocks.NewMockRepository(t)
		mockBot := clientsMocks.NewMockBot(t)
		mockTg := clientsMocks.NewMockTelegram(t)
		mockLLM := clientsMocks.NewMockLLM(t)

		mockTg.EXPECT().GetBot().Return(mockBot)
		svc := NewSupportService(mockRepo, mockTg, mockLLM, "ru", 0)

		ctx := context.Background()
		customerID := int64(123)
		workHours := "00:00-00:01" // Заведомо нерабочее время
		tz := "UTC"
		clientLang := "en"

		// 1. Проверка троттлинга (отбивок еще не было)
		mockRepo.EXPECT().GetSession(ctx, customerID).Return(&datastruct.SessionData{
			LastThrottle: nil,
		}, nil)

		// 2. Получаем системные сообщения
		// Имитируем JSON с нашими текстами
		mockRepo.EXPECT().GetSetting(ctx, "messages").Return(`{"OutOfHours":"Not working: %s"}`, nil)

		// 3. Перевод текста
		expectedTextToTranslate := "Not working: 00:00-00:01 (UTC)"
		translatedText := "Currently out of office: 00:00-00:01 (UTC)"
		mockLLM.EXPECT().Translate(ctx, expectedTextToTranslate, clientLang).Return(translatedText)

		// 4. Отправка сообщения
		mockBot.EXPECT().SendMessage(ctx, mock.MatchedBy(func(params *telego.SendMessageParams) bool {
			return params.ChatID.ID == customerID && params.Text == translatedText
		})).Return(&telego.Message{}, nil)

		// 5. Обновление таймера троттлинга в базе
		mockRepo.EXPECT().UpdateThrottle(ctx, customerID).Return(nil)

		svc.notifyOutOfHoursIfNeeded(ctx, customerID, &workHours, &tz, clientLang)
	})
}
