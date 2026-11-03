package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v4"
	"github.com/spf13/cobra"
)

const skillAlert = "ALERT: Agents must read `AGENT=1 agent-stats skill` before using this tool."

// fixtureHome makes HOME a temporary directory holding one Claude Code session with two
// model loads of docs-writer and one of bun, and one Pi session with a bun read, and
// clears every harness location override.
func fixtureHome(t *testing.T) string {
	t.Helper()
	for _, key := range []string{
		"CLAUDE_CONFIG_DIR", "CODEX_HOME", "CODEX_SESSIONS_DIR", "PI_CODING_AGENT_DIR",
		"PI_SESSIONS_DIR", "OPENCODE_DB_PATH", "OPENCODE_DB", "XDG_DATA_HOME",
	} {
		t.Setenv(key, "")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	skillCall := func(id, skill, at string) string {
		return `{"type":"assistant","sessionId":"s1","timestamp":"` + at + `","message":{"id":"` + id + `","role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","id":"t` + id + `","name":"Skill","input":{"skill":"` + skill + `"}}]}}`
	}
	writeFile(t, filepath.Join(home, ".claude/projects/-repo/s1.jsonl"),
		skillCall("1", "docs-writer", "2026-10-01T10:00:00Z"),
		skillCall("2", "docs-writer", "2026-11-02T10:00:00Z"),
		skillCall("3", "bun", "2026-11-02T11:00:00Z"),
	)
	writeFile(t, filepath.Join(home, ".pi/agent/sessions/--repo--/2026-11-02T10-00-00-000Z_p1.jsonl"),
		`{"type":"session","version":3,"id":"p1","timestamp":"2026-11-02T10:00:00Z","cwd":"/repo"}`,
		`{"type":"message","timestamp":"2026-11-02T10:00:01Z","message":{"role":"assistant","content":[{"type":"toolCall","id":"c1","name":"read","arguments":{"path":"/x/skills/bun/SKILL.md"}}]}}`,
	)
	return home
}

func writeFile(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func execute(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	t.Cleanup(func() { helpWriteError = nil })
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)
	cmd := newRootCommand()
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs(args)
	code = run(context.Background(), cmd)
	return out.String(), errOut.String(), code
}

func TestUsageSkillsAgentOutput(t *testing.T) {
	t.Setenv("AGENT", "1")
	fixtureHome(t)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"all harnesses", []string{"usage", "skills"}, "bun\t2\t0\t2\ndocs-writer\t2\t0\t2\n"},
		{"by harness", []string{"usage", "skills", "--by-harness"}, "docs-writer\tclaude-code\t2\t0\t2\nbun\tclaude-code\t1\t0\t1\nbun\tpi\t1\t0\t1\n"},
		{"one harness", []string{"usage", "skills", "--harness", "PI"}, "bun\t1\t0\t1\n"},
		{"limit", []string{"usage", "skills", "--limit", "1"}, "bun\t2\t0\t2\n"},
		{"nothing found", []string{"usage", "skills", "--harness", "codex"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code := execute(t, tt.args...)
			if code != 0 || stderr != "" || stdout != tt.want {
				t.Fatalf("code=%d stderr=%q stdout=%q, want %q", code, stderr, stdout, tt.want)
			}
		})
	}
}

func TestUsageSkillsSince(t *testing.T) {
	t.Setenv("AGENT", "1")
	fixtureHome(t)
	defer func(orig func() time.Time) { now = orig }(now)
	now = func() time.Time { return time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC) }

	stdout, _, code := execute(t, "usage", "skills", "--since", "7d")
	if code != 0 || stdout != "bun\t2\t0\t2\ndocs-writer\t1\t0\t1\n" {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
}

func TestUsageSkillsJSON(t *testing.T) {
	fixtureHome(t)
	for _, mode := range []string{"0", "1"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("AGENT", mode)
			stdout, _, code := execute(t, "usage", "skills", "--json", "--by-harness", "--harness", "pi")
			if code != 0 {
				t.Fatalf("code = %d", code)
			}
			var rows []countRow
			if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
				t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
			}
			want := []countRow{{Skill: "bun", Harness: "pi", Model: 1, Total: 1}}
			if len(rows) != 1 || rows[0] != want[0] {
				t.Errorf("rows = %+v, want %+v", rows, want)
			}
			if indented := strings.Contains(stdout, "\n  "); indented != (mode == "0") {
				t.Errorf("indentation in mode %s = %v", mode, indented)
			}
		})
	}
	t.Setenv("AGENT", "1")
	if stdout, _, _ := execute(t, "usage", "skills", "--json", "--harness", "codex"); stdout != "[]\n" {
		t.Errorf("empty JSON = %q, want []", stdout)
	}
}

