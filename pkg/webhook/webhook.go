// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/log"
)

const requestTimeout = 10 * time.Second

type task struct {
	url        string
	category   string
	deliveryID string
	secret     string
	payload    []byte
}

var (
	ch   chan task
	wg   sync.WaitGroup
	once sync.Once
)

func Init(ctx context.Context, bufSize int) {
	once.Do(func() {
		ch = make(chan task, bufSize)
		wg.Add(1)
		go worker(ctx)
	})
}

func Shutdown() {
	if ch != nil {
		close(ch)
		wg.Wait()
	}
}

func worker(ctx context.Context) {
	defer wg.Done()
	for t := range ch {
		post(ctx, &t)
	}
}

func Deliver(
	ctx context.Context,
	database *bun.DB,
	events []db.Event,
	webhooks []db.Webhook,
	project *db.Project,
) {
	if len(webhooks) == 0 || len(events) == 0 {
		return
	}

	q, err := db.Begin(ctx, database)
	if err != nil {
		log.Warnf("webhook: begin read tx: %v", err)
		return
	}
	defer q.Rollback()

	for i := range events {
		e := &events[i]

		payload, err := serializeEvent(q, e, project)
		if err != nil {
			log.Warnf("webhook serialize %s: %v", e.Category, err)
			continue
		}

		for j := range webhooks {
			w := &webhooks[j]
			if !w.MatchesEvent(e.Category) {
				continue
			}
			t := task{
				url:        w.URL,
				category:   e.Category,
				deliveryID: fmt.Sprintf("%d", e.ID),
				secret:     w.Secret,
				payload:    payload,
			}
			if ch != nil {
				ch <- t
			} else {
				post(ctx, &t)
			}
		}
	}
}

func post(ctx context.Context, t *task) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, t.url, bytes.NewReader(t.payload),
	)
	if err != nil {
		log.Warnf("webhook %s: %v", t.url, err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Patchwork-Event", t.category)
	req.Header.Set("X-Patchwork-Delivery", t.deliveryID)

	if t.secret != "" {
		mac := hmac.New(sha256.New, []byte(t.secret))
		mac.Write(t.payload)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Patchwork-Signature", "sha256="+sig)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Warnf("webhook %s: %v", t.url, err)
		return
	}
	resp.Body.Close()

	log.Debugf("webhook %s: %s -> %d", t.category, t.url, resp.StatusCode)
}
