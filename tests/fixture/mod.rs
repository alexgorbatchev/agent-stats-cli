use rusqlite::Connection;
use std::{fs, path::Path, process::Command};
use tempfile::TempDir;

pub const CLAUDE_MODEL: &str = r#"{"type":"assistant","sessionId":"s1","timestamp":"2025-11-03T10:00:00Z","message":{"id":"m1","role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","id":"t1","name":"Skill","input":{"skill":"docs-writer"}}]}}"#;
const CODEX: &str = ".codex/sessions/2025/11/03/rollout-2025-11-03T10-00-00-01a1aaaa-0000-7000-8000-000000000001.jsonl.zst";
pub const BAD_CODEX: &str = ".codex/sessions/2025/11/04/rollout-2025-11-04T10-00-00-01a1aaaa-0000-7000-8000-000000000009.jsonl.zst";
const PI: &str = ".pi/agent/sessions/--repo--/2025-11-03T10-00-00-000Z_parent";
const PI_READ: &str = r#"{"type":"message","timestamp":"2025-11-03T10:00:01Z","message":{"role":"assistant","content":[{"type":"toolCall","id":"c1","name":"read","arguments":{"path":"/x/skills/golang/SKILL.md"}}]}}"#;

pub struct Home(TempDir);

impl Home {
    pub fn empty() -> Self {
        Self(tempfile::tempdir().unwrap())
    }

    pub fn populated() -> Self {
        let home = Self::empty();
        home.write(
            ".claude/projects/-repo/s1.jsonl",
            &[
                CLAUDE_MODEL,
                r#"{"type":"user","sessionId":"s1","timestamp":"2025-11-03T10:01:00Z","message":{"role":"user","content":"<command-name>/docs-writer</command-name>"}}"#,
                r#"{"type":"user","sessionId":"s1","timestamp":"2025-11-03T10:01:01Z","isMeta":true,"message":{"role":"user","content":[{"type":"text","text":"Base directory for this skill: /x/skills/docs-writer"}]}}"#,
                "not json",
                r#"{"type":"assistant","sessionId":"s1","timestamp":"2025-11-03T10:05:00Z","message":{"id":"m9","role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","id":"toolu_spawn","name":"Agent","input":{}}]}}"#,
            ]
            .join("\n"),
        );
        home.write(
            ".claude/projects/-repo/s1/subagents/agent-fork.meta.json",
            r#"{"agentType":"fork","isFork":true,"toolUseId":"toolu_spawn"}"#,
        );
        home.write(
            ".claude/projects/-repo/s1/subagents/agent-fork.jsonl",
            &[
                CLAUDE_MODEL,
                r#"{"type":"assistant","sessionId":"s1","agentId":"fork","timestamp":"2025-11-03T10:05:00Z","message":{"id":"r2","role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","id":"toolu_spawn","name":"Skill","input":{"skill":"docs-writer"}}]}}"#,
                r#"{"type":"assistant","sessionId":"s1","agentId":"fork","timestamp":"2025-11-03T10:05:01Z","message":{"id":"f1","role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","id":"f1","name":"Skill","input":{"skill":"golang"}}]}}"#,
            ]
            .join("\n"),
        );
        let codex = [
            r#"{"timestamp":"2025-11-03T10:00:00Z","type":"event_msg","payload":{"type":"item_completed","item":{"type":"CommandExecution","command":["/bin/zsh","-lc","cat /x/skills/{docs-writer,bun,docs-writer}/SKILL.md"],"exit_code":0}}}"#,
            r#"{"timestamp":"2026-11-03T10:00:00Z","type":"event_msg","payload":{"type":"item_completed","item":{"type":"UserMessage","content":[{"type":"skill","name":"bun","path":"/x/skills/bun/SKILL.md"}]}}}"#,
        ]
        .join("\n");
        fs::write(
            home.prepare(CODEX),
            zstd::encode_all(codex.as_bytes(), 1).unwrap(),
        )
        .unwrap();
        home.write(
            &format!("{PI}.jsonl"),
            &[
                r#"{"type":"session","version":3,"id":"parent","timestamp":"2025-11-03T10:00:00Z","cwd":"/repo"}"#,
                PI_READ,
            ]
            .join("\n"),
        );
        home.write(
            &format!("{PI}/forks/2025-11-03T11-00-00-000Z_fork.jsonl"),
            &[
                r#"{"type":"session","version":3,"id":"fork","timestamp":"2025-11-03T11:00:00Z","cwd":"/repo"}"#,
                PI_READ,
                r#"{"type":"message","timestamp":"2025-11-03T11:00:01Z","message":{"role":"assistant","content":[{"type":"toolCall","id":"c2","name":"read","arguments":{"path":"/x/skills/bun/SKILL.md"}}]}}"#,
            ]
            .join("\n"),
        );
        home.write(
            ".gemini/antigravity-cli/brain/agy-1/.system_generated/logs/transcript_full.jsonl",
            r#"{"step_index":0,"type":"PLANNER_RESPONSE","created_at":"2025-11-03T10:00:00Z","tool_calls":[{"id":"c1","name":"view_file","args":{"AbsolutePath":"/x/skills/golang/SKILL.md"}},{"id":"c2","name":"run_command","args":{"CommandLine":"cat /x/skills/git-release/SKILL.md"}}]}"#,
        );
        home.write_opencode();
        home
    }

    pub fn path(&self) -> &Path {
        self.0.path()
    }

    pub fn configure(&self, cmd: &mut Command) {
        cmd.env("HOME", self.path()).env("USERPROFILE", self.path());
        for key in [
            "CLAUDE_CONFIG_DIR",
            "CODEX_HOME",
            "CODEX_SESSIONS_DIR",
            "PI_CODING_AGENT_DIR",
            "PI_SESSIONS_DIR",
            "OPENCODE_DB_PATH",
            "OPENCODE_DB",
            "XDG_DATA_HOME",
            "AGY_SESSIONS_DIR",
            "GEMINI_HOME",
            "COLUMNS",
        ] {
            cmd.env_remove(key);
        }
    }

    fn prepare(&self, path: &str) -> std::path::PathBuf {
        let path = self.path().join(path);
        fs::create_dir_all(path.parent().unwrap()).unwrap();
        path
    }

    pub fn write(&self, path: &str, contents: &str) {
        fs::write(self.prepare(path), format!("{contents}\n")).unwrap();
    }

    fn write_opencode(&self) {
        let db = Connection::open(self.prepare(".local/share/opencode/opencode.db")).unwrap();
        db.execute_batch(
            r#"
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
INSERT INTO session (id, directory, time_created, time_updated) VALUES ('ses_1', '/repo', 1762160000000, 1762160000000);
INSERT INTO message (id, session_id, time_created, data) VALUES ('msg_1', 'ses_1', 1762160000000, '{"role":"assistant"}');
INSERT INTO part (id, message_id, session_id, time_created, data) VALUES ('p1', 'msg_1', 'ses_1', 1762160001000,
    '{"type":"tool","tool":"skill","callID":"c1","state":{"status":"completed","input":{"name":"git-release"},"time":{"start":1762160001000}}}');
"#,
        )
        .unwrap();
    }
}
