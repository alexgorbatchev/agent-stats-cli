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
    go run ./cmd/agent-stats {{ args }}

# Run CLI in agent-facing mode
run-ai *args:
    AGENT=1 go run ./cmd/agent-stats {{ args }}

# Run the tests with the race detector
test:
    go test -race ./...

# Run the tests and fail when statement coverage drops below the floor
coverage:
    bash scripts/check-coverage.sh {{ min_coverage }}

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
