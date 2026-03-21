/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package service

import (
	"context"
	"testing"

	"go-support-bot/internal/app/datastruct"
	repoMocks "go-support-bot/internal/app/repository/mocks"

	"github.com/stretchr/testify/assert"
)

func TestSupportService_GetCategoriesKeyboard(t *testing.T) {
	// =====================================================================
	// Сценарий 1: Главное меню (Корень)
	// =====================================================================
	t.Run("Root menu", func(t *testing.T) {
		mockRepo := repoMocks.NewMockRepository(t)
		svc := NewSupportService(mockRepo, nil, nil, "ru", 0, []int64{16})
		ctx := context.Background()

		// Возвращаем список корневых категорий
		mockRepo.EXPECT().GetCategoriesByParent(ctx, (*int)(nil)).Return([]datastruct.Category{
			{ID: 1, Name: "Вопрос по заказу"},
			{ID: 2, Name: "Жалоба"},
		}, nil)

		kb, err := svc.GetCategoriesKeyboard(ctx, nil)

		assert.NoError(t, err)
		assert.NotNil(t, kb)
		// Ожидаем 2 ряда кнопок (по одной в ряду) и НИКАКИХ кнопок "Назад"
		assert.Len(t, kb.InlineKeyboard, 2)
		assert.Equal(t, "Вопрос по заказу", kb.InlineKeyboard[0][0].Text)
		assert.Equal(t, "cat_1", kb.InlineKeyboard[0][0].CallbackData)
	})

	// =====================================================================
	// Сценарий 2: Первый уровень вложенности (Только кнопка "Назад")
	// =====================================================================
	t.Run("First level submenu", func(t *testing.T) {
		mockRepo := repoMocks.NewMockRepository(t)
		svc := NewSupportService(mockRepo, nil, nil, "ru", 0, []int64{16})
		ctx := context.Background()

		parentID := 1

		// Возвращаем подкатегории
		mockRepo.EXPECT().GetCategoriesByParent(ctx, &parentID).Return([]datastruct.Category{
			{ID: 3, Name: "Где мой заказ?"},
		}, nil)

		// Имитируем получение текстов (чтобы достать надпись для кнопки "Назад")
		mockRepo.EXPECT().GetSetting(ctx, "messages").Return(`{"ButtonBack":"🔙 Назад"}`, nil)

		// Возвращаем информацию о родительской категории (у нее нет своего родителя)
		mockRepo.EXPECT().GetCategoryByID(ctx, parentID).Return(&datastruct.Category{
			ID:       parentID,
			ParentID: nil, // Значит мы на первом уровне
		}, nil)

		kb, err := svc.GetCategoriesKeyboard(ctx, &parentID)

		assert.NoError(t, err)
		// 1 ряд подкатегорий + 1 ряд с навигацией
		assert.Len(t, kb.InlineKeyboard, 2)
		// Проверяем кнопку "Назад"
		assert.Equal(t, "🔙 Назад", kb.InlineKeyboard[1][0].Text)
		assert.Equal(t, "cat_root", kb.InlineKeyboard[1][0].CallbackData) // Возврат в корень
	})

	// =====================================================================
	// Сценарий 3: Глубокая вложенность (Кнопки "Назад" и "В начало")
	// =====================================================================
	t.Run("Deep submenu", func(t *testing.T) {
		mockRepo := repoMocks.NewMockRepository(t)
		svc := NewSupportService(mockRepo, nil, nil, "ru", 0, []int64{16})
		ctx := context.Background()

		parentID := 4
		grandParentID := 1

		// Возвращаем подкатегории
		mockRepo.EXPECT().GetCategoriesByParent(ctx, &parentID).Return([]datastruct.Category{}, nil)

		mockRepo.EXPECT().GetSetting(ctx, "messages").Return(`{"ButtonBack":"🔙 Back", "ButtonHome":"🏠 Home"}`, nil)

		// Родительская категория имеет своего родителя (grandParentID)
		mockRepo.EXPECT().GetCategoryByID(ctx, parentID).Return(&datastruct.Category{
			ID:       parentID,
			ParentID: &grandParentID, // Значит мы глубоко в дереве
		}, nil)

		kb, err := svc.GetCategoriesKeyboard(ctx, &parentID)

		assert.NoError(t, err)
		assert.Len(t, kb.InlineKeyboard, 1)    // 0 категорий + 1 ряд навигации
		assert.Len(t, kb.InlineKeyboard[0], 2) // В ряду навигации должно быть 2 кнопки

		// Кнопка "Назад" на уровень выше
		assert.Equal(t, "🔙 Back", kb.InlineKeyboard[0][0].Text)
		assert.Equal(t, "cat_1", kb.InlineKeyboard[0][0].CallbackData)

		// Кнопка "В начало" в самый корень
		assert.Equal(t, "🏠 Home", kb.InlineKeyboard[0][1].Text)
		assert.Equal(t, "cat_root", kb.InlineKeyboard[0][1].CallbackData)
	})
}

