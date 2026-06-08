// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package webhook

import (
	"encoding/json"
	"fmt"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func buildPayload(q *db.Queries, e *db.Event) (map[string]any, error) {
	m := map[string]any{}

	switch e.Category {
	case "cover-created":
		c, err := q.GetCoverByID(*e.CoverID)
		if err != nil {
			return nil, fmt.Errorf("cover %d: %w", *e.CoverID, err)
		}
		m["cover"] = c

	case "patch-created":
		p, err := q.GetPatchByID(*e.PatchID)
		if err != nil {
			return nil, fmt.Errorf("patch %d: %w", *e.PatchID, err)
		}
		m["patch"] = p

	case "patch-completed":
		p, err := q.GetPatchByID(*e.PatchID)
		if err != nil {
			return nil, fmt.Errorf("patch %d: %w", *e.PatchID, err)
		}
		m["patch"] = p
		s, err := q.GetSeriesByID(*e.SeriesID)
		if err != nil {
			return nil, fmt.Errorf("series %d: %w", *e.SeriesID, err)
		}
		m["series"] = s

	case "series-created", "series-completed":
		s, err := q.GetSeriesByID(*e.SeriesID)
		if err != nil {
			return nil, fmt.Errorf("series %d: %w", *e.SeriesID, err)
		}
		m["series"] = s

	case "patch-comment-created":
		if pc, err := q.GetPatchCommentByID(*e.PatchCommentID); err == nil {
			m["comment"] = pc
		}
		if p, err := q.GetPatchByID(*e.PatchID); err == nil {
			m["patch"] = p
		}

	case "cover-comment-created":
		if cc, err := q.GetCoverCommentByID(*e.CoverCommentID); err == nil {
			m["comment"] = cc
		}
		if c, err := q.GetCoverByID(*e.CoverID); err == nil {
			m["cover"] = c
		}
	}

	return m, nil
}

func serializeEvent(q *db.Queries, e *db.Event, project *db.Project) ([]byte, error) {
	payload, err := buildPayload(q, e)
	if err != nil {
		return nil, err
	}

	e.Project = project
	e.Payload = payload

	return json.Marshal(e)
}
