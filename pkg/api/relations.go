// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/emersion/go-message/mail"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func loadPersons(ctx context.Context, database *bun.DB, ids []int32) map[int32]*db.Person {
	if len(ids) == 0 {
		return nil
	}
	var items []db.Person
	database.NewSelect().Model(&items).
		Where("id IN (?)", bun.In(dedup(ids))).
		Scan(ctx, &items)
	m := make(map[int32]*db.Person, len(items))
	for i := range items {
		m[items[i].ID] = &items[i]
	}
	return m
}

func loadProjects(ctx context.Context, database *bun.DB, ids []int32) map[int32]*db.Project {
	if len(ids) == 0 {
		return nil
	}
	var items []db.Project
	database.NewSelect().Model(&items).
		Where("id IN (?)", bun.In(dedup(ids))).
		Scan(ctx, &items)
	m := make(map[int32]*db.Project, len(items))
	for i := range items {
		m[items[i].ID] = &items[i]
	}
	return m
}

func loadStates(ctx context.Context, database *bun.DB, ids []int32) map[int32]*db.State {
	if len(ids) == 0 {
		return nil
	}
	var items []db.State
	database.NewSelect().Model(&items).
		Where("id IN (?)", bun.In(dedup(ids))).
		Scan(ctx, &items)
	m := make(map[int32]*db.State, len(items))
	for i := range items {
		m[items[i].ID] = &items[i]
	}
	return m
}

func loadUsers(ctx context.Context, database *bun.DB, ids []int32) map[int32]*db.User {
	if len(ids) == 0 {
		return nil
	}
	var items []db.User
	database.NewSelect().Model(&items).
		Where("id IN (?)", bun.In(dedup(ids))).
		Scan(ctx, &items)
	m := make(map[int32]*db.User, len(items))
	for i := range items {
		m[items[i].ID] = &items[i]
	}
	return m
}

func loadPatchTags(ctx context.Context, database *bun.DB, patches []db.Patch) {
	if len(patches) == 0 {
		return
	}
	ids := make([]int32, len(patches))
	for i := range patches {
		ids[i] = patches[i].ID
	}

	type tagRow struct {
		PatchID int32  `bun:"patch_id"`
		Abbrev  string `bun:"abbrev"`
		Count   int32  `bun:"count"`
	}
	var rows []tagRow
	database.NewRaw(`
		SELECT pt.patch_id, t.abbrev, pt.count
		FROM patchwork_patchtag pt
		JOIN patchwork_tag t ON t.id = pt.tag_id
		WHERE pt.patch_id IN (?)
	`, bun.In(dedup(ids))).Scan(ctx, &rows)

	tagMap := make(map[int32]map[string]int)
	for _, r := range rows {
		if tagMap[r.PatchID] == nil {
			tagMap[r.PatchID] = make(map[string]int)
		}
		tagMap[r.PatchID][r.Abbrev] = int(r.Count)
	}

	for i := range patches {
		patches[i].Tags = tagMap[patches[i].ID]
		if patches[i].Tags == nil {
			patches[i].Tags = map[string]int{}
		}
	}
}

func loadPatchSeries(ctx context.Context, database *bun.DB, patches []db.Patch) {
	for i := range patches {
		if patches[i].SeriesID != nil {
			var s db.Series
			err := database.NewSelect().Model(&s).
				Where("id = ?", *patches[i].SeriesID).
				Scan(ctx)
			if err == nil {
				patches[i].SeriesList = []db.SeriesRef{
					{ID: s.ID, Name: s.Name},
				}
			}
		}
		if patches[i].SeriesList == nil {
			patches[i].SeriesList = []db.SeriesRef{}
		}
	}
}

