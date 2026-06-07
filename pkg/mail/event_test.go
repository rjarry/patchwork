// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/getpatchwork/patchwork/pkg/webhook"
)

func TestEventCreation(t *testing.T) {
	database, _ := testDB(t, "test.example.com")

	parseEmail(t, database, sampleDiff,
		withSubject("[PATCH 1/2] test patch"),
		withMsgID("<event-patch-1@test>"),
		withListID("test.example.com"))

	if n := countEvents(t, database, "patch-created"); n != 1 {
		t.Errorf("patch-created events = %d, want 1", n)
	}
	if n := countEvents(t, database, "series-created"); n != 1 {
		t.Errorf("series-created events = %d, want 1", n)
	}

	parseEmail(t, database, sampleDiff,
		withSubject("[PATCH 2/2] test patch 2"),
		withMsgID("<event-patch-2@test>"),
		withInReplyTo("<event-patch-1@test>"),
		withListID("test.example.com"))

	if n := countEvents(t, database, "patch-created"); n != 2 {
		t.Errorf("patch-created events = %d, want 2", n)
	}
	if n := countEvents(t, database, "series-completed"); n != 1 {
		t.Errorf("series-completed events = %d, want 1", n)
	}
	if n := countEvents(t, database, "patch-completed"); n != 2 {
		t.Errorf("patch-completed events = %d, want 2", n)
	}

	parseEmail(t, database, "cover body\n",
		withSubject("[PATCH 0/2] test cover"),
		withMsgID("<event-cover@test>"),
		withInReplyTo("<event-patch-1@test>"),
		withListID("test.example.com"))

	if n := countEvents(t, database, "cover-created"); n != 1 {
		t.Errorf("cover-created events = %d, want 1", n)
	}

	parseEmail(t, database, "looks good\n",
		withSubject("Re: [PATCH 1/2] test patch"),
		withMsgID("<event-comment@test>"),
		withInReplyTo("<event-patch-1@test>"),
		withListID("test.example.com"))

	if n := countEvents(t, database, "patch-comment-created"); n != 1 {
		t.Errorf("patch-comment-created events = %d, want 1", n)
	}

	parseEmail(t, database, "overall looks good\n",
		withSubject("Re: [PATCH 0/2] test cover"),
		withMsgID("<event-cover-comment@test>"),
		withInReplyTo("<event-cover@test>"),
		withListID("test.example.com"))

	if n := countEvents(t, database, "cover-comment-created"); n != 1 {
		t.Errorf("cover-comment-created events = %d, want 1", n)
	}
}

func TestEventPatchCompletedOrder(t *testing.T) {
	database, _ := testDB(t, "test.example.com")

	parseEmail(t, database, sampleDiff,
		withSubject("[PATCH 2/3] second"),
		withMsgID("<order-2@test>"),
		withListID("test.example.com"))

	if n := countEvents(t, database, "patch-completed"); n != 0 {
		t.Errorf("after 2/3: patch-completed events = %d, want 0", n)
	}

	parseEmail(t, database, sampleDiff,
		withSubject("[PATCH 1/3] first"),
		withMsgID("<order-1@test>"),
		withInReplyTo("<order-2@test>"),
		withListID("test.example.com"))

	if n := countEvents(t, database, "patch-completed"); n != 2 {
		t.Errorf("after 1/3: patch-completed events = %d, want 2", n)
	}

	parseEmail(t, database, sampleDiff,
		withSubject("[PATCH 3/3] third"),
		withMsgID("<order-3@test>"),
		withInReplyTo("<order-1@test>"),
		withListID("test.example.com"))

	if n := countEvents(t, database, "patch-completed"); n != 3 {
		t.Errorf("after 3/3: patch-completed events = %d, want 3", n)
	}
	if n := countEvents(t, database, "series-completed"); n != 1 {
		t.Errorf("series-completed events = %d, want 1", n)
	}
}

func TestWebhookDelivery(t *testing.T) {
	var mu sync.Mutex
	var received []webhookRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = append(received, webhookRequest{
			event:     r.Header.Get("X-Patchwork-Event"),
			delivery:  r.Header.Get("X-Patchwork-Delivery"),
			signature: r.Header.Get("X-Patchwork-Signature"),
			body:      body,
		})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	database, _ := testDB(t, "test.example.com")

	ctx := context.Background()

	// create a user for the webhook creator FK
	database.NewRaw(`
		INSERT INTO auth_user (username, email, password, is_superuser,
			is_staff, is_active, date_joined, first_name, last_name)
		VALUES ('webhookuser', 'webhook@test', '', false,
			false, true, datetime('now'), '', '')
	`).Exec(ctx)

	// insert a webhook for the test project
	_, err := database.NewRaw(`
		INSERT INTO patchwork_webhook
			(project_id, url, secret, events, active, creator_id, created)
		VALUES (
			(SELECT id FROM patchwork_project LIMIT 1),
			?,
			'test-secret',
			'*',
			true,
			(SELECT id FROM auth_user LIMIT 1),
			datetime('now')
		)
	`, srv.URL).Exec(ctx)
	if err != nil {
		t.Fatalf("insert webhook: %v", err)
	}

	webhook.Init(ctx, 64)

	parseEmail(t, database, sampleDiff,
		withSubject("[PATCH 1/1] webhook test"),
		withMsgID("<webhook-test@test>"),
		withListID("test.example.com"))

	webhook.Shutdown()

	mu.Lock()
	defer mu.Unlock()

	if len(received) == 0 {
		t.Fatal("expected at least 1 webhook delivery")
	}

	// find the patch-created delivery
	var found *webhookRequest
	for i := range received {
		if received[i].event == "patch-created" {
			found = &received[i]
			break
		}
	}
	if found == nil {
		t.Fatal("no patch-created webhook received")
	}

	if found.delivery == "" {
		t.Error("X-Patchwork-Delivery header missing")
	}

	// verify HMAC signature
	mac := hmac.New(sha256.New, []byte("test-secret"))
	mac.Write(found.body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if found.signature != expected {
		t.Errorf("signature = %q, want %q", found.signature, expected)
	}

	// verify payload structure
	var payload map[string]any
	if err := json.Unmarshal(found.body, &payload); err != nil {
		t.Fatalf("invalid JSON payload: %v", err)
	}
	if payload["category"] != "patch-created" {
		t.Errorf("category = %v, want patch-created", payload["category"])
	}
	if payload["project"] == nil {
		t.Error("project field missing from payload")
	}
	if payload["payload"] == nil {
		t.Error("payload field missing from payload")
	}
	inner, ok := payload["payload"].(map[string]any)
	if !ok {
		t.Fatal("payload.payload is not an object")
	}
	if inner["patch"] == nil {
		t.Error("payload.payload.patch missing")
	}
}

type webhookRequest struct {
	event     string
	delivery  string
	signature string
	body      []byte
}
