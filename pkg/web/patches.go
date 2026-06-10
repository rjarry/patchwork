// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func (h *webHandler) patchList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	linkname := chi.URLParam(r, "linkname")

	var project db.Project
	err := h.db.NewSelect().Model(&project).
		Where("linkname = ?", linkname).
		Scan(ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	params := r.URL.Query()
	page, _ := strconv.Atoi(params.Get("page"))
	if page < 1 {
		page = 1
	}
	perPage := 200

	q := h.db.NewSelect().Model((*db.Patch)(nil)).
		Column("id", "msgid", "date", "submitter_id", "project_id",
			"name", "state_id", "delegate_id", "archived", "series_id").
		Where("project_id = ?", project.ID)

	var filters []appliedFilter
	baseQuery := fmt.Sprintf("/project/%s/list/", linkname)

	q, filters = applyWebFilters(q, params, baseQuery)

	sort := params.Get("order")
	if sort == "" {
		sort = "-date"
	}
	q = applySort(q, sort)

	total, _ := q.Count(ctx)
	totalPages := (total + perPage - 1) / perPage
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}

	var patches []db.Patch
	q.Offset((page - 1) * perPage).Limit(perPage).Scan(ctx, &patches)

	populateWebPatches(ctx, h.db, patches)

	// load tag abbreviations for column headers
	var tagAbbrevs []string
	h.db.NewRaw(`SELECT abbrev FROM patchwork_tag WHERE show_column = ? ORDER BY id`, true).
		Scan(ctx, &tagAbbrevs)

	// load series names
	seriesNames := make(map[int32]string)
	var seriesIDs []int32
	for _, p := range patches {
		if p.SeriesID != nil {
			seriesIDs = append(seriesIDs, *p.SeriesID)
		}
	}
	if len(seriesIDs) > 0 {
		type nameRow struct {
			ID   int32   `bun:"id"`
			Name *string `bun:"name"`
		}
		var rows []nameRow
		h.db.NewSelect().Model((*db.Series)(nil)).
			Column("id", "name").
			Where("id IN ?", bun.Tuple(seriesIDs)).
			Scan(ctx, &rows)
		for _, r := range rows {
			if r.Name != nil {
				seriesNames[r.ID] = *r.Name
			}
		}
	}

	// build base query string for pagination links
	qp := url.Values{}
	if sort != "-date" {
		qp.Set("order", sort)
	}
	for k, v := range params {
		if k != "page" && k != "order" {
			qp[k] = v
		}
	}
	bq := ""
	if len(qp) > 0 {
		bq = "?" + qp.Encode()
	}

	data := patchListData{
		Project:     project,
		Patches:     patches,
		Filters:     filters,
		Sort:        sort,
		Page:        page,
		PerPage:     perPage,
		Total:       total,
		TotalPages:  totalPages,
		BaseQuery:   bq,
		SeriesNames: seriesNames,
		TagAbbrevs:  tagAbbrevs,
	}
	patchListPage(data).Render(ctx, w)
}

func (h *webHandler) patchDetailRoute(w http.ResponseWriter, r *http.Request) {
	linkname := chi.URLParam(r, "linkname")
	rawMsgid, _ := url.PathUnescape(chi.URLParam(r, "msgid"))
	rest := chi.URLParam(r, "*")

	switch rest {
	case "", "/":
		h.patchDetail(w, r, linkname, rawMsgid)
	case "mbox/":
		h.patchMbox(w, r, linkname, rawMsgid)
	case "raw/":
		h.patchRaw(w, r, linkname, rawMsgid)
	default:
		notFoundPage(w)
	}
}