func updateRelated(
	ctx context.Context, database *bun.DB, r *http.Request,
	user *db.User, patch *db.Patch, relatedIDs []int32,
) error {
	if len(relatedIDs) == 0 {
		// remove from relation
		if patch.RelatedID != nil {
			oldRelID := *patch.RelatedID
			database.NewUpdate().Model((*db.Patch)(nil)).
				Set("related_id = NULL").
				Where("id = ?", patch.ID).
				Exec(ctx)
			patch.RelatedID = nil

			// if relation has < 2 patches left, delete it
			var remaining int
			database.NewSelect().Model((*db.Patch)(nil)).
				Where("related_id = ?", oldRelID).
				ColumnExpr("count(*)").Scan(ctx, &remaining)
			if remaining < 2 {
				database.NewUpdate().Model((*db.Patch)(nil)).
					Set("related_id = NULL").
					Where("related_id = ?", oldRelID).
					Exec(ctx)
				database.NewDelete().Model((*db.PatchRelation)(nil)).
					Where("id = ?", oldRelID).
					Exec(ctx)
			}
		}
		return nil
	}

	// verify all related patches exist and user is maintainer for each project
	for _, pid := range relatedIDs {
		var p db.Patch
		if err := database.NewSelect().Model(&p).
			Where("id = ?", pid).Column("id", "project_id", "related_id").
			Scan(ctx); err != nil {
			return fmt.Errorf("patch %d not found", pid)
		}
		if !isMaintainer(database, ctx, user.ID, p.ProjectID) {
			return fmt.Errorf("forbidden")
		}
		// check for conflict: patch already in a different relation
		if p.RelatedID != nil && patch.RelatedID != nil && *p.RelatedID != *patch.RelatedID {
			return fmt.Errorf("conflict")
		}
		if p.RelatedID != nil && patch.RelatedID == nil {
			// target has a relation, source doesn't -- join target's
			database.NewUpdate().Model((*db.Patch)(nil)).
				Set("related_id = ?", *p.RelatedID).
				Where("id = ?", patch.ID).
				Exec(ctx)
			patch.RelatedID = p.RelatedID
		}
	}

	// create new relation if neither side has one
	if patch.RelatedID == nil {
		var relID int32
		database.NewRaw(
			"INSERT INTO patchwork_patchrelation DEFAULT VALUES RETURNING id",
		).Scan(ctx, &relID)
		patch.RelatedID = &relID
		database.NewUpdate().Model((*db.Patch)(nil)).
			Set("related_id = ?", relID).
			Where("id = ?", patch.ID).
			Exec(ctx)
	}

	// assign all related patches to the same relation
	for _, pid := range relatedIDs {
		database.NewUpdate().Model((*db.Patch)(nil)).
			Set("related_id = ?", *patch.RelatedID).
			Where("id = ?", pid).
			Exec(ctx)
	}

	return nil
}

func loadPatchRelated(ctx context.Context, database *bun.DB, patches []db.Patch) {
	for i := range patches {
		if patches[i].RelatedID != nil {
			var related []db.Patch
			database.NewSelect().Model(&related).
				Where("related_id = ?", *patches[i].RelatedID).
				Where("id != ?", patches[i].ID).
				Scan(ctx)
			refs := make([]db.PatchRef, len(related))
			for j := range related {
				refs[j] = db.PatchRef{ID: related[j].ID, Name: related[j].Name}
			}
			patches[i].Related = refs
		}
		if patches[i].Related == nil {
			patches[i].Related = []db.PatchRef{}
		}
	}
}

func loadCombinedCheck(ctx context.Context, database *bun.DB, patches []db.Patch) {
	for i := range patches {
		var states []int16
		database.NewRaw(`
			SELECT DISTINCT state FROM patchwork_check WHERE patch_id = ?
		`, patches[i].ID).Scan(ctx, &states)
		if len(states) == 0 {
			continue
		}
		combined := "success"
		for _, s := range states {
			switch db.CheckState(s) {
			case db.CheckFail:
				combined = "fail"
			case db.CheckWarning:
				if combined != "fail" {
					combined = "warning"
				}
			case db.CheckPending:
				if combined != "fail" && combined != "warning" {
					combined = "pending"
				}
			}
		}
		patches[i].CombinedCheck = &combined
	}
}

