/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package repository

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

type SupportRepo struct {
	db *pgxpool.Pool
}

func NewSupportRepo(db *pgxpool.Pool) *SupportRepo {
	return &SupportRepo{db: db}
}
