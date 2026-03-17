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
	"time"

	"go-support-bot/internal/app/datastruct"
)

func (s *SupportService) GetNPSStats(ctx context.Context, startDate, endDate *time.Time) (*datastruct.NPSStats, error) {
	// 1. Получаем сырые данные из БД
	stats, err := s.repo.GetNPSStats(ctx, startDate, endDate)
	if err != nil {
		return nil, err
	}

	// 2. Получаем актуальный список менеджеров
	managers, err := s.GetManagersData(ctx) // или GetManagersData()
	if err != nil {
		log.Printf("⚠️ Не удалось получить список менеджеров для обогащения статистики: %v", err)
		// Если ошибка, ставим фолбэки
		for i := range stats.Managers {
			if stats.Managers[i].ManagerID == 0 {
				stats.Managers[i].FullName = "Без менеджера"
			} else {
				stats.Managers[i].FullName = fmt.Sprintf("ID: %d", stats.Managers[i].ManagerID)
			}
		}
		return stats, nil
	}

	// 3. Строим map для быстрого O(1) поиска по ID
	managerMap := make(map[int64]string)
	for _, m := range managers {
		managerMap[m.ID] = m.Name // или m.FullName, смотря какая у тебя структура
	}

	// 4. Обогащаем статистику именами
	for i, ms := range stats.Managers {
		if ms.ManagerID == 0 {
			stats.Managers[i].FullName = "Без менеджера"
		} else if name, exists := managerMap[ms.ManagerID]; exists {
			stats.Managers[i].FullName = name
		} else {
			// Если менеджер был удален или его нет в текущем списке
			stats.Managers[i].FullName = fmt.Sprintf("Удаленный менеджер (%d)", ms.ManagerID)
		}
	}

	return stats, nil
}