func loadCoverSeries(ctx context.Context, database *bun.DB, covers []db.Cover) {
	for i := range covers {
		var s db.Series
		err := database.NewSelect().Model(&s).
			Where("cover_letter_id = ?", covers[i].ID).
			Scan(ctx)
		if err == nil {
			covers[i].SeriesList = []db.SeriesRef{
				{ID: s.ID, Name: s.Name},
			}
		}
		if covers[i].SeriesList == nil {
			covers[i].SeriesList = []db.SeriesRef{}
		}
	}
}

func loadSeriesDetail(ctx context.Context, r *http.Request, database *bun.DB, series []db.Series) {
	base := apiBase(r)
	for i := range series {
		s := &series[i]

		var count int
		database.NewSelect().Model((*db.Patch)(nil)).
			Where("series_id = ?", s.ID).
			ColumnExpr("count(*)").
			Scan(ctx, &count)
		s.ReceivedTotal = count
		s.ReceivedAll = count >= int(s.Total)

		if s.CoverLetterID != nil {
			var cover db.Cover
			if err := database.NewSelect().Model(&cover).
				Where("id = ?", *s.CoverLetterID).
				Scan(ctx); err == nil {
				s.CoverLetter = &cover
			}
		}

		var patches []db.Patch
		database.NewSelect().Model(&patches).
			Where("series_id = ?", s.ID).
			OrderExpr("number ASC").
			Scan(ctx)
		if patches == nil {
			patches = []db.Patch{}
		}
		s.Patches = patches

		var meta []db.SeriesMetadata
		database.NewSelect().Model(&meta).
			Where("series_id = ?", s.ID).
			Scan(ctx)
		s.Metadata = make(map[string]string, len(meta))
		for _, m := range meta {
			s.Metadata[m.Key] = m.Value
		}

		// dependencies
		var depIDs []int32
		database.NewRaw(`
			SELECT to_series_id FROM patchwork_series_dependencies
			WHERE from_series_id = ?`, s.ID).Scan(ctx, &depIDs)
		s.Dependencies = make([]string, len(depIDs))
		for j, id := range depIDs {
			s.Dependencies[j] = fmt.Sprintf("%s/series/%d/", base, id)
		}

		// dependents (reverse)
		var revIDs []int32
		database.NewRaw(`
			SELECT from_series_id FROM patchwork_series_dependencies
			WHERE to_series_id = ?`, s.ID).Scan(ctx, &revIDs)
		s.Dependents = make([]string, len(revIDs))
		for j, id := range revIDs {
			s.Dependents[j] = fmt.Sprintf("%s/series/%d/", base, id)
		}

		// previous series
		if s.PreviousSeriesID != nil {
			u := fmt.Sprintf("%s/series/%d/", base, *s.PreviousSeriesID)
			s.PreviousSeries = &u
		}

		// next series (reverse FK)
		var nextIDs []int32
		database.NewRaw(`
			SELECT id FROM patchwork_series WHERE previous_series_id = ?
		`, s.ID).Scan(ctx, &nextIDs)
		s.NextSeries = make([]string, len(nextIDs))
		for j, id := range nextIDs {
			s.NextSeries[j] = fmt.Sprintf("%s/series/%d/", base, id)
		}
	}
}

func loadProjectMaintainers(ctx context.Context, database *bun.DB, projects []db.Project) {
	for i := range projects {
		var users []db.User
		database.NewRaw(`
			SELECT u.id, u.username, u.first_name, u.last_name, u.email
			FROM auth_user u
			JOIN patchwork_userprofile up ON up.user_id = u.id
			JOIN patchwork_userprofile_maintainer_projects mp ON mp.userprofile_id = up.id
			WHERE mp.project_id = ?
		`, projects[i].ID).Scan(ctx, &users)
		if users == nil {
			users = []db.User{}
		}
		projects[i].Maintainers = users
	}
}

