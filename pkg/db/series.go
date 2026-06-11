// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"time"

	"github.com/uptrace/bun"
)

func (q *Queries) CreateSeries(s *Series) error {
	return q.tx.NewInsert().Model(s).
		Returning("*").
		Scan(q.ctx)
}

func (q *Queries) GetSeriesByID(id int32) (*Series, error) {
	var s Series
	err := q.tx.NewSelect().Model(&s).
		Where("id = ?", id).
		Scan(q.ctx)
	return &s, err
}

func (q *Queries) FindSeriesByMsgID(msgid string) (*Series, error) {
	var s Series
	// check patches first
	err := q.tx.NewSelect().Model(&s).
		Where("id = (SELECT series_id FROM patchwork_patch WHERE msgid = ? AND series_id IS NOT NULL LIMIT 1)", msgid).
		Scan(q.ctx)
	if err == nil {
		return &s, nil
	}
	// check cover letters
	err = q.tx.NewSelect().Model(&s).
		Where("cover_letter_id = (SELECT id FROM patchwork_cover WHERE msgid = ? LIMIT 1)", msgid).
		Scan(q.ctx)
	return &s, err
}

func (q *Queries) FindSeriesByReference(projectID int32, msgid string) (*Series, error) {
	var s Series
	err := q.tx.NewSelect().Model(&s).
		Join("JOIN patchwork_seriesreference AS sr ON sr.series_id = series.id").
		Where("sr.project_id = ?", projectID).
		Where("sr.msgid = ?", truncStr(msgid, 255)).
		OrderBy("date", bun.OrderDesc).
		Limit(1).
		Scan(q.ctx)
	return &s, err
}

func (q *Queries) FindSeriesByMarkers(
	projectID *int32, submitterID int32,
	version, total int32,
	dateMin, dateMax time.Time, number *int16,
) (*Series, error) {
	var s Series
	err := q.tx.NewSelect().Model(&s).
		Where("project_id = ?", projectID).
		Where("submitter_id = ?", submitterID).
		Where("version = ?", version).
		Where("total = ?", total).
		Where("date >= ?", dateMin).
		Where("date <= ?", dateMax).
		Where("NOT EXISTS (SELECT 1 FROM patchwork_patch WHERE series_id = series.id AND number = ?)", number).
		OrderBy("date", bun.OrderDesc).
		Limit(1).
		Scan(q.ctx)
	return &s, err
}

func (q *Queries) FindSeries(
	projectID int32, submitterID int32, refs []string, msgid string,
	number int16, version, total int32,
	date time.Time, delta time.Duration,
) (*Series, error) {
	var s Series

	slotCheck := func(query *bun.SelectQuery) *bun.SelectQuery {
		if number != 0 {
			query = query.Where(
				"NOT EXISTS (SELECT 1 FROM patchwork_patch WHERE series_id = series.id AND number = ?)",
				number)
		}
		return query
	}
	dateCheck := func(query *bun.SelectQuery) *bun.SelectQuery {
		if !date.IsZero() {
			query = query.
				Where("series.date >= ?", date.Add(-delta)).
				Where("series.date <= ?", date.Add(delta))
		}
		return query
	}

	// tier 1: match by message references
	if len(refs) > 0 {
		refQuery := q.tx.NewSelect().Model(&s).
			Join("JOIN patchwork_seriesreference AS sr ON sr.series_id = series.id").
			Where("sr.msgid IN (?)", bun.In(refs)).
			Where("series.project_id = ?", projectID).
			Where("series.version = ?", version).
			Where("series.total = ?", total)
		refQuery = dateCheck(slotCheck(refQuery))
		if msgid != "" {
			refQuery = refQuery.OrderExpr(
				"CASE WHEN sr.msgid = ? THEN 0 ELSE 1 END", msgid)
		}
		refQuery = refQuery.OrderExpr("series.date DESC").Limit(1)
		if err := refQuery.Scan(q.ctx); err == nil {
			return &s, nil
		}
	}

	// tier 2: match by markers (submitter, version, total, date)
	markerQuery := q.tx.NewSelect().Model(&s).
		Where("project_id = ?", projectID).
		Where("submitter_id = ?", submitterID).
		Where("version = ?", version).
		Where("total = ?", total)
	markerQuery = dateCheck(slotCheck(markerQuery))
	markerQuery = markerQuery.OrderExpr("date DESC").Limit(1)

	err := markerQuery.Scan(q.ctx)
	return &s, err
}

func (q *Queries) FindPreviousSeriesByName(
	projectID *int32, submitterID int32, version int32,
) ([]Series, error) {
	var series []Series
	err := q.tx.NewSelect().Model(&series).
		Where("project_id = ?", projectID).
		Where("submitter_id = ?", submitterID).
		Where("version = ?", version).
		OrderBy("date", bun.OrderDesc).
		Scan(q.ctx)
	return series, err
}

func (q *Queries) CreateSeriesReference(projectID, seriesID int32, msgid string) error {
	ref := &SeriesReference{
		ProjectID: projectID,
		SeriesID:  seriesID,
		Msgid:     truncStr(msgid, 255),
	}
	_, err := q.tx.NewInsert().Model(ref).
		On("CONFLICT DO NOTHING").
		Exec(q.ctx)
	return err
}

func (q *Queries) UpdateSeriesCoverLetter(id int32, coverLetterID *int32) error {
	_, err := q.tx.NewUpdate().Model((*Series)(nil)).
		Set("cover_letter_id = ?", coverLetterID).
		Where("id = ?", id).
		Exec(q.ctx)
	return err
}

func (q *Queries) UpdateSeriesName(id int32, name *string) error {
	_, err := q.tx.NewUpdate().Model((*Series)(nil)).
		Set("name = ?", name).
		Where("id = ?", id).
		Exec(q.ctx)
	return err
}

func (q *Queries) UpdateSeriesPreviousSeries(id int32, previousSeriesID *int32) error {
	_, err := q.tx.NewUpdate().Model((*Series)(nil)).
		Set("previous_series_id = ?", previousSeriesID).
		Where("id = ?", id).
		Exec(q.ctx)
	return err
}

func (q *Queries) AddSeriesDependency(fromSeriesID, toSeriesID int32) error {
	_, err := q.tx.NewRaw(
		"INSERT INTO patchwork_series_dependencies (from_series_id, to_series_id) VALUES (?, ?) ON CONFLICT DO NOTHING",
		fromSeriesID, toSeriesID).
		Exec(q.ctx)
	return err
}
