/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package service

import (
	"context"
	"go-support-bot/internal/app/datastruct"
)

func (s *SupportService) GetCustomerProfiles(ctx context.Context, search string) ([]datastruct.CustomerProfile, error) {
	return s.repo.GetCustomerProfiles(ctx, search)
}

func (s *SupportService) CreateBroadcast(ctx context.Context, text string, customerIDs []int64) (int, error) {
	return s.repo.CreateBroadcast(ctx, text, customerIDs)
}

func (s *SupportService) GetBroadcasts(ctx context.Context) ([]datastruct.Broadcast, error) {
	return s.repo.GetBroadcasts(ctx)
}

func (s *SupportService) RetryBroadcast(ctx context.Context, broadcastID int) error {
	return s.repo.RetryBroadcast(ctx, broadcastID)
}
