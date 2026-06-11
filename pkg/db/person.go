// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import "strings"

func (q *Queries) GetOrCreatePerson(email, name string) (*Person, error) {
	p := &Person{Email: truncStr(strings.ToLower(email), 255)}
	if name != "" {
		n := truncStr(name, 255)
		p.Name = &n
	}
	err := q.tx.NewInsert().Model(p).
		On("CONFLICT (email) DO UPDATE").
		Set("name = EXCLUDED.name").
		Returning("*").
		Scan(q.ctx)
	return p, err
}
