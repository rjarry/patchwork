// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package admin

type CLI struct {
	Project   ProjectCmd   `cmd:"" help:"Manage projects."`
	Superuser SuperuserCmd `cmd:"" help:"Manage superusers."`
}
