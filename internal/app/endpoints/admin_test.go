/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package endpoints

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mymmrac/telego"
	"github.com/stretchr/testify/assert"
)

func TestAPIEndpoints_AuthMiddleware(t *testing.T) {
	// 1. Arrange: Создаем фейкового бота (токен выдуманный)
	bot, err := telego.NewBot("1234567890:ABCDEFGHIJKLMNOPQRSTUVWXYZ123456789")
	if err != nil {
		t.Fatalf("Failed to create mock bot: %v", err)
	}

	// Создаем наш API (сервис передаем nil, так как запрос должен отбиться ДО входа в сервис)
	api := NewAPIEndpoints(nil, bot, []int64{})

	// Оборачиваем простую заглушку в наши middleware (как в основном коде)
	testHandler := api.wrap(api.auth(func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	}))

	// =====================================================================
	// Сценарий 1: Запрос вообще без заголовка X-Telegram-Init-Data
	// =====================================================================
	t.Run("Missing Init-Data (Unauthorized)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/config/get", nil)
		rec := httptest.NewRecorder() // Шпион для записи ответа сервера

		testHandler(rec, req)

		// Проверяем, что хакер получил 401 Unauthorized
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Contains(t, rec.Body.String(), "Missing init data")
	})

	// =====================================================================
	// Сценарий 2: Запрос с фейковым заголовком (попытка подделать подпись)
	// =====================================================================
	t.Run("Invalid Init-Data (Hacker Attempt)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/config/get", nil)

		// Хакер пытается передать строку, похожую на Telegram, но с фейковым hash
		fakeInitData := "query_id=AAH&user=%7B%22id%22%3A123%7D&auth_date=1612345678&hash=invalid_hash_string"
		req.Header.Set("X-Telegram-Init-Data", fakeInitData)

		rec := httptest.NewRecorder()

		testHandler(rec, req)

		// Проверяем, что хакер получил 403 Forbidden
		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "Invalid init data")
	})
}
