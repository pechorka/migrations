package migrations_test

import (
    "context"
    "database/sql"
    "log"

    _ "modernc.org/sqlite" // pure Go SQLite driver

    migrations "github.com/pechorka/migrations"
)

// Example demonstrates applying two simple migrations against an in-memory
// SQLite database using default options.
func Example() {
    db, err := sql.Open("sqlite", ":memory:")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Each element is a migration, versions start from 1.
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
    if err := migrations.Apply(ctx, db, migs); err != nil {
        log.Fatal(err)
    }
}

// ExampleApply_withOptions shows how to customize options passed to Apply.
func ExampleApply_withOptions() {
    db, err := sql.Open("sqlite", ":memory:")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    ctx := context.Background()
    migs := []string{
        `CREATE TABLE t (id INTEGER PRIMARY KEY);`,
    }

    // Use a custom bookkeeping table name. The default dialect is SQLite,
    // so WithDialect is optional here; you can also set
    // migrations.WithDialect(migrations.DialectPostgres) when using a Postgres
    // connection/driver.
    if err := migrations.Apply(
        ctx,
        db,
        migs,
        migrations.WithTableName("schema_migrations"),
        // migrations.WithDialect(migrations.DialectSqlite),
    ); err != nil {
        log.Fatal(err)
    }
}

