// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import "strings"

func (q *Queries) GetActiveWebhooks(projectID int32) ([]Webhook, error) {
	var hooks []Webhook
	err := q.tx.NewSelect().Model(&hooks).
		Where("project_id = ?", projectID).
		Where("active = ?", true).
		Scan(q.ctx)
	return hooks, err
}

func (w *Webhook) MatchesEvent(category string) bool {
	if w.Events == "*" {
		return true
	}
	for _, e := range strings.Split(w.Events, ",") {
		if strings.TrimSpace(e) == category {
			return true
		}
	}
	return false
}
