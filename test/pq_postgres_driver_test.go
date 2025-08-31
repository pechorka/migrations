package test

import (
	"database/sql"
	"testing"

	_ "github.com/lib/pq" // Postgres driver
	migrations "github.com/pechorka/migrations"
	"github.com/stretchr/testify/require"
)

func TestPostgres(t *testing.T) {
	dsn := envOrDefault("POSTGRES_DSN", "postgres://postgres:postgres@localhost:55432/postgres?sslmode=disable")
	opts := []migrations.Option{
		migrations.WithDialect(migrations.DialectPostgres),
		migrations.WithTableName("pq_postgres_driver_test"),
	}

	t.Run("apply empty migrations", func(t *testing.T) {
		db := openDB(t, "postgres", dsn, resetPostgres)
		err := migrations.Apply(t.Context(), db, []string{}, opts...)
		require.NoError(t, err)
	})

	t.Run("reapply is idempotent", func(t *testing.T) {
		db := openDB(t, "postgres", dsn, resetPostgres)
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
		db := openDB(t, "postgres", dsn, resetPostgres)
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

	t.Run("migration has error", func(t *testing.T) {
		db := openDB(t, "postgres", dsn, resetPostgres)
		migs := []string{
			`INSERT INTO no_such_table (name) VALUES ('x')`,
		}
		err := migrations.Apply(t.Context(), db, migs, opts...)
		require.Error(t, err)
	})

	t.Run("connection is invalid", func(t *testing.T) {
		badDSN := "postgres://postgres:postgres@127.0.0.1:1/postgres?sslmode=disable&connect_timeout=1"
		badDB, err := sql.Open("postgres", badDSN)
		require.NoError(t, err)

		err = migrations.Apply(t.Context(), badDB, []string{"SELECT 1"}, opts...)
		require.Error(t, err)
		_ = badDB.Close()
	})
}
