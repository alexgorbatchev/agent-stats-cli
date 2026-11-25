set dotenv-load := false

default:
    @just --list

# Prepare development dependencies
setup:
    cargo fetch --locked

# Run the CLI
run *args:
    cargo run --locked --bin agent-stats -- {{ args }}

# Run the CLI in agent mode
run-ai *args:
    AGENT=1 cargo run --locked --bin agent-stats -- {{ args }}

# Run offline checks against the locked dependency graph
test:
    cargo test --locked

# Run tests when package or test inputs changed since the merge base
test-affected base="origin/main":
    #!/usr/bin/env bash
    set -euo pipefail
    candidate={{ quote(base) }}
    if git rev-parse --verify --quiet --end-of-options "$candidate^{commit}" >/dev/null; then
        base=$(git merge-base -- "$candidate" HEAD)
        files=$(git diff --name-only "$base" --; git ls-files --others --exclude-standard)
        affected=false
        while IFS= read -r file; do
            case "$file" in
                ""|README.md|docs/*|LICENSE|go-upstream.json) ;;
                *) affected=true ;;
            esac
        done <<< "$files"
        if ! "$affected"; then
            echo 'No agent-stats test inputs changed.'
            exit 0
        fi
    fi
    echo 'Selected agent-stats-cli: package or test inputs changed.'
    just test

# Check line and function coverage
coverage:
    cargo llvm-cov --all-targets --locked --ignore-filename-regex '(^|/)tests/' --fail-under-lines 90 --fail-under-functions 90

lint:
    cargo fmt --all -- --check
    cargo clippy --all-targets --locked -- -D warnings

fmt:
    cargo fmt --all

# Run static and coverage-gated checks
check: lint coverage

# Build the release binary
build:
    cargo build --release --locked

install:
    cargo install --path . --locked
