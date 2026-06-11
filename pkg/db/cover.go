// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

func (q *Queries) GetCoverByID(id int32) (*Cover, error) {
	var c Cover
	err := q.tx.NewSelect().Model(&c).
		Where("id = ?", id).
		Scan(q.ctx)
	return &c, err
}

func (q *Queries) CreateCover(cover *Cover) error {
	cover.Name = truncStr(cover.Name, 255)
	return q.tx.NewInsert().Model(cover).
		On("CONFLICT (msgid, project_id) DO NOTHING").
		Returning("*").
		Scan(q.ctx)
}

func (q *Queries) GetCoverByProjectAndMsgID(projectID int32, msgid string) (*Cover, error) {
	var c Cover
	err := q.tx.NewSelect().Model(&c).
		Where("project_id = ?", projectID).
		Where("msgid = ?", msgid).
		Scan(q.ctx)
	return &c, err
}

func (q *Queries) FindCoverByCommentMsgID(msgid string) (*Cover, error) {
	var c Cover
	err := q.tx.NewSelect().Model(&c).
		Join("JOIN patchwork_covercomment AS cc ON cc.cover_id = cover.id").
		Where("cc.msgid = ?", msgid).
		Limit(1).
		Scan(q.ctx)
	return &c, err
}
