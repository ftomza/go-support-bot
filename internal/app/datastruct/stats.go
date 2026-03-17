/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package datastruct

// NPSStats содержит всю статистику для дашборда
type NPSStats struct {
	TotalVotes        int            `json:"total_votes"`
	AverageScore      float64        `json:"average_score"`
	Promoters         int            `json:"promoters"`          // Оценки 5
	Passives          int            `json:"passives"`           // Оценки 4
	Detractors        int            `json:"detractors"`         // Оценки 1-3
	NPS               float64        `json:"nps"`                // Индекс лояльности (%)
	ScoreDistribution map[int]int    `json:"score_distribution"` // Распределение: балл -> количество
	Managers          []ManagerStats `json:"managers"`
}

// ManagerStats статистика по конкретному менеджеру
type ManagerStats struct {
	ManagerID    int64   `json:"manager_id"`
	FullName     string  `json:"full_name"`
	TotalVotes   int     `json:"total_votes"`
	AverageScore float64 `json:"average_score"`
}
