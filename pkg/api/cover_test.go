// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"fmt"
	"testing"
)

func TestCoverCreate405(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "POST", "/api/covers/", "", map[string]string{"name": "x"})
	if resp.StatusCode != 405 {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestCoverDelete405(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "DELETE", "/api/covers/1", "", nil)
	if resp.StatusCode != 405 {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestCoverDetail(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	coverID := s.insertCover(t, projID, "<cd@test>", "cover detail")
	c := getOne(t, s, fmt.Sprintf("/api/covers/%d", coverID))
	assertField(t, c, "content")
	assertField(t, c, "headers")
}

// --- Series ---

func TestCoverFilterMsgid(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertCover(t, projID, "<cm-filter@test>", "c")

	items := getList(t, s, "/api/covers/?msgid=cm-filter@test")
	if len(items) != 1 {
		t.Errorf("got %d, want 1", len(items))
	}
	items = getList(t, s, "/api/covers/?msgid=nope@test")
	if len(items) != 0 {
		t.Errorf("got %d, want 0", len(items))
	}
}

func TestCoverFilterProject(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertCover(t, projID, "<cf@test>", "c")

	items := getList(t, s, fmt.Sprintf("/api/covers/?project=%d", projID))
	if len(items) != 1 {
		t.Errorf("got %d, want 1", len(items))
	}
	items = getList(t, s, "/api/covers/?project=99999")
	if len(items) != 0 {
		t.Errorf("got %d, want 0", len(items))
	}
}

func TestCoverFilterSubmitter(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertCover(t, projID, "<cs@test>", "c")
	// insertCover creates person "test@example.com"
	personID := s.insertPerson(t, "test@example.com", "Test Author")

	items := getList(t, s, fmt.Sprintf("/api/covers/?submitter=%d", personID))
	if len(items) != 1 {
		t.Errorf("got %d, want 1", len(items))
	}
}

func TestCoverList(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	coverID := s.insertCover(t, projID, "<c1@test>", "test cover")
	items := getList(t, s, "/api/covers/")
	if len(items) != 1 {
		t.Fatalf("got %d, want 1", len(items))
	}
	c := items[0]
	assertValue(t, c, "id", float64(coverID))
	assertValue(t, c, "name", "test cover")
	assertValue(t, c, "msgid", "<c1@test>")
	assertContains(t, c, "url", fmt.Sprintf("/covers/%d/", coverID))
	assertContains(t, c, "mbox", fmt.Sprintf("/covers/%d/mbox/", coverID))
	assertContains(t, c, "comments", fmt.Sprintf("/covers/%d/comments/", coverID))
	assertField(t, c, "date")
	assertField(t, c, "series")
	assertNested(t, c, "submitter", "id")
	assertNested(t, c, "submitter", "email")
	assertNested(t, c, "project", "id")
}

func TestCoverListEmpty(t *testing.T) {
	s := newTestServer(t)
	items := getList(t, s, "/api/covers/")
	if len(items) != 0 {
		t.Errorf("got %d, want 0", len(items))
	}
}

func TestCoverNotFound(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/covers/99999")
	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// --- Series ---

func TestEventsFilterCover(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	coverID := s.insertCover(t, projID, "<efc@test>", "c")
	s.exec(t, `INSERT INTO patchwork_event (project_id, category, date, cover_id)
		VALUES (?, 'cover-created', datetime('now'), ?)`, projID, coverID)

	items := getList(t, s, fmt.Sprintf("/api/events/?cover=%d", coverID))
	if len(items) != 1 {
		t.Errorf("got %d, want 1", len(items))
	}
}
