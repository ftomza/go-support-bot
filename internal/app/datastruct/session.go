/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package datastruct

import "time"

type SessionData struct {
	WaitingName  bool
	FullName     string
	LastThrottle *time.Time
}
