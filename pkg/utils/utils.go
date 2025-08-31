package utils

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func InTx(ctx context.Context, db *sql.DB, fn func(ctx context.Context, tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(ctx, tx); err != nil {
		rerr := tx.Rollback()
		if rerr != nil {
			err = errors.Join(err, fmt.Errorf("also failed to rollback transaction: %w", rerr))
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func QuoteIdentBacktick(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}
