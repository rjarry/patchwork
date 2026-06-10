// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

//go:generate go tool templ generate -lazy

import (
	"embed"
	"fmt"
	"html"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

//go:embed static/*
var staticFS embed.FS

func NewRouter(database *bun.DB) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	h := &webHandler{db: database}

	sub, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))

	r.Get("/", h.projectList)
	r.Get("/project/{linkname}/", h.projectDetail)
	r.Get("/project/{linkname}/list/", h.patchList)
	r.Get("/project/{linkname}/patch/{msgid}/*", h.patchDetailRoute)
	r.Get("/project/{linkname}/cover/{msgid}/*", h.coverDetailRoute)

	r.Get("/patch/{id}", h.patchRedirect)
	r.Get("/patch/{id}/", h.patchRedirect)
	r.Get("/patch/{id}/mbox/", h.patchMboxByID)
	r.Get("/patch/{id}/raw/", h.patchRawByID)
	r.Get("/cover/{id}", h.coverRedirect)
	r.Get("/cover/{id}/", h.coverRedirect)
	r.Get("/cover/{id}/mbox/", h.coverMboxByID)

	r.Get("/series/{id}/mbox/", h.seriesMbox)
	r.Get("/bundle/{username}/{bundlename}/mbox/", h.bundleMbox)

	r.Get("/comment/{id}/", h.commentRedirect)

	return r
}

type webHandler struct {
	db *bun.DB
}

func intStr(n int) string {
	return strconv.Itoa(n)
}

func personName(p *db.Person) string {
	if p == nil {
		return ""
	}
	if p.Name != nil && *p.Name != "" {
		return *p.Name
	}
	return p.Email
}

func patchURL(linkname, msgid string) string {
	clean := strings.TrimPrefix(msgid, "<")
	clean = strings.TrimSuffix(clean, ">")
	return fmt.Sprintf("/project/%s/patch/%s/", linkname, url.PathEscape(clean))
}

func coverURL(linkname, msgid string) string {
	clean := strings.TrimPrefix(msgid, "<")
	clean = strings.TrimSuffix(clean, ">")
	return fmt.Sprintf("/project/%s/cover/%s/", linkname, url.PathEscape(clean))
}

func sortURL(d patchListData, newSort string) string {
	return fmt.Sprintf("/project/%s/list/?order=%s%s",
		d.Project.Linkname, newSort, d.BaseQuery)
}

func pageURL(baseQuery string, page int) string {
	if strings.Contains(baseQuery, "page=") {
		return baseQuery
	}
	sep := "&"
	if baseQuery == "" {
		sep = "?"
	}
	return fmt.Sprintf("%s%spage=%d", baseQuery, sep, page)
}

func checkClass(s db.CheckState) string {
	switch int16(s) {
	case 0:
		return "check-pending"
	case 1:
		return "check-success"
	case 2:
		return "check-warning"
	case 3:
		return "check-fail"
	}
	return ""
}

func checkLabel(s db.CheckState) string {
	switch int16(s) {
	case 0:
		return "pending"
	case 1:
		return "success"
	case 2:
		return "warning"
	case 3:
		return "fail"
	}
	return "unknown"
}

func highlightDiff(diff string) string {
	var b strings.Builder
	for _, line := range strings.Split(diff, "\n") {
		escaped := html.EscapeString(line)
		switch {
		case strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- "):
			b.WriteString(`<span class="diff-head">`)
			b.WriteString(escaped)
			b.WriteString("</span>\n")
		case strings.HasPrefix(line, "+"):
			b.WriteString(`<span class="diff-add">`)
			b.WriteString(escaped)
			b.WriteString("</span>\n")
		case strings.HasPrefix(line, "-"):
			b.WriteString(`<span class="diff-del">`)
			b.WriteString(escaped)
			b.WriteString("</span>\n")
		case strings.HasPrefix(line, "@@"):
			b.WriteString(`<span class="diff-hunk">`)
			b.WriteString(escaped)
			b.WriteString("</span>\n")
		case strings.HasPrefix(line, "diff "):
			b.WriteString(`<span class="diff-head">`)
			b.WriteString(escaped)
			b.WriteString("</span>\n")
		default:
			b.WriteString(escaped)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func notFoundPage(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Not found"))
}
