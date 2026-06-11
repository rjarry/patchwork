// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"fmt"
	"net/http"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func apiBase(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	return fmt.Sprintf("%s://%s/api/1.5", scheme, r.Host)
}

func setPatchURLs(r *http.Request, patches []db.Patch) {
	base := apiBase(r)
	for i := range patches {
		p := &patches[i]
		p.URL = fmt.Sprintf("%s/patches/%d/", base, p.ID)
		p.MboxURL = fmt.Sprintf("%s/patches/%d/mbox/", base, p.ID)
		p.CommentsURL = fmt.Sprintf("%s/patches/%d/comments/", base, p.ID)
		p.ChecksURL = fmt.Sprintf("%s/patches/%d/checks/", base, p.ID)
		if p.Project != nil && p.Project.WebURL != "" {
			p.WebURL = fmt.Sprintf("%s/patch/%s/",
				p.Project.WebURL, p.Msgid)
		}
		p.ListArchiveURL = listArchiveURL(p.Project, p.Msgid)
		if p.SeriesList == nil {
			p.SeriesList = []db.SeriesRef{}
		}
	}
}

func setCoverURLs(r *http.Request, covers []db.Cover) {
	base := apiBase(r)
	for i := range covers {
		c := &covers[i]
		c.URL = fmt.Sprintf("%s/covers/%d/", base, c.ID)
		c.MboxURL = fmt.Sprintf("%s/covers/%d/mbox/", base, c.ID)
		c.Comments = fmt.Sprintf("%s/covers/%d/comments/", base, c.ID)
		if c.Project != nil && c.Project.WebURL != "" {
			c.WebURL = fmt.Sprintf("%s/cover/%s/",
				c.Project.WebURL, c.Msgid)
		}
		c.ListArchiveURL = listArchiveURL(c.Project, c.Msgid)
		if c.SeriesList == nil {
			c.SeriesList = []db.SeriesRef{}
		}
	}
}

func setSeriesURLs(r *http.Request, series []db.Series) {
	base := apiBase(r)
	for i := range series {
		s := &series[i]
		s.URL = fmt.Sprintf("%s/series/%d/", base, s.ID)
		s.MboxURL = fmt.Sprintf("%s/series/%d/mbox/", base, s.ID)
		if s.Project != nil && s.Project.WebURL != "" {
			s.WebURL = fmt.Sprintf("%s/project/%s/list/?series=%d",
				s.Project.WebURL, s.Project.Linkname, s.ID)
		}
	}
}

func setProjectURLs(r *http.Request, projects []db.Project) {
	base := apiBase(r)
	for i := range projects {
		projects[i].APIURL = fmt.Sprintf("%s/projects/%d/", base, projects[i].ID)
	}
}

func setPersonURLs(r *http.Request, people []db.Person) {
	base := apiBase(r)
	for i := range people {
		people[i].URL = fmt.Sprintf("%s/people/%d/", base, people[i].ID)
	}
}

func setBundleURLs(r *http.Request, bundles []db.Bundle) {
	base := apiBase(r)
	for i := range bundles {
		b := &bundles[i]
		b.URL = fmt.Sprintf("%s/bundles/%d/", base, b.ID)
		b.MboxURL = fmt.Sprintf("%s/bundles/%d/mbox/", base, b.ID)
	}
}
