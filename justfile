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

# Run affected tests by test name
test-affected filter:
    cargo test --locked {{ quote(filter) }}

# Check line coverage
coverage:
    cargo llvm-cov --all-targets --locked --ignore-filename-regex '(^|/)tests/' --fail-under-lines 90

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
