// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"context"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/getpatchwork/patchwork/pkg/db"
)

var (
	postscriptRe = regexp.MustCompile(`(?m)^-{2,3} ?$`)
	responseRe   = regexp.MustCompile(`(?mi)^(Tested|Reviewed|Acked|Signed-off|Nacked|Reported)-by:.*$`)
)

func submissionToMbox(submission mboxSubmission) string {
	isPatch := submission.Diff != ""

	body := ""
	if submission.Content != "" {
		body = strings.TrimSpace(submission.Content) + "\n"
	}

	postscript := ""
	if loc := postscriptRe.FindStringIndex(body); loc != nil {
		postscript = body[loc[1]:]
		body = strings.TrimSpace(body[:loc[0]]) + "\n"
		postscript = strings.TrimRight(postscript, " \t\n")
	}

	// append tag lines from comments
	for _, content := range submission.CommentContents {
		for _, m := range responseRe.FindAllString(content, -1) {
			body += m + "\n"
		}
	}

	if postscript != "" {
		body += "---" + postscript + "\n"
	}

	if isPatch && submission.Diff != "" {
		body += "\n" + submission.Diff
	}

	// build headers
	var hdr strings.Builder

	fromLine := "From patchwork " + submission.Date.UTC().Format("Mon Jan  2 15:04:05 2006") + "\n"

	submitterName := submission.SubmitterEmail
	if submission.SubmitterName != "" {
		submitterName = submission.SubmitterName
	}
	xSubmitter := formatAddr(submitterName, submission.SubmitterEmail)

	origHeaders := parseHeaders(submission.Headers)

	hdr.WriteString(fromLine)

	dmarcReplaced := false
	for _, h := range origHeaders {
		key := h.key
		val := h.val

		if strings.EqualFold(key, "Content-Type") {
			if strings.Contains(val, "multipart/signed") {
				continue
			}
			continue
		}
		if strings.EqualFold(key, "Content-Transfer-Encoding") {
			continue
		}

		if strings.EqualFold(key, "From") {
			_, addr := parseFromHeader(val)
			if addr == submission.ListEmail {
				hdr.WriteString("X-Patchwork-Original-From: " + val + "\n")
				val = xSubmitter
				dmarcReplaced = true
			}
		}

		hdr.WriteString(key + ": " + val + "\n")
	}

	_ = dmarcReplaced

	hasDate := false
	for _, h := range origHeaders {
		if strings.EqualFold(h.key, "Date") {
			hasDate = true
			break
		}
	}
	if !hasDate {
		hdr.WriteString("Date: " + submission.Date.UTC().Format(time.RFC1123Z) + "\n")
	}

	hdr.WriteString("X-Patchwork-Submitter: " + xSubmitter + "\n")
	hdr.WriteString("X-Patchwork-Id: " + strconv.Itoa(int(submission.ID)) + "\n")
	if isPatch && submission.DelegateEmail != "" {
		hdr.WriteString("X-Patchwork-Delegate: " + submission.DelegateEmail + "\n")
	}

	hdr.WriteString("Content-Type: text/plain; charset=utf-8\n")
	hdr.WriteString("Content-Transfer-Encoding: 8bit\n")
	hdr.WriteString("\n")
	hdr.WriteString(body)

	return hdr.String()
}

type mboxSubmission struct {
	ID              int32
	Date            time.Time
	Content         string
	Diff            string
	Headers         string
	SubmitterName   string
	SubmitterEmail  string
	DelegateEmail   string
	ListEmail       string
	CommentContents []string
}

type headerPair struct {
	key, val string
}

func parseHeaders(raw string) []headerPair {
	var headers []headerPair
	var currentKey, currentVal string

	for _, line := range strings.Split(raw, "\n") {
		if line == "" {
			continue
		}
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			currentVal += "\n" + line
		} else {
			if currentKey != "" {
				headers = append(headers, headerPair{currentKey, currentVal})
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentKey = strings.TrimSpace(parts[0])
				currentVal = strings.TrimSpace(parts[1])
			} else {
				currentKey = ""
				currentVal = ""
			}
		}
	}
	if currentKey != "" {
		headers = append(headers, headerPair{currentKey, currentVal})
	}
	return headers
}

func parseFromHeader(from string) (name, addr string) {
	a, err := mail.ParseAddress(from)
	if err != nil {
		// fallback: try to extract bare email
		from = strings.TrimSpace(from)
		if strings.Contains(from, "<") {
			parts := strings.SplitN(from, "<", 2)
			name = strings.TrimSpace(parts[0])
			addr = strings.Trim(parts[1], "> ")
		} else {
			addr = from
		}
		return
	}
	return a.Name, a.Address
}

func formatAddr(name, email string) string {
	if name == "" || name == email {
		return email
	}
	return fmt.Sprintf("%s <%s>", name, email)
}

