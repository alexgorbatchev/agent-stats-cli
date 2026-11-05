set dotenv-load := false

# Minimum statement coverage the module must keep, enforced by `just coverage`
min_coverage := "90"

# Default recipe: list available recipes
default:
    @just --list

# Prepare development dependencies
setup:
    go mod download

# Run CLI in human mode (default)
run *args:
    go run ./cmd/agent-stats {{args}}

# Run CLI in agent-facing mode
run-ai *args:
    AGENT=1 go run ./cmd/agent-stats {{args}}

# Run the tests with the race detector
test:
    go test -race ./...

# Run the tests and fail when statement coverage drops below the floor
coverage:
    #!/usr/bin/env bash
    set -euo pipefail
    go test -race -coverprofile=coverage.out ./...
    total="$(go tool cover -func=coverage.out | awk '/^total:/ { print substr($3, 1, length($3) - 1) }')"
    if awk "BEGIN { exit ({{ min_coverage }} <= ${total}) ? 0 : 1 }"; then
        echo "statement coverage ${total}% meets the {{ min_coverage }}% floor"
    else
        echo "statement coverage ${total}% is below the {{ min_coverage }}% floor" >&2
        exit 1
    fi

# Build binary into bin/
build:
    mkdir -p bin
    go build -o bin/agent-stats ./cmd/agent-stats

# Install binary to GOPATH bin directory
install:
    go install ./cmd/agent-stats

# Check formatting and module hygiene, vet, and run golangci-lint
lint:
    @out="$(gofmt -l .)"; if [ -n "$out" ]; then echo "Unformatted Go files:"; echo "$out"; exit 1; fi
    go mod tidy -diff
    go vet ./...
    golangci-lint run

# Alias for lint
vet: lint

# Format Go code
fmt:
    go fmt ./...

# Run every static check and the coverage-gated tests
check:
    just lint
    just coverage

# Clean build and coverage outputs
clean:
    rm -rf bin coverage.out dist
