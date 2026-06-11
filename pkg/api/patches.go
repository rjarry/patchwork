// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

var patchListColumns = []string{
	"id", "msgid", "date", "submitter_id", "project_id",
	"name", "commit_ref", "pull_url", "state_id", "delegate_id",
	"archived", "hash", "series_id", "number",
}

func (h *handler) listPatches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p := parsePage(r)

	q := h.db.NewSelect().Model((*db.Patch)(nil)).
		Column(patchListColumns...)

	q = applyPatchFilters(q, r.URL.Query())

	total, _ := q.Count(ctx)
	setLinkHeader(w, r, p, total)

	var patches []db.Patch
	q.OrderExpr("id DESC").Offset(p.Offset).Limit(p.PerPage).Scan(ctx, &patches)

	populatePatches(ctx, h.db, patches)
	setPatchURLs(r, patches)
	loadPatchTags(ctx, h.db, patches)
	loadPatchSeries(ctx, h.db, patches)
	loadCombinedCheck(ctx, h.db, patches)
	loadPatchRelated(ctx, h.db, patches)

	writeList(w, patches)
}

func (h *handler) getPatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}

	var patch db.Patch
	if err := h.db.NewSelect().Model(&patch).Where("id = ?", id).Scan(ctx); err != nil {
		notFound(w)
		return
	}

	patches := []db.Patch{patch}
	populatePatches(ctx, h.db, patches)
	setPatchURLs(r, patches)
	loadPatchTags(ctx, h.db, patches)
	loadPatchSeries(ctx, h.db, patches)
	loadCombinedCheck(ctx, h.db, patches)
	loadPatchRelated(ctx, h.db, patches)

	writeJSON(w, http.StatusOK, &patches[0])
}

