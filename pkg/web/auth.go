// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/log"
)

var usernameRe = regexp.MustCompile(`^\w+$`)

func (h *webHandler) registerPage(w http.ResponseWriter, r *http.Request) {
	registerPage(h.pageCtx(r), nil).Render(r.Context(), w)
}

func (h *webHandler) registerSubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pc := h.pageCtx(r)
	r.ParseForm()

	if !h.validateCSRF(r) {
		registerPage(pc, []string{"Invalid request. Please try again."}).Render(ctx, w)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	firstName := strings.TrimSpace(r.FormValue("first_name"))
	lastName := strings.TrimSpace(r.FormValue("last_name"))

	var errors []string
	if username == "" || !usernameRe.MatchString(username) {
		errors = append(errors, "Username must contain only letters, digits, and underscores.")
	}
	if email == "" || !strings.Contains(email, "@") {
		errors = append(errors, "Enter a valid email address.")
	}
	if password == "" {
		errors = append(errors, "Password is required.")
	}
	if len(username) > 30 {
		errors = append(errors, "Username must be 30 characters or fewer.")
	}

	if len(errors) == 0 {
		var count int
		count, _ = h.db.NewSelect().Model((*db.User)(nil)).
			Where("LOWER(username) = LOWER(?)", username).
			Count(ctx)
		if count > 0 {
			errors = append(errors, "A user with that username already exists.")
		}
	}
	if len(errors) == 0 {
		var count int
		count, _ = h.db.NewSelect().Model((*db.User)(nil)).
			Where("LOWER(email) = LOWER(?)", email).
			Count(ctx)
		if count > 0 {
			errors = append(errors, "A user with that email already exists.")
		}
	}

	if len(errors) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		registerPage(pc, errors).Render(ctx, w)
		return
	}

	user := db.User{
		Username:   username,
		Email:      email,
		Password:   db.HashPassword(password),
		FirstName:  firstName,
		LastName:   lastName,
		IsActive:   false,
		IsStaff:    false,
		DateJoined: time.Now(),
	}
	err := db.Insert(ctx, h.db, &user)
	if err != nil {
		log.Errorf("register: insert user: %s", err)
		registerPage(pc, []string{"Registration failed. Please try again."}).Render(ctx, w)
		return
	}

	conf, err := db.CreateEmailConfirmation(ctx, h.db, "registration", email, &user.ID)
	if err != nil {
		log.Errorf("register: create confirmation: %s", err)
		registerPage(pc, []string{"Registration failed. Please try again."}).Render(ctx, w)
		return
	}

	baseURL := h.cfg.Http.BaseURL
	if baseURL == "" {
		baseURL = "http://" + r.Host
	}
	link := fmt.Sprintf("%s/confirm/%s/", baseURL, conf.Key)
	body := fmt.Sprintf(
		"Please click the following link to confirm your registration:\n\n%s\n\nThis link will expire in 7 days.\n",
		link,
	)
	err = SendEmail(&h.cfg.SMTP, email, "Patchwork registration confirmation", body)
	if err != nil {
		log.Errorf("register: send email: %s", err)
	}

	registerConfirmSentPage(pc, email).Render(ctx, w)
}

func (h *webHandler) confirmHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pc := h.pageCtx(r)
	key := chi.URLParam(r, "key")

	var conf db.EmailConfirmation
	err := h.db.NewSelect().Model(&conf).
		Where("key = ?", key).
		Scan(ctx)
	if err != nil {
		confirmResultPage(pc, "Confirmation not found.").Render(ctx, w)
		return
	}

	if !conf.IsValid() {
		msg := "This confirmation has expired."
		if !conf.Active {
			msg = "This confirmation has already been used."
		}
		confirmResultPage(pc, msg).Render(ctx, w)
		return
	}

	switch conf.Type {
	case "registration":
		h.confirmRegistration(w, r, &conf, pc)
	case "userperson":
		h.confirmLink(w, r, &conf, pc)
	default:
		confirmResultPage(pc, "Unknown confirmation type.").Render(ctx, w)
	}
}

func (h *webHandler) confirmRegistration(w http.ResponseWriter, r *http.Request, conf *db.EmailConfirmation, pc pageContext) {
	ctx := r.Context()

	if conf.UserID == nil {
		confirmResultPage(pc, "Invalid confirmation.").Render(ctx, w)
		return
	}

	_, err := h.db.NewUpdate().Model((*db.User)(nil)).
		Where("id = ?", *conf.UserID).
		Set("is_active = ?", true).
		Exec(ctx)
	if err != nil {
		log.Errorf("confirm registration: activate user: %s", err)
		confirmResultPage(pc, "Confirmation failed.").Render(ctx, w)
		return
	}

	// link person to user
	var person db.Person
	err = h.db.NewSelect().Model(&person).
		Where("LOWER(email) = LOWER(?)", conf.Email).
		Scan(ctx)
	if err != nil {
		person = db.Person{Email: conf.Email}
		db.Insert(ctx, h.db, &person)
	}
	h.db.NewUpdate().Model(&person).
		Where("id = ?", person.ID).
		Set("user_id = ?", *conf.UserID).
		Exec(ctx)

	// deactivate confirmation
	h.db.NewUpdate().Model(conf).
		Where("id = ?", conf.ID).
		Set("active = ?", false).
		Exec(ctx)

	confirmResultPage(pc, "Your registration has been confirmed. You can now log in.").Render(ctx, w)
}

func (h *webHandler) confirmLink(w http.ResponseWriter, r *http.Request, conf *db.EmailConfirmation, pc pageContext) {
	ctx := r.Context()

	if conf.UserID == nil {
		confirmResultPage(pc, "Invalid confirmation.").Render(ctx, w)
		return
	}

	var person db.Person
	err := h.db.NewSelect().Model(&person).
		Where("LOWER(email) = LOWER(?)", conf.Email).
		Scan(ctx)
	if err != nil {
		person = db.Person{Email: conf.Email}
		db.Insert(ctx, h.db, &person)
	}
	h.db.NewUpdate().Model(&person).
		Where("id = ?", person.ID).
		Set("user_id = ?", *conf.UserID).
		Exec(ctx)

	h.db.NewUpdate().Model(conf).
		Where("id = ?", conf.ID).
		Set("active = ?", false).
		Exec(ctx)

	confirmResultPage(pc, "Your email address has been linked to your account.").Render(ctx, w)
}
