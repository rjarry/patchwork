// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
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
	Ingress  IngressConfig  `embed:"" prefix:"ingress-"`
	Http     HttpConfig     `embed:"" prefix:"http-"`
	SMTP     SMTPConfig     `embed:"" prefix:"smtp-"`
}

type DatabaseConfig struct {
	URL string `name:"url" help:"Database connection URL."`
}

type IngressConfig struct {
	Listen         string `name:"listen" help:"SMTP listen address." default:":2525"`
	MaxMessageSize int    `name:"max-message-size" help:"Maximum message size in bytes." default:"10485760"`
	MaxRecipients  int    `name:"max-recipients" help:"Maximum number of recipients." default:"100"`
}

type SMTPConfig struct {
	Encryption string `name:"transport" help:"SMTP encryption" default:"none" enum:"none,starttls,tls"`
	Host       string `name:"host" help:"SMTP server hostname." default:"localhost"`
	Port       int    `name:"port" help:"SMTP server port." default:"25"`
	User       string `name:"user" help:"SMTP authentication username."`
	Password   string `name:"password" help:"SMTP authentication password."`
	From       string `name:"from" help:"Sender email address for outgoing mail." default:"patchwork@localhost"`
}

type HttpConfig struct {
	BaseURL   string `help:"Base URL of the patchwork instance."`
	Listen    string `name:"listen" help:"HTTP listen address." default:":8080"`
	SecretKey string `name:"secret-key" help:"Secret key for session signing and CSRF tokens."`
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