func (h *handler) updatePatch(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	id, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}

	var patch db.Patch
	if err := h.db.NewSelect().Model(&patch).Where("id = ?", id).Scan(ctx); err != nil {
		notFound(w)
		return
	}

	user := getUser(r)
	if !isMaintainer(h.db, ctx, user.ID, patch.ProjectID) {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"detail": "You do not have permission to perform this action.",
		})
		return
	}

	var body struct {
		State     *string  `json:"state"`
		Delegate  *int32   `json:"delegate"`
		Archived  *bool    `json:"archived"`
		CommitRef *string  `json:"commit_ref"`
		PullURL   *string  `json:"pull_url"`
		Related   *[]int32 `json:"related"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"detail": "Invalid JSON.",
		})
		return
	}

	q := h.db.NewUpdate().Model(&patch).Where("id = ?", id)
	changed := false

	if body.State != nil {
		var state db.State
		err := h.db.NewSelect().Model(&state).
			Where("LOWER(name) = LOWER(?)", *body.State).
			Scan(ctx)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"state": "Invalid state.",
			})
			return
		}
		q = q.Set("state_id = ?", state.ID)
		patch.StateID = &state.ID
		changed = true
	}
	if body.Delegate != nil {
		q = q.Set("delegate_id = ?", *body.Delegate)
		patch.DelegateID = body.Delegate
		changed = true
	}
	if body.Archived != nil {
		q = q.Set("archived = ?", *body.Archived)
		patch.Archived = *body.Archived
		changed = true
	}
	if body.CommitRef != nil {
		q = q.Set("commit_ref = ?", *body.CommitRef)
		patch.CommitRef = body.CommitRef
		changed = true
	}
	if body.PullURL != nil {
		q = q.Set("pull_url = ?", *body.PullURL)
		patch.PullURL = body.PullURL
		changed = true
	}

	if changed {
		if _, err := q.Exec(ctx); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"detail": "Update failed.",
			})
			return
		}
	}

	if body.Related != nil {
		if err := updateRelated(ctx, h.db, r, user, &patch, *body.Related); err != nil {
			status := http.StatusBadRequest
			if err.Error() == "forbidden" {
				status = http.StatusForbidden
			} else if err.Error() == "conflict" {
				status = http.StatusConflict
			}
			writeJSON(w, status, map[string]string{"detail": err.Error()})
			return
		}
	}

	patches := []db.Patch{patch}
	populatePatches(ctx, h.db, patches)
	setPatchURLs(r, patches)
	loadPatchTags(ctx, h.db, patches)
	loadPatchSeries(ctx, h.db, patches)
	loadCombinedCheck(ctx, h.db, patches)
	loadPatchRelated(ctx, h.db, patches)

	writeJSON(w, http.StatusOK, &patches[0])
}

func populatePatches(ctx context.Context, database *bun.DB, patches []db.Patch) {
	if len(patches) == 0 {
		return
	}

	var submitterIDs, projectIDs, stateIDs, delegateIDs []int32
	for i := range patches {
		submitterIDs = append(submitterIDs, patches[i].SubmitterID)
		projectIDs = append(projectIDs, patches[i].ProjectID)
		if patches[i].StateID != nil {
			stateIDs = append(stateIDs, *patches[i].StateID)
		}
		if patches[i].DelegateID != nil {
			delegateIDs = append(delegateIDs, *patches[i].DelegateID)
		}
	}

	persons := loadPersons(ctx, database, submitterIDs)
	projects := loadProjects(ctx, database, projectIDs)
	states := loadStates(ctx, database, stateIDs)
	users := loadUsers(ctx, database, delegateIDs)

	for i := range patches {
		patches[i].Submitter = persons[patches[i].SubmitterID]
		patches[i].Project = projects[patches[i].ProjectID]
		if patches[i].StateID != nil {
			patches[i].State = states[*patches[i].StateID]
		}
		if patches[i].DelegateID != nil {
			patches[i].Delegate = users[*patches[i].DelegateID]
		}
	}
}

func applyPatchFilters(q *bun.SelectQuery, params map[string][]string) *bun.SelectQuery {
	if v, ok := params["project"]; ok && len(v) > 0 {
		if id, err := strconv.Atoi(v[0]); err == nil {
			q = q.Where("project_id = ?", id)
		} else {
			q = q.Where("project_id IN (SELECT id FROM patchwork_project WHERE linkname = ?)", v[0])
		}
	}
	if v, ok := params["series"]; ok && len(v) > 0 {
		if id, err := strconv.Atoi(v[0]); err == nil {
			q = q.Where("series_id = ?", id)
		}
	}
	if v, ok := params["submitter"]; ok && len(v) > 0 {
		if id, err := strconv.Atoi(v[0]); err == nil {
			q = q.Where("submitter_id = ?", id)
		} else {
			q = q.Where("submitter_id IN (SELECT id FROM patchwork_person WHERE email = ?)", v[0])
		}
	}
	if v, ok := params["delegate"]; ok && len(v) > 0 {
		if id, err := strconv.Atoi(v[0]); err == nil {
			q = q.Where("delegate_id = ?", id)
		} else {
			q = q.Where("delegate_id IN (SELECT id FROM auth_user WHERE username = ?)", v[0])
		}
	}
	if v, ok := params["state"]; ok && len(v) > 0 {
		if id, err := strconv.Atoi(v[0]); err == nil {
			q = q.Where("state_id = ?", id)
		} else {
			q = q.Where("state_id IN (SELECT id FROM patchwork_state WHERE LOWER(name) = LOWER(?))", v[0])
		}
	}
	if v, ok := params["archived"]; ok && len(v) > 0 {
		q = q.Where("archived = ?", v[0] == "true" || v[0] == "1")
	}
	if v, ok := params["hash"]; ok && len(v) > 0 {
		q = q.Where("LOWER(hash) = LOWER(?)", v[0])
	}
	if v, ok := params["msgid"]; ok && len(v) > 0 {
		q = q.Where("msgid = ?", "<"+v[0]+">")
	}
	if v, ok := params["since"]; ok && len(v) > 0 {
		q = q.Where("date >= ?", v[0])
	}
	if v, ok := params["before"]; ok && len(v) > 0 {
		q = q.Where("date < ?", v[0])
	}
	if v, ok := params["q"]; ok && len(v) > 0 {
		q = q.Where("name LIKE ?", "%"+v[0]+"%")
	}
	return q
}
