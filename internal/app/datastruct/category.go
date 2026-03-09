/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package datastruct

type Category struct {
	ID         int
	ParentID   *int
	Name       string
	PromptText string
	ManagerID  *int64 // Pointer, так как у родительских тем менеджера нет
	WorkHours  *string
}

// CategoryNode используется для рекурсивного сохранения распарсенного YAML
type CategoryNode struct {
	Name       string
	PromptText string
	ManagerID  *int64
	Children   []*CategoryNode
	WorkHours  *string
}
