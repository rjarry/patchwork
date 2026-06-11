// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func (h *handler) listPatchComments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	patchID, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}

	p := parsePage(r)
	q := h.db.NewSelect().Model((*db.PatchComment)(nil)).
		Where("patch_id = ?", patchID)

	total, _ := q.Count(ctx)
	setLinkHeader(w, r, p, total)

	var comments []db.PatchComment
	q.OrderExpr("id ASC").Offset(p.Offset).Limit(p.PerPage).Scan(ctx, &comments)

	populatePatchComments(ctx, h.db, comments)
	populateCommentURLs(r, patchID, comments)

	writeList(w, comments)
}

func (h *handler) getPatchComment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	patchID, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}
	commentID, ok := pathID(r, "commentID")
	if !ok {
		notFound(w)
		return
	}

	var c db.PatchComment
	err := h.db.NewSelect().Model(&c).
		Where("id = ?", commentID).
		Where("patch_id = ?", patchID).
		Scan(ctx)
	if err != nil {
		notFound(w)
		return
	}

	comments := []db.PatchComment{c}
	populatePatchComments(ctx, h.db, comments)
	populateCommentURLs(r, patchID, comments)

	writeJSON(w, http.StatusOK, &comments[0])
}

func (h *handler) updatePatchComment(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	patchID, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}
	commentID, ok := pathID(r, "commentID")
	if !ok {
		notFound(w)
		return
	}

	var c db.PatchComment
	err := h.db.NewSelect().Model(&c).
		Where("patch_comment.id = ?", commentID).
		Where("patch_id = ?", patchID).
		Scan(ctx)
	if err != nil {
		notFound(w)
		return
	}

	var body struct {
		Addressed *bool `json:"addressed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"detail": "Invalid JSON.",
		})
		return
	}

	if body.Addressed != nil {
		h.db.NewUpdate().Model((*db.PatchComment)(nil)).
			Set("addressed = ?", *body.Addressed).
			Where("id = ?", commentID).
			Exec(ctx)
		c.Addressed = body.Addressed
	}

	comments := []db.PatchComment{c}
	populatePatchComments(ctx, h.db, comments)

	writeJSON(w, http.StatusOK, &comments[0])
}

func (h *handler) listCoverComments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	coverID, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}

	p := parsePage(r)
	q := h.db.NewSelect().Model((*db.CoverComment)(nil)).
		Where("cover_id = ?", coverID)

	total, _ := q.Count(ctx)
	setLinkHeader(w, r, p, total)

	var comments []db.CoverComment
	q.OrderExpr("id ASC").Offset(p.Offset).Limit(p.PerPage).Scan(ctx, &comments)

	populateCoverComments(ctx, h.db, comments)
	populateCoverCommentURLs(r, coverID, comments)

	writeList(w, comments)
}

func (h *handler) getCoverComment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	coverID, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}
	commentID, ok := pathID(r, "commentID")
	if !ok {
		notFound(w)
		return
	}

	var c db.CoverComment
	err := h.db.NewSelect().Model(&c).
		Where("id = ?", commentID).
		Where("cover_id = ?", coverID).
		Scan(ctx)
	if err != nil {
		notFound(w)
		return
	}

	comments := []db.CoverComment{c}
	populateCoverComments(ctx, h.db, comments)
	populateCoverCommentURLs(r, coverID, comments)

	writeJSON(w, http.StatusOK, &comments[0])
}

func (h *handler) updateCoverComment(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	coverID, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}
	commentID, ok := pathID(r, "commentID")
	if !ok {
		notFound(w)
		return
	}

	var c db.CoverComment
	err := h.db.NewSelect().Model(&c).
		Where("cover_comment.id = ?", commentID).
		Where("cover_id = ?", coverID).
		Scan(ctx)
	if err != nil {
		notFound(w)
		return
	}

	var body struct {
		Addressed *bool `json:"addressed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"detail": "Invalid JSON.",
		})
		return
	}

	if body.Addressed != nil {
		h.db.NewUpdate().Model((*db.CoverComment)(nil)).
			Set("addressed = ?", *body.Addressed).
			Where("id = ?", commentID).
			Exec(ctx)
		c.Addressed = body.Addressed
	}

	comments := []db.CoverComment{c}
	populateCoverComments(ctx, h.db, comments)
	populateCoverCommentURLs(r, coverID, comments)

	writeJSON(w, http.StatusOK, &comments[0])
}

func populatePatchComments(ctx context.Context, database *bun.DB, comments []db.PatchComment) {
	if len(comments) == 0 {
		return
	}
	var ids []int32
	for i := range comments {
		ids = append(ids, comments[i].SubmitterID)
	}
	persons := loadPersons(ctx, database, ids)
	for i := range comments {
		comments[i].Submitter = persons[comments[i].SubmitterID]
	}
}

func populateCoverComments(ctx context.Context, database *bun.DB, comments []db.CoverComment) {
	if len(comments) == 0 {
		return
	}
	var ids []int32
	for i := range comments {
		ids = append(ids, comments[i].SubmitterID)
	}
	persons := loadPersons(ctx, database, ids)
	for i := range comments {
		comments[i].Submitter = persons[comments[i].SubmitterID]
	}
}
