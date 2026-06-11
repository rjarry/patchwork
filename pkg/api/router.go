// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/uptrace/bun"
)

func NewRouter(database *bun.DB) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.StripSlashes)
	r.Use(middleware.SetHeader("Content-Type", "application/json"))

	h := &handler{db: database}
	r.Use(h.authMiddleware)

	mount := func(r chi.Router) {
		r.Get("/", h.index)

		r.Get("/users", h.listUsers)
		r.Get("/users/{id}", h.getUser)
		r.Patch("/users/{id}", h.updateUser)

		r.Get("/projects", h.listProjects)
		r.Get("/projects/{pk}", h.getProject)
		r.Patch("/projects/{pk}", h.updateProject)
		r.Get("/projects/{projectID}/webhooks", h.listWebhooks)
		r.Post("/projects/{projectID}/webhooks", h.createWebhook)
		r.Get("/projects/{projectID}/webhooks/{webhookID}", h.getWebhook)
		r.Patch("/projects/{projectID}/webhooks/{webhookID}", h.updateWebhook)
		r.Delete("/projects/{projectID}/webhooks/{webhookID}", h.deleteWebhook)

		r.Get("/patches", h.listPatches)
		r.Get("/patches/{id}", h.getPatch)
		r.Patch("/patches/{id}", h.updatePatch)
		r.Put("/patches/{id}", h.updatePatch)
		r.Get("/patches/{id}/checks", h.listChecks)
		r.Post("/patches/{id}/checks", h.createCheck)
		r.Get("/patches/{id}/checks/{checkID}", h.getCheck)
		r.Get("/patches/{id}/comments", h.listPatchComments)
		r.Get("/patches/{id}/comments/{commentID}", h.getPatchComment)
		r.Patch("/patches/{id}/comments/{commentID}", h.updatePatchComment)

		r.Get("/covers", h.listCovers)
		r.Get("/covers/{id}", h.getCover)
		r.Get("/covers/{id}/comments", h.listCoverComments)
		r.Get("/covers/{id}/comments/{commentID}", h.getCoverComment)
		r.Patch("/covers/{id}/comments/{commentID}", h.updateCoverComment)

		r.Get("/series", h.listSeries)
		r.Get("/series/{id}", h.getSeries)
		r.Patch("/series/{id}", h.updateSeries)

		r.Get("/people", h.listPeople)
		r.Get("/people/{id}", h.getPerson)

		r.Get("/events", h.listEvents)

		r.Get("/bundles", h.listBundles)
		r.Post("/bundles", h.createBundle)
		r.Get("/bundles/{id}", h.getBundle)
		r.Patch("/bundles/{id}", h.updateBundle)
		r.Put("/bundles/{id}", h.updateBundle)
		r.Delete("/bundles/{id}", h.deleteBundle)
	}

	r.Route("/api", func(r chi.Router) {
		r.Route("/1.5", mount)
		r.Group(mount)
	})

	return r
}

type handler struct {
	db *bun.DB
}

func (h *handler) index(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"patches":  "/api/1.5/patches/",
		"covers":   "/api/1.5/covers/",
		"series":   "/api/1.5/series/",
		"projects": "/api/1.5/projects/",
		"people":   "/api/1.5/people/",
		"users":    "/api/1.5/users/",
		"events":   "/api/1.5/events/",
		"bundles":  "/api/1.5/bundles/",
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeList[T any](w http.ResponseWriter, items []T) {
	if items == nil {
		items = []T{}
	}
	writeJSON(w, http.StatusOK, items)
}

func notFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, map[string]string{
		"detail": "Not found.",
	})
}

func pathID(r *http.Request, param string) (int32, bool) {
	v := chi.URLParam(r, param)
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return 0, false
	}
	return int32(n), true
}
