/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package endpoints

import (
	"context"
	"encoding/json"
	"fmt"
	"go-support-bot/internal/app/clients/telegram"
	"html"
	"io"
	"io/fs"
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"go-support-bot/internal/app/service"
	"go-support-bot/web"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
)

type APIEndpoints struct {
	svc    service.Service
	bot    telegram.Bot
	devIDs []int64
}

// Теперь принимаем экземпляр бота и массив разработчиков
func NewAPIEndpoints(svc service.Service, bot telegram.Bot, devIDs []int64) *APIEndpoints {
	return &APIEndpoints{
		svc:    svc,
		bot:    bot,
		devIDs: devIDs,
	}
}

// Метод для безопасной отправки ошибки разработчикам (аналогично telegram.go)
func (api *APIEndpoints) notifyDevelopers(text string) {
	if len(text) > 4000 {
		text = text[:4000] + "...</pre>"
	}
	for _, id := range api.devIDs {
		_, _ = api.bot.SendMessage(context.Background(), &telego.SendMessageParams{
			ChatID:    telegoutil.ID(id),
			Text:      text,
			ParseMode: telego.ModeHTML,
		})
	}
}

// apiHandler — это наш кастомный тип HTTP-хендлера, который умеет возвращать error
type apiHandler func(w http.ResponseWriter, r *http.Request) error

