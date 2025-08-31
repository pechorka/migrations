# migrations

Tiny, dependency-free Go library for applying SQL migrations via `database/sql`.

## Supported Databases

- SQLite 
- Postgres  
- MySQL 

Requires Go 1.22+.

## Quick Start

```go
package main

import (
    "context"
    "database/sql"
    "log"

	_ "modernc.org/sqlite" 
    migrations "github.com/pechorka/migrations"
)

func main() {
    db, err := sql.Open("sqlite", ":memory:")
    must(err)
    defer db.Close()

    migs := []string{
        `
        CREATE TABLE IF NOT EXISTS items (
            id INTEGER PRIMARY KEY,
            name TEXT NOT NULL
        );
        INSERT INTO items (name) VALUES ('a'),('b');
        `,
        `
        ALTER TABLE items ADD COLUMN qty INTEGER DEFAULT 0;
        `,
    }

    ctx := context.Background()
    err = migrations.Apply(
        ctx,
        db,
        migs,
        // Dialect defaults to SQLite
        // migrations.WithDialect(migrations.DialectPostgres) // you can override it for other DBs
        // migrations.WithTableName("migrations"), // in case your DB already has migrations table
    )
    must(err)
}

func must(err error) {
    if err != nil {
        log.Fatal(err)
    }
}
```

## How It Works

- Bookkeeping table: created if missing, with the shape (version, applied_at)
- Versioning model: the first element of your `[]string` has version `1`, the second `2`, etc.
- Statement splitting: each migration string is split on `;` at top level, i.e. never inside `'single'`/`"double"`/``backtick`` quotes, `-- line comments`, `/* block comments */` (nested supported), or Postgres dollar-quoted blocks like `$$ ... $$` or `$tag$ ... $tag$`.
!!!!!!!WARNING!!!!!!!
Library tries it's best to split statements properly, but very likely a lot of edge cases are not covered.
You can always split your multi statement migration in multiple single statement migrations if you have any issues
!!!!!!!WARNING!!!!!!!
- Idempotency: the library reads `MAX(version)` from the table and only executes migrations with `version > max`.
- Recording: after a migration succeeds, the library inserts the applied version into the table.

This simple model makes append‑only, linear migrations trivial and safe to re-run.

## Limitations (Intentional)

- Linear, append‑only migrations only — no down/rollback support.
- No checksums, squashing, or out‑of‑order application.
- No file loading, templating, or dependency graph — you own the SQL and its order.

If you need advanced features (locks, revision graphs, down migrations), consider a full‑featured framework.
This library aims to be the simplest thing that works for many services.

## License

MIT License. See `LICENSE` for details.
