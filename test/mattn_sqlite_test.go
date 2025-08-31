package test

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
	migrations "github.com/pechorka/migrations"
	"github.com/stretchr/testify/require"
)

func TestSQLite(t *testing.T) {
	dsn := envOrDefault("SQLITE_DSN", ":memory:")
	opts := []migrations.Option{
		migrations.WithDialect(migrations.DialectSqlite),
		migrations.WithTableName("mattn_sqlite_test"),
	}

	t.Run("apply empty migrations", func(t *testing.T) {
		db := openDB(t, "sqlite3", dsn, resetSQLite)
		err := migrations.Apply(t.Context(), db, []string{}, opts...)
		require.NoError(t, err)
	})

	t.Run("reapply is idempotent", func(t *testing.T) {
		db := openDB(t, "sqlite3", dsn, resetSQLite)
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
		db := openDB(t, "sqlite3", dsn, resetSQLite)
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

	t.Run("migration has error", func(t *testing.T) {
		db := openDB(t, "sqlite3", dsn, resetSQLite)
		migs := []string{
			`INSERT INTO no_such_table (name) VALUES ('x')`,
		}
		err := migrations.Apply(t.Context(), db, migs, opts...)
		require.Error(t, err)
	})

	t.Run("connection is invalid", func(t *testing.T) {
		badDB, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		require.NoError(t, badDB.Close())

		err = migrations.Apply(t.Context(), badDB, []string{}, opts...)
		require.Error(t, err)
	})
}
