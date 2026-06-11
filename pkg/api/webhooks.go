// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func (h *handler) listWebhooks(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	projID, ok := pathID(r, "projectID")
	if !ok {
		notFound(w)
		return
	}

	user := getUser(r)
	if !isMaintainer(h.db, ctx, user.ID, projID) {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"detail": "You do not have permission to perform this action.",
		})
		return
	}

	p := parsePage(r)
	var hooks []db.Webhook
	h.db.NewSelect().Model(&hooks).
		Where("project_id = ?", projID).
		OrderExpr("id ASC").Offset(p.Offset).Limit(p.PerPage).
		Scan(ctx)

	writeList(w, hooks)
}

func (h *handler) getWebhook(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	projID, ok := pathID(r, "projectID")
	if !ok {
		notFound(w)
		return
	}
	hookID, ok := pathID(r, "webhookID")
	if !ok {
		notFound(w)
		return
	}

	user := getUser(r)
	if !isMaintainer(h.db, ctx, user.ID, projID) {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"detail": "You do not have permission to perform this action.",
		})
		return
	}

	var hook db.Webhook
	err := h.db.NewSelect().Model(&hook).
		Where("id = ?", hookID).
		Where("project_id = ?", projID).
		Scan(ctx)
	if err != nil {
		notFound(w)
		return
	}

	writeJSON(w, http.StatusOK, &hook)
}

var validEventCategories = map[string]bool{
	"cover-created": true, "patch-created": true,
	"patch-completed": true, "patch-state-changed": true,
	"patch-delegated": true, "patch-relation-changed": true,
	"check-created": true, "series-created": true,
	"series-completed": true, "cover-comment-created": true,
	"patch-comment-created": true,
}

func (h *handler) createWebhook(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	projID, ok := pathID(r, "projectID")
	if !ok {
		notFound(w)
		return
	}

	user := getUser(r)
	if !isMaintainer(h.db, ctx, user.ID, projID) {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"detail": "You do not have permission to perform this action.",
		})
		return
	}

	var body struct {
		URL    string `json:"url"`
		Secret string `json:"secret"`
		Events string `json:"events"`
		Active *bool  `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"detail": "Invalid JSON.",
		})
		return
	}

	if body.Events != "" && body.Events != "*" {
		for _, e := range strings.Split(body.Events, ",") {
			if !validEventCategories[strings.TrimSpace(e)] {
				writeJSON(w, http.StatusBadRequest, map[string]string{
					"events": "Invalid event category.",
				})
				return
			}
		}
	}

	active := true
	if body.Active != nil {
		active = *body.Active
	}
	events := body.Events
	if events == "" {
		events = "*"
	}

	hook := db.Webhook{
		ProjectID: projID,
		URL:       body.URL,
		Secret:    body.Secret,
		Events:    events,
		Active:    active,
		CreatorID: user.ID,
		Created:   time.Now(),
	}
	err := db.Insert(ctx, h.db, &hook)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"detail": "Create failed.",
		})
		return
	}

	writeJSON(w, http.StatusCreated, &hook)
}

func (h *handler) updateWebhook(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	projID, ok := pathID(r, "projectID")
	if !ok {
		notFound(w)
		return
	}
	hookID, ok := pathID(r, "webhookID")
	if !ok {
		notFound(w)
		return
	}

	user := getUser(r)
	if !isMaintainer(h.db, ctx, user.ID, projID) {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"detail": "You do not have permission to perform this action.",
		})
		return
	}

	var hook db.Webhook
	err := h.db.NewSelect().Model(&hook).
		Where("id = ?", hookID).
		Where("project_id = ?", projID).
		Scan(ctx)
	if err != nil {
		notFound(w)
		return
	}

	var body struct {
		URL    *string `json:"url"`
		Secret *string `json:"secret"`
		Events *string `json:"events"`
		Active *bool   `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"detail": "Invalid JSON.",
		})
		return
	}

	q := h.db.NewUpdate().Model(&hook).Where("id = ?", hookID)
	if body.URL != nil {
		q = q.Set("url = ?", *body.URL)
	}
	if body.Secret != nil {
		q = q.Set("secret = ?", *body.Secret)
	}
	if body.Events != nil {
		q = q.Set("events = ?", *body.Events)
	}
	if body.Active != nil {
		q = q.Set("active = ?", *body.Active)
	}
	q.Exec(ctx)

	h.db.NewSelect().Model(&hook).Where("id = ?", hookID).Scan(ctx)

	writeJSON(w, http.StatusOK, &hook)
}

func (h *handler) deleteWebhook(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	projID, ok := pathID(r, "projectID")
	if !ok {
		notFound(w)
		return
	}
	hookID, ok := pathID(r, "webhookID")
	if !ok {
		notFound(w)
		return
	}

	user := getUser(r)
	if !isMaintainer(h.db, ctx, user.ID, projID) {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"detail": "You do not have permission to perform this action.",
		})
		return
	}

	h.db.NewDelete().Model((*db.Webhook)(nil)).
		Where("id = ?", hookID).
		Where("project_id = ?", projID).
		Exec(ctx)

	w.WriteHeader(http.StatusNoContent)
}
