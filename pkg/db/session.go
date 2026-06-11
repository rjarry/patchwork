// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

type Session struct {
	bun.BaseModel `bun:"table:django_session"`

	SessionKey  string    `bun:"session_key,pk"`
	SessionData string    `bun:"session_data,notnull"`
	ExpireDate  time.Time `bun:"expire_date,notnull"`
}

const sessionMaxAge = 14 * 24 * time.Hour // 2 weeks

func CreateSession(ctx context.Context, database *bun.DB, userID int32) (string, error) {
	key := make([]byte, 20)
	rand.Read(key)
	sessionKey := hex.EncodeToString(key)

	session := Session{
		SessionKey:  sessionKey,
		SessionData: encodeSessionData(userID),
		ExpireDate:  time.Now().Add(sessionMaxAge),
	}
	_, err := database.NewInsert().Model(&session).Exec(ctx)
	if err != nil {
		return "", err
	}
	return sessionKey, nil
}

func GetSessionUser(ctx context.Context, database *bun.DB, sessionKey string) (*User, error) {
	var session Session
	err := database.NewSelect().Model(&session).
		Where("session_key = ?", sessionKey).
		Where("expire_date > ?", time.Now()).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	userID := decodeSessionData(session.SessionData)
	if userID == 0 {
		return nil, err
	}

	var user User
	err = database.NewSelect().Model(&user).
		Where("id = ?", userID).
		Where("is_active = ?", true).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func DeleteSession(ctx context.Context, database *bun.DB, sessionKey string) {
	database.NewDelete().Model((*Session)(nil)).
		Where("session_key = ?", sessionKey).
		Exec(ctx)
}

func CleanExpiredSessions(ctx context.Context, database *bun.DB) {
	database.NewDelete().Model((*Session)(nil)).
		Where("expire_date <= ?", time.Now()).
		Exec(ctx)
}

func encodeSessionData(userID int32) string {
	// Simple format: just the user ID as a string.
	// Django uses base64-encoded JSON but we don't need compatibility
	// since we create our own sessions.
	return fmt.Sprintf(`{"_auth_user_id":"%d"}`, userID)
}

func decodeSessionData(data string) int32 {
	// Parse our simple JSON format
	var id int32
	fmt.Sscanf(data, `{"_auth_user_id":"%d"}`, &id)
	return id
}
