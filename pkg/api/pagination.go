// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const (
	defaultPerPage = 30
	maxPerPage     = 250
)

type page struct {
	PerPage int
	Number  int
	Offset  int
}

func parsePage(r *http.Request) page {
	p := page{PerPage: defaultPerPage, Number: 1}

	if v := r.URL.Query().Get("per_page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			p.PerPage = n
		}
	}
	if p.PerPage > maxPerPage {
		p.PerPage = maxPerPage
	}

	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			p.Number = n
		}
	}

	p.Offset = (p.Number - 1) * p.PerPage
	return p
}

func setLinkHeader(w http.ResponseWriter, r *http.Request, p page, total int) {
	lastPage := (total + p.PerPage - 1) / p.PerPage
	if lastPage < 1 {
		lastPage = 1
	}

	base := *r.URL
	q := base.Query()

	link := func(pageNum int, rel string) string {
		q.Set("page", strconv.Itoa(pageNum))
		base.RawQuery = q.Encode()
		return fmt.Sprintf("<%s>; rel=\"%s\"", base.String(), rel)
	}

	var parts []string
	parts = append(parts, link(1, "first"))
	parts = append(parts, link(lastPage, "last"))
	if p.Number > 1 {
		parts = append(parts, link(p.Number-1, "prev"))
	}
	if p.Number < lastPage {
		parts = append(parts, link(p.Number+1, "next"))
	}

	w.Header().Set("Link", strings.Join(parts, ", "))
}
