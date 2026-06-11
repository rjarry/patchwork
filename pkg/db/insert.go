// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"context"

	"github.com/uptrace/bun"
)

func Insert(ctx context.Context, database *bun.DB, model any) error {
	return database.NewInsert().Model(model).
		ExcludeColumn("id").Returning("id").Scan(ctx)
}
