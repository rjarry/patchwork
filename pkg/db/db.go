// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"

	"github.com/getpatchwork/patchwork/pkg/config"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/schema"

	_ "github.com/go-sql-driver/mysql" // register mysql driver
	_ "github.com/jackc/pgx/v5/stdlib" // register pgx driver
	_ "modernc.org/sqlite"             // register sqlite driver
)

// Open connects to a database from a parsed URL. The scheme determines
// the driver and dialect:
//
//   - postgres:// postgresql:// pgx://  -> pgx + pgdialect
//   - mysql:// mariadb://               -> mysql + mysqldialect
//   - sqlite:// sqlite3://              -> sqlite + sqlitedialect
//
// The URL is rewritten to the format each driver expects.
func Open(cfg *config.Config) (*bun.DB, error) {
	var driver string
	var dsn string
	var dialect schema.Dialect

	u, err := url.Parse(cfg.Database.URL)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "postgres", "postgresql", "pgx":
		// pgx accepts the standard postgres:// URL directly
		driver = "pgx"
		u.Scheme = "postgres"
		dsn = u.String()
		dialect = pgdialect.New()

	case "mysql", "mariadb":
		// go-sql-driver/mysql expects user:pass@tcp(host:port)/dbname
		host := u.Hostname()
		port := u.Port()
		if port == "" {
			port = "3306"
		}
		dbname := u.Path
		if len(dbname) > 0 && dbname[0] == '/' {
			dbname = dbname[1:]
		}
		userinfo := ""
		if u.User != nil {
			userinfo = u.User.String() + "@"
		}
		q := u.Query()
		q.Set("parseTime", "true")
		dsn = fmt.Sprintf("%stcp(%s:%s)/%s?%s",
			userinfo, host, port, dbname, q.Encode())
		driver = "mysql"
		dialect = mysqldialect.New()

	case "sqlite", "sqlite3":
		// modernc sqlite accepts file: URIs or plain paths.
		// url.Parse puts the path in Host for sqlite://foo.db
		// and in Path for sqlite:///tmp/foo.db.
		driver = "sqlite"
		path := u.Path
		if path == "" {
			path = u.Host
		}
		if path == ":memory:" {
			dsn = ":memory:"
		} else {
			q := u.Query()
			if !q.Has("_pragma") {
				q.Add("_pragma", "foreign_keys(1)")
				q.Add("_pragma", "journal_mode(WAL)")
			}
			dsn = fmt.Sprintf("file:%s?%s", path, q.Encode())
		}
		dialect = sqlitedialect.New()

	default:
		return nil, fmt.Errorf("unsupported database scheme %q", u.Scheme)
	}

	conn, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open(%s): %w", driver, err)
	}

	return bun.NewDB(conn, dialect), nil
}

// Queries provides typed database access methods. It wraps a bun
// transaction and the context it was started with.
type Queries struct {
	ctx context.Context
	tx  bun.Tx
}

// Begin starts a transaction and returns a Queries handle.
func Begin(ctx context.Context, database *bun.DB) (*Queries, error) {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &Queries{ctx: ctx, tx: tx}, nil
}

func (q *Queries) Commit() error   { return q.tx.Commit() }
func (q *Queries) Rollback() error { return q.tx.Rollback() }

const ellipsis = "…"

func truncStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	// need room for at least one byte + ellipsis
	if n < len(ellipsis)+1 {
		return s[:n]
	}
	n -= len(ellipsis)
	// UTF-8 uses 1 to 4 bytes per character. Continuation bytes always
	// have their two most significant bits set to 10:
	//
	//   0xxxxxxx  single byte (ASCII)
	//   110xxxxx  start of 2-byte sequence
	//   1110xxxx  start of 3-byte sequence
	//   11110xxx  start of 4-byte sequence
	//   10xxxxxx  continuation byte
	//
	// If we landed on a continuation byte, back up to the start of the
	// rune so we don't slice a character in half. For example, "café"
	// is [c a f 0xc3 0xa9]. Cutting at byte 4 would split the é, so
	// we back up to byte 3 and get "caf…" instead of "caf\xc3…".
	for n > 0 && s[n]>>6 == 0b10 {
		n--
	}
	return s[:n] + ellipsis
}
