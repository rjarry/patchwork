// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func (h *handler) listUsers(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	p := parsePage(r)

	q := h.db.NewSelect().Model((*db.User)(nil))
	if v := r.URL.Query().Get("q"); v != "" {
		q = q.WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Where("username LIKE ?", "%"+v+"%").
				WhereOr("first_name LIKE ?", "%"+v+"%").
				WhereOr("last_name LIKE ?", "%"+v+"%").
				WhereOr("email LIKE ?", "%"+v+"%")
		})
	}

	total, _ := q.Count(ctx)
	setLinkHeader(w, r, p, total)

	var users []db.User
	q.OrderExpr("id ASC").Offset(p.Offset).Limit(p.PerPage).Scan(ctx, &users)

	base := apiBase(r)
	for i := range users {
		users[i].URL = fmt.Sprintf("%s/users/%d/", base, users[i].ID)
	}

	writeList(w, users)
}

func (h *handler) getUser(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	id, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}

	var user db.User
	if err := h.db.NewSelect().Model(&user).Where("id = ?", id).Scan(ctx); err != nil {
		notFound(w)
		return
	}

	base := apiBase(r)
	user.URL = fmt.Sprintf("%s/users/%d/", base, user.ID)

	writeJSON(w, http.StatusOK, &user)
}

func (h *handler) updateUser(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	id, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}

	caller := getUser(r)
	if caller.ID != id {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"detail": "You do not have permission to perform this action.",
		})
		return
	}

	var user db.User
	if err := h.db.NewSelect().Model(&user).Where("id = ?", id).Scan(ctx); err != nil {
		notFound(w)
		return
	}

	var body struct {
		FirstName *string `json:"first_name"`
		LastName  *string `json:"last_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"detail": "Invalid JSON.",
		})
		return
	}

	q := h.db.NewUpdate().Model(&user).Where("id = ?", id)
	if body.FirstName != nil {
		q = q.Set("first_name = ?", *body.FirstName)
		user.FirstName = *body.FirstName
	}
	if body.LastName != nil {
		q = q.Set("last_name = ?", *body.LastName)
		user.LastName = *body.LastName
	}
	q.Exec(ctx)

	base := apiBase(r)
	user.URL = fmt.Sprintf("%s/users/%d/", base, user.ID)

	writeJSON(w, http.StatusOK, &user)
}
