// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package config

import (
	"os"

	"github.com/alecthomas/kong"
	kongtoml "github.com/alecthomas/kong-toml"
)

type Config struct {
	Syslog   bool           `short:"S" help:"Redirect logging to syslog."`
	Database DatabaseConfig `embed:"" prefix:"database-"`
	SMTPD    SMTPDConfig    `embed:"" prefix:"smtpd-"`
	Http     HttpConfig     `embed:"" prefix:"http-"`
}

type DatabaseConfig struct {
	URL string `name:"url" help:"Database connection URL."`
}

type SMTPDConfig struct {
	Listen         string `name:"listen" help:"SMTP listen address." default:":2525"`
	MaxMessageSize int    `name:"max-message-size" help:"Maximum message size in bytes." default:"10485760"`
	MaxRecipients  int    `name:"max-recipients" help:"Maximum number of recipients." default:"100"`
}

type HttpConfig struct {
	BaseURL string `help:"Base URL of the patchwork instance."`
	Listen  string `name:"listen" help:"HTTP listen address." default:":8080"`
}

const commonDescription = `

Configuration is loaded from:

	/etc/patchwork.toml
	~/.config/patchwork.toml
	patchwork.toml
	$PATCHWORK_TOML

In that order. Settings from later files override earlier ones. CLI
flags take precedence over any configuration file settings.
`

func Parse(cfg any, description string) *kong.Context {
	paths := []string{
		"~/.config/patchwork.toml",
		"/etc/patchwork.toml",
		"patchwork.toml",
	}
	p := os.Getenv("PATCHWORK_TOML")
	if p != "" {
		paths = append(paths, p)
	}

	app, err := kong.New(cfg,
		kong.Description(description+commonDescription),
		kong.Configuration(kongtoml.Loader, paths...),
	)
	if err != nil {
		panic(err)
	}

	if bashComplete(app) {
		os.Exit(0)
	}

	ctx, err := app.Parse(os.Args[1:])
	app.FatalIfErrorf(err)
	return ctx
}
