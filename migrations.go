// Package migrations provides a tiny, dependency‑free helper for applying
// append‑only SQL migrations using database/sql.
//
// Migrations are supplied as a slice of SQL strings. Each element in the slice
// represents a migration with an incrementing version starting at 1. A single
// migration string may contain multiple SQL statements separated by semicolons;
// splitting is done safely at the top level (never inside quoted strings,
// comments, or Postgres dollar‑quoted blocks).
//
// Apply creates a bookkeeping table if it does not exist yet and then executes
// only the migrations whose version is greater than the maximum recorded
// version. All statements run inside a single transaction; if any statement
// fails, nothing is recorded and the transaction is rolled back.
//
// Supported dialects: SQLite (default), Postgres, and MySQL.
package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pechorka/migrations/pkg/utils"
)

// Apply runs the pending migrations for the given database connection.
//
// Behavior
//   - Creates the bookkeeping table when missing (name is configurable).
//   - Determines the last applied version and executes only newer migrations.
//     The first element of migrations has version 1, the second version 2, and
//     so on.
//   - Splits each migration string by semicolons at the top level to allow
//     multiple statements per migration string.
//   - Wraps all statements in a single transaction. On error the transaction is
//     rolled back and no version is recorded.
//
// Dialect and table name can be customized via Option values, e.g.:
//
//	Apply(ctx, db, migs, WithDialect(DialectPostgres), WithTableName("schema_migrations"))
//
// The default dialect is SQLite and the default table name is "migrations".
func Apply(ctx context.Context, db *sql.DB, migrations []string, userOptions ...Option) error {
	opts := Options{
		Dialect:   DialectSqlite,
		TableName: "migrations",
	}

	for i, modifyOptions := range userOptions {
		err := modifyOptions(&opts)
		if err != nil {
			return fmt.Errorf("issue with option #%d: %w", i+1, err)
		}
	}

	if err := validateOptions(opts); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	switch opts.Dialect {
	case DialectSqlite:
		return applySqlite(ctx, db, migrations, opts)
	case DialectMysql:
		return applyMysql(ctx, db, migrations, opts)
	case DialectPostgres:
		return applyPostgres(ctx, db, migrations, opts)
	default:
		return fmt.Errorf("dialect %d is not supported (should never happen)", opts.Dialect)
	}
}

// Options begin

// Options controls how Apply behaves.
//
// Use Option helpers (WithDialect, WithTableName) to construct and pass
// configuration to Apply.
type Options struct {
	Dialect   Dialect
	TableName string
}

// Option mutates Options passed to Apply.
//
// Use WithDialect and WithTableName to construct Option values.
type Option func(opts *Options) error

// WithDialect selects the SQL dialect used for DDL/DML and placeholders.
//
// Supported values: DialectSqlite (default), DialectPostgres, DialectMysql.
// Returns an error from Apply if an unsupported dialect is provided.
func WithDialect(dialect Dialect) Option {
	// Validation is performed centrally by validateOptions during Apply.
	return func(opts *Options) error {
		opts.Dialect = dialect
		return nil
	}
}

// WithTableName overrides the bookkeeping table name (default: "migrations").
//
// Note: identifier safety ([A-Za-z_][A-Za-z0-9_]*) is enforced centrally by
// validateOptions during Apply.
func WithTableName(table string) Option {
	// Validation is performed centrally by validateOptions during Apply.
	return func(opts *Options) error {
		opts.TableName = table
		return nil
	}
}

// Dialect enumerates supported SQL dialects.
type Dialect int32

const (
	dialectBegin Dialect = iota

	DialectSqlite
	DialectPostgres
	DialectMysql

	dialectEnd
)

// IsValidDialect reports whether d is one of the supported Dialect constants.
func IsValidDialect(d Dialect) bool {
	return dialectBegin < d && d < dialectEnd
}

// Options end

