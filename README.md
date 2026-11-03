`agent-stats` reports how AI coding agents are used on your machine, starting with which skills they use most. It reads the session history that Claude Code, Codex, Pi, and OpenCode already keep locally, so there is nothing to install into the agents and no daemon to run.

# What It Does

- **Ranks skills by use**: Counts every skill invocation across all stored sessions and lists skills from most to least used.
- **Separates who asked**: Splits each count into loads the agent made on its own and skills you invoked explicitly.
- **Covers four harnesses**: Reads Claude Code, Codex, Pi, and OpenCode history, including Codex's compressed and archived sessions.
- **Filters by harness and time**: Narrows the report to selected harnesses and to a recent window such as the last 30 days.
- **Serves people and agents**: Prints a table for people, and tab-separated lines or JSON for scripts and agents.

# How It Works

- Finds every session the supported harnesses have stored on this machine.
- Reads each session and picks out the moments a skill was invoked.
- Adds the invocations up per skill, or per skill and harness, and prints them sorted by total.

# How it Really Works

- Sessions are read from the harnesses' own locations: `~/.claude/projects`, `~/.codex/sessions` and `~/.codex/archived_sessions`, `~/.pi/agent/sessions`, and `~/.local/share/opencode`. The harnesses' environment variables, such as `CLAUDE_CONFIG_DIR`, `CODEX_HOME`, `PI_SESSIONS_DIR`, `OPENCODE_DB`, and `XDG_DATA_HOME`, move these locations the same way they move them for the harness.
- Nothing is written. Transcripts are only read, and OpenCode databases are opened read-only, so running `agent-stats` next to active agents is safe.
- A skill counts as **model** when the agent loaded it itself and **user** when you invoked it: a Claude Code `Skill` call or slash command, a Codex `$name` mention or a read of the skill's `SKILL.md`, a Pi `read` of `SKILL.md` or `/skill:name`, or an OpenCode `skill` call. Commands you ran yourself with Codex's `!` are not counted.
- Each load is counted once. Reading the rest of a long `SKILL.md` in later chunks does not add invocations, and a Claude Code or Pi fork, which starts by replaying its parent's history, counts only what happened after it was forked.
- Every run reads all matching history from scratch; nothing is cached. Thousands of sessions take seconds to tens of seconds, spread across all CPU cores. On a terminal, a progress bar on stderr shows how many sessions have been read, and it clears itself before the results print.
- A session that cannot be read, such as a damaged archive, is skipped with a warning on stderr while the rest are still counted, and the exit status stays 0. Invalid flags exit 1.
- Results go to stdout and warnings and errors to stderr, with the home directory shown as `~`. Setting `AGENT=1` switches to compact, tab-separated output meant for agents and scripts.

# Installation

Download the prebuilt binary for your platform from the [latest release](https://github.com/alexgorbatchev/agent-stats-cli/releases/latest).

```bash
# macOS (Apple Silicon)
curl -fsSL https://github.com/alexgorbatchev/agent-stats-cli/releases/latest/download/agent-stats_darwin_arm64.tar.gz | tar -xz -C ~/.local/bin
```

# Quick Start

```bash
# Show the ten most-used skills across every harness
agent-stats usage skills --limit 10

# Compare harnesses over the last 30 days
agent-stats usage skills --by-harness --since 30d

# Only Codex and Pi, as JSON
agent-stats usage skills --harness codex,pi --json
```

Sample Output:
```
┌───────────────────────┬───────┬──────┬───────┐
│ Skill                 │ Model │ User │ Total │
├───────────────────────┼───────┼──────┼───────┤
│ typescript            │ 722   │ 0    │ 722   │
│ claude-code-codegraph │ 669   │ 0    │ 669   │
│ docs-writer           │ 667   │ 0    │ 667   │
└───────────────────────┴───────┴──────┴───────┘
7654 invocations of 148 skills across 2200 sessions. Model: the agent loaded the skill itself. User: you invoked it.
```

# Options & Flags

| Flag | Short | Default | Description |
| :--- | :--- | :--- | :--- |
| `--help` | `-h` | `false` | Print help for any command |
| `--version` | `-v` | `false` | Print the version and exit |

`agent-stats usage skills`

| Flag | Short | Default | Description |
| :--- | :--- | :--- | :--- |
| `--harness <names>` | | all harnesses | Only count `claude-code`, `codex`, `pi`, or `opencode`; repeat the flag or separate names with commas |
| `--by-harness` | | `false` | Show one row per skill and harness |
| `--since <time>` | | everything | Only count invocations since a time: days such as `30d`, a duration such as `12h`, a date such as `2026-10-01`, or an RFC 3339 time |
| `--limit <n>` | | `0` | Show at most this many rows; `0` shows all |
| `--json` | | `false` | Print the rows as a JSON array |

`agent-stats skill` prints how to install the agent skill that teaches AI agents to operate `agent-stats`.

# License

[MIT License](LICENSE) (c) 2026 Alex Gorbatchev
