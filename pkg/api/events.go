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

func (h *handler) listEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p := parsePage(r)

	q := h.db.NewSelect().Model((*db.Event)(nil))
	q = applyEventFilters(q, r.URL.Query())

	total, _ := q.Count(ctx)
	setLinkHeader(w, r, p, total)

	order := "date DESC"
	if v := r.URL.Query().Get("order"); v == "date" {
		order = "date ASC"
	} else if v == "-date" {
		order = "date DESC"
	}

	var events []db.Event
	q.OrderExpr(order).Offset(p.Offset).Limit(p.PerPage).Scan(ctx, &events)

	if len(events) > 0 {
		var projectIDs, actorIDs []int32
		for i := range events {
			projectIDs = append(projectIDs, events[i].ProjectID)
			if events[i].ActorID != nil {
				actorIDs = append(actorIDs, *events[i].ActorID)
			}
		}
		projects := loadProjects(ctx, h.db, projectIDs)
		actors := loadUsers(ctx, h.db, actorIDs)
		for i := range events {
			events[i].Project = projects[events[i].ProjectID]
			if events[i].ActorID != nil {
				events[i].Actor = actors[*events[i].ActorID]
			}
			events[i].Payload = buildEventPayload(ctx, h.db, &events[i])
		}
	}

	writeList(w, events)
}

func buildEventPayload(ctx context.Context, database *bun.DB, e *db.Event) map[string]any {
	m := map[string]any{}

	if e.PatchID != nil {
		var p db.Patch
		if err := database.NewSelect().Model(&p).Where("id = ?", *e.PatchID).Scan(ctx); err == nil {
			m["patch"] = map[string]any{"id": p.ID, "msgid": p.Msgid, "date": p.Date, "name": p.Name}
		}
	}
	if e.SeriesID != nil {
		var s db.Series
		if err := database.NewSelect().Model(&s).Where("id = ?", *e.SeriesID).Scan(ctx); err == nil {
			m["series"] = map[string]any{"id": s.ID, "name": s.Name, "date": s.Date, "version": s.Version}
		}
	}
	if e.CoverID != nil {
		var c db.Cover
		if err := database.NewSelect().Model(&c).Where("id = ?", *e.CoverID).Scan(ctx); err == nil {
			m["cover"] = map[string]any{"id": c.ID, "msgid": c.Msgid, "date": c.Date, "name": c.Name}
		}
	}
	if e.PatchCommentID != nil {
		var c db.PatchComment
		if err := database.NewSelect().Model(&c).Where("id = ?", *e.PatchCommentID).Scan(ctx); err == nil {
			m["comment"] = map[string]any{"id": c.ID, "msgid": c.Msgid, "date": c.Date}
		}
	}
	if e.CoverCommentID != nil {
		var c db.CoverComment
		if err := database.NewSelect().Model(&c).Where("id = ?", *e.CoverCommentID).Scan(ctx); err == nil {
			m["comment"] = map[string]any{"id": c.ID, "msgid": c.Msgid, "date": c.Date}
		}
	}
	if e.PreviousStateID != nil {
		var s db.State
		if err := database.NewSelect().Model(&s).Where("id = ?", *e.PreviousStateID).Scan(ctx); err == nil {
			m["previous_state"] = s.Slug
		}
	}
	if e.CurrentStateID != nil {
		var s db.State
		if err := database.NewSelect().Model(&s).Where("id = ?", *e.CurrentStateID).Scan(ctx); err == nil {
			m["current_state"] = s.Slug
		}
	}

	if len(m) == 0 {
		return nil
	}
	return m
}

func applyEventFilters(q *bun.SelectQuery, params map[string][]string) *bun.SelectQuery {
	if v, ok := params["project"]; ok && len(v) > 0 {
		if id, err := strconv.Atoi(v[0]); err == nil {
			q = q.Where("project_id = ?", id)
		} else {
			q = q.Where("project_id IN (SELECT id FROM patchwork_project WHERE linkname = ?)", v[0])
		}
	}
	if v, ok := params["category"]; ok && len(v) > 0 {
		q = q.Where("category = ?", v[0])
	}
	if v, ok := params["patch"]; ok && len(v) > 0 {
		if id, err := strconv.Atoi(v[0]); err == nil {
			q = q.Where("patch_id = ?", id)
		}
	}
	if v, ok := params["cover"]; ok && len(v) > 0 {
		if id, err := strconv.Atoi(v[0]); err == nil {
			q = q.Where("cover_id = ?", id)
		}
	}
	if v, ok := params["series"]; ok && len(v) > 0 {
		if id, err := strconv.Atoi(v[0]); err == nil {
			q = q.Where("series_id = ?", id)
		}
	}
	if v, ok := params["actor"]; ok && len(v) > 0 {
		if id, err := strconv.Atoi(v[0]); err == nil {
			q = q.Where("actor_id = ?", id)
		}
	}
	if v, ok := params["since"]; ok && len(v) > 0 {
		q = q.Where("date >= ?", v[0])
	}
	if v, ok := params["before"]; ok && len(v) > 0 {
		q = q.Where("date < ?", v[0])
	}
	return q
}
