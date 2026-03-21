/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"go-support-bot/internal/app/middleware"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-support-bot/internal/app/clients/llm"
	"go-support-bot/internal/app/clients/telegram"
	"go-support-bot/internal/app/config"
	"go-support-bot/internal/app/endpoints"
	"go-support-bot/internal/app/repository"
	"go-support-bot/internal/app/service"
	"go-support-bot/migration"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/pressly/goose/v3"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.Parse()

	if configPath == "" {
		configPath = os.Getenv("CONFIG_PATH")
		if configPath == "" {
			configPath = "config/local.yaml"
		}
	}

	cfg := config.MustLoad(configPath)
	log.Printf("Starting bot in %s environment...", cfg.Env)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// ==========================================================
	// 1. Выполняем миграции БД (Goose)
	// ==========================================================
	log.Println("Running database migrations...")

	// Goose использует стандартный database/sql, поэтому открываем временное соединение
	migrationDB, err := sql.Open("pgx", cfg.Database.URL)
	if err != nil {
		log.Fatalf("Failed to open DB for migrations: %v", err)
	}

	goose.SetBaseFS(migration.FS) // Передаем нашу встроенную файловую систему
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("Failed to set goose dialect: %v", err)
	}

	// Накатываем миграции из корня встроенной ФС
	if err := goose.Up(migrationDB, "."); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	migrationDB.Close() // Закрываем временное соединение
	log.Println("Migrations applied successfully!")

	// ==========================================================
	// 2. Инициализируем основной пул БД (pgxpool)
	// ==========================================================
	dbPool, err := pgxpool.New(ctx, cfg.Database.URL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer dbPool.Close()

	// 3. Инициализируем Telegram клиента
	clientBot, err := telegram.NewTelegramBot(cfg.Telegram.Token)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	clientLLM, err := llm.NewGeminiClient(ctx, cfg.LLM.GeminiAPIKey, cfg.LLM.EnableTranslate)
	if err != nil {
		log.Fatalf("Failed to create LLM: %v", err)
	}

	// 4. Собираем слои
	repo := repository.NewSupportRepo(dbPool)
	svc := service.NewSupportService(repo, clientBot, clientLLM, cfg.LLM.ManagerLang, cfg.Telegram.SupportGroupID, cfg.Telegram.DeveloperIDs)
	eps := endpoints.NewTelegramEndpoints(svc, cfg.Telegram.DeveloperIDs, cfg.Telegram.MiniAppURL)

	svc.StartBroadcastWorker(ctx)

	// ==========================================================
	// Выбор режима работы: Webhooks или Long Polling
	// ==========================================================

	var updatesChan <-chan telego.Update

	if cfg.Telegram.UseWebhooks {
		log.Printf("Starting bot via Webhooks on %s...", cfg.Telegram.WebhookURL)

		// Создаем стандартный роутер
		mux := http.NewServeMux()

		// Регистрируем наши API ручки для WebApp
		apiEps := endpoints.NewAPIEndpoints(svc, clientBot.Bot, cfg.Telegram.DeveloperIDs)
		apiEps.Register(mux)

		// Устанавливаем вебхук в Telegram API
		err = clientBot.Bot.SetWebhook(ctx, &telego.SetWebhookParams{
			URL:         cfg.Telegram.WebhookURL,
			SecretToken: clientBot.Bot.SecretToken(),
		})

		if err != nil {
			log.Fatalf("Failed to set webhook: %v", err)
		}

		// Настраиваем обработчик вебхуков от telego (он сам займет путь "/hook" внутри mux)
		updatesChan, _ = clientBot.Bot.UpdatesViaWebhook(ctx, telego.WebhookHTTPServeMux(mux, "/hook", clientBot.Bot.SecretToken()))

		srv := &http.Server{
			Addr:    ":" + cfg.Server.Port,
			Handler: mux,
		}
		go func() {
			if cfg.Server.CertFile != "" && cfg.Server.KeyFile != "" {
				log.Printf("Starting HTTPS Webhook server on port %s...", cfg.Server.Port)
				if err := srv.ListenAndServeTLS(cfg.Server.CertFile, cfg.Server.KeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Fatalf("HTTPS server failed: %v", err)
				}
			} else {
				// Иначе запускаем обычный HTTP (для локального тестирования через ngrok)
				log.Printf("Starting HTTP Webhook server on port %s...", cfg.Server.Port)
				if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Fatalf("HTTP server failed: %v", err)
				}
			}
		}()
		go func() {
			<-ctx.Done()
			if cfg.Telegram.UseWebhooks {
				if err := srv.Shutdown(ctx); err != nil {
					log.Printf("failed to shutdown webhook server: %v", err)
				}
			}
		}()

	} else {
		// Режим Long Polling
		log.Println("Starting bot via Long Polling...")
		updatesChan, err = clientBot.Bot.UpdatesViaLongPolling(ctx, nil)
		if err != nil {
			log.Fatalf("Failed to start long polling: %v", err)
		}
	}

	// 4. Настраиваем обработчик (telegohandler)
	bh, err := telegohandler.NewBotHandler(clientBot.Bot, updatesChan)
	if err != nil {
		log.Fatalf("Failed to create bot handler: %v", err)
	}

	// Инициализируем middleware
	antiSpam := middleware.NewAntiSpamMiddleware(svc, clientBot.Bot)

	// Подключаем к роутеру
	bh.Use(antiSpam.Handler())

	// 5. Регистрируем наши эндпоинты в хендлере
	eps.Register(bh)

	// Запускаем обработку входящих сообщений (блокирует поток)
	log.Println("Bot handler is running...")
	go func() {
		if err = bh.Start(); err != nil {
			log.Fatalf("Failed to start bot: %v", err)
		}
	}()

	// Initialize done chan
	done := make(chan struct{}, 1)

	// Handle stop signal (Ctrl+C)
	go func() {
		// Wait for stop signal
		<-ctx.Done()
		log.Println("Stopping...")

		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second*20)
		defer stopCancel()

	loop:
		for len(updatesChan) > 0 {
			select {
			case <-stopCtx.Done():
				break loop
			case <-time.After(time.Microsecond * 100):
				// Continue
			}
		}
		log.Println("Long polling done")

		_ = bh.StopWithContext(stopCtx)
		log.Println("Bot handler done")

		// Notify that stop is done
		done <- struct{}{}
	}()

	// Wait for the stop process to be completed
	<-done
	log.Println("Done")
}
