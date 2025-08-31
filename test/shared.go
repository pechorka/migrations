package test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/pechorka/migrations/pkg/utils"
	"github.com/stretchr/testify/require"
)

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func openDB(t *testing.T, driver, dsn string, reset func(t *testing.T, db *sql.DB)) *sql.DB {
	t.Helper()
	db, err := sql.Open(driver, dsn)
	require.NoError(t, err)

	ctx := t.Context()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		err := db.PingContext(ctx)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			require.NoError(t, ctx.Err(), "database not ready in time")
		case <-ticker.C:
		}
	}

	t.Cleanup(func() {
		reset(t, db)

		cerr := db.Close()
		if cerr != nil {
			t.Error("failed to close connection", err)
		}
	})
	return db
}

func resetSQLite(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// PRAGMA foreign_keys is *per-connection*. We run everything on the
	// transaction's connection (via tx.Exec) so the setting applies.
	err := utils.InTx(ctx, db, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `PRAGMA foreign_keys = OFF;`); err != nil {
			return fmt.Errorf("sqlite: disable foreign_keys: %w", err)
		}
		defer func() {
			_, _ = tx.ExecContext(ctx, `PRAGMA foreign_keys = ON;`)
		}()

		rows, err := tx.QueryContext(ctx, `
			SELECT name
			FROM sqlite_master
			WHERE type='table' AND name NOT LIKE 'sqlite_%'
		`)
		if err != nil {
			return fmt.Errorf("sqlite: list tables: %w", err)
		}
		defer rows.Close()

		var names []string
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				return fmt.Errorf("sqlite: scan: %w", err)
			}
			names = append(names, n)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("sqlite: rows err: %w", err)
		}

		for _, n := range names {
			stmt := `DROP TABLE IF EXISTS ` + strconv.Quote(n) + `;`
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("sqlite: drop %q: %w", n, err)
			}
		}
		return nil
	})
	require.NoError(t, err)
}

func resetMySQL(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// MySQL DDL (DROP TABLE) auto-commits and does not reliably participate in a
	// sql.Tx. We pin a single session with db.Conn so that:
	//   1) SET FOREIGN_KEY_CHECKS applies to the same session as the DROPs
	//   2) we can safely defer restoring it.
	conn, err := db.Conn(ctx)
	require.NoError(t, err, "mysql: get dedicated connection")
	defer conn.Close()

	_, err = conn.ExecContext(ctx, `SET FOREIGN_KEY_CHECKS = 0;`)
	require.NoError(t, err, "mysql: disable fk checks")
	defer func() {
		_, _ = conn.ExecContext(ctx, `SET FOREIGN_KEY_CHECKS = 1;`)
	}()

	// List all base tables in the current DB.
	rows, err := conn.QueryContext(ctx, `
		SELECT TABLE_NAME
		FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'
	`)
	require.NoError(t, err, "mysql: list tables")
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		names = append(names, n)
	}
	require.NoError(t, rows.Err(), "mysql: rows err")

	for _, n := range names {
		stmt := `DROP TABLE IF EXISTS ` + utils.QuoteIdentBacktick(n) + `;`
		_, err := conn.ExecContext(ctx, stmt)
		require.NoErrorf(t, err, "mysql: drop %q failed", n)
	}
}

func resetPostgres(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// There is no global "disable FK checks" knob in Postgres. We do it in a tx
	// and rely on CASCADE to remove dependent objects (views/constraints).
	err := utils.InTx(ctx, db, func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT schemaname, tablename
			FROM pg_tables
			WHERE schemaname NOT IN ('pg_catalog','information_schema')
		`)
		if err != nil {
			return fmt.Errorf("postgres: list tables: %w", err)
		}
		defer rows.Close()

		type st struct{ s, t string }
		var items []st
		for rows.Next() {
			var rec st
			if err := rows.Scan(&rec.s, &rec.t); err != nil {
				return fmt.Errorf("postgres: scan: %w", err)
			}
			items = append(items, rec)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("postgres: rows err: %w", err)
		}

		for _, it := range items {
			stmt := `DROP TABLE IF EXISTS ` + strconv.Quote(it.s) + `.` + strconv.Quote(it.t) + ` CASCADE;`
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("postgres: drop %s.%s: %w", it.s, it.t, err)
			}
		}
		return nil
	})
	require.NoError(t, err)
}
