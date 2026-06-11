// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

type contextKey int

const userKey contextKey = iota

func getUser(r *http.Request) *db.User {
	u, _ := r.Context().Value(userKey).(*db.User)
	return u
}

func (h *handler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if token, ok := strings.CutPrefix(auth, "Token "); ok {
			token = strings.TrimSpace(token)
			var user db.User
			err := h.db.NewSelect().Model(&user).
				Where("id = (SELECT user_id FROM authtoken_token WHERE key = ?)", token).
				Scan(r.Context())
			if err == nil {
				ctx := context.WithValue(r.Context(), userKey, &user)
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isMaintainer(database *bun.DB, ctx context.Context, userID, projectID int32) bool {
	var count int
	database.NewRaw(`
		SELECT count(*) FROM patchwork_userprofile_maintainer_projects mp
		JOIN patchwork_userprofile up ON up.id = mp.userprofile_id
		WHERE up.user_id = ? AND mp.project_id = ?
	`, userID, projectID).Scan(ctx, &count)
	return count > 0
}

func requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if getUser(r) == nil {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"detail": "Authentication credentials were not provided.",
		})
		return false
	}
	return true
}
