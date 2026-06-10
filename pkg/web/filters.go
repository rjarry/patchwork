// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/uptrace/bun"
)

func applyWebFilters(q *bun.SelectQuery, params url.Values, basePath string) (*bun.SelectQuery, []appliedFilter) {
	var filters []appliedFilter

	if v := params.Get("q"); v != "" {
		q = q.Where("name LIKE ?", "%"+v+"%")
		filters = append(filters, appliedFilter{
			Label:     "Search",
			Value:     v,
			RemoveURL: removeParam(basePath, params, "q"),
		})
	}

	if v := params.Get("state"); v != "" {
		if v == "*" {
			q = q.Where("state_id IN (SELECT id FROM patchwork_state WHERE action_required = ?)", true)
			filters = append(filters, appliedFilter{
				Label:     "State",
				Value:     "Action required",
				RemoveURL: removeParam(basePath, params, "state"),
			})
		} else if id, err := strconv.Atoi(v); err == nil {
			q = q.Where("state_id = ?", id)
			filters = append(filters, appliedFilter{
				Label:     "State",
				Value:     v,
				RemoveURL: removeParam(basePath, params, "state"),
			})
		}
	}

	archive := params.Get("archive")
	switch archive {
	case "true":
		q = q.Where("archived = ?", true)
		filters = append(filters, appliedFilter{
			Label:     "Archive",
			Value:     "Archived",
			RemoveURL: removeParam(basePath, params, "archive"),
		})
	case "both":
		// no filter
		filters = append(filters, appliedFilter{
			Label:     "Archive",
			Value:     "Both",
			RemoveURL: removeParam(basePath, params, "archive"),
		})
	default:
		q = q.Where("archived = ?", false)
	}

	if v := params.Get("submitter"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			q = q.Where("submitter_id = ?", id)
		} else {
			q = q.Where("submitter_id IN (SELECT id FROM patchwork_person WHERE name LIKE ?)", "%"+v+"%")
		}
		filters = append(filters, appliedFilter{
			Label:     "Submitter",
			Value:     v,
			RemoveURL: removeParam(basePath, params, "submitter"),
		})
	}

	if v := params.Get("delegate"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			q = q.Where("delegate_id = ?", id)
		} else {
			q = q.Where("delegate_id IN (SELECT id FROM auth_user WHERE username LIKE ?)", "%"+v+"%")
		}
		filters = append(filters, appliedFilter{
			Label:     "Delegate",
			Value:     v,
			RemoveURL: removeParam(basePath, params, "delegate"),
		})
	}

	if v := params.Get("series"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			q = q.Where("series_id = ?", id)
			filters = append(filters, appliedFilter{
				Label:     "Series",
				Value:     fmt.Sprintf("#%d", id),
				RemoveURL: removeParam(basePath, params, "series"),
			})
		}
	}

	return q, filters
}

func removeParam(basePath string, params url.Values, key string) string {
	cp := url.Values{}
	for k, v := range params {
		if k != key && k != "page" {
			cp[k] = v
		}
	}
	if len(cp) == 0 {
		return basePath
	}
	return basePath + "?" + cp.Encode()
}

func sanitizeFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	s := b.String()
	s = strings.Trim(s, "-")
	if s == "" {
		s = "patch"
	}
	return s
}
