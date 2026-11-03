package usage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	parser "github.com/alexgorbatchev/agent-parser"
	"github.com/klauspost/compress/zstd"
)

func isolateEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"CLAUDE_CONFIG_DIR", "CODEX_HOME", "CODEX_SESSIONS_DIR", "PI_CODING_AGENT_DIR",
		"PI_SESSIONS_DIR", "OPENCODE_DB_PATH", "OPENCODE_DB", "XDG_DATA_HOME",
	} {
		t.Setenv(key, "")
	}
}

func write(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeZstd(t *testing.T, path string, lines ...string) {
	t.Helper()
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = encoder.Close() }()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data := encoder.EncodeAll([]byte(strings.Join(lines, "\n")+"\n"), nil)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

const opencodeSchema = `
CREATE TABLE project (id TEXT PRIMARY KEY, name TEXT NOT NULL, worktree TEXT, vcs TEXT);
CREATE TABLE session (id TEXT PRIMARY KEY, project_id TEXT, parent_id TEXT, slug TEXT, directory TEXT NOT NULL,
	title TEXT, version TEXT, agent TEXT, model TEXT, cost REAL,
	tokens_input INTEGER NOT NULL DEFAULT 0, tokens_output INTEGER NOT NULL DEFAULT 0,
	tokens_reasoning INTEGER NOT NULL DEFAULT 0, tokens_cache_read INTEGER NOT NULL DEFAULT 0,
	tokens_cache_write INTEGER NOT NULL DEFAULT 0, time_created INTEGER NOT NULL, time_updated INTEGER NOT NULL,
	time_archived INTEGER, metadata TEXT);
CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
CREATE TABLE permission (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, type TEXT NOT NULL, data TEXT NOT NULL, time_created INTEGER NOT NULL);
`

func writeOpencode(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	stmts := []string{
		opencodeSchema,
		`INSERT INTO session (id, directory, time_created, time_updated) VALUES ('ses_1', '/repo', 1762160000000, 1762160000000)`,
		`INSERT INTO message (id, session_id, time_created, data) VALUES ('msg_1', 'ses_1', 1762160000000, '{"role":"assistant"}')`,
		`INSERT INTO part (id, message_id, session_id, time_created, data) VALUES ('p1', 'msg_1', 'ses_1', 1762160001000,
			'{"type":"tool","tool":"skill","callID":"c1","state":{"status":"completed","input":{"name":"git-release"},"time":{"start":1762160001000}}}')`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
}

// buildHome writes one session per harness. Every invocation is on 2025-11-03 except
// the Codex user mention, which is a year later.
func buildHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	write(t, filepath.Join(home, ".claude/projects/-repo/s1.jsonl"),
		`{"type":"assistant","sessionId":"s1","timestamp":"2025-11-03T10:00:00Z","message":{"id":"m1","role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","id":"t1","name":"Skill","input":{"skill":"docs-writer"}}]}}`,
		`{"type":"user","sessionId":"s1","timestamp":"2025-11-03T10:01:00Z","message":{"role":"user","content":"<command-name>/docs-writer</command-name>"}}`,
		`{"type":"user","sessionId":"s1","timestamp":"2025-11-03T10:01:01Z","isMeta":true,"message":{"role":"user","content":[{"type":"text","text":"Base directory for this skill: /x/skills/docs-writer"}]}}`,
		`not json`,
	)
	writeZstd(t, filepath.Join(home, ".codex/sessions/2025/11/03/rollout-2025-11-03T10-00-00-01a1aaaa-0000-7000-8000-000000000001.jsonl.zst"),
		`{"timestamp":"2025-11-03T10:00:00Z","type":"event_msg","payload":{"type":"item_completed","item":{"type":"CommandExecution","command":["/bin/zsh","-lc","cat /x/skills/docs-writer/SKILL.md /x/skills/bun/SKILL.md"],"exit_code":0}}}`,
		`{"timestamp":"2026-11-03T10:00:00Z","type":"event_msg","payload":{"type":"item_completed","item":{"type":"UserMessage","content":[{"type":"skill","name":"bun","path":"/x/skills/bun/SKILL.md"}]}}}`,
	)
	write(t, filepath.Join(home, ".pi/agent/sessions/--repo--/2025-11-03T10-00-00-000Z_parent.jsonl"),
		`{"type":"session","version":3,"id":"parent","timestamp":"2025-11-03T10:00:00Z","cwd":"/repo"}`,
		`{"type":"message","timestamp":"2025-11-03T10:00:01Z","message":{"role":"assistant","content":[{"type":"toolCall","id":"c1","name":"read","arguments":{"path":"/x/skills/golang/SKILL.md"}}]}}`,
	)
	// The fork replays the parent's read, then reads a skill of its own.
	write(t, filepath.Join(home, ".pi/agent/sessions/--repo--/2025-11-03T10-00-00-000Z_parent/forks/2025-11-03T11-00-00-000Z_fork.jsonl"),
		`{"type":"session","version":3,"id":"fork","timestamp":"2025-11-03T11:00:00Z","cwd":"/repo"}`,
		`{"type":"message","timestamp":"2025-11-03T10:00:01Z","message":{"role":"assistant","content":[{"type":"toolCall","id":"c1","name":"read","arguments":{"path":"/x/skills/golang/SKILL.md"}}]}}`,
		`{"type":"message","timestamp":"2025-11-03T11:00:01Z","message":{"role":"assistant","content":[{"type":"toolCall","id":"c2","name":"read","arguments":{"path":"/x/skills/bun/SKILL.md"}}]}}`,
	)
	writeOpencode(t, filepath.Join(home, ".local/share/opencode/opencode.db"))

	// A Claude Code fork the session spawned at 10:05: it replays the session's Skill call
	// and the spawning call itself, then loads a skill of its own.
	spawn := `{"type":"assistant","sessionId":"s1","timestamp":"2025-11-03T10:05:00Z","message":{"id":"m9","role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","id":"toolu_spawn","name":"Agent","input":{}}]}}`
	session := filepath.Join(home, ".claude/projects/-repo/s1.jsonl")
	data, err := os.ReadFile(session)
	if err != nil {
		t.Fatal(err)
	}
	write(t, session, strings.TrimRight(string(data), "\n"), spawn)
	subagents := filepath.Join(home, ".claude/projects/-repo/s1/subagents")
	write(t, filepath.Join(subagents, "agent-fork.meta.json"), `{"agentType":"fork","isFork":true,"toolUseId":"toolu_spawn"}`)
	write(t, filepath.Join(subagents, "agent-fork.jsonl"),
		`{"type":"assistant","sessionId":"s1","agentId":"fork","timestamp":"2025-11-03T10:00:00Z","message":{"id":"r1","role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","id":"t1","name":"Skill","input":{"skill":"docs-writer"}}]}}`,
		`{"type":"assistant","sessionId":"s1","agentId":"fork","timestamp":"2025-11-03T10:05:00Z","message":{"id":"r2","role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","id":"toolu_spawn","name":"Skill","input":{"skill":"docs-writer"}}]}}`,
		`{"type":"assistant","sessionId":"s1","agentId":"fork","timestamp":"2025-11-03T10:05:01Z","message":{"id":"f1","role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","id":"f1","name":"Skill","input":{"skill":"golang"}}]}}`,
	)
	return home
}

func TestSkillCountsAcrossHarnesses(t *testing.T) {
	isolateEnv(t)
	report, err := SkillCounts(context.Background(), Options{Home: buildHome(t), Workers: 2})
	if err != nil {
		t.Fatalf("SkillCounts: %v", err)
	}
	// The fork's replayed docs-writer calls (before and at the spawn) are not counted again;
	// its own golang load after the spawn is.
	want := []Count{
		{Skill: "bun", Harness: parser.HarnessCodex, Model: 1, User: 1},
		{Skill: "docs-writer", Harness: parser.HarnessClaudeCode, Model: 1, User: 1},
		{Skill: "bun", Harness: parser.HarnessPi, Model: 1},
		{Skill: "docs-writer", Harness: parser.HarnessCodex, Model: 1},
		{Skill: "git-release", Harness: parser.HarnessOpencode, Model: 1},
		{Skill: "golang", Harness: parser.HarnessClaudeCode, Model: 1},
		{Skill: "golang", Harness: parser.HarnessPi, Model: 1},
	}
	if !reflect.DeepEqual(report.Counts, want) {
		t.Errorf("counts = %+v\nwant     %+v", report.Counts, want)
	}
	if report.Sessions != 6 {
		t.Errorf("sessions = %d, want 6", report.Sessions)
	}
	if len(report.Problems) != 0 {
		t.Errorf("problems = %+v", report.Problems)
	}

	combined := report.Combined()
	wantCombined := []Count{
		{Skill: "bun", Model: 2, User: 1},
		{Skill: "docs-writer", Model: 2, User: 1},
		{Skill: "golang", Model: 2},
		{Skill: "git-release", Model: 1},
	}
	if !reflect.DeepEqual(combined, wantCombined) {
		t.Errorf("combined = %+v\nwant       %+v", combined, wantCombined)
	}
}

func TestSkillCountsFiltersHarnessesAndTime(t *testing.T) {
	isolateEnv(t)
	home := buildHome(t)
	report, err := SkillCounts(context.Background(), Options{
		Home:      home,
		Harnesses: []parser.Harness{parser.HarnessCodex},
		Since:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("SkillCounts: %v", err)
	}
	want := []Count{{Skill: "bun", Harness: parser.HarnessCodex, User: 1}}
	if !reflect.DeepEqual(report.Counts, want) || report.Sessions != 1 {
		t.Errorf("counts = %+v (sessions %d), want %+v from 1 session", report.Counts, report.Sessions, want)
	}
}

func TestSkillCountsReportsUnreadableSessions(t *testing.T) {
	isolateEnv(t)
	home := buildHome(t)
	corrupt := filepath.Join(home, ".codex/sessions/2025/11/04/rollout-2025-11-04T10-00-00-01a1aaaa-0000-7000-8000-000000000009.jsonl.zst")
	write(t, corrupt, "not zstd")
	write(t, filepath.Join(home, ".local/share/opencode/opencode-broken.db"), "not a database")

	report, err := SkillCounts(context.Background(), Options{Home: home})
	if err != nil {
		t.Fatalf("SkillCounts: %v", err)
	}
	var paths []string
	for _, problem := range report.Problems {
		paths = append(paths, problem.Path)
	}
	joined := strings.Join(paths, "\n")
	if !strings.Contains(joined, corrupt) {
		t.Errorf("problems %v do not name the corrupt rollout", paths)
	}
	if len(report.Problems) < 2 {
		t.Errorf("problems = %+v, want the corrupt rollout and the broken database", report.Problems)
	}
	if len(report.Counts) == 0 {
		t.Error("readable sessions were not counted")
	}
	// buildHome holds 6 readable sessions; the corrupt rollout and the broken database
	// are reported, not counted as read.
	if report.Sessions != 6 {
		t.Errorf("sessions = %d, want only the 6 readable ones", report.Sessions)
	}
}

func TestSkillCountsStopsOnCancel(t *testing.T) {
	isolateEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := SkillCounts(ctx, Options{Home: buildHome(t)}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestCountTranscriptReportsReadFailure(t *testing.T) {
	counts := map[countKey]*Count{}
	reader := iotest.TimeoutReader(strings.NewReader(`{"type":"session","id":"x","timestamp":"2025-11-03T10:00:00Z"}` + "\n"))
	err := countTranscript(reader, parser.SessionSource{Harness: parser.HarnessPi}, 0, counts)
	if err == nil {
		t.Fatal("expected the read failure to be returned")
	}
}

func TestOpencodeDBsReusesHandles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.db")
	writeOpencode(t, path)
	dbs := newOpencodeDBs()
	defer dbs.closeAll()
	first, err := dbs.open(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := dbs.open(path)
	if err != nil || first != second {
		t.Fatalf("second open = %p, %v; want the same handle %p", second, err, first)
	}
}
