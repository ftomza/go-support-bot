/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package repository

import (
	"context"
	"go-support-bot/internal/app/datastruct"

	"github.com/jackc/pgx/v5"
)

// GetCustomerProfiles возвращает список клиентов с возможностью поиска
func (r *SupportRepo) GetCustomerProfiles(ctx context.Context, search string) ([]datastruct.CustomerProfile, error) {
	query := "SELECT id, full_name, is_blocked FROM customers"
	args := []any{}

	if search != "" {
		query += " WHERE full_name ILIKE $1 OR id::text ILIKE $1"
		args = append(args, "%"+search+"%")
	}
	query += " ORDER BY full_name ASC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []datastruct.CustomerProfile
	for rows.Next() {
		var p datastruct.CustomerProfile
		if err := rows.Scan(&p.CustomerID, &p.FullName, &p.IsBlocked); err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

// CreateBroadcast создает новую рассылку и заполняет очередь отправки
func (r *SupportRepo) CreateBroadcast(ctx context.Context, text string, customerIDs []int64) (int, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var broadcastID int
	err = tx.QueryRow(ctx, "INSERT INTO broadcasts (text) VALUES ($1) RETURNING id", text).Scan(&broadcastID)
	if err != nil {
		return 0, err
	}

	// Эффективная массовая вставка (Bulk Insert)
	var rows [][]any
	for _, id := range customerIDs {
		rows = append(rows, []any{broadcastID, id})
	}

	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"broadcast_recipients"},
		[]string{"broadcast_id", "customer_id"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return 0, err
	}

	return broadcastID, tx.Commit(ctx)
}

// GetBroadcasts возвращает историю рассылок вместе со статистикой
func (r *SupportRepo) GetBroadcasts(ctx context.Context) ([]datastruct.Broadcast, error) {
	query := `
		SELECT 
			b.id, b.text, b.status, b.created_at,
			COUNT(r.id) as total,
			COUNT(r.id) FILTER (WHERE r.status = 'sent') as sent,
			COUNT(r.id) FILTER (WHERE r.status = 'failed') as failed,
			COUNT(r.id) FILTER (WHERE r.status = 'pending') as pending
		FROM broadcasts b
		LEFT JOIN broadcast_recipients r ON b.id = r.broadcast_id
		GROUP BY b.id
		ORDER BY b.created_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var broadcasts []datastruct.Broadcast
	for rows.Next() {
		var b datastruct.Broadcast
		if err := rows.Scan(&b.ID, &b.Text, &b.Status, &b.CreatedAt, &b.Total, &b.Sent, &b.Failed, &b.Pending); err != nil {
			return nil, err
		}
		broadcasts = append(broadcasts, b)
	}
	return broadcasts, rows.Err()
}

// RetryBroadcast переводит ошибочные сообщения обратно в статус pending
func (r *SupportRepo) RetryBroadcast(ctx context.Context, broadcastID int) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "UPDATE broadcast_recipients SET status = 'pending', error_text = NULL WHERE broadcast_id = $1 AND status = 'failed'", broadcastID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, "UPDATE broadcasts SET status = 'pending' WHERE id = $1", broadcastID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetPendingBroadcastTasks берет пачку сообщений из очереди с блокировкой строк
func (r *SupportRepo) GetPendingBroadcastTasks(ctx context.Context, limit int) ([]datastruct.BroadcastTask, error) {
	query := `
		SELECT r.id, r.broadcast_id, r.customer_id, b.text 
		FROM broadcast_recipients r
		JOIN broadcasts b ON r.broadcast_id = b.id
		WHERE r.status = 'pending' AND b.status = 'pending'
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`
	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []datastruct.BroadcastTask
	for rows.Next() {
		var t datastruct.BroadcastTask
		if err := rows.Scan(&t.RecipientID, &t.BroadcastID, &t.CustomerID, &t.Text); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// UpdateBroadcastRecipientStatus обновляет статус конкретного сообщения
func (r *SupportRepo) UpdateBroadcastRecipientStatus(ctx context.Context, recipientID int, status string, errText *string) error {
	_, err := r.db.Exec(ctx, "UPDATE broadcast_recipients SET status = $1, error_text = $2, sent_at = NOW() WHERE id = $3", status, errText, recipientID)
	return err
}

// MarkCustomerAsBlocked ставит клиенту флаг блокировки
func (r *SupportRepo) MarkCustomerAsBlocked(ctx context.Context, customerID int64) error {
	_, err := r.db.Exec(ctx, "UPDATE customers SET is_blocked = true WHERE id = $1", customerID)
	return err
}

// CheckAndCompleteBroadcast проверяет, остались ли еще сообщения в рассылке, и если нет - закрывает её
func (r *SupportRepo) CheckAndCompleteBroadcast(ctx context.Context, broadcastID int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE broadcasts 
		SET status = 'completed' 
		WHERE id = $1 AND NOT EXISTS (
			SELECT 1 FROM broadcast_recipients WHERE broadcast_id = $1 AND status = 'pending'
		)
	`, broadcastID)
	return err
}
