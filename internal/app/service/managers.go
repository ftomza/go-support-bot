/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package service

import (
	"context"
	"fmt"
	"html"
	"strings"
)

func (s *SupportService) GetManagersList(ctx context.Context) (string, error) {
	admins, err := s.bot.GetChatAdministrators(ctx, s.supportGroup)
	if err != nil {
		return "", fmt.Errorf("failed to get administrators: %w", err)
	}

	var sb strings.Builder
	// Используем HTML-теги вместо звездочек
	sb.WriteString("📋 <b>Список менеджеров (администраторов группы):</b>\n\n")

	for _, admin := range admins {
		user := admin.MemberUser()

		if user.IsBot {
			continue
		}

		// Экранируем имена и юзернеймы, чтобы <, > или & ничего не сломали
		name := html.EscapeString(user.FirstName)
		if user.LastName != "" {
			name += " " + html.EscapeString(user.LastName)
		}
		if user.Username != "" {
			name += fmt.Sprintf(" (@%s)", html.EscapeString(user.Username))
		}

		// Используем тег <code> для копирования по клику
		sb.WriteString(fmt.Sprintf("• %s — <code>%d</code>\n", name, user.ID))
	}

	sb.WriteString("\n<i>Используйте эти ID для заполнения CSV-файла. Чтобы менеджер появился в этом списке, выдайте ему права администратора в группе.</i>")

	return sb.String(), nil
}

// Добавь структуру для JSON
type ManagerData struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username,omitempty"`
}

// Добавь метод для выгрузки в API
func (s *SupportService) GetManagersData(ctx context.Context) ([]ManagerData, error) {
	admins, err := s.bot.GetChatAdministrators(ctx, s.supportGroup)
	if err != nil {
		return nil, fmt.Errorf("failed to get administrators: %w", err)
	}

	var res []ManagerData
	for _, admin := range admins {
		user := admin.MemberUser()
		if user.IsBot {
			continue
		}

		name := user.FirstName
		if user.LastName != "" {
			name += " " + user.LastName
		}

		res = append(res, ManagerData{
			ID:       user.ID,
			Name:     name,
			Username: user.Username,
		})
	}

	return res, nil
}
