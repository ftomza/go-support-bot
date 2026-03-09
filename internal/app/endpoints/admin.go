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
	"html"
	"io/fs"
	"log"
	"net/http"
	"runtime/debug"

	"go-support-bot/internal/app/service"
	"go-support-bot/web"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
)

type APIEndpoints struct {
	svc    *service.SupportService
	bot    *telego.Bot
	devIDs []int64
}

// Теперь принимаем экземпляр бота и массив разработчиков
func NewAPIEndpoints(svc *service.SupportService, bot *telego.Bot, devIDs []int64) *APIEndpoints {
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
	// ОБОРАЧИВАЕМ API ЭНДПОИНТЫ ЧЕРЕЗ api.wrap
	// ==========================================

	// Отдаем текущую конфигурацию в формате JSON
	mux.HandleFunc("/api/config/get", api.wrap(func(w http.ResponseWriter, r *http.Request) error {
		cfg, err := api.svc.ExportConfig(r.Context())
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return fmt.Errorf("ExportConfig error: %w", err)
		}
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(cfg)
	}))

	// Сохраняем новую конфигурацию из WebApp
	mux.HandleFunc("/api/config/save", api.wrap(func(w http.ResponseWriter, r *http.Request) error {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return fmt.Errorf("invalid method: %s", r.Method)
		}

		// 1. ПРОВЕРКА БЕЗОПАСНОСТИ
		initData := r.Header.Get("X-Telegram-Init-Data")
		if initData == "" {
			http.Error(w, "Missing init data", http.StatusUnauthorized)
			return fmt.Errorf("missing init data")
		}

		if _, err := telegoutil.ValidateWebAppData(api.bot.Token(), initData); err != nil {
			http.Error(w, "Invalid init data: Hacker detected!", http.StatusForbidden)
			return fmt.Errorf("invalid webapp init data: %w", err)
		}

		// 2. Парсим присланный JSON
		var cfg service.YamlConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return fmt.Errorf("decode json error: %w", err)
		}

		// 3. Сохраняем в БД
		if err := api.svc.ImportConfig(r.Context(), &cfg); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return fmt.Errorf("ImportConfig error: %w", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte(`{"status":"ok"}`))
		return err
	}))

	// Отдаем список менеджеров
	mux.HandleFunc("/api/managers", api.wrap(func(w http.ResponseWriter, r *http.Request) error {
		managers, err := api.svc.GetManagersData(r.Context())
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return fmt.Errorf("GetManagersData error: %w", err)
		}
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(managers)
	}))
}
