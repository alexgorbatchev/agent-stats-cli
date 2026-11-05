#!/usr/bin/env bash
set -euo pipefail

minimum=$1
mkdir -p .tmp
run_dir=$(mktemp -d "$PWD/.tmp/coverage.XXXXXX")
consumer_pid=

live_consumers() {
    ps -axo pgid=,stat= | awk -v group="$consumer_pid" '$1 == group && $2 !~ /^Z/ { print $1 }'
}

stop_consumer() {
    local signal attempt live
    [[ -n "$consumer_pid" ]] || return 0
    for signal in TERM KILL; do
        kill -"$signal" -- "-$consumer_pid" 2>/dev/null || true
        for ((attempt = 0; attempt < 50; attempt++)); do
            live=$(live_consumers) || return 1
            if [[ -z "$live" ]]; then
                # The caller already handles its exit status; cancellation may interrupt wait.
                wait "$consumer_pid" 2>/dev/null || true
                consumer_pid=
                return 0
            fi
            sleep 0.1
        done
    done
    return 1
}

cleanup() {
    local result=$?
    trap - EXIT
    trap '' HUP INT TERM
    if ! stop_consumer; then
        printf 'ERROR: coverage processes did not stop; preserving %s\n' "$run_dir" >&2
        exit 1
    fi
    rm -r -- "$run_dir" || result=1
    exit "$result"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

export TMPDIR="$run_dir" GOTMPDIR="$run_dir"
# Each consumer owns a process group so cancellation also stops its compiler/test children.
set -m
run_consumer() {
    local result=0
    "$@" &
    consumer_pid=$!
    wait "$consumer_pid" || result=$?
    stop_consumer || return 1
    return "$result"
}

run_consumer go test -race -coverprofile="$run_dir/coverage.out" ./...
run_consumer go tool cover -func="$run_dir/coverage.out" > "$run_dir/summary"
total=$(awk '/^total:/ { print substr($3, 1, length($3) - 1) }' "$run_dir/summary")
if awk "BEGIN { exit ($minimum <= $total) ? 0 : 1 }"; then
    echo "statement coverage ${total}% meets the ${minimum}% floor"
else
    echo "statement coverage ${total}% is below the ${minimum}% floor" >&2
    exit 1
fi