func TestUsageSkillsHumanOutput(t *testing.T) {
	t.Setenv("AGENT", "0")
	fixtureHome(t)
	stdout, stderr, code := execute(t, "usage", "skills", "--by-harness", "--limit", "2")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	for _, want := range []string{"Skill", "Harness", "docs-writer", "claude-code", "4 invocations of 2 skills across 2 sessions."} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output does not contain %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "│ pi ") {
		t.Errorf("--limit 2 still shows the third row:\n%s", stdout)
	}

	stdout, _, _ = execute(t, "usage", "skills", "--harness", "opencode")
	if stdout != "[INFO] No skill invocations found in 0 sessions.\n" {
		t.Errorf("empty human output = %q", stdout)
	}
}

func TestUsageSkillsWarnsAboutUnreadableData(t *testing.T) {
	home := fixtureHome(t)
	corrupt := filepath.Join(home, ".codex/sessions/2026/11/02/rollout-2026-11-02T10-00-00-01a1aaaa-0000-7000-8000-000000000001.jsonl.zst")
	writeFile(t, corrupt, "not zstd")
	for mode, prefix := range map[string]string{"0": "[WARN] ", "1": "WARN: "} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("AGENT", mode)
			_, stderr, code := execute(t, "usage", "skills", "--harness", "codex")
			want := prefix + "skipped unreadable data: ~/.codex/sessions/2026/11/02/"
			if code != 0 || !strings.HasPrefix(stderr, want) || strings.Contains(stderr, home) {
				t.Fatalf("code=%d stderr=%q, want a folded %q", code, stderr, want)
			}
		})
	}
}

func TestUsageSkillsRejectsBadFlags(t *testing.T) {
	fixtureHome(t)
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"--harness", "cursor"}, `unknown harness "cursor": use one of claude-code, codex, pi, opencode`},
		{[]string{"--since", "last week"}, `invalid --since "last week"`},
		{[]string{"--since", "-2h"}, "must not be negative"},
		{[]string{"--limit", "-1"}, "--limit must be zero or more, got -1"},
		{[]string{"extra"}, `unknown command "extra"`},
	}
	for _, tt := range tests {
		for mode, prefix := range map[string]string{"0": "[ERROR] ", "1": "ERR: "} {
			t.Run(tt.want+"/"+mode, func(t *testing.T) {
				t.Setenv("AGENT", mode)
				_, stderr, code := execute(t, append([]string{"usage", "skills"}, tt.args...)...)
				if code != 1 || !strings.HasPrefix(stderr, prefix) || !strings.Contains(stderr, tt.want) {
					t.Fatalf("code=%d stderr=%q, want %q", code, stderr, tt.want)
				}
			})
		}
	}
}

func TestUsageSkillsStopsWhenInterrupted(t *testing.T) {
	t.Setenv("AGENT", "1")
	fixtureHome(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	errOut := new(bytes.Buffer)
	cmd := newRootCommand()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"usage", "skills"})
	if code := run(ctx, cmd); code != 1 || !strings.Contains(errOut.String(), "context canceled") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
}

func TestParseSince(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	tests := map[string]time.Time{
		"":                     {},
		"30d":                  now.AddDate(0, 0, -30),
		"12h":                  now.Add(-12 * time.Hour),
		"2026-10-01":           time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		"2026-10-01T08:00:00Z": time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC),
	}
	for value, want := range tests {
		got, err := parseSince(value, now)
		if err != nil || !got.Equal(want) {
			t.Errorf("parseSince(%q) = %v, %v; want %v", value, got, err, want)
		}
	}
	if _, err := parseSince("99999999999999999999d", now); err == nil {
		t.Error("an out-of-range day count must fail")
	}
}

func TestParseHarnessesDeduplicates(t *testing.T) {
	got, err := parseHarnesses([]string{"pi", " Codex ", "pi"})
	if err != nil || len(got) != 2 || got[0] != "pi" || got[1] != "codex" {
		t.Fatalf("parseHarnesses = %v, %v", got, err)
	}
}

func TestVersion(t *testing.T) {
	stdout, _, code := execute(t, "--version")
	if code != 0 || stdout != version+"\n" {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
}

func TestSkillCommand(t *testing.T) {
	want := "Install the agent-stats skill using the Skills CLI:\n\n  npx skills add alexgorbatchev/agent-stats-cli --skill agent-stats\n\nSkill:\n\n  https://github.com/alexgorbatchev/agent-stats-cli/tree/main/skills/agent-stats\n"
	for _, mode := range []string{"0", "1"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("AGENT", mode)
			stdout, _, code := execute(t, "skill")
			if code != 0 || stdout != want {
				t.Fatalf("code=%d stdout=%q", code, stdout)
			}
		})
	}
	if _, _, code := execute(t, "skill", "extra"); code != 1 {
		t.Fatal("skill must reject arguments")
	}
}

func walk(cmd *cobra.Command, visit func(*cobra.Command)) {
	if cmd.Hidden {
		return
	}
	visit(cmd)
	for _, child := range cmd.Commands() {
		walk(child, visit)
	}
}

