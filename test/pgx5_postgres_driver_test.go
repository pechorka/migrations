//go:build pgx5
// +build pgx5

package test

import (
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib" // pgx v5 database/sql driver
	migrations "github.com/pechorka/migrations"
	"github.com/stretchr/testify/require"
)

func TestPostgres_PGX5(t *testing.T) {
	dsn := envOrDefault("POSTGRES_DSN", "postgres://postgres:postgres@localhost:55432/postgres?sslmode=disable")
	opts := []migrations.Option{
		migrations.WithDialect(migrations.DialectPostgres),
		migrations.WithTableName("pgx5_postgres_driver_test"),
	}

	t.Run("apply empty migrations", func(t *testing.T) {
		db := openDB(t, "pgx", dsn, resetPostgres)
		err := migrations.Apply(t.Context(), db, []string{}, opts...)
		require.NoError(t, err)
	})

	t.Run("reapply is idempotent", func(t *testing.T) {
		db := openDB(t, "pgx", dsn, resetPostgres)
		migs := []string{
			`CREATE TABLE IF NOT EXISTS test_items (
        id SERIAL PRIMARY KEY,
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
		db := openDB(t, "pgx", dsn, resetPostgres)
		migs := []string{
			`CREATE TABLE IF NOT EXISTS test_items (
        id SERIAL PRIMARY KEY,
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
		db := openDB(t, "pgx", dsn, resetPostgres)
		migs := []string{
			`CREATE TABLE IF NOT EXISTS ms_items (
        id SERIAL PRIMARY KEY,
        name TEXT NOT NULL
    ); INSERT INTO ms_items (name) VALUES ('alpha');`,
		}
		err := migrations.Apply(t.Context(), db, migs, opts...)
		require.NoError(t, err)

		var n int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM ms_items`).Scan(&n))
		require.Equal(t, 1, n)
	})

	t.Run("single migration: ; in quotes", func(t *testing.T) {
		db := openDB(t, "pgx", dsn, resetPostgres)
		migs := []string{
			`CREATE TABLE IF NOT EXISTS ms_items (
        id SERIAL PRIMARY KEY,
        name TEXT NOT NULL
    ); INSERT INTO ms_items (name) VALUES ('a; b');`,
		}
		err := migrations.Apply(t.Context(), db, migs, opts...)
		require.NoError(t, err)

		var got string
		require.NoError(t, db.QueryRow(`SELECT name FROM ms_items`).Scan(&got))
		require.Equal(t, "a; b", got)
	})

	t.Run("single migration: ; in comment", func(t *testing.T) {
		db := openDB(t, "pgx", dsn, resetPostgres)
		migs := []string{
			`CREATE TABLE IF NOT EXISTS ms_items (
        id SERIAL PRIMARY KEY,
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
		db := openDB(t, "pgx", dsn, resetPostgres)
		migs := []string{
			`INSERT INTO no_such_table (name) VALUES ('x')`,
		}
		err := migrations.Apply(t.Context(), db, migs, opts...)
		require.Error(t, err)
	})

	t.Run("connection is invalid", func(t *testing.T) {
		badDSN := "postgres://postgres:postgres@127.0.0.1:1/postgres?sslmode=disable&connect_timeout=1"
		badDB, err := sql.Open("pgx", badDSN)
		require.NoError(t, err)

		err = migrations.Apply(t.Context(), badDB, []string{"SELECT 1"}, opts...)
		require.Error(t, err)
		_ = badDB.Close()
	})
}
