// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package admin

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/db"
)

type SuperuserCmd struct {
	List   SuperuserListCmd   `cmd:"" help:"List superusers."`
	Create SuperuserCreateCmd `cmd:"" help:"Create a superuser."`
	Delete SuperuserDeleteCmd `cmd:"" help:"Remove superuser privileges."`
	Passwd SuperuserPasswdCmd `cmd:"" help:"Change a user password."`
}

type SuperuserListCmd struct{}

func (c *SuperuserListCmd) Run(ctx *pw.Context) error {
	var users []db.User
	err := ctx.DB.NewSelect().Model(&users).
		Where("is_superuser = ?", true).
		OrderExpr("username ASC").
		Scan(ctx)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tUSERNAME\tEMAIL\tACTIVE\n")
	for _, u := range users {
		fmt.Fprintf(w, "%d\t%s\t%s\t%v\n",
			u.ID, u.Username, u.Email, u.IsActive)
	}
	return w.Flush()
}

type SuperuserCreateCmd struct {
	Username string `short:"u" help:"Username for the superuser."`
	Email    string `short:"e" help:"Email address."`
}

func (c *SuperuserCreateCmd) Run(ctx *pw.Context) error {
	if c.Username == "" {
		var err error
		c.Username, err = readLine("Username: ")
		if err != nil {
			return err
		}
		if c.Username == "" {
			return fmt.Errorf("username cannot be empty")
		}
	}
	if c.Email == "" {
		var err error
		c.Email, err = readLine("Email: ")
		if err != nil {
			return err
		}
	}

	password, err := readPassword("Password: ")
	if err != nil {
		return err
	}
	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}
	confirm, err := readPassword("Confirm password: ")
	if err != nil {
		return err
	}
	if password != confirm {
		return fmt.Errorf("passwords do not match")
	}

	user := db.User{
		Username:    c.Username,
		Email:       c.Email,
		Password:    db.HashPassword(password),
		IsSuperuser: true,
		IsStaff:     true,
		IsActive:    true,
		DateJoined:  time.Now(),
	}
	err = db.Insert(ctx, ctx.DB, &user)
	if err != nil {
		return err
	}

	fmt.Printf("Created superuser %q (id=%d)\n", user.Username, user.ID)
	return nil
}

type SuperuserDeleteCmd struct {
	Username string `arg:"" help:"Username to demote."`
}

func (c *SuperuserDeleteCmd) Run(ctx *pw.Context) error {
	var user db.User
	err := ctx.DB.NewSelect().Model(&user).
		Where("username = ?", c.Username).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("user %q not found", c.Username)
	}

	_, err = ctx.DB.NewUpdate().Model(&user).
		Where("id = ?", user.ID).
		Set("is_superuser = ?", false).
		Set("is_staff = ?", false).
		Exec(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Removed superuser privileges from %q\n", c.Username)
	return nil
}

type SuperuserPasswdCmd struct {
	Username string `arg:"" help:"Username to change password for."`
}

func (c *SuperuserPasswdCmd) Run(ctx *pw.Context) error {
	var user db.User
	err := ctx.DB.NewSelect().Model(&user).
		Where("username = ?", c.Username).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("user %q not found", c.Username)
	}

	password, err := readPassword("New password: ")
	if err != nil {
		return err
	}
	confirm, err := readPassword("Confirm password: ")
	if err != nil {
		return err
	}
	if password != confirm {
		return fmt.Errorf("passwords do not match")
	}
	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}

	_, err = ctx.DB.NewUpdate().Model(&user).
		Where("id = ?", user.ID).
		Set("password = ?", db.HashPassword(password)).
		Exec(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Password updated for %q\n", c.Username)
	return nil
}

func readLine(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	return strings.TrimSpace(line), err
}

func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		pw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		return string(pw), err
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	return strings.TrimSpace(line), err
}
