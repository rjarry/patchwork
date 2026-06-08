// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
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

func (h *handler) listSeries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p := parsePage(r)

	q := h.db.NewSelect().Model((*db.Series)(nil))

	q = applySeriesFilters(q, r.URL.Query())

	total, _ := q.Count(ctx)
	setLinkHeader(w, r, p, total)

	var series []db.Series
	q.OrderExpr("id DESC").Offset(p.Offset).Limit(p.PerPage).Scan(ctx, &series)

	populateSeries(ctx, h.db, series)
	setSeriesURLs(r, series)
	loadSeriesDetail(ctx, r, h.db, series)

	writeList(w, series)
}

func (h *handler) getSeries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}

	var s db.Series
	if err := h.db.NewSelect().Model(&s).Where("id = ?", id).Scan(ctx); err != nil {
		notFound(w)
		return
	}

	series := []db.Series{s}
	populateSeries(ctx, h.db, series)
	setSeriesURLs(r, series)
	loadSeriesDetail(ctx, r, h.db, series)

	writeJSON(w, http.StatusOK, &series[0])
}

func (h *handler) updateSeries(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	id, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}

	var s db.Series
	if err := h.db.NewSelect().Model(&s).Where("id = ?", id).Scan(ctx); err != nil {
		notFound(w)
		return
	}

	var body struct {
		Version  *int32             `json:"version"`
		Metadata *map[string]string `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"detail": "Invalid JSON.",
		})
		return
	}

	if body.Version != nil {
		h.db.NewUpdate().Model(&s).
			Set("version = ?", *body.Version).
			Where("id = ?", id).Exec(ctx)
		s.Version = *body.Version
	}

	if body.Metadata != nil {
		h.db.NewDelete().Model((*db.SeriesMetadata)(nil)).
			Where("series_id = ?", id).Exec(ctx)
		for k, v := range *body.Metadata {
			h.db.NewInsert().Model(&db.SeriesMetadata{
				SeriesID: id, Key: k, Value: v,
			}).Exec(ctx)
		}
	}

	series := []db.Series{s}
	populateSeries(ctx, h.db, series)
	setSeriesURLs(r, series)
	loadSeriesDetail(ctx, r, h.db, series)

	writeJSON(w, http.StatusOK, &series[0])
}

func populateSeries(ctx context.Context, database *bun.DB, series []db.Series) {
	if len(series) == 0 {
		return
	}

	var submitterIDs, projectIDs []int32
	for i := range series {
		submitterIDs = append(submitterIDs, series[i].SubmitterID)
		if series[i].ProjectID != nil {
			projectIDs = append(projectIDs, *series[i].ProjectID)
		}
	}

	persons := loadPersons(ctx, database, submitterIDs)
	projects := loadProjects(ctx, database, projectIDs)

	for i := range series {
		series[i].Submitter = persons[series[i].SubmitterID]
		if series[i].ProjectID != nil {
			series[i].Project = projects[*series[i].ProjectID]
		}
	}
}

func applySeriesFilters(q *bun.SelectQuery, params map[string][]string) *bun.SelectQuery {
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
	if v, ok := params["since"]; ok && len(v) > 0 {
		q = q.Where("date >= ?", v[0])
	}
	if v, ok := params["before"]; ok && len(v) > 0 {
		q = q.Where("date < ?", v[0])
	}
	if v, ok := params["metadata_key"]; ok && len(v) > 0 {
		q = q.Where("id IN (SELECT series_id FROM patchwork_seriesmetadata WHERE key = ?)", v[0])
	}
	if v, ok := params["metadata_value"]; ok && len(v) > 0 {
		q = q.Where("id IN (SELECT series_id FROM patchwork_seriesmetadata WHERE value = ?)", v[0])
	}
	if v, ok := params["q"]; ok && len(v) > 0 {
		q = q.Where("name LIKE ?", "%"+v[0]+"%")
	}
	return q
}