func TestAgentHelpStartsWithSkillAlert(t *testing.T) {
	t.Setenv("AGENT", "1")
	root := newRootCommand()
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()
	var paths [][]string
	walk(root, func(cmd *cobra.Command) {
		paths = append(paths, append(strings.Fields(cmd.CommandPath())[1:], "--help"))
	})
	if len(paths) < 8 {
		t.Fatalf("only %d help screens", len(paths))
	}
	for _, args := range paths {
		stdout, _, code := execute(t, args...)
		if code != 0 || !strings.HasPrefix(stdout, skillAlert) {
			t.Fatalf("%v: code=%d help does not start with the alert:\n%s", args, code, stdout)
		}
	}
	t.Setenv("AGENT", "0")
	if stdout, _, _ := execute(t, "--help"); strings.Contains(stdout, skillAlert) {
		t.Error("human help must not carry the agent alert")
	}
}

func TestSkillDocumentsCommandInterface(t *testing.T) {
	path := filepath.Join("..", "..", "skills", "agent-stats", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `name: "agent-stats"`) {
		t.Fatal("the skill's frontmatter name must equal the --skill value")
	}
	if omissions := cobrahelptree.SkillOmissions(newRootCommand(), techCatalog, string(data)); len(omissions) > 0 {
		t.Fatalf("SKILL.md is missing:\n%s", strings.Join(omissions, "\n"))
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestRunFailsWhenOutputCannotBeWritten(t *testing.T) {
	fixtureHome(t)
	tests := []struct {
		name string
		mode string
		args []string
		want string
	}{
		{"human help", "0", []string{"--help"}, "[ERROR] "},
		{"agent help", "1", []string{"--help"}, "ERR: "},
		{"bare invocation", "0", nil, "[ERROR] "},
		{"usage group", "1", []string{"usage"}, "ERR: "},
		{"agent rows", "1", []string{"usage", "skills"}, "ERR: "},
		{"human table", "0", []string{"usage", "skills"}, "[ERROR] "},
		{"human empty", "0", []string{"usage", "skills", "--harness", "codex"}, "[ERROR] "},
		{"json", "1", []string{"usage", "skills", "--json"}, "ERR: "},
		{"skill", "1", []string{"skill"}, "ERR: "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AGENT", tt.mode)
			t.Cleanup(func() { helpWriteError = nil })
			stderr := new(bytes.Buffer)
			cmd := newRootCommand()
			cmd.SetOut(failingWriter{})
			cmd.SetErr(stderr)
			cmd.SetArgs(tt.args)
			if code := run(context.Background(), cmd); code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if !strings.HasPrefix(stderr.String(), tt.want) || !strings.Contains(stderr.String(), "disk full") {
				t.Fatalf("stderr = %q", stderr)
			}
		})
	}
}

func TestRunFailsWhenWarningCannotBeWritten(t *testing.T) {
	t.Setenv("AGENT", "1")
	home := fixtureHome(t)
	writeFile(t, filepath.Join(home, ".codex/sessions/2026/11/02/rollout-2026-11-02T10-00-00-01a1aaaa-0000-7000-8000-000000000001.jsonl.zst"), "not zstd")
	cmd := newRootCommand()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(failingWriter{})
	cmd.SetArgs([]string{"usage", "skills", "--harness", "codex"})
	if code := run(context.Background(), cmd); code != 1 {
		t.Fatalf("exit code = %d, want 1 when a warning cannot be written", code)
	}
}

func TestUsageSkillsScanIndicator(t *testing.T) {
	fixtureHome(t)
	defer func(orig func(io.Writer) bool) { isTerminal = orig }(isTerminal)
	isTerminal = func(io.Writer) bool { return true }

	tests := []struct {
		name      string
		mode      string
		args      []string
		indicator bool
	}{
		{"human on a terminal", "0", []string{"usage", "skills"}, true},
		{"agent mode", "1", []string{"usage", "skills"}, false},
		{"json", "0", []string{"usage", "skills", "--json"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AGENT", tt.mode)
			stdout, stderr, code := execute(t, tt.args...)
			if code != 0 {
				t.Fatalf("code = %d, stderr = %q", code, stderr)
			}
			if shown := strings.Contains(stderr, "Reading sessions") && strings.Contains(stderr, "/2"); shown != tt.indicator {
				t.Errorf("indicator shown = %v, want %v; stderr = %q", shown, tt.indicator, stderr)
			}
			if strings.Contains(stdout, "Reading sessions") || strings.Contains(stdout, "Finding sessions") {
				t.Errorf("the indicator leaked into stdout: %q", stdout)
			}
		})
	}

	// Results are unchanged by the indicator.
	t.Setenv("AGENT", "0")
	withIndicator, _, _ := execute(t, "usage", "skills")
	isTerminal = func(io.Writer) bool { return false }
	withoutIndicator, stderr, _ := execute(t, "usage", "skills")
	if withIndicator != withoutIndicator || stderr != "" {
		t.Errorf("stdout differs with the indicator, or stderr is not empty off a terminal: %q", stderr)
	}
}

func TestIsTerminalRejectsNonFiles(t *testing.T) {
	if isTerminal(new(bytes.Buffer)) {
		t.Error("a buffer is not a terminal")
	}
	f, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if isTerminal(f) {
		t.Error("a regular file is not a terminal")
	}
}