func TestSupportService_GetMessages(t *testing.T) {
	// =====================================================================
	// Сценарий 1: База пустая (возвращаем дефолтные тексты)
	// =====================================================================
	t.Run("Empty DB returns defaults", func(t *testing.T) {
		mockRepo := repoMocks.NewMockRepository(t)
		svc := NewSupportService(mockRepo, nil, nil, "ru", 0, []int64{16})
		ctx := context.Background()

		mockRepo.EXPECT().GetSetting(ctx, "messages").Return("", nil)

		msgs, err := svc.GetMessages(ctx)
		assert.NoError(t, err)
		// Проверяем, что дефолтное значение подставилось
		assert.Equal(t, "❌ Завершить обращение", msgs.CloseTopicButton)
		assert.Equal(t, "🔙 Назад", msgs.ButtonBack)
	})

	// =====================================================================
	// Сценарий 2: В базе есть только часть текстов (проверяем Merge)
	// =====================================================================
	t.Run("Partial DB returns merged with defaults", func(t *testing.T) {
		mockRepo := repoMocks.NewMockRepository(t)
		svc := NewSupportService(mockRepo, nil, nil, "ru", 0, []int64{16})
		ctx := context.Background()

		// В БД сохранено только кастомное приветствие, остальные поля пустые
		dbJSON := `{"WelcomeNewUser": "Кастомный Привет!"}`
		mockRepo.EXPECT().GetSetting(ctx, "messages").Return(dbJSON, nil)

		msgs, err := svc.GetMessages(ctx)
		assert.NoError(t, err)
		// Проверяем, что наше кастомное значение применилось
		assert.Equal(t, "Кастомный Привет!", msgs.WelcomeNewUser)
		// И проверяем, что недостающее поле заполнилось дефолтом (защита от пустых кнопок)
		assert.Equal(t, "❌ Завершить обращение", msgs.CloseTopicButton)
	})
}

func TestSupportService_ExportConfig(t *testing.T) {
	// =====================================================================
	// Сценарий 3: Проверка сборки дерева категорий
	// =====================================================================
	t.Run("Build tree from flat DB records", func(t *testing.T) {
		mockRepo := repoMocks.NewMockRepository(t)
		svc := NewSupportService(mockRepo, nil, nil, "ru", 0, []int64{16})
		ctx := context.Background()

		// Мокаем тексты
		mockRepo.EXPECT().GetMainPrompt(ctx).Return("Главный вопрос", nil)
		mockRepo.EXPECT().GetSetting(ctx, "messages").Return("", nil)
		mockRepo.EXPECT().GetSetting(ctx, "antispam").Return("", nil)

		parentID := 1
		managerID := int64(999)

		// Мокаем плоский ответ от БД (как делает SQL)
		mockRepo.EXPECT().GetAllCategoriesFull(ctx).Return([]datastruct.Category{
			{ID: 1, Name: "Корневая Папка", PromptText: "Текст корня"},
			{ID: 2, ParentID: &parentID, Name: "Подкатегория 1", ManagerID: &managerID},
		}, nil)

		cfg, err := svc.ExportConfig(ctx)
		assert.NoError(t, err)

		// Проверяем, что конфиг собрался правильно
		assert.Equal(t, "Главный вопрос", cfg.Text)

		// Проверяем корень
		rootNode, ok := cfg.Themes["Корневая Папка"]
		assert.True(t, ok)
		assert.Equal(t, "Текст корня", rootNode.Text)

		// Проверяем, что Подкатегория попала внутрь корня
		subNode, ok := rootNode.SubTheme["Подкатегория 1"]
		assert.True(t, ok)
		assert.Equal(t, &managerID, subNode.Manager)
	})
}

func TestSupportService_SetCustomerLangByTopic(t *testing.T) {
	t.Run("Success update language", func(t *testing.T) {
		mockRepo := repoMocks.NewMockRepository(t)
		svc := NewSupportService(mockRepo, nil, nil, "ru", 0, []int64{16})

		ctx := context.Background()
		topicID := 42
		customerID := int64(123)
		langCode := "es" // Испанский

		// 1. Бот ищет клиента по топику
		mockRepo.EXPECT().GetCustomerID(ctx, topicID).Return(customerID, nil)

		// 2. Бот обновляет язык
		mockRepo.EXPECT().UpdateCustomerLang(ctx, customerID, langCode).Return(nil)

		err := svc.SetCustomerLangByTopic(ctx, topicID, langCode)
		assert.NoError(t, err)
	})
}
