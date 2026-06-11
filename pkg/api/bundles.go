// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func (h *handler) listBundles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p := parsePage(r)

	q := h.db.NewSelect().Model((*db.Bundle)(nil))
	if user := getUser(r); user != nil {
		q = q.Where("public = ? OR owner_id = ?", true, user.ID)
	} else {
		q = q.Where("public = ?", true)
	}
	q = applyBundleFilters(q, r.URL.Query())

	total, _ := q.Count(ctx)
	setLinkHeader(w, r, p, total)

	var bundles []db.Bundle
	q.OrderExpr("id ASC").Offset(p.Offset).Limit(p.PerPage).Scan(ctx, &bundles)

	populateBundles(ctx, h.db, bundles)
	setBundleURLs(r, bundles)

	writeList(w, bundles)
}

func (h *handler) getBundle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}

	var bundle db.Bundle
	if err := h.db.NewSelect().Model(&bundle).Where("id = ?", id).Scan(ctx); err != nil {
		notFound(w)
		return
	}

	bundles := []db.Bundle{bundle}
	populateBundles(ctx, h.db, bundles)
	setBundleURLs(r, bundles)
	loadBundlePatches(ctx, h.db, bundles)

	writeJSON(w, http.StatusOK, &bundles[0])
}

func (h *handler) createBundle(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	user := getUser(r)

	var body struct {
		Name    string  `json:"name"`
		Public  bool    `json:"public"`
		Patches []int32 `json:"patches"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"detail": "Invalid JSON.",
		})
		return
	}

	if len(body.Patches) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"patches": []string{"Bundles cannot be empty."},
		})
		return
	}

	projectID, err := validateBundlePatches(ctx, h.db, body.Patches)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"patches": []string{err.Error()},
		})
		return
	}

	bundle := db.Bundle{
		OwnerID:   user.ID,
		ProjectID: projectID,
		Name:      body.Name,
		Public:    body.Public,
	}
	if err := db.Insert(ctx, h.db, &bundle); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"detail": "Create failed.",
		})
		return
	}

	insertBundlePatches(ctx, h.db, bundle.ID, body.Patches)

	bundles := []db.Bundle{bundle}
	populateBundles(ctx, h.db, bundles)
	setBundleURLs(r, bundles)
	loadBundlePatches(ctx, h.db, bundles)

	writeJSON(w, http.StatusCreated, &bundles[0])
}

func (h *handler) updateBundle(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	user := getUser(r)

	id, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}

	var bundle db.Bundle
	if err := h.db.NewSelect().Model(&bundle).Where("id = ?", id).Scan(ctx); err != nil {
		notFound(w)
		return
	}

	if bundle.OwnerID != user.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"detail": "You do not have permission to perform this action.",
		})
		return
	}

	var body struct {
		Name    *string  `json:"name"`
		Public  *bool    `json:"public"`
		Patches *[]int32 `json:"patches"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"detail": "Invalid JSON.",
		})
		return
	}

	if body.Patches != nil {
		if len(*body.Patches) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"patches": []string{"Bundles cannot be empty."},
			})
			return
		}
		projectID, err := validateBundlePatches(ctx, h.db, *body.Patches)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"patches": []string{err.Error()},
			})
			return
		}
		h.db.NewDelete().Model((*db.BundlePatch)(nil)).
			Where("bundle_id = ?", id).Exec(ctx)
		insertBundlePatches(ctx, h.db, id, *body.Patches)
		bundle.ProjectID = projectID
	}

	q := h.db.NewUpdate().Model(&bundle).Where("id = ?", id)
	if body.Name != nil {
		q = q.Set("name = ?", *body.Name)
	}
	if body.Public != nil {
		q = q.Set("public = ?", *body.Public)
	}
	if body.Patches != nil {
		q = q.Set("project_id = ?", bundle.ProjectID)
	}
	q.Exec(ctx)

	h.db.NewSelect().Model(&bundle).Where("id = ?", id).Scan(ctx)

	bundles := []db.Bundle{bundle}
	populateBundles(ctx, h.db, bundles)
	setBundleURLs(r, bundles)
	loadBundlePatches(ctx, h.db, bundles)

	writeJSON(w, http.StatusOK, &bundles[0])
}

func (h *handler) deleteBundle(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	user := getUser(r)

	id, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}

	var bundle db.Bundle
	if err := h.db.NewSelect().Model(&bundle).Where("id = ?", id).Scan(ctx); err != nil {
		notFound(w)
		return
	}

	if bundle.OwnerID != user.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"detail": "You do not have permission to perform this action.",
		})
		return
	}

	h.db.NewDelete().Model((*db.BundlePatch)(nil)).
		Where("bundle_id = ?", id).Exec(ctx)
	h.db.NewDelete().Model((*db.Bundle)(nil)).
		Where("id = ?", id).Exec(ctx)

	w.WriteHeader(http.StatusNoContent)
}

func validateBundlePatches(ctx context.Context, database *bun.DB, patchIDs []int32) (int32, error) {
	var projectIDs []int32
	database.NewSelect().Model((*db.Patch)(nil)).
		Column("project_id").
		Where("id IN (?)", bun.In(patchIDs)).
		Scan(ctx, &projectIDs)

	if len(projectIDs) == 0 {
		return 0, fmt.Errorf("Invalid patch IDs.")
	}

	projectID := projectIDs[0]
	for _, pid := range projectIDs[1:] {
		if pid != projectID {
			return 0, fmt.Errorf("Bundle patches must belong to the same project.")
		}
	}

	return projectID, nil
}

func insertBundlePatches(ctx context.Context, database *bun.DB, bundleID int32, patchIDs []int32) {
	for i, pid := range patchIDs {
		bp := db.BundlePatch{
			BundleID: bundleID,
			PatchID:  pid,
			Order:    int32(i),
		}
		db.Insert(ctx, database, &bp)
	}
}

func populateBundles(ctx context.Context, database *bun.DB, bundles []db.Bundle) {
	if len(bundles) == 0 {
		return
	}
	var ownerIDs, projectIDs []int32
	for i := range bundles {
		ownerIDs = append(ownerIDs, bundles[i].OwnerID)
		projectIDs = append(projectIDs, bundles[i].ProjectID)
	}
	users := loadUsers(ctx, database, ownerIDs)
	projects := loadProjects(ctx, database, projectIDs)
	for i := range bundles {
		bundles[i].Owner = users[bundles[i].OwnerID]
		bundles[i].Project = projects[bundles[i].ProjectID]
	}
}

func applyBundleFilters(q *bun.SelectQuery, params map[string][]string) *bun.SelectQuery {
	if v, ok := params["project"]; ok && len(v) > 0 {
		if id, err := strconv.Atoi(v[0]); err == nil {
			q = q.Where("project_id = ?", id)
		} else {
			q = q.Where("project_id IN (SELECT id FROM patchwork_project WHERE linkname = ?)", v[0])
		}
	}
	if v, ok := params["owner"]; ok && len(v) > 0 {
		if id, err := strconv.Atoi(v[0]); err == nil {
			q = q.Where("owner_id = ?", id)
		} else {
			q = q.Where("owner_id IN (SELECT id FROM auth_user WHERE username = ?)", v[0])
		}
	}
	if v, ok := params["public"]; ok && len(v) > 0 {
		q = q.Where("public = ?", v[0] == "true" || v[0] == "1")
	}
	return q
}
