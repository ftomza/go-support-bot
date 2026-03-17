/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package repository

import (
	"context"
	"fmt"
	"time"

	"go-support-bot/internal/app/datastruct"
)

func (r *SupportRepo) GetNPSStats(ctx context.Context, startDate, endDate *time.Time) (*datastruct.NPSStats, error) {
	stats := &datastruct.NPSStats{
		ScoreDistribution: make(map[int]int),
		Managers:          make([]datastruct.ManagerStats, 0),
	}

	dateFilter := ""
	args := []any{}
	argId := 1

	if startDate != nil {
		dateFilter += fmt.Sprintf(" AND created_at >= $%d", argId)
		args = append(args, *startDate)
		argId++
	}
	if endDate != nil {
		dateFilter += fmt.Sprintf(" AND created_at <= $%d", argId)
		args = append(args, *endDate)
		argId++
	}

	// 1. ПОЛУЧАЕМ ОБЩУЮ СТАТИСТИКУ
	queryTotal := `
		SELECT 
			COUNT(*) as total_votes,
			COALESCE(AVG(score), 0) as average_score,
			COUNT(CASE WHEN score = 5 THEN 1 END) as promoters,
			COUNT(CASE WHEN score = 4 THEN 1 END) as passives,
			COUNT(CASE WHEN score IN (1, 2, 3) THEN 1 END) as detractors,
			COUNT(CASE WHEN score = 1 THEN 1 END) as score_1,
			COUNT(CASE WHEN score = 2 THEN 1 END) as score_2,
			COUNT(CASE WHEN score = 3 THEN 1 END) as score_3,
			COUNT(CASE WHEN score = 4 THEN 1 END) as score_4,
			COUNT(CASE WHEN score = 5 THEN 1 END) as score_5
		FROM topic_ratings
		WHERE 1=1 ` + dateFilter

	var s1, s2, s3, s4, s5 int
	err := r.db.QueryRow(ctx, queryTotal, args...).Scan(
		&stats.TotalVotes, &stats.AverageScore,
		&stats.Promoters, &stats.Passives, &stats.Detractors,
		&s1, &s2, &s3, &s4, &s5,
	)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения общей статистики: %w", err)
	}

	stats.ScoreDistribution[1] = s1
	stats.ScoreDistribution[2] = s2
	stats.ScoreDistribution[3] = s3
	stats.ScoreDistribution[4] = s4
	stats.ScoreDistribution[5] = s5

	if stats.TotalVotes > 0 {
		stats.NPS = float64(stats.Promoters-stats.Detractors) / float64(stats.TotalVotes) * 100
	}

	// 2. СТАТИСТИКА ПО МЕНЕДЖЕРАМ (только сырые данные)
	queryManagers := `
		SELECT 
			COALESCE(manager_id, 0) as manager_id,
			COUNT(id) as total_votes,
			AVG(score) as average_score
		FROM topic_ratings
		WHERE 1=1 ` + dateFilter + `
		GROUP BY manager_id
		ORDER BY average_score DESC, total_votes DESC
	`

	rows, err := r.db.Query(ctx, queryManagers, args...)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения статистики менеджеров: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ms datastruct.ManagerStats
		if err := rows.Scan(&ms.ManagerID, &ms.TotalVotes, &ms.AverageScore); err != nil {
			continue
		}
		stats.Managers = append(stats.Managers, ms)
	}

	return stats, nil
}
