// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

func (q *Queries) GetPatchByID(id int32) (*Patch, error) {
	var p Patch
	err := q.tx.NewSelect().Model(&p).
		Where("id = ?", id).
		Scan(q.ctx)
	return &p, err
}

func (q *Queries) CreatePatch(patch *Patch) error {
	patch.Msgid = truncStr(patch.Msgid, 255)
	patch.Name = truncStr(patch.Name, 255)
	if patch.PullURL != nil {
		s := truncStr(*patch.PullURL, 255)
		patch.PullURL = &s
	}
	return q.tx.NewInsert().Model(patch).
		On("CONFLICT (msgid, project_id) DO NOTHING").
		Returning("*").
		Scan(q.ctx)
}

func (q *Queries) GetPatchByProjectAndMsgID(projectID int32, msgid string) (*Patch, error) {
	var p Patch
	err := q.tx.NewSelect().Model(&p).
		Where("project_id = ?", projectID).
		Where("msgid = ?", truncStr(msgid, 255)).
		Scan(q.ctx)
	return &p, err
}

func (q *Queries) GetPatchByMsgID(msgid string) ([]Patch, error) {
	var patches []Patch
	err := q.tx.NewSelect().Model(&patches).
		Where("msgid = ?", truncStr(msgid, 255)).
		Scan(q.ctx)
	return patches, err
}

func (q *Queries) FindPatchByCommentMsgID(msgid string) ([]Patch, error) {
	var patches []Patch
	err := q.tx.NewSelect().Model(&patches).
		Join("JOIN patchwork_patchcomment AS pc ON pc.patch_id = patch.id").
		Where("pc.msgid = ?", msgid).
		Scan(q.ctx)
	return patches, err
}

func (q *Queries) UpdatePatchSeries(id int32, seriesID *int32, number *int16) error {
	_, err := q.tx.NewUpdate().Model((*Patch)(nil)).
		Set("series_id = ?", seriesID).
		Set("number = ?", number).
		Where("id = ?", id).
		Exec(q.ctx)
	return err
}

func (q *Queries) GetPatchBySeriesAndNumber(seriesID int32, number int) (*Patch, error) {
	var p Patch
	err := q.tx.NewSelect().Model(&p).
		Where("series_id = ?", seriesID).
		Where("number = ?", number).
		Scan(q.ctx)
	return &p, err
}

func (q *Queries) CountPredecessorPatches(seriesID int32, number int) (int, error) {
	return q.tx.NewSelect().Model((*Patch)(nil)).
		Where("series_id = ?", seriesID).
		Where("number < ?", number).
		Count(q.ctx)
}

func (q *Queries) GetSuccessorPatches(seriesID int32, number int) ([]Patch, error) {
	var patches []Patch
	err := q.tx.NewSelect().Model(&patches).
		Where("series_id = ?", seriesID).
		Where("number > ?", number).
		OrderExpr("number ASC").
		Scan(q.ctx)
	return patches, err
}

func (q *Queries) UpdatePatchesBySeriesToState(seriesID, stateID *int32) error {
	_, err := q.tx.NewUpdate().Model((*Patch)(nil)).
		Set("state_id = ?", stateID).
		Where("series_id = ?", seriesID).
		Exec(q.ctx)
	return err
}