// wrap — это Middleware, который перехватывает паники и ошибки
func (api *APIEndpoints) wrap(next apiHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Defer для перехвата критических сбоев (panic)
		defer func() {
			if rec := recover(); rec != nil {
				stack := string(debug.Stack())
				log.Printf("PANIC RECOVERED IN API: %v\n%s", rec, stack)

				errStr := fmt.Sprintf("🚨 <b>ПАНИКА В API!</b>\n\n<b>URL:</b> <code>%s</code>\n<b>Ошибка:</b> %v\n\n<pre>%s</pre>",
					r.URL.Path, rec, html.EscapeString(stack))

				api.notifyDevelopers(errStr)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		// 2. Выполняем сам хендлер
		err := next(w, r)

		// 3. Если хендлер вернул обычную ошибку, шлем её разработчикам
		if err != nil {
			errStr := fmt.Sprintf("⚠️ <b>Ошибка в API:</b>\n<b>URL:</b> <code>%s</code>\n<pre>%v</pre>",
				r.URL.Path, html.EscapeString(err.Error()))
			api.notifyDevelopers(errStr)
		}
	}
}

func (api *APIEndpoints) Register(mux *http.ServeMux) {
	// 1. Достаем папку dist из встроенной файловой системы
	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		panic(err)
	}

	// 2. Отдаем статику (HTML, JS, CSS)
	mux.Handle("/admin/", http.StripPrefix("/admin/", http.FileServer(http.FS(distFS))))

	// 3. Редирект
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
	})

	// ==========================================
	// ОБОРАЧИВАЕМ API ЭНДПОИНТЫ ЧЕРЕЗ api.wrap(api.auth(...))
	// ==========================================

	// Отдаем текущую конфигурацию в формате JSON
	mux.HandleFunc("/api/config/get", api.wrap(api.auth(func(w http.ResponseWriter, r *http.Request) error {
		cfg, err := api.svc.ExportConfig(r.Context())
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return fmt.Errorf("ExportConfig error: %w", err)
		}
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(cfg)
	})))

	// Сохраняем новую конфигурацию из WebApp
	mux.HandleFunc("/api/config/save", api.wrap(api.auth(func(w http.ResponseWriter, r *http.Request) error {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return fmt.Errorf("invalid method: %s", r.Method)
		}

		// (Проверка безопасности X-Telegram-Init-Data теперь работает в middleware api.auth)

		// 2. Читаем сырой JSON от фронтенда и логгируем его
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return fmt.Errorf("read body error: %w", err)
		}

		log.Printf("📥 RAW JSON FROM FRONTEND: %s", string(bodyBytes))

		var cfg service.YamlConfig
		if err := json.Unmarshal(bodyBytes, &cfg); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return fmt.Errorf("decode json error: %w", err)
		}

		log.Printf("📦 PARSED CONFIG: %+v", cfg)

		// ========================================================
		// 🔥 ВАЛИДАЦИЯ КОНФИГА ПЕРЕД СОХРАНЕНИЕМ
		if err := service.ValidateYamlConfig(&cfg); err != nil {
			// Отдаем текст ошибки (400 Bad Request) фронтенду, чтобы он показал Alert
			http.Error(w, err.Error(), http.StatusBadRequest)
			return nil // Возвращаем nil, чтобы не логировало в консоль как панику сервера
		}
		// ========================================================

		// 3. Сохраняем в БД
		if err := api.svc.ImportConfig(r.Context(), &cfg); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return fmt.Errorf("ImportConfig error: %w", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte(`{"status":"ok"}`))
		return err
	})))

	// Отдаем список менеджеров
	mux.HandleFunc("/api/managers", api.wrap(api.auth(func(w http.ResponseWriter, r *http.Request) error {
		managers, err := api.svc.GetManagersData(r.Context())
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return fmt.Errorf("GetManagersData error: %w", err)
		}
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(managers)
	})))

	// Получение списка клиентов (с поддержкой поиска ?search=...)
	mux.HandleFunc("/api/customers", api.wrap(api.auth(func(w http.ResponseWriter, r *http.Request) error {
		search := r.URL.Query().Get("search")
		profiles, err := api.svc.GetCustomerProfiles(r.Context(), search)
		if err != nil {
			return err
		}
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(profiles)
	})))

	// Получение истории рассылок
	mux.HandleFunc("/api/broadcasts/history", api.wrap(api.auth(func(w http.ResponseWriter, r *http.Request) error {
		broadcasts, err := api.svc.GetBroadcasts(r.Context())
		if err != nil {
			return err
		}
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(broadcasts)
	})))

	// Создание новой рассылки
	mux.HandleFunc("/api/broadcasts/create", api.wrap(api.auth(func(w http.ResponseWriter, r *http.Request) error {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return nil
		}

		var req struct {
			Text        string  `json:"text"`
			CustomerIDs []int64 `json:"customer_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return err
		}

		if err := service.ValidateTelegramHTML(req.Text); err != nil {
			http.Error(w, fmt.Sprintf("Ошибка HTML в тексте рассылки: %v", err), http.StatusBadRequest)
			return nil
		}

		_, err := api.svc.CreateBroadcast(r.Context(), req.Text, req.CustomerIDs)
		if err != nil {
			return err
		}

		// Здесь мы позже добавим триггер для воркера, чтобы он сразу начал отправку
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		return nil
	})))

	// Повторная отправка по ошибкам (Retry)
	mux.HandleFunc("/api/broadcasts/retry", api.wrap(api.auth(func(w http.ResponseWriter, r *http.Request) error {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return nil
		}

		var req struct {
			BroadcastID int `json:"broadcast_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return err
		}

		if err := api.svc.RetryBroadcast(r.Context(), req.BroadcastID); err != nil {
			return err
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		return nil
	})))

	// Получение статистики NPS с фильтрами по дате
	mux.HandleFunc("/api/stats/nps", api.wrap(api.auth(func(w http.ResponseWriter, r *http.Request) error {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return nil
		}

		var startDate, endDate *time.Time

		// Парсим даты из Query-параметров (формат: 2026-03-01)
		if startStr := r.URL.Query().Get("start"); startStr != "" {
			if parsed, err := time.Parse("2006-01-02", startStr); err == nil {
				// Устанавливаем начало дня
				startOfDay := parsed.Truncate(24 * time.Hour)
				startDate = &startOfDay
			}
		}

		if endStr := r.URL.Query().Get("end"); endStr != "" {
			if parsed, err := time.Parse("2006-01-02", endStr); err == nil {
				// Устанавливаем конец дня (23:59:59)
				endOfDay := parsed.Truncate(24 * time.Hour).Add(24*time.Hour - time.Second)
				endDate = &endOfDay
			}
		}

		stats, err := api.svc.GetNPSStats(r.Context(), startDate, endDate)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return fmt.Errorf("get nps stats error: %w", err)
		}

		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(stats)
	})))
}

// auth — это Middleware для проверки подлинности запроса от Telegram WebApp
func (api *APIEndpoints) auth(next apiHandler) apiHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		initData := r.Header.Get("X-Telegram-Init-Data")
		if initData == "" {
			http.Error(w, "Unauthorized: Missing init data", http.StatusUnauthorized)
			return fmt.Errorf("missing init data in request to %s", r.URL.Path)
		}

		if _, err := telegoutil.ValidateWebAppData(api.bot.Token(), initData); err != nil {
			http.Error(w, "Forbidden: Invalid authentication data", http.StatusForbidden)
			return fmt.Errorf("invalid webapp init data: %w", err)
		}

		return next(w, r)
	}
}
