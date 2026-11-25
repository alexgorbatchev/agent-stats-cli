Use `just setup` to fetch locked dependencies and `just check` before publishing changes. Tests use synthetic local histories and require no harness credentials.

Session locations, transcript formats, and skill-invocation detection belong in [agent-parser](https://github.com/alexgorbatchev/agent-parser). This CLI owns aggregation and rendering. Keep the CLI commands, README, and bundled skill in sync. Update `go-upstream.json` after verifying changes against the referenced Go commit.
