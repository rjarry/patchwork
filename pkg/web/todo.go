// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/getpatchwork/patchwork/pkg/db"
)

type todoProject struct {
	Project  db.Project
	NPatches int
}

func (h *webHandler) todoLists(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	user := getWebUser(r)
	pc := h.pageCtx(r)

	var projects []db.Project
	h.db.NewSelect().Model(&projects).
		OrderExpr("name ASC").
		Scan(ctx)

	var todos []todoProject
	for _, p := range projects {
		count, _ := h.db.NewSelect().Model((*db.Patch)(nil)).
			Where("project_id = ?", p.ID).
			Where("archived = ?", false).
			Where("delegate_id = ?", user.ID).
			Where("state_id IN (SELECT id FROM patchwork_state WHERE action_required = ?)", true).
			Count(ctx)
		if count > 0 {
			todos = append(todos, todoProject{Project: p, NPatches: count})
		}
	}

	if len(todos) == 1 {
		http.Redirect(w, r,
			"/user/todo/"+todos[0].Project.Linkname+"/",
			http.StatusFound)
		return
	}

	todoListsPage(pc, todos).Render(ctx, w)
}

func (h *webHandler) todoList(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	user := getWebUser(r)
	pc := h.pageCtx(r)
	linkname := chi.URLParam(r, "linkname")

	var project db.Project
	err := h.db.NewSelect().Model(&project).
		Where("linkname = ?", linkname).
		Scan(ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	var patches []db.Patch
	h.db.NewSelect().Model(&patches).
		Where("project_id = ?", project.ID).
		Where("archived = ?", false).
		Where("delegate_id = ?", user.ID).
		Where("state_id IN (SELECT id FROM patchwork_state WHERE action_required = ?)", true).
		OrderExpr("date DESC").
		Scan(ctx)

	populateWebPatches(ctx, h.db, patches)

	todoListPage(pc, project, patches).Render(ctx, w)
}
