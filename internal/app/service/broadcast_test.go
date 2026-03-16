/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/mymmrac/telego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	clientsMocks "go-support-bot/internal/app/clients/mocks"
	"go-support-bot/internal/app/datastruct"
	repoMocks "go-support-bot/internal/app/repository/mocks"
)

func TestSupportService_processBroadcastBatch_Success(t *testing.T) {
	mockRepo := repoMocks.NewMockRepository(t)
	mockBot := clientsMocks.NewMockBot(t)
	mockTg := clientsMocks.NewMockTelegram(t)

	// Мокаем бота (как мы делали в других тестах)
	mockTg.EXPECT().GetBot().Return(mockBot).Maybe()

	// Создаем сервис (передаем nil для LLM и пустой массив для DeveloperIDs)
	svc := NewSupportService(mockRepo, mockTg, nil, "ru", 0, []int64{})
	ctx := context.Background()

	// 1. Мокаем очередь: воркер забирает 1 задачу
	tasks := []datastruct.BroadcastTask{
		{RecipientID: 1, BroadcastID: 10, CustomerID: 100, Text: "Привет, это рассылка!"},
	}
	mockRepo.EXPECT().GetPendingBroadcastTasks(ctx, 50).Return(tasks, nil)

	// 2. Мокаем успешную отправку в Телеграм
	mockBot.EXPECT().SendMessage(ctx, mock.MatchedBy(func(params *telego.SendMessageParams) bool {
		return params.ChatID.ID == 100 && params.Text == "Привет, это рассылка!"
	})).Return(&telego.Message{}, nil)

	// 3. Ожидаем, что статус обновится на 'sent' (и текст ошибки будет nil)
	mockRepo.EXPECT().UpdateBroadcastRecipientStatus(ctx, 1, "sent", (*string)(nil)).Return(nil)

	// 4. Ожидаем проверку на завершение рассылки
	mockRepo.EXPECT().CheckAndCompleteBroadcast(ctx, 10).Return(nil)

	// ЗАПУСК
	svc.processBroadcastBatch(ctx)
}

func TestSupportService_processBroadcastBatch_BlockedUser(t *testing.T) {
	mockRepo := repoMocks.NewMockRepository(t)
	mockBot := clientsMocks.NewMockBot(t)
	mockTg := clientsMocks.NewMockTelegram(t)
	mockTg.EXPECT().GetBot().Return(mockBot).Maybe()

	svc := NewSupportService(mockRepo, mockTg, nil, "ru", 0, []int64{})
	ctx := context.Background()

	// 1. Мокаем очередь
	tasks := []datastruct.BroadcastTask{
		{RecipientID: 2, BroadcastID: 11, CustomerID: 200, Text: "Скидки!"},
	}
	mockRepo.EXPECT().GetPendingBroadcastTasks(ctx, 50).Return(tasks, nil)

	// 2. Мокаем ОШИБКУ ОТ ТЕЛЕГРАМА (юзер заблокировал бота)
	errMsg := "Forbidden: bot was blocked by the user"
	mockBot.EXPECT().SendMessage(ctx, mock.AnythingOfType("*telego.SendMessageParams")).Return(nil, errors.New(errMsg))

	// 3. САМОЕ ВАЖНОЕ: Ожидаем, что бот пометит юзера как заблокированного в БД
	mockRepo.EXPECT().MarkCustomerAsBlocked(ctx, int64(200)).Return(nil)

	// 4. Ожидаем, что статус обновится на 'failed' и запишется текст ошибки
	mockRepo.EXPECT().UpdateBroadcastRecipientStatus(ctx, 2, "failed", mock.MatchedBy(func(s *string) bool {
		return s != nil && *s == errMsg
	})).Return(nil)

	// 5. Ожидаем проверку на завершение рассылки
	mockRepo.EXPECT().CheckAndCompleteBroadcast(ctx, 11).Return(nil)

	// ЗАПУСК
	svc.processBroadcastBatch(ctx)
}

