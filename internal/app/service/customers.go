/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package service

import (
	"context"
)

func (s *SupportService) SaveCustomer(ctx context.Context, customerID int64, fullName, username string) error {
	return s.repo.SaveCustomer(ctx, customerID, fullName, username)
}
