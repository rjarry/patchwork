// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"net/http"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func (h *handler) listPeople(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p := parsePage(r)

	q := h.db.NewSelect().Model((*db.Person)(nil))

	if v := r.URL.Query().Get("q"); v != "" {
		q = q.WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("name LIKE ?", "%"+v+"%").
				WhereOr("email LIKE ?", "%"+v+"%")
		})
	}

	total, _ := q.Count(ctx)
	setLinkHeader(w, r, p, total)

	var people []db.Person
	q.OrderExpr("id ASC").Offset(p.Offset).Limit(p.PerPage).Scan(ctx, &people)

	populatePersonUsers(ctx, h.db, people)
	setPersonURLs(r, people)

	writeList(w, people)
}

func (h *handler) getPerson(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}

	var person db.Person
	if err := h.db.NewSelect().Model(&person).Where("id = ?", id).Scan(ctx); err != nil {
		notFound(w)
		return
	}

	people := []db.Person{person}
	populatePersonUsers(ctx, h.db, people)
	setPersonURLs(r, people)

	writeJSON(w, http.StatusOK, &people[0])
}
