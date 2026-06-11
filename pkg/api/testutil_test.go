// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func pythonBin() string {
	if p := os.Getenv("PYTHON"); p != "" {
		return p
	}
	return "python3"
}

var templateDB string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "patchwork-api-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mkdtemp: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	templateDB = filepath.Join(dir, "template.db")

	wd, _ := os.Getwd()
	root := filepath.Join(wd, "..", "..")
	python := pythonBin()
	manage := filepath.Join(root, "manage.py")

	env := append(os.Environ(),
		"DJANGO_SETTINGS_MODULE=patchwork.settings.dev",
		"DATABASE_TYPE=sqlite3",
		"DATABASE_NAME="+templateDB,
	)

	cmd := exec.Command(python, manage, "migrate", "--run-syncdb", "-v0")
	cmd.Dir = root
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n%s", err, out)
		os.Exit(1)
	}

	cmd = exec.Command(python, manage, "loaddata", "default_states", "default_tags")
	cmd.Dir = root
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "loaddata: %v\n%s", err, out)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

type testServer struct {
	*httptest.Server
	db *bun.DB
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	src, err := os.ReadFile(templateDB)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath, src, 0o644); err != nil {
		t.Fatal(err)
	}

	u, _ := url.Parse("sqlite://" + dbPath)
	database, err := db.Open(u)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	router := NewRouter(database)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	return &testServer{Server: srv, db: database}
}

