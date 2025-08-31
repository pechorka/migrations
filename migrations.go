package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pechorka/migrations/pkg/utils"
)

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

type Options struct {
	Dialect   Dialect
	TableName string
}

type Option func(opts *Options) error

func WithDialect(dialect Dialect) Option {
	return func(opts *Options) error {
		if !IsValidDialect(dialect) {
			return fmt.Errorf("diealect %d is not supported. check 'migrations.Dialect*' for supported options", dialect)
		}

		opts.Dialect = dialect
		return nil
	}
}

func WithTableName(table string) Option {
	return func(opts *Options) error {
		if table == "" {
			return fmt.Errorf("table name cannot be empty")
		}

		// Enforce a conservative identifier policy to avoid SQL injection
		// since identifiers are interpolated into SQL statements.
		for i, r := range table {
			if r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9' && i > 0) {
				continue
			}
			return fmt.Errorf("invalid table name %q: only [A-Za-z_][A-Za-z0-9_]* allowed", table)
		}

		opts.TableName = table
		return nil
	}
}

type Dialect int32

const (
	dialectBegin Dialect = iota

	DialectSqlite
	DialectPostgres
	DialectMysql

	dialectEnd
)

func IsValidDialect(d Dialect) bool {
	return dialectBegin < d && d < dialectEnd
}

// Options end

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
			if _, err := tx.ExecContext(ctx, migration); err != nil {
				return fmt.Errorf("failed to apply migration #%d: %w", version, err)
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
			if _, err := tx.ExecContext(ctx, migration); err != nil {
				return fmt.Errorf("failed to apply migration #%d: %w", version, err)
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
			if _, err := tx.ExecContext(ctx, migration); err != nil {
				return fmt.Errorf("failed to apply migration #%d: %w", version, err)
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
