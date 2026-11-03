---
created_on: 2026-11-03 10:39
last_modified: 2026-11-03 14:25
status: current
---

# agent-stats-cli

Single-run CLI (`agent-stats`) that reports usage statistics, such as most-used skills, from the session history Claude Code, Codex, Pi, and OpenCode keep locally.

## Commands

- Prepare dependencies: `just setup`
- Run the CLI: `just run [args...]`; agent mode: `just run-ai [args...]` (`AGENT=1`)
- Test with the race detector: `just test`
- Enforce the 90% statement-coverage floor: `just coverage`
- Lint (gofmt, `go mod tidy -diff`, vet, golangci-lint): `just lint`
- Run every gate: `just check` (lint, then coverage); CI runs the same recipe
- Build `bin/agent-stats`: `just build`

## Layout

- `cmd/agent-stats/`: command tree, help setup, output rendering
- `internal/usage/`: session discovery, transcript parsing, and skill counting
- `internal/displaypath/`: home-directory folding for every printed path
- `skills/agent-stats/SKILL.md`: the agent skill the `skill` command points to

## Conventions

- Harness knowledge belongs in `github.com/alexgorbatchev/agent-parser`: session locations come from `parser.DiscoverSessions`, transcripts open through `parser.OpenTranscript`, and skill invocations come from `EventData.Skills`. Add harness formats or detection rules there, not here.
- Help, the `skill` command, and the agent-help alert come from `cobra-help-tree` v4 (`HelpOptions.Skill`). Human output uses `lipgloss` tables; agent output is tab-separated lines; `--json` prints arrays.
- Route every printed path, including paths inside error messages, through `displaypath.Fold` or `displaypath.FoldText`.
- Tests stay offline and hermetic: set `HOME` to a temporary directory and clear the harness location variables before discovering sessions.

## Agent skill

- Keep `skills/agent-stats/SKILL.md` in sync in the same change whenever a command, argument, flag, shorthand, type, default, accepted value, environment variable, output, error, or side effect changes, including generated help and completion commands. Update `last_modified`.
- `TestSkillDocumentsCommandInterface` compares the skill with the live command tree and fails on omissions; keep it passing rather than loosening it.
- Write affirmative usage instructions and factual contracts in the skill. Keep development instructions and prohibitions in this file. Never embed the skill in the binary or keep a second copy.

## Boundaries

- Always: keep at least 90% statement coverage; `just coverage` enforces it.
- Always: run `just check` before committing.
- Always: change results-affecting code together with tests that observe the change.
- Never: write to, lock, or modify harness storage; open OpenCode databases read-only.
- Never: commit compiled binaries (`bin/`).
