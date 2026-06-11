// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func (h *webHandler) coverDetailPage(w http.ResponseWriter, r *http.Request) {
	linkname := chi.URLParam(r, "linkname")
	rawMsgid, _ := url.PathUnescape(chi.URLParam(r, "msgid"))
	ctx := r.Context()
	msgid := "<" + rawMsgid + ">"

	var project db.Project
	err := h.db.NewSelect().Model(&project).
		Where("linkname = ?", linkname).
		Scan(ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	var cover db.Cover
	err = h.db.NewSelect().Model(&cover).
		Where("project_id = ?", project.ID).
		Where("msgid = ?", msgid).
		Scan(ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	var submitter db.Person
	if h.db.NewSelect().Model(&submitter).Where("id = ?", cover.SubmitterID).Scan(ctx) == nil {
		cover.Submitter = &submitter
	}

	var series *db.Series
	if cover.ID > 0 {
		var s db.Series
		if h.db.NewSelect().Model(&s).Where("cover_letter_id = ?", cover.ID).Scan(ctx) == nil {
			series = &s
		}
	}

	var seriesPatches []seriesPatchRef
	metadata := make(map[string]string)

	if series != nil {
		var sPatches []db.Patch
		h.db.NewSelect().Model(&sPatches).
			Column("id", "msgid", "name").
			Where("series_id = ?", series.ID).
			OrderExpr(`"number" ASC`).
			Scan(ctx)
		for _, sp := range sPatches {
			seriesPatches = append(seriesPatches, seriesPatchRef{
				Name: sp.Name,
				URL:  patchURL(project.Linkname, sp.Msgid),
			})
		}

		var rows []db.SeriesMetadata
		h.db.NewSelect().Model(&rows).
			Where("series_id = ?", series.ID).
			Scan(ctx)
		for _, r := range rows {
			metadata[r.Key] = r.Value
		}
	}

	var comments []db.CoverComment
	h.db.NewSelect().Model(&comments).
		Where("cover_id = ?", cover.ID).
		OrderExpr("date ASC").
		Scan(ctx)
	if len(comments) > 0 {
		var submitterIDs []int32
		for i := range comments {
			submitterIDs = append(submitterIDs, comments[i].SubmitterID)
		}
		var people []db.Person
		h.db.NewSelect().Model(&people).
			Where("id IN ?", bun.Tuple(submitterIDs)).
			Scan(ctx)
		pm := make(map[int32]*db.Person)
		for i := range people {
			pm[people[i].ID] = &people[i]
		}
		for i := range comments {
			comments[i].Submitter = pm[comments[i].SubmitterID]
		}
	}

	data := coverDetailData{
		PC:       h.pageCtx(r),
		Project:  project,
		Cover:    cover,
		Comments: comments,
		Series:   series,
		Patches:  seriesPatches,
		Metadata: metadata,
	}
	coverDetailPage(data).Render(ctx, w)
}

func (h *webHandler) coverRedirect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		notFoundPage(w)
		return
	}

	var cover struct {
		Msgid     string
		ProjectID int32
	}
	err = h.db.NewRaw(`SELECT msgid, project_id FROM patchwork_cover WHERE id = ?`, id).
		Scan(ctx, &cover)
	if err != nil {
		notFoundPage(w)
		return
	}

	var linkname string
	h.db.NewRaw(`SELECT linkname FROM patchwork_project WHERE id = ?`, cover.ProjectID).
		Scan(ctx, &linkname)

	http.Redirect(w, r, coverURL(linkname, cover.Msgid), http.StatusMovedPermanently)
}

func (h *webHandler) coverMboxByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		notFoundPage(w)
		return
	}

	var cover db.Cover
	err = h.db.NewSelect().Model(&cover).Where("id = ?", id).Scan(ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	var project db.Project
	h.db.NewSelect().Model(&project).Where("id = ?", cover.ProjectID).Scan(ctx)

	h.serveCoverMbox(w, cover, project)
}
