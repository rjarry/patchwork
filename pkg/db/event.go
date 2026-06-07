// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

func (q *Queries) CreateEvent(e *Event) error {
	return q.tx.NewInsert().Model(e).
		Returning("*").
		Scan(q.ctx)
}

func (q *Queries) CountPatchesInSeries(seriesID int32) (int, error) {
	return q.tx.NewSelect().Model((*Patch)(nil)).
		Where("series_id = ?", seriesID).
		Count(q.ctx)
}
