/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package datastruct

import "time"

// Broadcast представляет саму рассылку
type Broadcast struct {
	ID        int       `json:"id"`
	Text      string    `json:"text"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`

	// Поля для статистики (будем заполнять JOIN'ами из БД)
	Total   int `json:"total"`
	Sent    int `json:"sent"`
	Failed  int `json:"failed"`
	Pending int `json:"pending"`
}

// BroadcastRecipient представляет конкретного получателя в рассылке
type BroadcastRecipient struct {
	ID          int        `json:"id"`
	BroadcastID int        `json:"broadcast_id"`
	CustomerID  int64      `json:"customer_id"`
	Status      string     `json:"status"` // "pending", "sent", "failed"
	ErrorText   *string    `json:"error_text,omitempty"`
	SentAt      *time.Time `json:"sent_at,omitempty"`
}

// CustomerProfile нужен для вывода списка клиентов в админке с чекбоксами
type CustomerProfile struct {
	CustomerID int64  `json:"customer_id"`
	FullName   string `json:"full_name"`
	IsBlocked  bool   `json:"is_blocked"`
	IsBanned   bool   `json:"is_banned"`
}

// BroadcastTask используется воркером для получения данных из очереди
type BroadcastTask struct {
	RecipientID int
	BroadcastID int
	CustomerID  int64
	Text        string
}