// validateOptions performs centralized validation of Options.
// - Dialect must be one of the supported constants.
// - TableName must be non-empty and match [A-Za-z_][A-Za-z0-9_]*.
func validateOptions(opts Options) error {
	if !IsValidDialect(opts.Dialect) {
		return fmt.Errorf("dialect %d is not supported", opts.Dialect)
	}
	if opts.TableName == "" {
		return fmt.Errorf("table name cannot be empty")
	}
	for i, r := range opts.TableName {
		if r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9' && i > 0) {
			continue
		}
		return fmt.Errorf("invalid table name %q: only [A-Za-z_][A-Za-z0-9_]* allowed", opts.TableName)
	}
	return nil
}
func applySqlite(ctx context.Context, db *sql.DB, migrations []string, opts Options) error {
	err := utils.InTx(ctx, db, func(ctx context.Context, tx *sql.Tx) error {
		createStmt := fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS "%s" (
                version INTEGER PRIMARY KEY,
                applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
            )`, opts.TableName,
		)
		if _, err := tx.ExecContext(ctx, createStmt); err != nil {
			return fmt.Errorf("failed to create migrations table %q: %w", opts.TableName, err)
		}

		var lastAppliedVersion int
		queryLast := fmt.Sprintf(`SELECT COALESCE(MAX(version), -1) FROM "%s"`, opts.TableName)
		if err := tx.QueryRowContext(ctx, queryLast).Scan(&lastAppliedVersion); err != nil {
			return fmt.Errorf("failed to read last applied migration version: %w", err)
		}

		for version, migration := range migrations {
			version++ // so first version is 1 instead of 0
			if version <= lastAppliedVersion {
				continue
			}
			stmts := utils.SplitStatements(migration)
			for i, stmt := range stmts {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return fmt.Errorf("failed to apply migration #%d (statement %d): %w", version, i+1, err)
				}
			}

			insertStmt := fmt.Sprintf(`INSERT INTO "%s" (version) VALUES (?)`, opts.TableName)
			if _, err := tx.ExecContext(ctx, insertStmt, version); err != nil {
				return fmt.Errorf("failed to record migration #%d: %w", version, err)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to apply migrations for sqlitedb: %w", err)
	}
	return nil
}

func applyMysql(ctx context.Context, db *sql.DB, migrations []string, opts Options) error {
	err := utils.InTx(ctx, db, func(ctx context.Context, tx *sql.Tx) error {
		createStmt := `CREATE TABLE IF NOT EXISTS ` + utils.QuoteIdentBacktick(opts.TableName) + `(
			    version INT NOT NULL PRIMARY KEY,
			    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`
		if _, err := tx.ExecContext(ctx, createStmt); err != nil {
			return fmt.Errorf("failed to create migrations table %q: %w", opts.TableName, err)
		}

		var lastAppliedVersion int
		queryLast := fmt.Sprintf("SELECT COALESCE(MAX(version), -1) FROM `%s`", opts.TableName)
		if err := tx.QueryRowContext(ctx, queryLast).Scan(&lastAppliedVersion); err != nil {
			return fmt.Errorf("failed to read last applied migration version: %w", err)
		}

		for version, migration := range migrations {
			version++ // so first version is 1 instead of 0
			if version <= lastAppliedVersion {
				continue
			}
			stmts := utils.SplitStatements(migration)
			for i, stmt := range stmts {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return fmt.Errorf("failed to apply migration #%d (statement %d): %w", version, i+1, err)
				}
			}

			insertStmt := fmt.Sprintf("INSERT INTO `%s` (version) VALUES (?)", opts.TableName)
			if _, err := tx.ExecContext(ctx, insertStmt, version); err != nil {
				return fmt.Errorf("failed to record migration #%d: %w", version, err)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to apply migrations for mysql: %w", err)
	}
	return nil
}

func applyPostgres(ctx context.Context, db *sql.DB, migrations []string, opts Options) error {
	err := utils.InTx(ctx, db, func(ctx context.Context, tx *sql.Tx) error {
		createStmt := fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS "%s" (
                version INTEGER PRIMARY KEY,
                applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
            )`, opts.TableName,
		)
		if _, err := tx.ExecContext(ctx, createStmt); err != nil {
			return fmt.Errorf("failed to create migrations table %q: %w", opts.TableName, err)
		}

		var lastAppliedVersion int
		queryLast := fmt.Sprintf(`SELECT COALESCE(MAX(version), -1) FROM "%s"`, opts.TableName)
		if err := tx.QueryRowContext(ctx, queryLast).Scan(&lastAppliedVersion); err != nil {
			return fmt.Errorf("failed to read last applied migration version: %w", err)
		}

		for version, migration := range migrations {
			version++ // so first version is 1 instead of 0
			if version <= lastAppliedVersion {
				continue
			}
			stmts := utils.SplitStatements(migration)
			for i, stmt := range stmts {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return fmt.Errorf("failed to apply migration #%d (statement %d): %w", version, i+1, err)
				}
			}

			insertStmt := fmt.Sprintf(`INSERT INTO "%s" (version) VALUES ($1)`, opts.TableName)
			if _, err := tx.ExecContext(ctx, insertStmt, version); err != nil {
				return fmt.Errorf("failed to record migration #%d: %w", version, err)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to apply migrations for postgres: %w", err)
	}
	return nil
}
