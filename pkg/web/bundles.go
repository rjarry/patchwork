// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

type bundleListData struct {
	PC      pageContext
	Bundles []db.Bundle
}

func (h *webHandler) bundleList(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	user := getWebUser(r)
	pc := h.pageCtx(r)

	var bundles []db.Bundle
	h.db.NewSelect().Model(&bundles).
		ColumnExpr("*, (SELECT count(*) FROM patchwork_bundlepatch WHERE bundle_id = bundle.id) AS patch_count").
		Where("owner_id = ?", user.ID).
		OrderExpr("name ASC").
		Scan(ctx)

	populateWebBundles(ctx, h.db, bundles)

	bundleListPage(bundleListData{PC: pc, Bundles: bundles}).Render(ctx, w)
}

func (h *webHandler) projectBundleList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pc := h.pageCtx(r)
	linkname := chi.URLParam(r, "linkname")

	var project db.Project
	err := h.db.NewSelect().Model(&project).
		Where("linkname = ?", linkname).Scan(ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	q := h.db.NewSelect().Model((*db.Bundle)(nil)).
		ColumnExpr("*, (SELECT count(*) FROM patchwork_bundlepatch WHERE bundle_id = bundle.id) AS patch_count").
		Where("project_id = ?", project.ID).
		OrderExpr("name ASC")

	user := getWebUser(r)
	if user != nil {
		q = q.Where("(public = ? OR owner_id = ?)", true, user.ID)
	} else {
		q = q.Where("public = ?", true)
	}

	var bundles []db.Bundle
	q.Scan(ctx, &bundles)

	populateWebBundles(ctx, h.db, bundles)

	bundleListPage(bundleListData{PC: pc, Bundles: bundles}).Render(ctx, w)
}

type bundleDetailData struct {
	PC      pageContext
	Bundle  db.Bundle
	Patches []db.Patch
	IsOwner bool
}

func (h *webHandler) bundleDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := chi.URLParam(r, "username")
	bundlename := chi.URLParam(r, "bundlename")
	pc := h.pageCtx(r)
	user := getWebUser(r)

	var bundle db.Bundle
	err := h.db.NewRaw(`
		SELECT b.* FROM patchwork_bundle b
		JOIN auth_user u ON u.id = b.owner_id
		WHERE u.username = ? AND b.name = ?
	`, username, bundlename).Scan(ctx, &bundle)
	if err != nil {
		notFoundPage(w)
		return
	}

	isOwner := user != nil && user.ID == bundle.OwnerID
	if !bundle.Public && !isOwner {
		notFoundPage(w)
		return
	}

	var patches []db.Patch
	h.db.NewRaw(`
		SELECT p.* FROM patchwork_patch p
		JOIN patchwork_bundlepatch bp ON bp.patch_id = p.id
		WHERE bp.bundle_id = ?
		ORDER BY bp."order" ASC
	`, bundle.ID).Scan(ctx, &patches)

	populateWebPatches(ctx, h.db, patches)

	// load bundle owner and project
	bundles := []db.Bundle{bundle}
	populateWebBundles(ctx, h.db, bundles)
	bundle = bundles[0]

	data := bundleDetailData{
		PC:      pc,
		Bundle:  bundle,
		Patches: patches,
		IsOwner: isOwner,
	}
	bundleDetailPage(data).Render(ctx, w)
}

func (h *webHandler) bundleUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	user := getWebUser(r)
	username := chi.URLParam(r, "username")
	bundlename := chi.URLParam(r, "bundlename")

	if !h.validateCSRF(r) {
		http.Redirect(w, r, fmt.Sprintf("/bundle/%s/%s/", username, bundlename), http.StatusFound)
		return
	}

	var bundle db.Bundle
	err := h.db.NewRaw(`
		SELECT b.* FROM patchwork_bundle b
		JOIN auth_user u ON u.id = b.owner_id
		WHERE u.username = ? AND b.name = ?
	`, username, bundlename).Scan(ctx, &bundle)
	if err != nil || bundle.OwnerID != user.ID {
		notFoundPage(w)
		return
	}

	r.ParseForm()
	action := r.FormValue("action")

	switch action {
	case "delete":
		h.db.NewDelete().Model((*db.BundlePatch)(nil)).
			Where("bundle_id = ?", bundle.ID).Exec(ctx)
		h.db.NewDelete().Model((*db.Bundle)(nil)).
			Where("id = ?", bundle.ID).Exec(ctx)
		http.Redirect(w, r, "/user/bundles/", http.StatusFound)

	case "update":
		newName := strings.TrimSpace(r.FormValue("name"))
		public := r.FormValue("public") == "on"
		if newName != "" {
			h.db.NewUpdate().Model(&bundle).
				Where("id = ?", bundle.ID).
				Set("name = ?", newName).
				Set("public = ?", public).
				Exec(ctx)
			http.Redirect(w, r, fmt.Sprintf("/bundle/%s/%s/", username, newName), http.StatusFound)
		} else {
			http.Redirect(w, r, fmt.Sprintf("/bundle/%s/%s/", username, bundlename), http.StatusFound)
		}

	case "remove-patches":
		for _, idStr := range r.Form["patch_id"] {
			patchID, err := strconv.ParseInt(idStr, 10, 32)
			if err != nil {
				continue
			}
			h.db.NewDelete().Model((*db.BundlePatch)(nil)).
				Where("bundle_id = ?", bundle.ID).
				Where("patch_id = ?", patchID).
				Exec(ctx)
		}
		http.Redirect(w, r, fmt.Sprintf("/bundle/%s/%s/", username, bundlename), http.StatusFound)

	default:
		http.Redirect(w, r, fmt.Sprintf("/bundle/%s/%s/", username, bundlename), http.StatusFound)
	}
}

func populateWebBundles(ctx context.Context, database *bun.DB, bundles []db.Bundle) {
	if len(bundles) == 0 {
		return
	}
	var ownerIDs, projectIDs []int32
	for i := range bundles {
		ownerIDs = append(ownerIDs, bundles[i].OwnerID)
		projectIDs = append(projectIDs, bundles[i].ProjectID)
	}

	users := make(map[int32]*db.User)
	var us []db.User
	database.NewSelect().Model(&us).
		Where("id IN ?", bun.Tuple(ownerIDs)).Scan(ctx)
	for i := range us {
		users[us[i].ID] = &us[i]
	}

	projects := make(map[int32]*db.Project)
	var ps []db.Project
	database.NewSelect().Model(&ps).
		Where("id IN ?", bun.Tuple(projectIDs)).Scan(ctx)
	for i := range ps {
		projects[ps[i].ID] = &ps[i]
	}

	for i := range bundles {
		bundles[i].Owner = users[bundles[i].OwnerID]
		bundles[i].Project = projects[bundles[i].ProjectID]
	}
}
