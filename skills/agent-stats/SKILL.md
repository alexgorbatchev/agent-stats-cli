---
name: "agent-stats"
description: "Use when answering questions about how AI coding agents were used on this machine, such as which skills are used most, with the agent-stats CLI."
author: "Alex Gorbatchev"
metadata:
  created_on: 2026-11-03 10:39
  last_modified: 2026-11-10 15:03
  status: current
---

## Execution rules

Set `AGENT=1` on every invocation, or export it for the session. Values `1`, `true`, and `yes` enable agent output, with case and surrounding whitespace ignored; anything else gives human output. Read the exit status as well as stdout: 0 means success, 1 means the command failed, and failures print one `ERR: <message>` line to stderr.

`agent-stats` only reads. It never writes to harness storage, and it opens OpenCode databases read-only. It reads the session history on the machine it runs on, which can take tens of seconds for thousands of sessions.

Paths in messages show the home directory as `~`.

## `agent-stats`

Prints help. Accepts no positional arguments. `--help`, `-h` bool prints help for any command, and `--version`, `-v` bool prints the version followed by a newline.

### Environment variables

These select where sessions are read from. They are the harnesses' own settings, read the same way the harnesses read them.

| Variable | Meaning |
| --- | --- |
| `AGENT` | `1`, `true`, or `yes` switches to agent output |
| `CLAUDE_CONFIG_DIR` | Claude Code configuration directory; its `projects/` is searched before `~/.claude/projects` |
| `CODEX_HOME` | Codex home directory; `sessions/` and `archived_sessions/` live under it (default `~/.codex`) |
| `CODEX_SESSIONS_DIR` | Codex sessions directory, overriding `CODEX_HOME/sessions` |
| `PI_SESSIONS_DIR` | Pi sessions directory |
| `PI_CODING_AGENT_DIR` | Pi agent directory; sessions are read from its `sessions/` when `PI_SESSIONS_DIR` is unset (default `~/.pi/agent`) |
| `OPENCODE_DB_PATH` | OpenCode database file, overriding every other OpenCode location |
| `OPENCODE_DB` | OpenCode's own database setting: an absolute path, or a name relative to the OpenCode data directory; `:memory:` means none |
| `XDG_DATA_HOME` | Data directory holding `opencode/` (default `~/.local/share`) |

## `agent-stats usage`

Prints help for usage reports. Accepts no positional arguments.

## `agent-stats usage skills`

Counts every skill invocation in the stored sessions of Claude Code, Codex, Pi, and OpenCode, and lists skills from most to least used. Accepts no positional arguments.

An invocation is counted as `model` when the agent loaded the skill itself and `user` when the user invoked it:

- Claude Code: a `Skill` tool call (model); a slash command followed by the skill's content (user).
- Codex: a successful shell command that reads a skill's `SKILL.md` from its first line (model); a `$name` mention (user). Commands the user ran with `!` are not counted.
- Pi: a `read` of a skill's `SKILL.md` from its first line (model); a `/skill:name` command (user).
- OpenCode: a `skill` tool call (model).

Shell reads expand unquoted brace groups such as `skills/{programming,golang}/SKILL.md` into individual skills, counting each skill once per command. Quoted braces remain literal.

A Claude Code fork subagent or a Pi fork starts by replaying its parent's history; it counts only what happened after it was created, so the parent's invocations are not counted twice. A fork whose creation time cannot be determined is skipped and reported as unreadable data.

Rows are sorted by total, then skill name, then harness.

| Flag | Type | Default | Meaning |
| --- | --- | --- | --- |
| `--harness` | stringSlice | `[]` (all harnesses) | Only count these harnesses: `claude-code`, `codex`, `pi`, `opencode`. Repeat the flag or separate names with commas; case is ignored. |
| `--by-harness` | bool | `false` | Show one row per skill and harness instead of one row per skill. |
| `--since` | string | `""` (everything) | Only count invocations at or after a time: a number of days such as `30d`, a Go duration such as `12h` or `90m`, a date such as `2026-10-01` (local midnight), or an RFC 3339 time. |
| `--limit` | int | `0` (all rows) | Show at most this many rows. Must be zero or more. |
| `--json` | bool | `false` | Print the rows as a JSON array instead. |

### Output

Agent mode prints one tab-separated line per row and nothing else:

- default: `skill<TAB>model<TAB>user<TAB>total`
- with `--by-harness`: `skill<TAB>harness<TAB>model<TAB>user<TAB>total`

No invocations prints nothing.

With `--json`, stdout is a JSON array of objects with `skill`, `model`, `user`, and `total`, plus `harness` with `--by-harness`. Agent mode prints it on one line; human mode indents it. No invocations prints `[]`.

Human mode prints a table and a summary line with the total invocations, distinct skills, and sessions read. While scanning, human mode shows a progress bar on stderr when stderr is a terminal and clears it before the results; agent mode and `--json` show none.

### Errors and warnings

- An unknown `--harness` value, an unparseable or negative `--since`, or a negative `--limit` exits 1 with `ERR: <message>`.
- A transcript or database that cannot be read is skipped and reported on stderr as `WARN: skipped unreadable data: <path>: <reason>`; counting continues and the exit status stays 0.
- Interrupting the command exits 1.

### Examples

```sh
AGENT=1 agent-stats usage skills --limit 10
AGENT=1 agent-stats usage skills --harness codex,pi --by-harness
AGENT=1 agent-stats usage skills --since 30d --json
```

## `agent-stats skill`

Prints how to install this skill with the Skills CLI. Accepts no positional arguments or flags, and prints the same text in both modes.

## `agent-stats completion`

Generates shell completion scripts: `agent-stats completion bash`, `agent-stats completion zsh`, `agent-stats completion fish`, and `agent-stats completion powershell`. Each accepts `--no-descriptions` bool to leave out command descriptions. Writes the script to stdout.

## `agent-stats help`

`agent-stats help [command]` prints help for a command, like `--help`.
