/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package endpoints

import (
	"encoding/json"
	"go-support-bot/web"
	"io/fs"
	"net/http"

	"go-support-bot/internal/app/service"

	"github.com/mymmrac/telego/telegoutil"
)

type APIEndpoints struct {
	svc      *service.SupportService
	botToken string
}

func NewAPIEndpoints(svc *service.SupportService, botToken string) *APIEndpoints {
	return &APIEndpoints{
		svc:      svc,
		botToken: botToken,
	}
}

func (api *APIEndpoints) Register(mux *http.ServeMux) {
	// 1. Достаем папку dist из встроенной файловой системы
	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		panic(err) // Падаем только при старте, если забыли сделать npm run build
	}

	// 2. Отдаем статику (HTML, JS, CSS) по пути /admin/
	mux.Handle("/admin/", http.StripPrefix("/admin/", http.FileServer(http.FS(distFS))))

	// 3. Если кто-то зайдет без слеша на конце — делаем редирект, чтобы не ломались пути
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
	})

	// Отдаем текущую конфигурацию в формате JSON
	mux.HandleFunc("/api/config/get", func(w http.ResponseWriter, r *http.Request) {
		cfg, err := api.svc.ExportConfig(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
	})

	// Сохраняем новую конфигурацию из WebApp
	mux.HandleFunc("/api/config/save", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		// 1. ПРОВЕРКА БЕЗОПАСНОСТИ: убеждаемся, что запрос пришел от Telegram
		initData := r.Header.Get("X-Telegram-Init-Data")
		if initData == "" {
			http.Error(w, "Missing init data", http.StatusUnauthorized)
			return
		}
		if _, err := telegoutil.ValidateWebAppData(api.botToken, initData); err != nil {
			http.Error(w, "Invalid init data: Hacker detected!", http.StatusForbidden)
			return
		}

		// 2. Парсим присланный JSON
		var cfg service.YamlConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// 3. Сохраняем в БД
		if err := api.svc.ImportConfig(r.Context(), &cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Отдаем список менеджеров
	mux.HandleFunc("/api/managers", func(w http.ResponseWriter, r *http.Request) {
		managers, err := api.svc.GetManagersData(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(managers)
	})
}
