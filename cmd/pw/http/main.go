// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/api"
	"github.com/getpatchwork/patchwork/pkg/log"
	"github.com/getpatchwork/patchwork/pkg/web"
)

type CLI struct{}

func (c *CLI) Run(ctx *pw.Context) error {
	router := web.NewRouter(ctx.Config, ctx.DB)
	router.Mount("/", api.NewRouter(ctx.DB))

	srv := &http.Server{
		Addr:     ctx.Config.Http.Listen,
		Handler:  router,
		ErrorLog: log.ErrLogger(),
	}
	sock, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Noticef("listening on %s", srv.Addr)
		if e := srv.Serve(sock); e != nil && e != http.ErrServerClosed {
			err = fmt.Errorf("serve: %w", e)
			done <- syscall.SIGCHLD
		}
	}()

	sig := <-done
	if sig != syscall.SIGCHLD {
		log.Noticef("received signal %v, shutting down", sig)
		timeout, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if e := srv.Shutdown(timeout); e != nil {
			err = fmt.Errorf("shutdown: %w", e)
		}
	}

	return err
}
