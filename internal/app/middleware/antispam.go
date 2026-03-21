/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package middleware

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"

	"go-support-bot/internal/app/service"
)

// userRate хранит состояние активности пользователя
type userRate struct {
	mu           sync.Mutex
	msgCount     int       // Количество сообщений в текущем окне
	windowStart  time.Time // Начало текущего окна
	blockedUntil time.Time // До какого времени заблокирован (за спам)
	isBanned     bool      // Вечный бан (кэш из БД)
	lastDBCheck  time.Time // Когда последний раз проверяли бан в БД
}

// AntiSpamMiddleware - структура middleware для защиты от флуда
type AntiSpamMiddleware struct {
	svc   service.Service // Наш сервис, чтобы дергать конфиг и БД
	bot   *telego.Bot
	rates sync.Map // Кэш активности пользователей (key: int64, value: *userRate)
}

// NewAntiSpamMiddleware создает новый экземпляр AntiSpamMiddleware
func NewAntiSpamMiddleware(svc service.Service, bot *telego.Bot) *AntiSpamMiddleware {
	m := &AntiSpamMiddleware{
		svc: svc,
		bot: bot,
	}

	// Запускаем фоновую горутину для очистки старых записей (чтобы память не текла)
	go m.cleanupLoop()

	return m
}

// Handler возвращает функцию middleware для telego
func (m *AntiSpamMiddleware) Handler() th.Handler {
	return func(ctx *th.Context, update telego.Update) error {
		var userID int64
		var chatID int64

		// Получаем ID пользователя и чата в зависимости от типа апдейта
		if update.Message != nil {
			userID = update.Message.From.ID
			chatID = update.Message.Chat.ID
		} else if update.CallbackQuery != nil {
			userID = update.CallbackQuery.From.ID
			if update.CallbackQuery.Message != nil {
				chatID = update.CallbackQuery.Message.GetChat().ID
			}
		} else {
			// Если это какой-то другой апдейт (например, my_chat_member), пропускаем
			return ctx.Next(update)
		}

		// Получаем актуальный конфиг (он кэшируется внутри сервиса, так что это быстро)
		cfg, err := m.svc.ExportConfig(context.Background())
		if err != nil || !cfg.AntiSpam.Enabled {
			// Если конфиг недоступен или антиспам выключен - пропускаем
			return ctx.Next(update)
		}

		// Получаем или создаем запись для пользователя
		val, _ := m.rates.LoadOrStore(userID, &userRate{
			windowStart: time.Now(),
		})
		rate := val.(*userRate)

		rate.mu.Lock()
		defer rate.mu.Unlock()

		now := time.Now()

		// --- 1. ПРОВЕРКА ВЕЧНОГО БАНА (Read-Through Cache) ---
		// Проверяем БД не чаще раза в 5 минут, чтобы не дергать базу на каждое сообщение
		if now.Sub(rate.lastDBCheck) > 5*time.Minute {
			isBanned, err := m.svc.CheckUserBanned(context.Background(), userID)
			if err == nil {
				rate.isBanned = isBanned
				rate.lastDBCheck = now
			} else {
				log.Printf("⚠️ Ошибка проверки бана для %d: %v", userID, err)
			}
		}

		if rate.isBanned {
			// Юзер в вечном бане. Молча дропаем апдейт.
			return nil
		}

		// --- 2. ПРОВЕРКА ВРЕМЕННОЙ БЛОКИРОВКИ (ЗА СПАМ) ---
		if now.Before(rate.blockedUntil) {
			// Юзер всё еще наказан за спам. Молча дропаем апдейт.
			return nil
		}

		// --- 3. АЛГОРИТМ TOKEN BUCKET (Окно времени) ---
		// Если текущее окно времени истекло, сбрасываем счетчик
		if now.Sub(rate.windowStart) > time.Duration(cfg.AntiSpam.WindowSeconds)*time.Second {
			rate.msgCount = 0
			rate.windowStart = now
		}

		rate.msgCount++

		// Если превышен лимит сообщений
		if rate.msgCount > cfg.AntiSpam.MaxMessages {
			// Блокируем пользователя
			rate.blockedUntil = now.Add(time.Duration(cfg.AntiSpam.BlockDuration) * time.Second)

			// Отправляем предупреждение
			warningText := cfg.Messages.AntiSpamWarning

			// Пытаемся отправить сообщение асинхронно, чтобы не блокировать текущую горутину
			go func(cID int64, text string) {
				_, err := m.bot.SendMessage(context.Background(), &telego.SendMessageParams{
					ChatID: telego.ChatID{ID: cID},
					Text:   text,
				})
				if err != nil {
					log.Printf("Не удалось отправить предупреждение о спаме: %v", err)
				}
			}(chatID, warningText)

			log.Printf("🛡️ [AntiSpam] Пользователь %d заблокирован на %d сек за флуд (%d сообщений)", userID, cfg.AntiSpam.BlockDuration, rate.msgCount)

			// Дропаем апдейт, так как он превысил лимит
			return nil
		}

		// --- ВСЁ ОК, ПРОПУСКАЕМ ДАЛЬШЕ ---
		// Разблокируем мьютекс до вызова next, чтобы не держать лок во время выполнения бизнес-логики
		rate.mu.Unlock()
		err = ctx.Next(update)
		rate.mu.Lock() // Возвращаем лок для defer
		return err
	}
}

// cleanupLoop периодически очищает старые записи из кэша
func (m *AntiSpamMiddleware) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		now := time.Now()
		m.rates.Range(func(key, value interface{}) bool {
			rate := value.(*userRate)
			rate.mu.Lock()
			// Если с момента начала окна прошло больше часа и юзер не заблокирован и не в вечном бане - удаляем
			if now.Sub(rate.windowStart) > 1*time.Hour && now.After(rate.blockedUntil) && !rate.isBanned {
				m.rates.Delete(key)
			}
			rate.mu.Unlock()
			return true
		})
	}
}
