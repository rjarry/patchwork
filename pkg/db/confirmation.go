// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"math/big"
	"time"

	"github.com/uptrace/bun"
)

type EmailConfirmation struct {
	bun.BaseModel `bun:"table:patchwork_emailconfirmation"`

	ID     int32     `bun:"id,pk,autoincrement"`
	Type   string    `bun:"type,notnull"`
	Email  string    `bun:"email,notnull"`
	UserID *int32    `bun:"user_id"`
	Key    string    `bun:"key,notnull,unique"`
	Date   time.Time `bun:"date,notnull"`
	Active bool      `bun:"active,notnull"`
}

const confirmationValidityDays = 7

func (c *EmailConfirmation) IsValid() bool {
	return c.Active && time.Since(c.Date) < confirmationValidityDays*24*time.Hour
}

func CreateEmailConfirmation(ctx context.Context, database *bun.DB, confType, email string, userID *int32) (*EmailConfirmation, error) {
	n, _ := rand.Int(rand.Reader, big.NewInt(1<<31))
	raw := fmt.Sprintf("%v%s%d", userID, email, n.Int64())
	key := fmt.Sprintf("%x", sha1.Sum([]byte(raw)))

	conf := &EmailConfirmation{
		Type:   confType,
		Email:  email,
		UserID: userID,
		Key:    key,
		Date:   time.Now(),
		Active: true,
	}
	err := Insert(ctx, database, conf)
	if err != nil {
		return nil, err
	}
	return conf, nil
}

type EmailOptout struct {
	bun.BaseModel `bun:"table:patchwork_emailoptout"`

	Email string `bun:"email,pk"`
}
