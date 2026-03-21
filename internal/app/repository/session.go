/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package repository

import (
	"context"
	"errors"

	"go-support-bot/internal/app/datastruct"

	"github.com/jackc/pgx/v5"
)

func (r *SupportRepo) GetSession(ctx context.Context, customerID int64) (*datastruct.SessionData, error) {
	var s datastruct.SessionData
	err := r.db.QueryRow(ctx, "SELECT waiting_name, full_name, last_throttle FROM customer_sessions WHERE customer_id = $1", customerID).
		Scan(&s.WaitingName, &s.FullName, &s.LastThrottle)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Сессия еще не создана
		}
		return nil, err
	}
	return &s, nil
}

func (r *SupportRepo) SetWaitingName(ctx context.Context, customerID int64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO customer_sessions (customer_id, waiting_name) 
		VALUES ($1, true)
		ON CONFLICT (customer_id) DO UPDATE SET waiting_name = true`,
		customerID)
	return err
}

func (r *SupportRepo) SaveName(ctx context.Context, customerID int64, name string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO customer_sessions (customer_id, waiting_name, full_name) 
		VALUES ($1, false, $2)
		ON CONFLICT (customer_id) DO UPDATE SET waiting_name = false, full_name = EXCLUDED.full_name`,
		customerID, name)
	return err
}

func (r *SupportRepo) ClearSession(ctx context.Context, customerID int64) error {
	_, err := r.db.Exec(ctx, "DELETE FROM customer_sessions WHERE customer_id = $1", customerID)
	return err
}

func (r *SupportRepo) UpdateThrottle(ctx context.Context, customerID int64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO customer_sessions (customer_id, last_throttle) 
		VALUES ($1, NOW())
		ON CONFLICT (customer_id) DO UPDATE SET last_throttle = NOW()`,
		customerID)
	return err
}

// SaveCustomer сохраняет или обновляет профиль клиента
func (r *SupportRepo) SaveCustomer(ctx context.Context, id int64, fullName string, username string) error {
	query := `
		INSERT INTO customers (id, full_name, username, updated_at) 
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (id) DO UPDATE 
		SET full_name = EXCLUDED.full_name,
		    username = EXCLUDED.username,
		    updated_at = NOW()
	`
	_, err := r.db.Exec(ctx, query, id, fullName, username)
	return err
}

// CheckUserBanned проверяем бан
func (r *SupportRepo) CheckUserBanned(ctx context.Context, customerID int64) (bool, error) {
	var isBanned bool
	err := r.db.QueryRow(ctx, "SELECT is_banned FROM customers WHERE id = $1", customerID).Scan(&isBanned)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil // Если юзера нет в базе, значит он не забанен
		}
		return false, err
	}
	return isBanned, nil
}

func (r *SupportRepo) SetUserBanned(ctx context.Context, customerID int64, isBanned bool) error {
	_, err := r.db.Exec(ctx, "UPDATE customers SET is_banned = $1 WHERE id = $2", isBanned, customerID)
	return err
}