func TestSupportService_processBroadcastBatch_EmptyQueue(t *testing.T) {
	mockRepo := repoMocks.NewMockRepository(t)
	svc := NewSupportService(mockRepo, nil, nil, "ru", 0, []int64{})
	ctx := context.Background()

	// Если очередь пуста, воркер должен просто тихо выйти, не дергая бота
	mockRepo.EXPECT().GetPendingBroadcastTasks(ctx, 50).Return([]datastruct.BroadcastTask{}, nil)

	// Функция не должна упасть (паника будет перехвачена дефером, если что-то пойдет не так)
	assert.NotPanics(t, func() {
		svc.processBroadcastBatch(ctx)
	})
}

func TestSupportService_CreateBroadcast(t *testing.T) {
	mockRepo := repoMocks.NewMockRepository(t)
	// Для этого метода бот не нужен, только БД
	svc := NewSupportService(mockRepo, nil, nil, "ru", 0, nil)
	ctx := context.Background()

	text := "Новогодняя скидка 50%!"
	customerIDs := []int64{101, 102, 103}
	expectedBroadcastID := 42

	// Ожидаем, что сервис просто передаст данные в репозиторий
	mockRepo.EXPECT().CreateBroadcast(ctx, text, customerIDs).Return(expectedBroadcastID, nil)

	id, err := svc.CreateBroadcast(ctx, text, customerIDs)

	assert.NoError(t, err)
	assert.Equal(t, expectedBroadcastID, id)
}

func TestSupportService_GetBroadcasts(t *testing.T) {
	mockRepo := repoMocks.NewMockRepository(t)
	svc := NewSupportService(mockRepo, nil, nil, "ru", 0, nil)
	ctx := context.Background()

	expectedHistory := []datastruct.Broadcast{
		{ID: 1, Text: "Test 1", Status: "completed", Total: 10, Sent: 10, Failed: 0, Pending: 0},
		{ID: 2, Text: "Test 2", Status: "pending", Total: 5, Sent: 2, Failed: 1, Pending: 2},
	}

	mockRepo.EXPECT().GetBroadcasts(ctx).Return(expectedHistory, nil)

	history, err := svc.GetBroadcasts(ctx)

	assert.NoError(t, err)
	assert.Len(t, history, 2)
	assert.Equal(t, "completed", history[0].Status)
	assert.Equal(t, 1, history[1].Failed)
}

func TestSupportService_RetryBroadcast(t *testing.T) {
	mockRepo := repoMocks.NewMockRepository(t)
	svc := NewSupportService(mockRepo, nil, nil, "ru", 0, nil)
	ctx := context.Background()
	broadcastID := 42

	mockRepo.EXPECT().RetryBroadcast(ctx, broadcastID).Return(nil)

	err := svc.RetryBroadcast(ctx, broadcastID)

	assert.NoError(t, err)
}

func TestSupportService_NotifyDevelopers(t *testing.T) {
	mockRepo := repoMocks.NewMockRepository(t)
	mockBot := clientsMocks.NewMockBot(t)
	mockTg := clientsMocks.NewMockTelegram(t)
	mockTg.EXPECT().GetBot().Return(mockBot).Maybe()

	// Настраиваем двух разработчиков в конфиге
	devIDs := []int64{11111, 22222}
	svc := NewSupportService(mockRepo, mockTg, nil, "ru", 0, devIDs)
	ctx := context.Background()

	alertText := "🔥 Тестовый алерт!"

	// Ожидаем, что бот отправит сообщение ПЕРВОМУ разработчику
	mockBot.EXPECT().SendMessage(ctx, mock.MatchedBy(func(params *telego.SendMessageParams) bool {
		return params.ChatID.ID == 11111 && params.Text == alertText
	})).Return(&telego.Message{}, nil)

	// Ожидаем, что бот отправит сообщение ВТОРОМУ разработчику
	mockBot.EXPECT().SendMessage(ctx, mock.MatchedBy(func(params *telego.SendMessageParams) bool {
		return params.ChatID.ID == 22222 && params.Text == alertText
	})).Return(&telego.Message{}, nil)

	// ЗАПУСК
	svc.NotifyDevelopers(ctx, alertText)
}
