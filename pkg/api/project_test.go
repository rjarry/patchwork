// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"fmt"
	"testing"
)

func TestPatchFilterProject(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertPatch(t, projID, "<fp@test>", "filtered")

	items := getList(t, s, fmt.Sprintf("/api/patches/?project=%d", projID))
	if len(items) != 1 {
		t.Errorf("got %d, want 1", len(items))
	}
	items = getList(t, s, "/api/patches/?project=99999")
	if len(items) != 0 {
		t.Errorf("got %d, want 0", len(items))
	}
}

func TestProjectCreate405(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "POST", "/api/projects/", "", map[string]string{"name": "x"})
	if resp.StatusCode != 405 {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestProjectDelete405(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "pdel", "pdel@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "DELETE",
		fmt.Sprintf("/api/projects/%d", projID), token, nil)
	if resp.StatusCode != 405 {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestProjectDetailByID(t *testing.T) {
	s := newTestServer(t)
	id := s.insertProject(t)
	p := getOne(t, s, fmt.Sprintf("/api/projects/%d", id))
	if p["name"] != "Test Project" {
		t.Errorf("name = %v", p["name"])
	}
}

func TestProjectDetailByLinkname(t *testing.T) {
	s := newTestServer(t)
	s.insertProject(t)
	p := getOne(t, s, "/api/projects/test-project")
	if p["link_name"] != "test-project" {
		t.Errorf("link_name = %v", p["link_name"])
	}
}

func TestProjectDetailByNumericLinkname(t *testing.T) {
	s := newTestServer(t)
	s.exec(t, `
		INSERT INTO patchwork_project (
			linkname, name, listid, listemail, subject_match,
			web_url, scm_url, webscm_url,
			list_archive_url, list_archive_url_format, commit_url_format,
			send_notifications, use_tags, show_dependencies, auto_supersede
		) VALUES ('12345', 'Numeric Project', 'num.example.com',
			'num@example.com', '', '', '', '', '', '', '',
			false, true, false, false)
	`)

	p := getOne(t, s, "/api/projects/12345")
	if p["link_name"] != "12345" {
		t.Errorf("link_name = %v, want 12345", p["link_name"])
	}
}

// --- Bundle with data ---

func TestProjectList(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	items := getList(t, s, "/api/projects/")
	if len(items) != 1 {
		t.Fatalf("got %d, want 1", len(items))
	}
	p := items[0]
	assertValue(t, p, "id", float64(projID))
	assertValue(t, p, "name", "Test Project")
	assertValue(t, p, "link_name", "test-project")
	assertValue(t, p, "list_id", "test.example.com")
	assertValue(t, p, "list_email", "test@test.example.com")
	assertContains(t, p, "url", fmt.Sprintf("/projects/%d/", projID))
	assertField(t, p, "web_url")
	assertField(t, p, "maintainers")

	maintainers := p["maintainers"].([]any)
	if len(maintainers) != 0 {
		t.Errorf("maintainers should be empty, got %d", len(maintainers))
	}
}

func TestProjectListEmpty(t *testing.T) {
	s := newTestServer(t)
	items := getList(t, s, "/api/projects/")
	if len(items) != 0 {
		t.Errorf("got %d, want 0", len(items))
	}
}

func TestProjectMaintainers(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "maintainer", "maint@test")
	s.makeMaintainer(t, userID, projID)

	p := getOne(t, s, fmt.Sprintf("/api/projects/%d", projID))
	maintainers, ok := p["maintainers"].([]any)
	if !ok {
		t.Fatal("maintainers should be an array")
	}
	if len(maintainers) != 1 {
		t.Errorf("maintainers: got %d, want 1", len(maintainers))
	}
}

// --- Series metadata filter ---

func TestProjectNotFound(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/projects/nonexistent")
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// --- Patches ---

func TestProjectUpdateAnonymous(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/projects/%d", projID), "",
		map[string]string{"web_url": "https://hack.com"})
	if resp.StatusCode != 403 {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

// --- Series update ---

func TestProjectUpdateMaintainer(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "pmaint", "pm@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/projects/%d", projID), token,
		map[string]string{"web_url": "https://updated.example.com"})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var proj map[string]any
	decodeJSON(t, resp, &proj)
	if proj["web_url"] != "https://updated.example.com" {
		t.Errorf("web_url = %v", proj["web_url"])
	}
}

func TestProjectUpdateReadonlyField(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "pro", "pro@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/projects/%d", projID), token,
		map[string]string{"link_name": "hacked"})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var proj map[string]any
	decodeJSON(t, resp, &proj)
	if proj["link_name"] == "hacked" {
		t.Error("link_name should not be writable")
	}
}

// --- Webhook edge cases ---
