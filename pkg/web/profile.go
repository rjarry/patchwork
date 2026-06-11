// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/log"
)

type profileData struct {
	PC           pageContext
	User         *db.User
	Profile      *db.UserProfile
	LinkedEmails []linkedEmail
	Bundles      []db.Bundle
	Token        string
}

type linkedEmail struct {
	PersonID int32
	Email    string
	OptedOut bool
}

func (h *webHandler) profilePage(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	user := getWebUser(r)
	pc := h.pageCtx(r)

	// load or create profile
	var profile db.UserProfile
	err := h.db.NewSelect().Model(&profile).
		Where("user_id = ?", user.ID).Scan(ctx)
	if err != nil {
		profile = db.UserProfile{
			UserID:       user.ID,
			ItemsPerPage: 100,
		}
		db.Insert(ctx, h.db, &profile)
	}

	// load linked emails
	var people []db.Person
	h.db.NewSelect().Model(&people).
		Where("user_id = ?", user.ID).
		OrderExpr("email ASC").
		Scan(ctx)

	var emails []linkedEmail
	for _, p := range people {
		optedOut := false
		count, _ := h.db.NewSelect().Model((*db.EmailOptout)(nil)).
			Where("email = ?", p.Email).Count(ctx)
		if count > 0 {
			optedOut = true
		}
		emails = append(emails, linkedEmail{
			PersonID: p.ID,
			Email:    p.Email,
			OptedOut: optedOut,
		})
	}

	// load bundles
	var bundles []db.Bundle
	h.db.NewSelect().Model(&bundles).
		Where("owner_id = ?", user.ID).
		OrderExpr("name ASC").
		Scan(ctx)
	for i := range bundles {
		bundles[i].Owner = user
	}

	// load API token
	var token string
	h.db.NewRaw(`SELECT key FROM authtoken_token WHERE user_id = ?`, user.ID).
		Scan(ctx, &token)

	data := profileData{
		PC:           pc,
		User:         user,
		Profile:      &profile,
		LinkedEmails: emails,
		Bundles:      bundles,
		Token:        token,
	}
	profilePage(data).Render(ctx, w)
}

func (h *webHandler) profileUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	user := getWebUser(r)

	if !h.validateCSRF(r) {
		http.Redirect(w, r, "/user/", http.StatusFound)
		return
	}

	r.ParseForm()
	itemsPerPage, _ := strconv.Atoi(r.FormValue("items_per_page"))
	if itemsPerPage < 1 {
		itemsPerPage = 100
	}
	showIds := r.FormValue("show_ids") == "on"

	h.db.NewUpdate().Model((*db.UserProfile)(nil)).
		Where("user_id = ?", user.ID).
		Set("items_per_page = ?", itemsPerPage).
		Set("show_ids = ?", showIds).
		Exec(ctx)

	http.Redirect(w, r, "/user/", http.StatusFound)
}

func (h *webHandler) linkEmail(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	user := getWebUser(r)
	pc := h.pageCtx(r)

	if r.Method == "GET" {
		linkEmailPage(pc, "").Render(ctx, w)
		return
	}

	if !h.validateCSRF(r) {
		linkEmailPage(pc, "Invalid request.").Render(ctx, w)
		return
	}

	r.ParseForm()
	email := r.FormValue("email")
	if email == "" {
		linkEmailPage(pc, "Email is required.").Render(ctx, w)
		return
	}

	conf, err := db.CreateEmailConfirmation(ctx, h.db, "userperson", email, &user.ID)
	if err != nil {
		log.Errorf("link email: %s", err)
		linkEmailPage(pc, "Failed to create confirmation.").Render(ctx, w)
		return
	}

	baseURL := h.cfg.Http.BaseURL
	if baseURL == "" {
		baseURL = "http://" + r.Host
	}
	link := fmt.Sprintf("%s/confirm/%s/", baseURL, conf.Key)
	body := fmt.Sprintf(
		"Please click the following link to link this email to your Patchwork account:\n\n%s\n",
		link,
	)
	err = SendEmail(&h.cfg.SMTP, email, "Patchwork email confirmation", body)
	if err != nil {
		log.Errorf("link email: send: %s", err)
	}

	confirmResultPage(pc, fmt.Sprintf("A confirmation email has been sent to %s.", email)).
		Render(ctx, w)
}

func (h *webHandler) unlinkEmail(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	user := getWebUser(r)

	if !h.validateCSRF(r) {
		http.Redirect(w, r, "/user/", http.StatusFound)
		return
	}

	personID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		http.Redirect(w, r, "/user/", http.StatusFound)
		return
	}

	var person db.Person
	err = h.db.NewSelect().Model(&person).
		Where("id = ?", personID).
		Where("user_id = ?", user.ID).
		Scan(ctx)
	if err != nil {
		http.Redirect(w, r, "/user/", http.StatusFound)
		return
	}

	if person.Email == user.Email {
		http.Redirect(w, r, "/user/", http.StatusFound)
		return
	}

	h.db.NewUpdate().Model(&person).
		Where("id = ?", person.ID).
		Set("user_id = NULL").
		Exec(ctx)

	http.Redirect(w, r, "/user/", http.StatusFound)
}

