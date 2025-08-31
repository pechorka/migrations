package test

import (
	"database/sql"
	"testing"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
	migrations "github.com/pechorka/migrations"
	"github.com/pechorka/migrations/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestMySQL(t *testing.T) {
	dsn := envOrDefault("MYSQL_DSN", "root:root@tcp(localhost:53306)/testdb?parseTime=true&multiStatements=true")
	opts := []migrations.Option{
		migrations.WithDialect(migrations.DialectMysql),
		migrations.WithTableName("mysql_driver_test"),
	}

	t.Run("apply empty migrations", func(t *testing.T) {
		db := openDB(t, "mysql", dsn, resetMySQL)
		err := migrations.Apply(t.Context(), db, []string{}, opts...)
		require.NoError(t, err)
	})

	t.Run("reapply is idempotent", func(t *testing.T) {
		db := openDB(t, "mysql", dsn, resetMySQL)
		tbl := utils.QuoteIdentBacktick("test_items")
		migs := []string{
			"CREATE TABLE IF NOT EXISTS " + tbl + ` (
    id INT NOT NULL AUTO_INCREMENT,
    name VARCHAR(255) NOT NULL,
    PRIMARY KEY (id)
)`,
			"INSERT INTO " + tbl + " (name) VALUES ('a'),('b')",
		}
		err := migrations.Apply(t.Context(), db, migs, opts...)
		require.NoError(t, err)
		err = migrations.Apply(t.Context(), db, migs, opts...)
		require.NoError(t, err)
	})

	t.Run("apply first, then second", func(t *testing.T) {
		db := openDB(t, "mysql", dsn, resetMySQL)
		tbl := utils.QuoteIdentBacktick("test_items")
		migs := []string{
			"CREATE TABLE IF NOT EXISTS " + tbl + ` (
    id INT NOT NULL AUTO_INCREMENT,
    name VARCHAR(255) NOT NULL,
    PRIMARY KEY (id)
)`,
			"INSERT INTO " + tbl + " (name) VALUES ('a'),('b')",
		}
		err := migrations.Apply(t.Context(), db, migs[:1], opts...)
		require.NoError(t, err)
		err = migrations.Apply(t.Context(), db, migs[:2], opts...)
		require.NoError(t, err)
	})

	t.Run("single migration: multiple statements (basic)", func(t *testing.T) {
		db := openDB(t, "mysql", dsn, resetMySQL)
		tbl := utils.QuoteIdentBacktick("ms_items")
		migs := []string{
			"CREATE TABLE IF NOT EXISTS " + tbl + ` (
    id INT NOT NULL AUTO_INCREMENT,
    name VARCHAR(255) NOT NULL,
    PRIMARY KEY (id)
); INSERT INTO ` + tbl + ` (name) VALUES ('alpha');`,
		}
		err := migrations.Apply(t.Context(), db, migs, opts...)
		require.NoError(t, err)

		var n int
		require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM "+tbl).Scan(&n))
		require.Equal(t, 1, n)
	})

	t.Run("single migration: ; in quotes", func(t *testing.T) {
		db := openDB(t, "mysql", dsn, resetMySQL)
		tbl := utils.QuoteIdentBacktick("ms_items")
		migs := []string{
			"CREATE TABLE IF NOT EXISTS " + tbl + ` (
    id INT NOT NULL AUTO_INCREMENT,
    name VARCHAR(255) NOT NULL,
    PRIMARY KEY (id)
); INSERT INTO ` + tbl + ` (name) VALUES ('a; b');`,
		}
		err := migrations.Apply(t.Context(), db, migs, opts...)
		require.NoError(t, err)

		var got string
		require.NoError(t, db.QueryRow("SELECT name FROM "+tbl).Scan(&got))
		require.Equal(t, "a; b", got)
	})

	t.Run("single migration: ; in comment", func(t *testing.T) {
		db := openDB(t, "mysql", dsn, resetMySQL)
		tbl := utils.QuoteIdentBacktick("ms_items")
		migs := []string{
			"CREATE TABLE IF NOT EXISTS " + tbl + ` (
    id INT NOT NULL AUTO_INCREMENT,
    name VARCHAR(255) NOT NULL,
    PRIMARY KEY (id)
); /* comment ; with semicolon */ INSERT INTO ` + tbl + ` (name) VALUES ('ok');`,
		}
		err := migrations.Apply(t.Context(), db, migs, opts...)
		require.NoError(t, err)

		var n int
		require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM "+tbl).Scan(&n))
		require.Equal(t, 1, n)
	})

	t.Run("migration has error", func(t *testing.T) {
		db := openDB(t, "mysql", dsn, resetMySQL)
		migs := []string{
			"INSERT INTO `does_not_exist` (name) VALUES ('x')",
		}
		err := migrations.Apply(t.Context(), db, migs, opts...)
		require.Error(t, err)
	})

	t.Run("connection is invalid", func(t *testing.T) {
		badDSN := "root:root@tcp(127.0.0.1:1)/testdb?parseTime=true&multiStatements=true&timeout=1s&readTimeout=1s&writeTimeout=1s"
		badDB, err := sql.Open("mysql", badDSN)
		require.NoError(t, err)

		err = migrations.Apply(t.Context(), badDB, []string{"SELECT 1"}, opts...)
		require.Error(t, err)
		_ = badDB.Close()
	})
}