func (h *webHandler) patchMboxPage(w http.ResponseWriter, r *http.Request) {
	linkname := chi.URLParam(r, "linkname")
	rawMsgid, _ := url.PathUnescape(chi.URLParam(r, "msgid"))
	ctx := r.Context()
	msgid := "<" + rawMsgid + ">"

	var patch db.Patch
	err := h.db.NewRaw(`
		SELECT p.* FROM patchwork_patch p
		JOIN patchwork_project pr ON pr.id = p.project_id
		WHERE pr.linkname = ? AND p.msgid = ?
	`, linkname, msgid).Scan(ctx, &patch)
	if err != nil {
		notFoundPage(w)
		return
	}

	var project db.Project
	h.db.NewSelect().Model(&project).Where("id = ?", patch.ProjectID).Scan(ctx)

	seriesParam := r.URL.Query().Get("series")
	if seriesParam != "" {
		h.seriesPatchMbox(w, r, patch, project, seriesParam)
		return
	}

	h.servePatchMbox(w, patch, project)
}

func (h *webHandler) servePatchMbox(w http.ResponseWriter, patch db.Patch, project db.Project) {
	ctx := context.Background()
	sub := h.buildMboxSubmission(ctx, patch.ID, patch.Date,
		derefStr(patch.Content), derefStr(patch.Diff),
		patch.Headers, patch.SubmitterID, patch.DelegateID,
		project.Listemail, true)

	mbox := submissionToMbox(sub)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.patch", sanitizeFilename(patch.Name)))
	w.Write([]byte(mbox))
}

func (h *webHandler) coverMboxPage(w http.ResponseWriter, r *http.Request) {
	linkname := chi.URLParam(r, "linkname")
	rawMsgid, _ := url.PathUnescape(chi.URLParam(r, "msgid"))
	ctx := r.Context()
	msgid := "<" + rawMsgid + ">"

	var cover db.Cover
	err := h.db.NewRaw(`
		SELECT c.* FROM patchwork_cover c
		JOIN patchwork_project pr ON pr.id = c.project_id
		WHERE pr.linkname = ? AND c.msgid = ?
	`, linkname, msgid).Scan(ctx, &cover)
	if err != nil {
		notFoundPage(w)
		return
	}

	var project db.Project
	h.db.NewSelect().Model(&project).Where("id = ?", cover.ProjectID).Scan(ctx)

	h.serveCoverMbox(w, cover, project)
}

func (h *webHandler) serveCoverMbox(w http.ResponseWriter, cover db.Cover, project db.Project) {
	ctx := context.Background()
	sub := h.buildCoverMboxSubmission(ctx, cover, project)

	mbox := submissionToMbox(sub)

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.mbox", sanitizeFilename(cover.Name)))
	w.Write([]byte(mbox))
}

func (h *webHandler) seriesMbox(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		notFoundPage(w)
		return
	}

	var series db.Series
	err = h.db.NewSelect().Model(&series).Where("id = ?", id).Scan(ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	var project db.Project
	if series.ProjectID != nil {
		h.db.NewSelect().Model(&project).Where("id = ?", *series.ProjectID).Scan(ctx)
	}

	var patches []db.Patch
	h.db.NewSelect().Model(&patches).
		Where("series_id = ?", series.ID).
		OrderExpr(`"number" ASC`).
		Scan(ctx)

	var parts []string
	for _, p := range patches {
		sub := h.buildMboxSubmission(ctx, p.ID, p.Date,
			derefStr(p.Content), derefStr(p.Diff),
			p.Headers, p.SubmitterID, p.DelegateID,
			project.Listemail, true)
		parts = append(parts, submissionToMbox(sub))
	}

	name := "series"
	if series.Name != nil {
		name = *series.Name
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.patch", sanitizeFilename(name)))
	w.Write([]byte(strings.Join(parts, "\n")))
}

func (h *webHandler) seriesPatchMbox(w http.ResponseWriter, r *http.Request, patch db.Patch, project db.Project, seriesParam string) {
	ctx := r.Context()

	if patch.SeriesID == nil {
		notFoundPage(w)
		return
	}

	if seriesParam != "*" {
		sid, err := strconv.ParseInt(seriesParam, 10, 32)
		if err != nil || int32(sid) != *patch.SeriesID {
			notFoundPage(w)
			return
		}
	}

	var deps []db.Patch
	if patch.Number != nil {
		h.db.NewSelect().Model(&deps).
			Where("series_id = ?", *patch.SeriesID).
			Where(`"number" < ?`, *patch.Number).
			OrderExpr(`"number" ASC`).
			Scan(ctx)
	}

	var parts []string
	for _, dep := range deps {
		sub := h.buildMboxSubmission(ctx, dep.ID, dep.Date,
			derefStr(dep.Content), derefStr(dep.Diff),
			dep.Headers, dep.SubmitterID, dep.DelegateID,
			project.Listemail, true)
		parts = append(parts, submissionToMbox(sub))
	}

	sub := h.buildMboxSubmission(ctx, patch.ID, patch.Date,
		derefStr(patch.Content), derefStr(patch.Diff),
		patch.Headers, patch.SubmitterID, patch.DelegateID,
		project.Listemail, true)
	parts = append(parts, submissionToMbox(sub))

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.patch", sanitizeFilename(patch.Name)))
	w.Write([]byte(strings.Join(parts, "\n")))
}

func (h *webHandler) bundleMbox(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := chi.URLParam(r, "username")
	bundlename := chi.URLParam(r, "bundlename")

	var bundle db.Bundle
	err := h.db.NewRaw(`
		SELECT b.* FROM patchwork_bundle b
		JOIN auth_user u ON u.id = b.owner_id
		WHERE u.username = ? AND b.name = ?
	`, username, bundlename).Scan(ctx, &bundle)
	if err != nil {
		notFoundPage(w)
		return
	}

	if !bundle.Public {
		notFoundPage(w)
		return
	}

	var project db.Project
	h.db.NewSelect().Model(&project).Where("id = ?", bundle.ProjectID).Scan(ctx)

	var patches []db.Patch
	h.db.NewRaw(`
		SELECT p.* FROM patchwork_patch p
		JOIN patchwork_bundlepatch bp ON bp.patch_id = p.id
		WHERE bp.bundle_id = ?
		ORDER BY bp."order" ASC
	`, bundle.ID).Scan(ctx, &patches)

	var parts []string
	for _, p := range patches {
		sub := h.buildMboxSubmission(ctx, p.ID, p.Date,
			derefStr(p.Content), derefStr(p.Diff),
			p.Headers, p.SubmitterID, p.DelegateID,
			project.Listemail, true)
		parts = append(parts, submissionToMbox(sub))
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=bundle-%d-%s.mbox",
			bundle.ID, sanitizeFilename(bundle.Name)))
	w.Write([]byte(strings.Join(parts, "\n")))
}