func (s *testServer) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	_, err := s.db.NewRaw(query, args...).Exec(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func (s *testServer) insertUser(t *testing.T, username, email string) int32 {
	t.Helper()
	var id int32
	s.db.NewRaw(`
		INSERT INTO auth_user (username, email, password, is_superuser,
			is_staff, is_active, date_joined, first_name, last_name)
		VALUES (?, ?, '', false, false, true, datetime('now'), '', '')
		RETURNING id
	`, username, email).Scan(context.Background(), &id)
	return id
}

func (s *testServer) insertProject(t *testing.T) int32 {
	t.Helper()
	var id int32
	s.db.NewRaw(`
		INSERT INTO patchwork_project (
			linkname, name, listid, listemail, subject_match,
			web_url, scm_url, webscm_url,
			list_archive_url, list_archive_url_format, commit_url_format,
			send_notifications, use_tags, show_dependencies, auto_supersede
		) VALUES ('test-project', 'Test Project', 'test.example.com',
			'test@test.example.com', '', 'http://example.com', '', '',
			'', '', '', false, true, false, false)
		RETURNING id
	`).Scan(context.Background(), &id)
	return id
}

func (s *testServer) insertPerson(t *testing.T, email, name string) int32 {
	t.Helper()
	var id int32
	s.db.NewRaw(`
		INSERT INTO patchwork_person (email, name)
		VALUES (?, ?)
		ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, email, name).Scan(context.Background(), &id)
	return id
}

func (s *testServer) insertPatch(t *testing.T, projectID int32, msgid, name string) int32 {
	t.Helper()
	personID := s.insertPerson(t, "test@example.com", "Test Author")
	var stateID int32
	s.db.NewRaw(`SELECT id FROM patchwork_state WHERE ordering = 0`).
		Scan(context.Background(), &stateID)

	var id int32
	s.db.NewRaw(`
		INSERT INTO patchwork_patch (
			msgid, date, headers, submitter_id, project_id,
			name, archived, state_id, diff, hash
		) VALUES (?, datetime('now'), 'From: test\n', ?, ?, ?, false, ?,
			'--- a/file\n+++ b/file\n@@ -1 +1 @@\n-old\n+new\n', 'abc123')
		RETURNING id
	`, msgid, personID, projectID, name, stateID).Scan(context.Background(), &id)
	return id
}

func (s *testServer) insertCover(t *testing.T, projectID int32, msgid, name string) int32 {
	t.Helper()
	personID := s.insertPerson(t, "test@example.com", "Test Author")
	var id int32
	s.db.NewRaw(`
		INSERT INTO patchwork_cover (
			msgid, date, headers, submitter_id, project_id, name, content
		) VALUES (?, datetime('now'), 'From: test\n', ?, ?, ?, 'cover body')
		RETURNING id
	`, msgid, personID, projectID, name).Scan(context.Background(), &id)
	return id
}

func (s *testServer) insertSeries(t *testing.T, projectID, submitterID int32, name string) int32 {
	t.Helper()
	var id int32
	s.db.NewRaw(`
		INSERT INTO patchwork_series (
			project_id, date, submitter_id, version, total, name
		) VALUES (?, datetime('now'), ?, 1, 2, ?)
		RETURNING id
	`, projectID, submitterID, name).Scan(context.Background(), &id)
	return id
}

func (s *testServer) insertComment(t *testing.T, patchID int32, msgid string) int32 {
	t.Helper()
	personID := s.insertPerson(t, "reviewer@example.com", "Reviewer")
	var id int32
	s.db.NewRaw(`
		INSERT INTO patchwork_patchcomment (
			msgid, date, headers, submitter_id, patch_id, content
		) VALUES (?, datetime('now'), '', ?, ?, 'looks good')
		RETURNING id
	`, msgid, personID, patchID).Scan(context.Background(), &id)
	return id
}

func (s *testServer) get(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(s.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
}

func getList(t *testing.T, s *testServer, path string) []map[string]any {
	t.Helper()
	resp := s.get(t, path)
	if resp.StatusCode != 200 {
		t.Fatalf("%s: status = %d, want 200", path, resp.StatusCode)
	}
	var items []map[string]any
	decodeJSON(t, resp, &items)
	return items
}

func getOne(t *testing.T, s *testServer, path string) map[string]any {
	t.Helper()
	resp := s.get(t, path)
	if resp.StatusCode != 200 {
		t.Fatalf("%s: status = %d, want 200", path, resp.StatusCode)
	}
	var item map[string]any
	decodeJSON(t, resp, &item)
	return item
}

func assertField(t *testing.T, obj map[string]any, key string) {
	t.Helper()
	if _, ok := obj[key]; !ok {
		t.Errorf("missing field %q", key)
	}
}

func assertValue(t *testing.T, obj map[string]any, key string, want any) {
	t.Helper()
	got, ok := obj[key]
	if !ok {
		t.Errorf("missing field %q", key)
		return
	}
	if got != want {
		t.Errorf("%s = %v (%T), want %v (%T)", key, got, got, want, want)
	}
}

func assertContains(t *testing.T, obj map[string]any, key, substr string) {
	t.Helper()
	v, ok := obj[key]
	if !ok {
		t.Errorf("missing field %q", key)
		return
	}
	s, ok := v.(string)
	if !ok {
		t.Errorf("field %q is not a string: %v", key, v)
		return
	}
	if !strings.Contains(s, substr) {
		t.Errorf("%s = %q, want to contain %q", key, s, substr)
	}
}

func assertNested(t *testing.T, obj map[string]any, key, nestedKey string) {
	t.Helper()
	v, ok := obj[key]
	if !ok {
		t.Errorf("missing field %q", key)
		return
	}
	nested, ok := v.(map[string]any)
	if !ok {
		t.Errorf("field %q is not an object", key)
		return
	}
	if _, ok := nested[nestedKey]; !ok {
		t.Errorf("missing nested field %q.%q", key, nestedKey)
	}
}

func (s *testServer) insertToken(t *testing.T, userID int32) string {
	t.Helper()
	token := "test-token-abcdef1234567890abcdef1234567890"
	s.exec(t, `
		INSERT INTO authtoken_token (key, created, user_id)
		VALUES (?, datetime('now'), ?)
	`, token, userID)
	return token
}

func (s *testServer) makeMaintainer(t *testing.T, userID, projectID int32) {
	t.Helper()
	s.exec(t, `
		INSERT INTO patchwork_userprofile (user_id, send_email, items_per_page, show_ids)
		VALUES (?, false, 100, false)
		ON CONFLICT (user_id) DO NOTHING
	`, userID)
	var profileID int32
	s.db.NewRaw(`SELECT id FROM patchwork_userprofile WHERE user_id = ?`, userID).
		Scan(context.Background(), &profileID)
	s.exec(t, `
		INSERT INTO patchwork_userprofile_maintainer_projects (userprofile_id, project_id)
		VALUES (?, ?)
	`, profileID, projectID)
}

func (s *testServer) authRequest(t *testing.T, method, path, token string, body any) *http.Response {
	t.Helper()
	var reqBody *bytes.Buffer
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(data)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}
	req, err := http.NewRequest(method, s.URL+path, reqBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Token "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
