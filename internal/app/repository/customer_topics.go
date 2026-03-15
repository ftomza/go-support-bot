/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package repository

import (
	"context"
	"go-support-bot/internal/app/datastruct"
)

// GetCustomerTopic возвращает инфо о топике клиента (если он когда-либо создавался)
func (r *SupportRepo) GetCustomerTopic(ctx context.Context, customerID int64) (*datastruct.CustomerTopic, error) {
	var t datastruct.CustomerTopic
	err := r.db.QueryRow(ctx, "SELECT topic_id, category_id, lang_code, is_closed FROM customer_topics WHERE customer_id = $1", customerID).
		Scan(&t.TopicID, &t.CategoryID, &t.LangCode, &t.IsClosed)
	return &t, err
}

// Теперь принимаем langCode
func (r *SupportRepo) SaveTopic(ctx context.Context, customerID int64, topicID int, categoryID int, langCode string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO customer_topics (customer_id, topic_id, category_id, lang_code, is_closed) 
		VALUES ($1, $2, $3, $4, false) 
		ON CONFLICT (customer_id) DO UPDATE SET topic_id = EXCLUDED.topic_id, category_id = EXCLUDED.category_id, lang_code = EXCLUDED.lang_code, is_closed = false`,
		customerID, topicID, categoryID, langCode)
	return err
}

func (r *SupportRepo) GetCustomerID(ctx context.Context, topicID int) (int64, error) {
	var studentID int64
	err := r.db.QueryRow(ctx, "SELECT customer_id FROM customer_topics WHERE topic_id = $1", topicID).Scan(&studentID)
	return studentID, err
}

// SetTopicStatus меняет статус топика (закрыт/открыт)
func (r *SupportRepo) SetTopicStatus(ctx context.Context, topicID int, isClosed bool) error {
	_, err := r.db.Exec(ctx, "UPDATE customer_topics SET is_closed = $1 WHERE topic_id = $2", isClosed, topicID)
	return err
}

// UpdateCustomerLang принудительно обновляет язык для клиента
func (r *SupportRepo) UpdateCustomerLang(ctx context.Context, customerID int64, langCode string) error {
	_, err := r.db.Exec(ctx, "UPDATE customer_topics SET lang_code = $1 WHERE customer_id = $2", langCode, customerID)
	return err
}

func (r *SupportRepo) UpdateActiveManager(ctx context.Context, topicID int, managerID int64) error {
	_, err := r.db.Exec(ctx, "UPDATE customer_topics SET active_manager_id = $1 WHERE topic_id = $2", managerID, topicID)
	return err
}