func (h *webHandler) patchDetail(w http.ResponseWriter, r *http.Request, linkname, rawMsgid string) {
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

	var patch db.Patch
	err = h.db.NewSelect().Model(&patch).
		Where("project_id = ?", project.ID).
		Where("msgid = ?", msgid).
		Scan(ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	// load relations
	var submitter db.Person
	if h.db.NewSelect().Model(&submitter).Where("id = ?", patch.SubmitterID).Scan(ctx) == nil {
		patch.Submitter = &submitter
	}
	if patch.StateID != nil {
		var state db.State
		if h.db.NewSelect().Model(&state).Where("id = ?", *patch.StateID).Scan(ctx) == nil {
			patch.State = &state
		}
	}
	if patch.DelegateID != nil {
		var delegate db.User
		if h.db.NewSelect().Model(&delegate).Where("id = ?", *patch.DelegateID).Scan(ctx) == nil {
			patch.Delegate = &delegate
		}
	}

	var series *db.Series
	if patch.SeriesID != nil {
		var s db.Series
		if h.db.NewSelect().Model(&s).Where("id = ?", *patch.SeriesID).Scan(ctx) == nil {
			series = &s
		}
	}

	var comments []db.PatchComment
	h.db.NewSelect().Model(&comments).
		Where("patch_id = ?", patch.ID).
		OrderExpr("date ASC").
		Scan(ctx)
	var submitterIDs []int32
	for i := range comments {
		submitterIDs = append(submitterIDs, comments[i].SubmitterID)
	}
	if len(submitterIDs) > 0 {
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

	var checks []db.Check
	h.db.NewSelect().Model(&checks).
		Where("patch_id = ?", patch.ID).
		OrderExpr("date DESC").
		Scan(ctx)

	var metadata map[string]string
	if series != nil {
		var rows []db.SeriesMetadata
		h.db.NewSelect().Model(&rows).
			Where("series_id = ?", series.ID).
			Scan(ctx)
		if len(rows) > 0 {
			metadata = make(map[string]string)
			for _, r := range rows {
				metadata[r.Key] = r.Value
			}
		}
	}

	data := patchDetailData{
		Project:        project,
		Patch:          patch,
		Comments:       comments,
		Checks:         checks,
		Series:         series,
		SeriesMetadata: metadata,
	}
	patchDetailPage(data).Render(ctx, w)
}

func (h *webHandler) patchRaw(w http.ResponseWriter, r *http.Request, linkname, rawMsgid string) {
	ctx := r.Context()
	msgid := "<" + rawMsgid + ">"

	var patch db.Patch
	err := h.db.NewRaw(`
		SELECT p.diff, p.name FROM patchwork_patch p
		JOIN patchwork_project pr ON pr.id = p.project_id
		WHERE pr.linkname = ? AND p.msgid = ?
	`, linkname, msgid).Scan(ctx, &patch)
	if err != nil || patch.Diff == nil {
		notFoundPage(w)
		return
	}

	w.Header().Set("Content-Type", "text/x-patch; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.diff", sanitizeFilename(patch.Name)))
	w.Write([]byte(*patch.Diff))
}

func (h *webHandler) patchRedirect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		notFoundPage(w)
		return
	}

	var patch struct {
		Msgid     string
		ProjectID int32
	}
	err = h.db.NewRaw(`SELECT msgid, project_id FROM patchwork_patch WHERE id = ?`, id).
		Scan(ctx, &patch)
	if err != nil {
		notFoundPage(w)
		return
	}

	var linkname string
	h.db.NewRaw(`SELECT linkname FROM patchwork_project WHERE id = ?`, patch.ProjectID).
		Scan(ctx, &linkname)

	http.Redirect(w, r, patchURL(linkname, patch.Msgid), http.StatusMovedPermanently)
}

func (h *webHandler) patchMboxByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		notFoundPage(w)
		return
	}

	var patch db.Patch
	err = h.db.NewSelect().Model(&patch).Where("id = ?", id).Scan(ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	var project db.Project
	h.db.NewSelect().Model(&project).Where("id = ?", patch.ProjectID).Scan(ctx)

	h.servePatchMbox(w, patch, project)
}

func (h *webHandler) patchRawByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		notFoundPage(w)
		return
	}

	var patch db.Patch
	err = h.db.NewSelect().Model(&patch).
		Column("diff", "name").
		Where("id = ?", id).Scan(ctx)
	if err != nil || patch.Diff == nil {
		notFoundPage(w)
		return
	}

	w.Header().Set("Content-Type", "text/x-patch; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.diff", sanitizeFilename(patch.Name)))
	w.Write([]byte(*patch.Diff))
}

