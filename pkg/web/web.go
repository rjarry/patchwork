// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
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
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/config"
	"github.com/getpatchwork/patchwork/pkg/db"
)

//go:embed static/*
var staticFS embed.FS

func NewRouter(cfg *config.Config, database *bun.DB) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.StripSlashes)

	h := &webHandler{cfg: cfg, db: database}
	r.Use(h.sessionMiddleware)

	sub, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))

	r.Get("/", h.projectList)
	r.Get("/project/{linkname}", h.projectDetail)
	r.Get("/project/{linkname}/list", h.patchList)
	r.Get("/project/{linkname}/bundles", h.projectBundleList)
	r.Post("/project/{linkname}/list", h.patchListAction)
	r.Get("/project/{linkname}/patch/{msgid}", h.patchDetailPage)
	r.Post("/project/{linkname}/patch/{msgid}", h.patchUpdate)
	r.Get("/project/{linkname}/patch/{msgid}/mbox", h.patchMboxPage)
	r.Get("/project/{linkname}/patch/{msgid}/raw", h.patchRawPage)
	r.Get("/project/{linkname}/cover/{msgid}", h.coverDetailPage)
	r.Get("/project/{linkname}/cover/{msgid}/mbox", h.coverMboxPage)

	r.Get("/patch/{id}", h.patchRedirect)
	r.Get("/patch/{id}/mbox", h.patchMboxByID)
	r.Get("/patch/{id}/raw", h.patchRawByID)
	r.Get("/cover/{id}", h.coverRedirect)
	r.Get("/cover/{id}/mbox", h.coverMboxByID)

	r.Get("/series/{id}/mbox", h.seriesMbox)
	r.Get("/bundle/{username}/{bundlename}/mbox", h.bundleMbox)

	r.Get("/comment/{id}", h.commentRedirect)

	r.Get("/user/login", h.loginPage)
	r.Post("/user/login", h.loginSubmit)
	r.Post("/user/logout", h.logoutSubmit)

	r.Get("/register", h.registerPage)
	r.Post("/register", h.registerSubmit)
	r.Get("/confirm/{key}", h.confirmHandler)

	r.Get("/user", h.profilePage)
	r.Post("/user", h.profileUpdate)
	r.Get("/user/link", h.linkEmail)
	r.Post("/user/link", h.linkEmail)
	r.Post("/user/unlink/{id}", h.unlinkEmail)
	r.Get("/user/password-change", h.changePassword)
	r.Post("/user/password-change", h.changePassword)
	r.Post("/user/generate-token", h.generateToken)
	r.Get("/user/password-reset", h.passwordReset)
	r.Post("/user/password-reset", h.passwordReset)
	r.Get("/password-reset/{key}", h.passwordResetConfirm)
	r.Post("/password-reset/{key}", h.passwordResetConfirm)

	r.Get("/user/todo", h.todoLists)
	r.Get("/user/todo/{linkname}", h.todoList)
	r.Get("/user/bundles", h.bundleList)
	r.Get("/bundle/{username}/{bundlename}", h.bundleDetail)
	r.Post("/bundle/{username}/{bundlename}", h.bundleUpdate)

	return r
}

type webHandler struct {
	cfg *config.Config
	db  *bun.DB
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

func highlightDiff(diff string) string {
	var b strings.Builder
	for _, line := range strings.Split(diff, "\n") {
		escaped := html.EscapeString(line)
		switch {
		case strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- "):
			b.WriteString("<b>")
			b.WriteString(escaped)
			b.WriteString("</b>\n")
		case strings.HasPrefix(line, "+"):
			b.WriteString("<ins>")
			b.WriteString(escaped)
			b.WriteString("</ins>\n")
		case strings.HasPrefix(line, "-"):
			b.WriteString("<del>")
			b.WriteString(escaped)
			b.WriteString("</del>\n")
		case strings.HasPrefix(line, "@@"):
			b.WriteString("<mark>")
			b.WriteString(escaped)
			b.WriteString("</mark>\n")
		case strings.HasPrefix(line, "diff "):
			b.WriteString("<b>")
			b.WriteString(escaped)
			b.WriteString("</b>\n")
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

var diffstatRe = regexp.MustCompile(`^(\s\S+\s+\|\s+\d+\s)([+-]+)$`)

func highlightMessage(text string) string {
	var b strings.Builder
	for _, line := range strings.Split(text, "\n") {
		escaped := html.EscapeString(line)
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "signed-off-by:"):
			b.WriteString("<sob>")
			b.WriteString(escaped)
			b.WriteString("</sob>\n")
		case strings.HasPrefix(lower, "acked-by:"),
			strings.HasPrefix(lower, "reviewed-by:"),
			strings.HasPrefix(lower, "tested-by:"),
			strings.HasPrefix(lower, "reported-by:"),
			strings.HasPrefix(lower, "nacked-by:"):
			b.WriteString("<trailer>")
			b.WriteString(escaped)
			b.WriteString("</trailer>\n")
		case strings.HasPrefix(line, "> ") || line == ">":
			b.WriteString("<q>")
			b.WriteString(escaped)
			b.WriteString("</q>\n")
		case diffstatRe.MatchString(line):
			m := diffstatRe.FindStringSubmatch(line)
			prefix := html.EscapeString(m[1])
			bar := m[2]
			b.WriteString(html.EscapeString(prefix))
			for _, c := range bar {
				if c == '+' {
					b.WriteString(`<ins>+</ins>`)
				} else {
					b.WriteString(`<del>-</del>`)
				}
			}
			b.WriteByte('\n')
		default:
			b.WriteString(escaped)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func commitMessage(p db.Patch) string {
	msg := p.Name
	if p.Content != nil && *p.Content != "" {
		msg += "\n\n" + *p.Content
	}
	return msg
}

func coverMessage(c db.Cover) string {
	msg := c.Name
	if c.Content != nil && *c.Content != "" {
		msg += "\n\n" + *c.Content
	}
	return msg
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func (h *webHandler) pageCtx(r *http.Request) pageContext {
	return pageContext{
		User:      getWebUser(r),
		CSRFToken: h.csrfToken(r),
	}
}

func notFoundPage(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Not found"))
}
