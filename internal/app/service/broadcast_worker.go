/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package service

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// StartBroadcastWorker запускает фоновый процесс рассылки
func (s *SupportService) StartBroadcastWorker(ctx context.Context) {
	// Воркер просыпается каждые 5 секунд
	ticker := time.NewTicker(5 * time.Second)

	go func() {
		log.Println("🚀 Broadcast worker started")
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				log.Println("🛑 Broadcast worker stopped")
				return
			case <-ticker.C:
				s.processBroadcastBatch(ctx)
			}
		}
	}()
}

func (s *SupportService) processBroadcastBatch(ctx context.Context) {
	// =====================================================================
	// НАШ ЛЮБИМЫЙ ОТЛОВЛЯТОР ПАНИК И ОШИБОК 🛡️
	// =====================================================================
	defer func() {
		if r := recover(); r != nil {
			panicText := fmt.Sprintf("🔥 <b>КРИТИЧЕСКАЯ ОШИБКА (ПАНИКА)</b>\nВоркер рассылок упал!\n\n<pre>%v</pre>\n\n<b>Стек-трейс:</b>\n<pre>%s</pre>", r, string(debug.Stack()))

			// Пишем в логи сервера
			log.Printf("🔥 ПАНИКА В ВОРКЕРЕ РАССЫЛОК: %v\nСтек-трейс:\n%s", r, debug.Stack())

			// И сразу пушим в Телеграм разработчикам!
			s.NotifyDevelopers(ctx, panicText)
		}
	}()
	// =====================================================================

	// Берем 50 сообщений за один проход
	tasks, err := s.repo.GetPendingBroadcastTasks(ctx, 50)
	if err != nil {
		log.Printf("❌ Ошибка получения задач для рассылки: %v", err)
		return
	}
	if len(tasks) == 0 {
		return // Очередь пуста
	}

	processedBroadcasts := make(map[int]bool)

	for _, task := range tasks {
		// RATE LIMITING ТЕЛЕГРАМА: Пауза 50 миллисекунд (максимум 20 сообщений в секунду)
		time.Sleep(50 * time.Millisecond)

		var errText *string
		status := "sent"

		// Отправляем сообщение
		_, err := s.bot.GetBot().SendMessage(ctx, tu.Message(
			tu.ID(task.CustomerID),
			task.Text,
		).WithParseMode(telego.ModeHTML))

		// Если произошла ошибка
		if err != nil {
			status = "failed"
			errMsg := err.Error()
			errText = &errMsg

			// Анализируем ошибку. Если юзер нас заблокировал:
			if strings.Contains(errMsg, "bot was blocked by the user") || strings.Contains(errMsg, "user is deactivated") {
				_ = s.repo.MarkCustomerAsBlocked(ctx, task.CustomerID)
				log.Printf("🚫 Пользователь %d заблокировал бота. Помечен как is_blocked=true.", task.CustomerID)
			} else {
				// Логируем остальные ошибки (чтобы видеть, почему не дошло)
				log.Printf("⚠️ Ошибка отправки рассылки юзеру %d: %v", task.CustomerID, err)
			}
		}

		// Обновляем статус в БД
		err = s.repo.UpdateBroadcastRecipientStatus(ctx, task.RecipientID, status, errText)
		if err != nil {
			log.Printf("❌ Ошибка обновления статуса рассылки в БД: %v", err)
		}

		processedBroadcasts[task.BroadcastID] = true
	}

	// После завершения пачки проверяем, не закончились ли рассылки целиком
	for broadcastID := range processedBroadcasts {
		err = s.repo.CheckAndCompleteBroadcast(ctx, broadcastID)
		if err != nil {
			log.Printf("❌ Ошибка закрытия рассылки %d: %v", broadcastID, err)
		}
	}
}
