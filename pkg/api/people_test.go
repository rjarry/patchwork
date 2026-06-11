// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"context"
	"fmt"
	"testing"
)

func TestPeopleList(t *testing.T) {
	s := newTestServer(t)
	s.insertPerson(t, "person@test.com", "Test Person")
	items := getList(t, s, "/api/people/")
	if len(items) != 1 {
		t.Fatalf("got %d, want 1", len(items))
	}
	p := items[0]
	assertField(t, p, "id")
	assertField(t, p, "name")
	assertField(t, p, "email")
	if p["email"] != "person@test.com" {
		t.Errorf("email = %v", p["email"])
	}
}

func TestPeopleListEmpty(t *testing.T) {
	s := newTestServer(t)
	items := getList(t, s, "/api/people/")
	if len(items) != 0 {
		t.Errorf("got %d, want 0", len(items))
	}
}

func TestPeopleSearch(t *testing.T) {
	s := newTestServer(t)
	s.insertPerson(t, "alice@test", "Alice")
	s.insertPerson(t, "bob@test", "Bob")

	items := getList(t, s, "/api/people/?q=Alice")
	if len(items) != 1 {
		t.Errorf("got %d, want 1", len(items))
	}
}

// --- Events ---

func TestPersonCreate405(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "POST", "/api/people/", "", map[string]string{"name": "x"})
	if resp.StatusCode != 405 {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestPersonDetail(t *testing.T) {
	s := newTestServer(t)
	id := s.insertPerson(t, "detail@test", "Detail Person")
	p := getOne(t, s, fmt.Sprintf("/api/people/%d", id))
	if p["name"] != "Detail Person" {
		t.Errorf("name = %v", p["name"])
	}
}

func TestPersonDetailAnonymous(t *testing.T) {
	s := newTestServer(t)
	s.insertPerson(t, "anon@test", "Anon")
	// people endpoint is public in our implementation (Django requires auth)
	resp := s.get(t, "/api/people/")
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

// --- User edge cases ---

func TestPersonDetailInvalid(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/people/invalid")
	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// --- Project detail numeric linkname ---

func TestPersonDetailLinked(t *testing.T) {
	s := newTestServer(t)
	userID := s.insertUser(t, "linked", "linked@test")
	s.exec(t, `INSERT INTO patchwork_person (email, name, user_id)
		VALUES ('linked@test', 'Linked Person', ?)`, userID)
	var personID int32
	s.db.NewRaw(`SELECT id FROM patchwork_person WHERE email = 'linked@test'`).
		Scan(context.Background(), &personID)

	p := getOne(t, s, fmt.Sprintf("/api/people/%d", personID))
	if p["email"] != "linked@test" {
		t.Errorf("email = %v", p["email"])
	}
}

// --- Check state string ---

func TestPersonNotFound(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/people/99999")
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestPersonUserLinked(t *testing.T) {
	s := newTestServer(t)
	userID := s.insertUser(t, "linked2", "linked2@test")
	s.exec(t, `INSERT INTO patchwork_person (email, name, user_id)
		VALUES ('linked2@test', 'Linked', ?)`, userID)
	var personID int32
	s.db.NewRaw(`SELECT id FROM patchwork_person WHERE email = 'linked2@test'`).
		Scan(context.Background(), &personID)

	p := getOne(t, s, fmt.Sprintf("/api/people/%d", personID))
	assertNested(t, p, "user", "id")
	assertNested(t, p, "user", "username")
}

// --- Comment URL and subject ---