func populatePersonUsers(ctx context.Context, database *bun.DB, people []db.Person) {
	for i := range people {
		if people[i].UserID != nil {
			var u db.User
			if err := database.NewSelect().Model(&u).
				Where("id = ?", *people[i].UserID).
				Scan(ctx); err == nil {
				people[i].User = &u
			}
		}
	}
}

func populateChecks(ctx context.Context, r *http.Request, database *bun.DB, patchID int32, checks []db.Check) {
	if len(checks) == 0 {
		return
	}
	base := apiBase(r)
	var userIDs []int32
	for i := range checks {
		checks[i].URL = fmt.Sprintf("%s/patches/%d/checks/%d/", base, patchID, checks[i].ID)
		if checks[i].UserID != nil {
			userIDs = append(userIDs, *checks[i].UserID)
		}
	}
	users := loadUsers(ctx, database, userIDs)
	for i := range checks {
		if checks[i].UserID != nil {
			checks[i].User = users[*checks[i].UserID]
		}
	}
}

func populateCommentURLs(r *http.Request, patchID int32, comments []db.PatchComment) {
	base := apiBase(r)
	for i := range comments {
		comments[i].URL = fmt.Sprintf("%s/patches/%d/comments/%d/",
			base, patchID, comments[i].ID)
		comments[i].Subject = parseSubjectFromHeaders(comments[i].Headers)
	}
}

func populateCoverCommentURLs(r *http.Request, coverID int32, comments []db.CoverComment) {
	base := apiBase(r)
	for i := range comments {
		comments[i].URL = fmt.Sprintf("%s/covers/%d/comments/%d/",
			base, coverID, comments[i].ID)
		comments[i].Subject = parseSubjectFromHeaders(comments[i].Headers)
	}
}

func parseSubjectFromHeaders(headers string) string {
	if headers == "" {
		return ""
	}
	raw := strings.ReplaceAll(headers, "\n", "\r\n")
	if !strings.HasSuffix(raw, "\r\n\r\n") {
		raw += "\r\n"
	}
	m, err := mail.CreateReader(strings.NewReader(raw))
	if err != nil {
		return ""
	}
	subject, _ := m.Header.Subject()
	return subject
}

func listArchiveURL(project *db.Project, msgid string) string {
	if project == nil || project.ListArchiveURLFormat == "" {
		return ""
	}
	bare := strings.TrimPrefix(strings.TrimSuffix(msgid, ">"), "<")
	return strings.ReplaceAll(project.ListArchiveURLFormat, "{}", url.PathEscape(bare))
}

func loadBundlePatches(ctx context.Context, database *bun.DB, bundles []db.Bundle) {
	for i := range bundles {
		var patches []db.Patch
		database.NewRaw(`
			SELECT p.* FROM patchwork_patch p
			JOIN patchwork_bundlepatch bp ON bp.patch_id = p.id
			WHERE bp.bundle_id = ?
		`, bundles[i].ID).Scan(ctx, &patches)
		if patches == nil {
			patches = []db.Patch{}
		}
		bundles[i].BundlePatches = patches
	}
}

func dedup(ids []int32) []int32 {
	seen := make(map[int32]bool, len(ids))
	result := make([]int32, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

func applyOrdering(q *bun.SelectQuery, r *http.Request, defaultOrder string, allowed map[string]string) *bun.SelectQuery {
	order := r.URL.Query().Get("order")
	if order == "" {
		return q.OrderExpr(defaultOrder)
	}

	desc := false
	if strings.HasPrefix(order, "-") {
		desc = true
		order = order[1:]
	}

	col, ok := allowed[order]
	if !ok {
		return q.OrderExpr(defaultOrder)
	}

	if desc {
		return q.OrderExpr(col + " DESC")
	}
	return q.OrderExpr(col + " ASC")
}
