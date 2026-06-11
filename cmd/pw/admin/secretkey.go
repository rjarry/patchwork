// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package admin

import (
	"crypto/rand"
	"fmt"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
)

type SecretKeyCmd struct{}

const secretKeyChars = "abcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*(-_=+)"

func (c *SecretKeyCmd) Run(ctx *pw.Context) error {
	buf := make([]byte, 64)
	if _, err := rand.Read(buf); err != nil {
		return err
	}
	key := make([]byte, 64)
	for i := range key {
		key[i] = secretKeyChars[int(buf[i])%len(secretKeyChars)]
	}
	fmt.Println("# Add the following to patchwork.toml")
	fmt.Println("[http]")
	fmt.Printf("secret-key = %q\n", string(key))
	return nil
}