func (h *webHandler) commentRedirect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		notFoundPage(w)
		return
	}

	// try patch comment first
	var pc struct {
		PatchID int32
	}
	err = h.db.NewRaw(`SELECT patch_id FROM patchwork_patchcomment WHERE id = ?`, id).
		Scan(ctx, &pc)
	if err == nil {
		var patch struct {
			Msgid     string
			ProjectID int32
		}
		h.db.NewRaw(`SELECT msgid, project_id FROM patchwork_patch WHERE id = ?`, pc.PatchID).
			Scan(ctx, &patch)
		var linkname string
		h.db.NewRaw(`SELECT linkname FROM patchwork_project WHERE id = ?`, patch.ProjectID).
			Scan(ctx, &linkname)
		http.Redirect(w, r,
			patchURL(linkname, patch.Msgid)+fmt.Sprintf("#comment-%d", id),
			http.StatusMovedPermanently)
		return
	}

	// try cover comment
	var cc struct {
		CoverID int32
	}
	err = h.db.NewRaw(`SELECT cover_id FROM patchwork_covercomment WHERE id = ?`, id).
		Scan(ctx, &cc)
	if err == nil {
		var cover struct {
			Msgid     string
			ProjectID int32
		}
		h.db.NewRaw(`SELECT msgid, project_id FROM patchwork_cover WHERE id = ?`, cc.CoverID).
			Scan(ctx, &cover)
		var linkname string
		h.db.NewRaw(`SELECT linkname FROM patchwork_project WHERE id = ?`, cover.ProjectID).
			Scan(ctx, &linkname)
		http.Redirect(w, r,
			coverURL(linkname, cover.Msgid)+fmt.Sprintf("#comment-%d", id),
			http.StatusMovedPermanently)
		return
	}

	notFoundPage(w)
}

func (h *webHandler) buildMboxSubmission(
	ctx context.Context, patchID int32, date time.Time,
	content, diff, headers string, submitterID int32, delegateID *int32,
	listEmail string, isPatch bool,
) mboxSubmission {
	sub := mboxSubmission{
		ID:        patchID,
		Date:      date,
		Content:   content,
		Diff:      diff,
		Headers:   headers,
		ListEmail: listEmail,
	}

	var submitter db.Person
	if h.db.NewSelect().Model(&submitter).Where("id = ?", submitterID).Scan(ctx) == nil {
		sub.SubmitterEmail = submitter.Email
		if submitter.Name != nil {
			sub.SubmitterName = *submitter.Name
		}
	}

	if isPatch && delegateID != nil {
		var delegate db.User
		if h.db.NewSelect().Model(&delegate).Where("id = ?", *delegateID).Scan(ctx) == nil {
			sub.DelegateEmail = delegate.Email
		}
	}

	// load comment contents for tag extraction
	if isPatch {
		var contents []string
		h.db.NewRaw(`SELECT content FROM patchwork_patchcomment WHERE patch_id = ? ORDER BY date ASC`, patchID).
			Scan(ctx, &contents)
		sub.CommentContents = contents
	} else {
		var contents []string
		h.db.NewRaw(`SELECT content FROM patchwork_covercomment WHERE cover_id = ? ORDER BY date ASC`, patchID).
			Scan(ctx, &contents)
		sub.CommentContents = contents
	}

	return sub
}

func (h *webHandler) buildCoverMboxSubmission(ctx context.Context, cover db.Cover, project db.Project) mboxSubmission {
	return h.buildMboxSubmission(ctx, cover.ID, cover.Date,
		derefStr(cover.Content), "",
		cover.Headers, cover.SubmitterID, nil,
		project.Listemail, false)
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
