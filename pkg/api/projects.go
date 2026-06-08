// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func (h *handler) listProjects(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p := parsePage(r)

	q := h.db.NewSelect().Model((*db.Project)(nil))

	if v := r.URL.Query().Get("q"); v != "" {
		q = q.Where("name LIKE ?", "%"+v+"%")
	}

	total, _ := q.Count(ctx)
	setLinkHeader(w, r, p, total)

	var projects []db.Project
	q.OrderExpr("id ASC").Offset(p.Offset).Limit(p.PerPage).Scan(ctx, &projects)
	setProjectURLs(r, projects)
	loadProjectMaintainers(ctx, h.db, projects)

	writeList(w, projects)
}

func (h *handler) getProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pk := chi.URLParam(r, "pk")

	var project db.Project

	// try by ID first, fall back to linkname
	if id, err := strconv.ParseInt(pk, 10, 32); err == nil {
		err = h.db.NewSelect().Model(&project).
			Where("id = ?", id).Scan(ctx)
		if err != nil {
			err = h.db.NewSelect().Model(&project).
				Where("linkname = ?", pk).Scan(ctx)
		}
		if err != nil {
			notFound(w)
			return
		}
	} else {
		if err := h.db.NewSelect().Model(&project).
			Where("linkname = ?", pk).Scan(ctx); err != nil {
			notFound(w)
			return
		}
	}

	projects := []db.Project{project}
	setProjectURLs(r, projects)
	loadProjectMaintainers(ctx, h.db, projects)

	writeJSON(w, http.StatusOK, &projects[0])
}

func (h *handler) updateProject(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	pk := chi.URLParam(r, "pk")

	var project db.Project
	if id, err := strconv.ParseInt(pk, 10, 32); err == nil {
		h.db.NewSelect().Model(&project).Where("id = ?", id).Scan(ctx)
	} else {
		h.db.NewSelect().Model(&project).Where("linkname = ?", pk).Scan(ctx)
	}
	if project.ID == 0 {
		notFound(w)
		return
	}

	user := getUser(r)
	if !isMaintainer(h.db, ctx, user.ID, project.ID) {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"detail": "You do not have permission to perform this action.",
		})
		return
	}

	var body struct {
		WebURL               *string `json:"web_url"`
		ScmURL               *string `json:"scm_url"`
		WebScmURL            *string `json:"webscm_url"`
		ListArchiveURL       *string `json:"list_archive_url"`
		ListArchiveURLFormat *string `json:"list_archive_url_format"`
		CommitURLFormat      *string `json:"commit_url_format"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"detail": "Invalid JSON.",
		})
		return
	}

	q := h.db.NewUpdate().Model(&project).Where("id = ?", project.ID)
	if body.WebURL != nil {
		q = q.Set("web_url = ?", *body.WebURL)
	}
	if body.ScmURL != nil {
		q = q.Set("scm_url = ?", *body.ScmURL)
	}
	if body.WebScmURL != nil {
		q = q.Set("webscm_url = ?", *body.WebScmURL)
	}
	if body.ListArchiveURL != nil {
		q = q.Set("list_archive_url = ?", *body.ListArchiveURL)
	}
	if body.ListArchiveURLFormat != nil {
		q = q.Set("list_archive_url_format = ?", *body.ListArchiveURLFormat)
	}
	if body.CommitURLFormat != nil {
		q = q.Set("commit_url_format = ?", *body.CommitURLFormat)
	}
	q.Exec(ctx)

	h.db.NewSelect().Model(&project).Where("id = ?", project.ID).Scan(ctx)
	projects := []db.Project{project}
	setProjectURLs(r, projects)
	loadProjectMaintainers(ctx, h.db, projects)

	writeJSON(w, http.StatusOK, &projects[0])
}
