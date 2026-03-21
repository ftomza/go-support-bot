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

// ReplaceCategoriesTree теперь принимает и сохраняет JSON с сообщениями
func (r *SupportRepo) ReplaceCategoriesTree(ctx context.Context, mainPrompt string, messagesJSON []byte, antiSpamJSON []byte, roots []*datastruct.CategoryNode) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// МЯГКОЕ УДАЛЕНИЕ: помечаем все старые категории как неактивные
	if _, err = tx.Exec(ctx, "UPDATE categories SET is_active = false"); err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
       INSERT INTO bot_settings (key, value) 
       VALUES ('main_prompt', $1), ('messages', $2), ('antispam', $3)
       ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		mainPrompt, string(messagesJSON), string(antiSpamJSON))
	if err != nil {
		return err
	}

	var insertNode func(parentID *int, node *datastruct.CategoryNode) error
	insertNode = func(parentID *int, node *datastruct.CategoryNode) error {
		var id int
		err := tx.QueryRow(ctx, `
          INSERT INTO categories (parent_id, name, prompt_text, manager_id, work_hours, timezone, image) 
          VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
			parentID, node.Name, node.PromptText, node.ManagerID, node.WorkHours, node.Timezone, node.Image).Scan(&id)
		if err != nil {
			return err
		}

		for _, child := range node.Children {
			if err := insertNode(&id, child); err != nil {
				return err
			}
		}
		return nil
	}

	for _, root := range roots {
		if err := insertNode(nil, root); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// Универсальный метод для ключей из bot_settings
func (r *SupportRepo) GetSetting(ctx context.Context, key string) (string, error) {
	var val string
	err := r.db.QueryRow(ctx, "SELECT value FROM bot_settings WHERE key = $1", key).Scan(&val)
	return val, err
}

func (r *SupportRepo) GetMainPrompt(ctx context.Context) (string, error) {
	return r.GetSetting(ctx, "main_prompt")
}

// GetAllCategoriesFull выгружает всё дерево для экспорта в YAML
func (r *SupportRepo) GetAllCategoriesFull(ctx context.Context) ([]datastruct.Category, error) {
	// ДОБАВЛЕН ФИЛЬТР WHERE is_active = true
	rows, err := r.db.Query(ctx, "SELECT id, parent_id, name, prompt_text, manager_id, work_hours, timezone, image FROM categories WHERE is_active = true ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []datastruct.Category
	for rows.Next() {
		var c datastruct.Category
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Name, &c.PromptText, &c.ManagerID, &c.WorkHours, &c.Timezone, &c.Image); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

func (r *SupportRepo) GetCategoriesByParent(ctx context.Context, parentID *int) ([]datastruct.Category, error) {
	// ДОБАВЛЕН ФИЛЬТР is_active = true В ОБА ЗАПРОСА
	query := "SELECT id, parent_id, name, prompt_text, manager_id, work_hours, timezone, image FROM categories WHERE parent_id IS NULL AND is_active = true ORDER BY id"
	args := []any{}

	if parentID != nil {
		query = "SELECT id, parent_id, name, prompt_text, manager_id, work_hours, timezone, image FROM categories WHERE parent_id = $1 AND is_active = true ORDER BY id"
		args = append(args, *parentID)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []datastruct.Category
	for rows.Next() {
		var c datastruct.Category
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Name, &c.PromptText, &c.ManagerID, &c.WorkHours, &c.Timezone, &c.Image); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

func (r *SupportRepo) GetCategoryByID(ctx context.Context, id int) (*datastruct.Category, error) {
	var c datastruct.Category
	err := r.db.QueryRow(ctx, "SELECT id, parent_id, name, prompt_text, manager_id, work_hours, timezone, image FROM categories WHERE id = $1", id).
		Scan(&c.ID, &c.ParentID, &c.Name, &c.PromptText, &c.ManagerID, &c.WorkHours, &c.Timezone, &c.Image)
	return &c, err
}

func (r *SupportRepo) SaveRating(ctx context.Context, customerID int64, topicID int, score int) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO topic_ratings (customer_id, topic_id, score, manager_id) 
		VALUES ($1, $2, $3, (
			SELECT COALESCE(ct.active_manager_id, c.manager_id)
			FROM customer_topics ct 
			JOIN categories c ON ct.category_id = c.id 
			WHERE ct.topic_id = $2 
			LIMIT 1
		))`,
		customerID, topicID, score)
	return err
}