func (h *webHandler) changePassword(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	user := getWebUser(r)
	pc := h.pageCtx(r)

	if r.Method == "GET" {
		changePasswordPage(pc, "").Render(ctx, w)
		return
	}

	if !h.validateCSRF(r) {
		changePasswordPage(pc, "Invalid request.").Render(ctx, w)
		return
	}

	r.ParseForm()
	oldPassword := r.FormValue("old_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	if !db.CheckPassword(oldPassword, user.Password) {
		changePasswordPage(pc, "Current password is incorrect.").Render(ctx, w)
		return
	}
	if newPassword == "" {
		changePasswordPage(pc, "New password is required.").Render(ctx, w)
		return
	}
	if newPassword != confirmPassword {
		changePasswordPage(pc, "New passwords do not match.").Render(ctx, w)
		return
	}

	h.db.NewUpdate().Model((*db.User)(nil)).
		Where("id = ?", user.ID).
		Set("password = ?", db.HashPassword(newPassword)).
		Exec(ctx)

	http.Redirect(w, r, "/user/", http.StatusFound)
}

func (h *webHandler) generateToken(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	user := getWebUser(r)

	if !h.validateCSRF(r) {
		http.Redirect(w, r, "/user/", http.StatusFound)
		return
	}

	h.db.NewRaw(`DELETE FROM authtoken_token WHERE user_id = ?`, user.ID).Exec(ctx)

	key := make([]byte, 20)
	rand.Read(key)
	token := hex.EncodeToString(key)

	h.db.NewRaw(`INSERT INTO authtoken_token (key, created, user_id) VALUES (?, ?, ?)`,
		token, time.Now(), user.ID).Exec(ctx)

	http.Redirect(w, r, "/user/", http.StatusFound)
}

func (h *webHandler) passwordReset(w http.ResponseWriter, r *http.Request) {
	pc := h.pageCtx(r)

	if r.Method == "GET" {
		passwordResetPage(pc, "").Render(r.Context(), w)
		return
	}

	if !h.validateCSRF(r) {
		passwordResetPage(pc, "Invalid request.").Render(r.Context(), w)
		return
	}

	ctx := r.Context()
	r.ParseForm()
	email := r.FormValue("email")

	var user db.User
	err := h.db.NewSelect().Model(&user).
		Where("LOWER(email) = LOWER(?)", email).
		Where("is_active = ?", true).
		Scan(ctx)
	if err != nil {
		// don't reveal whether user exists
		confirmResultPage(pc, "If an account exists with that email, a reset link has been sent.").
			Render(ctx, w)
		return
	}

	conf, err := db.CreateEmailConfirmation(ctx, h.db, "password_reset", email, &user.ID)
	if err != nil {
		log.Errorf("password reset: %s", err)
		confirmResultPage(pc, "If an account exists with that email, a reset link has been sent.").
			Render(ctx, w)
		return
	}

	baseURL := h.cfg.Http.BaseURL
	if baseURL == "" {
		baseURL = "http://" + r.Host
	}
	link := fmt.Sprintf("%s/password-reset/%s/", baseURL, conf.Key)
	body := fmt.Sprintf(
		"Please click the following link to reset your password:\n\n%s\n\nThis link will expire in 7 days.\n",
		link,
	)
	SendEmail(&h.cfg.SMTP, email, "Patchwork password reset", body)

	confirmResultPage(pc, "If an account exists with that email, a reset link has been sent.").
		Render(ctx, w)
}

func (h *webHandler) passwordResetConfirm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pc := h.pageCtx(r)
	key := chi.URLParam(r, "key")

	var conf db.EmailConfirmation
	err := h.db.NewSelect().Model(&conf).
		Where("key = ?", key).
		Where("type = ?", "password_reset").
		Scan(ctx)
	if err != nil || !conf.IsValid() {
		confirmResultPage(pc, "Invalid or expired reset link.").Render(ctx, w)
		return
	}

	if r.Method == "GET" {
		passwordResetConfirmPage(pc, key, "").Render(ctx, w)
		return
	}

	if !h.validateCSRF(r) {
		passwordResetConfirmPage(pc, key, "Invalid request.").Render(ctx, w)
		return
	}

	r.ParseForm()
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	if newPassword == "" {
		passwordResetConfirmPage(pc, key, "Password is required.").Render(ctx, w)
		return
	}
	if newPassword != confirmPassword {
		passwordResetConfirmPage(pc, key, "Passwords do not match.").Render(ctx, w)
		return
	}

	h.db.NewUpdate().Model((*db.User)(nil)).
		Where("id = ?", *conf.UserID).
		Set("password = ?", db.HashPassword(newPassword)).
		Exec(ctx)

	h.db.NewUpdate().Model(&conf).
		Where("id = ?", conf.ID).
		Set("active = ?", false).
		Exec(ctx)

	confirmResultPage(pc, "Your password has been reset. You can now log in.").Render(ctx, w)
}
