// Package storetest opens a store on a real Postgres for tests. Every call gets
// its own schema, so test packages can share one database and run in parallel.
package storetest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

// URL returns DATABASE_URL pointed at a fresh schema that is dropped when the
// test ends. Without DATABASE_URL the test is skipped, except in CI, where a
// missing database must fail rather than hide the tests.
func URL(t testing.TB) string {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("DATABASE_URL is not set in CI")
		}
		t.Skip("DATABASE_URL not set; run Postgres to include this test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	b := make([]byte, 6)
	_, _ = rand.Read(b)
	schema := "test_" + hex.EncodeToString(b)
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		conn, err := pgx.Connect(ctx, base)
		if err != nil {
			t.Errorf("drop schema: %v", err)
			return
		}
		defer conn.Close(ctx)
		if _, err := conn.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("drop schema: %v", err)
		}
	})

	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("DATABASE_URL: %v", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String()
}

// Open returns a migrated store in a fresh schema, closed when the test ends.
func Open(t testing.TB) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), URL(t))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}
