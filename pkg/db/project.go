// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

func (q *Queries) GetProjectByListID(listid string) ([]Project, error) {
	var projects []Project
	err := q.tx.NewSelect().Model(&projects).
		Where("listid = ?", listid).
		Scan(q.ctx)
	return projects, err
}

func (q *Queries) GetDefaultState() (*State, error) {
	var s State
	err := q.tx.NewSelect().Model(&s).
		Where("ordering = 0").
		Scan(q.ctx)
	return &s, err
}

func (q *Queries) GetStateByName(name string) (*State, error) {
	var s State
	err := q.tx.NewSelect().Model(&s).
		Where("lower(name) = lower(?)", name).
		Scan(q.ctx)
	return &s, err
}

func (q *Queries) GetStateBySlug(slug string) (*State, error) {
	var s State
	err := q.tx.NewSelect().Model(&s).
		Where("slug = ?", slug).
		Scan(q.ctx)
	return &s, err
}

func (q *Queries) ListDelegationRulesByProject(projectID int32) ([]DelegationRule, error) {
	var rules []DelegationRule
	err := q.tx.NewSelect().Model(&rules).
		Where("project_id = ?", projectID).
		OrderExpr("priority DESC, path").
		Scan(q.ctx)
	return rules, err
}

func (q *Queries) GetUserIDByEmail(email string) (int32, error) {
	var id int32
	err := q.tx.NewRaw(
		"SELECT id FROM auth_user WHERE lower(email) = lower(?) AND is_active = true",
		email).Scan(q.ctx, &id)
	return id, err
}
