# agent-stats

`agent-stats` reports which skills you and your AI coding agents use across Claude Code, Codex, Pi, OpenCode, and Antigravity. It reads locally stored session history and opens OpenCode databases in read-only mode.

## Install

Download the prebuilt archive for your platform from the [latest GitHub release](https://github.com/alexgorbatchev/agent-stats-cli/releases/latest) and place `agent-stats` on your `PATH`.

```sh
# macOS (Apple Silicon)
mkdir -p "$HOME/.local/bin"
curl -fsSLO https://github.com/alexgorbatchev/agent-stats-cli/releases/latest/download/agent-stats_darwin_arm64.tar.gz
tar -xzf agent-stats_darwin_arm64.tar.gz -C "$HOME/.local/bin"
"$HOME/.local/bin/agent-stats" --version
```

## Usage

```sh
agent-stats usage skills --limit 10
agent-stats usage skills --by-harness --since 30d
agent-stats usage skills --harness codex,pi --json
```

```text
┌─────────────┬───────┬──────┬───────┐
│ Skill       │ Model │ User │ Total │
├─────────────┼───────┼──────┼───────┤
│ bun         │ 2     │ 1    │ 3     │
│ docs-writer │ 2     │ 1    │ 3     │
└─────────────┴───────┴──────┴───────┘
6 invocations of 2 skills across 3 sessions. Model: the agent loaded the skill itself. User: you invoked it.
```

| Flag | Meaning |
| --- | --- |
| `--harness <names>` | Select `claude-code`, `codex`, `pi`, `opencode`, or `agy` (alias `antigravity`). Repeat the flag or supply a comma-separated list. |
| `--by-harness` | Show one row per skill and harness. |
| `--since <time>` | Count invocations at or after a date (`2026-10-01`, local midnight), an RFC 3339 timestamp, or a lookback (`30d` or `12h`). |
| `--limit <n>` | Show at most this many rows. The default, `0`, shows all rows. |
| `--json` | Print a JSON array with `skill`, `model`, `user`, and `total`. With `--by-harness`, include `harness`. |

Set `AGENT=1` for headerless TSV or compact JSON. Agent mode separates columns as `skill`, optional `harness`, `model`, `user`, and `total`. Human mode shows a table and a summary over the full report. Warnings go to stderr when a transcript or database cannot be read, and processing continues.

Default session locations and their environment overrides appear in `agent-stats --help`. Codex includes compressed and archived sessions. Claude Code and Pi forks count invocations recorded after the fork was created. Use `agent-stats skill` to install the bundled agent skill.

## Development

```sh
just setup
just run usage skills --limit 10
just check
```

The check requires Clippy, rustfmt, and `cargo-llvm-cov` with the `llvm-tools-preview` component. The Go reference repository and verified commit are recorded in [go-upstream.json](go-upstream.json).

## License

[MIT](LICENSE)