func applySort(q *bun.SelectQuery, sort string) *bun.SelectQuery {
	desc := false
	field := sort
	if strings.HasPrefix(sort, "-") {
		desc = true
		field = sort[1:]
	}

	colMap := map[string]string{
		"date":      "date",
		"name":      "name",
		"submitter": "submitter_id",
		"delegate":  "delegate_id",
		"state":     "state_id",
	}

	col, ok := colMap[field]
	if !ok {
		col = "date"
		desc = true
	}

	if desc {
		return q.OrderExpr(col + " DESC")
	}
	return q.OrderExpr(col + " ASC")
}

func populateWebPatches(ctx context.Context, database *bun.DB, patches []db.Patch) {
	if len(patches) == 0 {
		return
	}
	var submitterIDs, stateIDs, delegateIDs []int32
	for i := range patches {
		submitterIDs = append(submitterIDs, patches[i].SubmitterID)
		if patches[i].StateID != nil {
			stateIDs = append(stateIDs, *patches[i].StateID)
		}
		if patches[i].DelegateID != nil {
			delegateIDs = append(delegateIDs, *patches[i].DelegateID)
		}
	}

	people := make(map[int32]*db.Person)
	if len(submitterIDs) > 0 {
		var ps []db.Person
		database.NewSelect().Model(&ps).
			Where("id IN ?", bun.Tuple(submitterIDs)).
			Scan(ctx)
		for i := range ps {
			people[ps[i].ID] = &ps[i]
		}
	}

	states := make(map[int32]*db.State)
	if len(stateIDs) > 0 {
		var ss []db.State
		database.NewSelect().Model(&ss).
			Where("id IN ?", bun.Tuple(stateIDs)).
			Scan(ctx)
		for i := range ss {
			states[ss[i].ID] = &ss[i]
		}
	}

	delegates := make(map[int32]*db.User)
	if len(delegateIDs) > 0 {
		var us []db.User
		database.NewSelect().Model(&us).
			Where("id IN ?", bun.Tuple(delegateIDs)).
			Scan(ctx)
		for i := range us {
			delegates[us[i].ID] = &us[i]
		}
	}

	ids := make([]int32, len(patches))
	for i := range patches {
		ids[i] = patches[i].ID
		patches[i].Submitter = people[patches[i].SubmitterID]
		if patches[i].StateID != nil {
			patches[i].State = states[*patches[i].StateID]
		}
		if patches[i].DelegateID != nil {
			patches[i].Delegate = delegates[*patches[i].DelegateID]
		}
	}

	// load tags
	type tagRow struct {
		PatchID int32  `bun:"patch_id"`
		Abbrev  string `bun:"abbrev"`
		Count   int    `bun:"count"`
	}
	var tagRows []tagRow
	database.NewRaw(`
		SELECT pt.patch_id, t.abbrev, pt.count
		FROM patchwork_patchtag pt
		JOIN patchwork_tag t ON t.id = pt.tag_id
		WHERE pt.patch_id IN ?
	`, bun.Tuple(ids)).Scan(ctx, &tagRows)

	tagMap := make(map[int32]map[string]int)
	for _, r := range tagRows {
		if tagMap[r.PatchID] == nil {
			tagMap[r.PatchID] = make(map[string]int)
		}
		tagMap[r.PatchID][r.Abbrev] = r.Count
	}

	// load check counts
	type checkRow struct {
		PatchID int32 `bun:"patch_id"`
		State   int16 `bun:"state"`
		Count   int   `bun:"count"`
	}
	var checkRows []checkRow
	database.NewRaw(`
		SELECT patch_id, state, count(*) as count
		FROM patchwork_check
		WHERE patch_id IN ?
		GROUP BY patch_id, state
	`, bun.Tuple(ids)).Scan(ctx, &checkRows)

	checkMap := make(map[int32][4]int)
	for _, r := range checkRows {
		c := checkMap[r.PatchID]
		if r.State >= 0 && r.State < 4 {
			c[r.State] = r.Count
		}
		checkMap[r.PatchID] = c
	}

	for i := range patches {
		patches[i].Tags = tagMap[patches[i].ID]
		if patches[i].Tags == nil {
			patches[i].Tags = map[string]int{}
		}
		patches[i].CheckCounts = checkMap[patches[i].ID]
	}
}
