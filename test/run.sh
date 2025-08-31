#!/usr/bin/env bash
set -euo pipefail

# Pick docker compose command (v2 or legacy)
if docker compose version >/dev/null 2>&1; then
  COMPOSE_CMD=(docker compose)
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE_CMD=(docker-compose)
else
  echo "docker compose is required but not found (install Docker)" >&2
  exit 1
fi

cleanup() {
  echo "Stopping test databases..."
  "${COMPOSE_CMD[@]}" down -v || true
}
trap cleanup EXIT

echo "Starting test databases..."
"${COMPOSE_CMD[@]}" up -d

echo "Running tests..."
# each test is cleaning db afterwards, so they can't run in parallel.
export GOFLAGS="${GOFLAGS:-} -p=1 -timeout=30s"

go test -v ./...

echo "Running pgx4 tests..."
go test -v -tags pgx4 -run '^TestPostgres_PGX4$' ./...

echo "Running pgx5 tests..."
go test -v -tags pgx5 -run '^TestPostgres_PGX5$' ./...

echo "Running nocgo sqlite tests..."
CGO_ENABLED=0 go test -v -tags sqlite_nocgo -run '^TestSQLite_NoCGO$' ./...

echo "Tests finished. Cleaning up."
