set dotenv-load

MIGRATION_SCRIPT := "./sqlite/migration.sql"
BUILD_DIR := "bin"
DB := "fran.db"
BINARY_NAME := "fran"

default:
    just --list

tidy:
    go mod tidy

clean:
    rm -rf {{BUILD_DIR}}
    go clean -testcache

create-tables:
    sqlite3 {{DB}} < {{MIGRATION_SCRIPT}}

build: tidy
    go build -o {{BUILD_DIR}}/{{BINARY_NAME}} ./cmd/fran.go

test:
    go test -v -race ./...
