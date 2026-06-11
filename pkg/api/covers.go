// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

var coverListColumns = []string{
	"id", "msgid", "date", "submitter_id", "project_id", "name",
}

func (h *handler) listCovers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p := parsePage(r)

	q := h.db.NewSelect().Model((*db.Cover)(nil)).
		Column(coverListColumns...)

	q = applyCoverFilters(q, r.URL.Query())

	total, _ := q.Count(ctx)
	setLinkHeader(w, r, p, total)

	var covers []db.Cover
	q.OrderExpr("id DESC").Offset(p.Offset).Limit(p.PerPage).Scan(ctx, &covers)

	populateCovers(ctx, h.db, covers)
	setCoverURLs(r, covers)
	loadCoverSeries(ctx, h.db, covers)

	writeList(w, covers)
}

func (h *handler) getCover(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}

	var cover db.Cover
	if err := h.db.NewSelect().Model(&cover).Where("id = ?", id).Scan(ctx); err != nil {
		notFound(w)
		return
	}

	covers := []db.Cover{cover}
	populateCovers(ctx, h.db, covers)
	setCoverURLs(r, covers)
	loadCoverSeries(ctx, h.db, covers)

	writeJSON(w, http.StatusOK, &covers[0])
}

func populateCovers(ctx context.Context, database *bun.DB, covers []db.Cover) {
	if len(covers) == 0 {
		return
	}

	var submitterIDs, projectIDs []int32
	for i := range covers {
		submitterIDs = append(submitterIDs, covers[i].SubmitterID)
		projectIDs = append(projectIDs, covers[i].ProjectID)
	}

	persons := loadPersons(ctx, database, submitterIDs)
	projects := loadProjects(ctx, database, projectIDs)

	for i := range covers {
		covers[i].Submitter = persons[covers[i].SubmitterID]
		covers[i].Project = projects[covers[i].ProjectID]
	}
}

func applyCoverFilters(q *bun.SelectQuery, params map[string][]string) *bun.SelectQuery {
	if v, ok := params["project"]; ok && len(v) > 0 {
		if id, err := strconv.Atoi(v[0]); err == nil {
			q = q.Where("project_id = ?", id)
		} else {
			q = q.Where("project_id IN (SELECT id FROM patchwork_project WHERE linkname = ?)", v[0])
		}
	}
	if v, ok := params["submitter"]; ok && len(v) > 0 {
		if id, err := strconv.Atoi(v[0]); err == nil {
			q = q.Where("submitter_id = ?", id)
		}
	}
	if v, ok := params["series"]; ok && len(v) > 0 {
		if id, err := strconv.Atoi(v[0]); err == nil {
			q = q.Where("id IN (SELECT cover_letter_id FROM patchwork_series WHERE id = ?)", id)
		}
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
