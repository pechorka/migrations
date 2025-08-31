//go:build sqlite_nocgo
// +build sqlite_nocgo

package test

import (
	"database/sql"
	"testing"

	migrations "github.com/pechorka/migrations"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite" // pure Go (no CGO) SQLite driver
)

func TestSQLite_NoCGO(t *testing.T) {
	dsn := envOrDefault("SQLITE_DSN", ":memory:")
	opts := []migrations.Option{
		migrations.WithDialect(migrations.DialectSqlite),
		migrations.WithTableName("modernc_sqlite_test"),
	}

	t.Run("apply empty migrations", func(t *testing.T) {
		db := openDB(t, "sqlite", dsn, resetSQLite)
		err := migrations.Apply(t.Context(), db, []string{}, opts...)
		require.NoError(t, err)
	})

	t.Run("reapply is idempotent", func(t *testing.T) {
		db := openDB(t, "sqlite", dsn, resetSQLite)
		migs := []string{
			`CREATE TABLE IF NOT EXISTS test_items (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL
    )`,
			`INSERT INTO test_items (name) VALUES ('a'),('b')`,
		}
		err := migrations.Apply(t.Context(), db, migs, opts...)
		require.NoError(t, err)
		err = migrations.Apply(t.Context(), db, migs, opts...)
		require.NoError(t, err)
	})

	t.Run("apply first, then second", func(t *testing.T) {
		db := openDB(t, "sqlite", dsn, resetSQLite)
		migs := []string{
			`CREATE TABLE IF NOT EXISTS test_items (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL
    )`,
			`INSERT INTO test_items (name) VALUES ('a'),('b')`,
		}
		err := migrations.Apply(t.Context(), db, migs[:1], opts...)
		require.NoError(t, err)
		err = migrations.Apply(t.Context(), db, migs[:2], opts...)
		require.NoError(t, err)
	})

	t.Run("single migration: multiple statements (basic)", func(t *testing.T) {
		db := openDB(t, "sqlite", dsn, resetSQLite)
		migs := []string{
			`CREATE TABLE IF NOT EXISTS ms_items (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL
    ); 
            INSERT INTO ms_items (name) VALUES ('alpha');`,
		}
		err := migrations.Apply(t.Context(), db, migs, opts...)
		require.NoError(t, err)

		var n int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM ms_items`).Scan(&n))
		require.Equal(t, 1, n)
	})

	t.Run("single migration: ; in quotes", func(t *testing.T) {
		db := openDB(t, "sqlite", dsn, resetSQLite)
		migs := []string{
			`CREATE TABLE IF NOT EXISTS ms_items (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL
    ); 
            INSERT INTO ms_items (name) VALUES ('a; b');`,
		}
		err := migrations.Apply(t.Context(), db, migs, opts...)
		require.NoError(t, err)

		var got string
		require.NoError(t, db.QueryRow(`SELECT name FROM ms_items`).Scan(&got))
		require.Equal(t, "a; b", got)
	})

	t.Run("single migration: ; in comment", func(t *testing.T) {
		db := openDB(t, "sqlite", dsn, resetSQLite)
		migs := []string{
			`CREATE TABLE IF NOT EXISTS ms_items (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL
    ); -- comment with semicolon ; should not split
    INSERT INTO ms_items (name) VALUES ('ok');`,
		}
		err := migrations.Apply(t.Context(), db, migs, opts...)
		require.NoError(t, err)

		var n int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM ms_items`).Scan(&n))
		require.Equal(t, 1, n)
	})

	t.Run("migration has error", func(t *testing.T) {
		db := openDB(t, "sqlite", dsn, resetSQLite)
		migs := []string{
			`INSERT INTO no_such_table (name) VALUES ('x')`,
		}
		err := migrations.Apply(t.Context(), db, migs, opts...)
		require.Error(t, err)
	})

	t.Run("connection is invalid", func(t *testing.T) {
		badDB, err := sql.Open("sqlite", ":memory:")
		require.NoError(t, err)
		require.NoError(t, badDB.Close())

		err = migrations.Apply(t.Context(), badDB, []string{}, opts...)
		require.Error(t, err)
	})
}
