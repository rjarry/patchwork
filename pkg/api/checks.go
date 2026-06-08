// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func (h *handler) listChecks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	patchID, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}

	p := parsePage(r)
	q := h.db.NewSelect().Model((*db.Check)(nil)).
		Where("patch_id = ?", patchID)

	total, _ := q.Count(ctx)
	setLinkHeader(w, r, p, total)

	var checks []db.Check
	q.OrderExpr("id ASC").Offset(p.Offset).Limit(p.PerPage).Scan(ctx, &checks)

	populateChecks(ctx, r, h.db, patchID, checks)

	writeList(w, checks)
}

var checkStates = map[string]db.CheckState{
	"pending": db.CheckPending,
	"success": db.CheckSuccess,
	"warning": db.CheckWarning,
	"fail":    db.CheckFail,
}

func (h *handler) createCheck(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	patchID, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}

	var patch db.Patch
	if err := h.db.NewSelect().Model(&patch).
		Where("id = ?", patchID).Column("id", "project_id").
		Scan(ctx); err != nil {
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
		State       string `json:"state"`
		TargetURL   string `json:"target_url"`
		Context     string `json:"context"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"detail": "Invalid JSON.",
		})
		return
	}

	stateVal, ok := checkStates[body.State]
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"state": "Invalid state.",
		})
		return
	}

	check := db.Check{
		PatchID:     patchID,
		UserID:      &user.ID,
		Date:        time.Now(),
		State:       stateVal,
		TargetURL:   body.TargetURL,
		Context:     body.Context,
		Description: body.Description,
	}
	_, err := h.db.NewInsert().Model(&check).Exec(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"detail": "Create failed.",
		})
		return
	}

	writeJSON(w, http.StatusCreated, &check)
}

func (h *handler) getCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	patchID, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}
	checkID, ok := pathID(r, "checkID")
	if !ok {
		notFound(w)
		return
	}

	var c db.Check
	err := h.db.NewSelect().Model(&c).
		Where("id = ?", checkID).
		Where("patch_id = ?", patchID).
		Scan(ctx)
	if err != nil {
		notFound(w)
		return
	}

	checks := []db.Check{c}
	populateChecks(ctx, r, h.db, patchID, checks)

	writeJSON(w, http.StatusOK, &checks[0])
}
