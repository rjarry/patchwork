// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

func (q *Queries) GetPatchCommentByID(id int32) (*PatchComment, error) {
	var c PatchComment
	err := q.tx.NewSelect().Model(&c).
		Where("id = ?", id).
		Scan(q.ctx)
	return &c, err
}

func (q *Queries) GetCoverCommentByID(id int32) (*CoverComment, error) {
	var c CoverComment
	err := q.tx.NewSelect().Model(&c).
		Where("id = ?", id).
		Scan(q.ctx)
	return &c, err
}

func (q *Queries) CreatePatchComment(c *PatchComment) error {
	return q.tx.NewInsert().Model(c).
		On("CONFLICT (msgid, patch_id) DO NOTHING").
		Returning("*").
		Scan(q.ctx)
}

func (q *Queries) CreateCoverComment(c *CoverComment) error {
	return q.tx.NewInsert().Model(c).
		On("CONFLICT (msgid, cover_id) DO NOTHING").
		Returning("*").
		Scan(q.ctx)
}
