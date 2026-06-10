// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func (h *webHandler) projectList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var projects []db.Project
	h.db.NewSelect().Model(&projects).
		OrderExpr("name ASC").
		Scan(ctx)
	if len(projects) == 1 {
		http.Redirect(w, r,
			"/project/"+projects[0].Linkname+"/list/",
			http.StatusFound)
		return
	}
	projectListPage(projects).Render(ctx, w)
}

func (h *webHandler) projectDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	linkname := chi.URLParam(r, "linkname")

	var project db.Project
	err := h.db.NewSelect().Model(&project).
		Where("linkname = ?", linkname).
		Scan(ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	var maintainers []db.User
	h.db.NewRaw(`
		SELECT u.* FROM auth_user u
		JOIN patchwork_userprofile up ON up.user_id = u.id
		JOIN patchwork_userprofile_maintainer_projects mp ON mp.userprofile_id = up.id
		WHERE mp.project_id = ?
	`, project.ID).Scan(ctx, &maintainers)

	var nPatches int
	nPatches, _ = h.db.NewSelect().Model((*db.Patch)(nil)).
		Where("project_id = ?", project.ID).
		Where("archived = ?", false).
		Count(ctx)

	projectDetailPage(project, maintainers, nPatches).Render(ctx, w)
}
