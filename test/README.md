# Test Submodule

This directory is a standalone Go module for integration tests against popular drivers.
The root `go.mod` remains clean; all driver imports are isolated here.

## Start databases (local)

Requires Docker.

```
cd test
docker compose up -d
```

Default DSNs (override via env):

- POSTGRES_DSN: `postgres://postgres:postgres@localhost:55432/postgres?sslmode=disable`
- MYSQL_DSN: `root:root@tcp(localhost:53306)/testdb?parseTime=true&multiStatements=true`
- SQLITE_DSN: `:memory:`

## Run tests

```
cd test
# one-shot with docker: starts DBs, runs tests, cleans up
bash ./run.sh

# or run manually (requires DBs already running)
go test -v ./...
```
