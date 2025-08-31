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

// SplitStatements splits SQL text into individual statements by semicolons.
// It avoids splitting inside quoted strings/identifiers, Postgres dollar-quoted
// blocks, and SQL comments. Empty/whitespace-only statements are dropped.
func SplitStatements(s string) []string {
	var out []string
	var b strings.Builder

	inS, inD, inB := false, false, false // single ', double ", backtick `
	inLineComment := false               // -- ... \n
	// nested block comments
	inBlockComment := 0 // 0 == not in, >0 == nesting level

	dollarTag := "" // when non-empty, we are inside $tag$...$tag$

	hasPrefixAt := func(i int, p string) bool {
		return i+len(p) <= len(s) && s[i:i+len(p)] == p
	}

	flush := func() {
		stmt := strings.TrimSpace(b.String())
		if stmt != "" {
			out = append(out, stmt)
		}
		b.Reset()
	}

	for i := 0; i < len(s); i++ {
		c := s[i]

		if inLineComment {
			if c == '\n' {
				inLineComment = false
			}
			continue
		}

		if inBlockComment > 0 {
			if hasPrefixAt(i, "/*") {
				inBlockComment++
				i++
				continue
			}
			if hasPrefixAt(i, "*/") {
				inBlockComment--
				i++
			}
			continue
		}

		if dollarTag != "" {
			if hasPrefixAt(i, dollarTag) {
				b.WriteString(dollarTag)
				i += len(dollarTag) - 1
				dollarTag = ""
				continue
			}
			b.WriteByte(c)
			continue
		}

		if inS {
			b.WriteByte(c)
			if c == '\\' { // backslash escape (MySQL)
				if i+1 < len(s) {
					b.WriteByte(s[i+1])
					i++
				}
				continue
			}
			if c == '\'' {
				if i+1 < len(s) && s[i+1] == '\'' { // doubled quote
					b.WriteByte(s[i+1])
					i++
					continue
				}
				inS = false
			}
			continue
		}
		if inD {
			b.WriteByte(c)
			if c == '"' {
				if i+1 < len(s) && s[i+1] == '"' {
					b.WriteByte(s[i+1])
					i++
					continue
				}
				inD = false
			}
			continue
		}
		if inB {
			b.WriteByte(c)
			if c == '`' {
				if i+1 < len(s) && s[i+1] == '`' {
					b.WriteByte(s[i+1])
					i++
					continue
				}
				inB = false
			}
			continue
		}

		// Top-level
		if hasPrefixAt(i, "--") {
			inLineComment = true
			i++
			continue
		}
		if hasPrefixAt(i, "/*") {
			inBlockComment = 1
			i++
			continue
		}

		if c == '$' {
			j := i + 1
			for j < len(s) {
				cj := s[j]
				if (cj >= 'a' && cj <= 'z') || (cj >= 'A' && cj <= 'Z') || (cj >= '0' && cj <= '9') || cj == '_' {
					j++
					continue
				}
				break
			}
			if j < len(s) && s[j] == '$' { // $tag$ or $$
				dollarTag = s[i : j+1]
				b.WriteString(dollarTag)
				i = j
				continue
			}
		}

		if c == '\'' {
			inS = true
			b.WriteByte(c)
			continue
		}
		if c == '"' {
			inD = true
			b.WriteByte(c)
			continue
		}
		if c == '`' {
			inB = true
			b.WriteByte(c)
			continue
		}

		if c == ';' {
			flush()
			continue
		}

		b.WriteByte(c)
	}

	flush()
	return out
}
