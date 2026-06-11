// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package admin

type CLI struct {
	Project   ProjectCmd   `cmd:"" help:"Manage projects."`
	Superuser SuperuserCmd `cmd:"" help:"Manage superusers."`
	SecretKey SecretKeyCmd `cmd:"" name:"secret-key" help:"Generate a secret key."`
}
