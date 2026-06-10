// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package pw

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/config"
)

type Context struct {
	context.Context

	Config *config.Config
	DB     *bun.DB
}

func (c *Context) Value(key any) any {
	switch key := key.(type) {
	case string:
		switch key {
		case "cfg", "config":
			return c.Config
		case "db", "database":
			return c.DB
		}
	}
	return nil
}
