/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package middleware_test

import (
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go-support-bot/internal/app/middleware"
	"go-support-bot/internal/app/service"
	"go-support-bot/internal/app/service/mocks"
)

// dummyRoundTripper заглушает реальные HTTP-запросы к Telegram API
type dummyRoundTripper struct{}

func (d *dummyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Возвращаем фейковый успешный ответ (чтобы бот не падал при отправке Warning)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
	}, nil
}

func setupTestBot(t *testing.T) *telego.Bot {
	validFormatToken := "1234567890:ABCDEFGHIJKLMNOPQRSTUVWXYZ123456789"

	bot, err := telego.NewBot(validFormatToken, telego.WithHTTPClient(&http.Client{
		Transport: &dummyRoundTripper{},
	}))
	require.NoError(t, err)
	return bot
}

func TestAntiSpamMiddleware_TokenBucket(t *testing.T) {
	mockSvc := mocks.NewMockService(t)
	bot := setupTestBot(t)

	// Настраиваем конфиг: лимит 2 сообщения
	testCfg := &service.YamlConfig{
		AntiSpam: service.AntiSpamConfig{
			Enabled:       true,
			MaxMessages:   2,
			WindowSeconds: 10,
			BlockDuration: 60,
		},
		Messages: service.YamlMessages{
			AntiSpamWarning: "Stop spamming!",
		},
	}

	mockSvc.On("ExportConfig", mock.Anything).Return(testCfg, nil)
	// Юзер 123 НЕ в вечном бане
	mockSvc.On("CheckUserBanned", mock.Anything, int64(123)).Return(false, nil)

	mw := middleware.NewAntiSpamMiddleware(mockSvc, bot)
	handlerFunc := mw.Handler()

	// Фейковый апдейт (сообщение от пользователя)
	update := telego.Update{
		Message: &telego.Message{
			From: &telego.User{ID: 123},
			Chat: telego.Chat{ID: 123},
		},
	}

	// Атомарный счетчик, чтобы избежать race conditions в тестах
	var nextCalled int32

	// Helper-функция для симуляции прохождения одного сообщения через роутер
	callHandler := func() {
		updates := make(chan telego.Update, 1)
		bh, err := th.NewBotHandler(bot, updates)
		require.NoError(t, err)

		// Подключаем наш мидлвар (в telego v3+ сигнатура совпадает)
		bh.Use(handlerFunc)

		// Регистрируем пустой хэндлер, который просто инкрементит счетчик
		bh.Handle(func(ctx *th.Context, u telego.Update) error {
			atomic.AddInt32(&nextCalled, 1)
			return nil
		})

		// Отправляем апдейт в очередь
		updates <- update

		// Запускаем обработку в фоне
		go bh.Start()

		// Даем роутеру немного времени на обработку (хватит даже 50 мс)
		time.Sleep(50 * time.Millisecond)

		// Аккуратно тушим роутер
		bh.Stop()
	}

	// 1-е сообщение: должно пройти успешно
	callHandler()
	assert.Equal(t, int32(1), atomic.LoadInt32(&nextCalled), "Первое сообщение должно быть пропущено")

	// 2-е сообщение: должно пройти (достигли лимита)
	callHandler()
	assert.Equal(t, int32(2), atomic.LoadInt32(&nextCalled), "Второе сообщение должно быть пропущено")

	// 3-е сообщение: СПАМ! Должно быть заблокировано мидлваром
	callHandler()
	assert.Equal(t, int32(2), atomic.LoadInt32(&nextCalled), "Третье сообщение должно быть ЗАБЛОКИРОВАНО (счетчик не увеличился)")

	mockSvc.AssertExpectations(t)
}

func TestAntiSpamMiddleware_PermanentBan(t *testing.T) {
	mockSvc := mocks.NewMockService(t)
	bot := setupTestBot(t)

	testCfg := &service.YamlConfig{
		AntiSpam: service.AntiSpamConfig{Enabled: true, MaxMessages: 10},
	}

	mockSvc.EXPECT().ExportConfig(mock.Anything).Return(testCfg, nil)
	// 🔥 Этот юзер сидит в вечном бане!
	mockSvc.On("CheckUserBanned", mock.Anything, int64(666)).Return(true, nil)

	mw := middleware.NewAntiSpamMiddleware(mockSvc, bot)
	handlerFunc := mw.Handler()

	update := telego.Update{
		Message: &telego.Message{From: &telego.User{ID: 666}},
	}

	var nextCalled int32

	updates := make(chan telego.Update, 1)
	bh, err := th.NewBotHandler(bot, updates)
	require.NoError(t, err)

	bh.Use(handlerFunc)
	bh.Handle(func(ctx *th.Context, u telego.Update) error {
		atomic.AddInt32(&nextCalled, 1)
		return nil
	})

	updates <- update

	go bh.Start()
	time.Sleep(50 * time.Millisecond)
	bh.Stop()

	// Запрос должен быть сразу отклонен без отправки warning-а
	assert.Equal(t, int32(0), atomic.LoadInt32(&nextCalled), "Запрос от забаненного юзера должен быть проигнорирован")
}
