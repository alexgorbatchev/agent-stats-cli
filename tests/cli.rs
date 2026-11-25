mod fixture;

use fixture::Home;
use serde_json::json;
use std::{
    process::{Command, Output},
    str,
};

fn run(home: &Home, args: &[&str], agent: bool) -> Output {
    let mut cmd = Command::new(env!("CARGO_BIN_EXE_agent-stats"));
    home.configure(&mut cmd);
    cmd.env("AGENT", if agent { "1" } else { "0" })
        .args(args)
        .output()
        .expect("run agent-stats")
}

fn stdout(home: &Home, args: &[&str], agent: bool) -> String {
    let output = run(home, args, agent);
    assert!(output.status.success(), "{output:?}");
    assert!(output.stderr.is_empty(), "{output:?}");
    String::from_utf8(output.stdout).expect("UTF-8 stdout")
}

#[test]
fn counts_and_sorts_skills_across_all_harnesses() {
    let home = Home::populated();
    let args = ["usage", "skills"];
    assert_eq!(
        stdout(&home, &args, true),
        "bun\t2\t1\t3\ndocs-writer\t2\t1\t3\ngolang\t3\t0\t3\ngit-release\t2\t0\t2\n"
    );
    let report = stdout(&home, &args, false);
    assert!(report.contains("11 invocations of 4 skills across 7 sessions."));
    assert!(report.contains("│ Skill"));
    assert!(report.contains("Model: the agent loaded the skill itself. User: you invoked it."));

    let json = stdout(&home, &["usage", "skills", "--by-harness", "--json"], true);
    assert_eq!(
        serde_json::from_str::<serde_json::Value>(&json).unwrap(),
        json!([
            {"skill":"bun","harness":"codex","model":1,"user":1,"total":2},
            {"skill":"docs-writer","harness":"claude-code","model":1,"user":1,"total":2},
            {"skill":"bun","harness":"pi","model":1,"user":0,"total":1},
            {"skill":"docs-writer","harness":"codex","model":1,"user":0,"total":1},
            {"skill":"git-release","harness":"agy","model":1,"user":0,"total":1},
            {"skill":"git-release","harness":"opencode","model":1,"user":0,"total":1},
            {"skill":"golang","harness":"agy","model":1,"user":0,"total":1},
            {"skill":"golang","harness":"claude-code","model":1,"user":0,"total":1},
            {"skill":"golang","harness":"pi","model":1,"user":0,"total":1}
        ])
    );
}

#[test]
fn filters_harness_and_time_before_limiting() {
    let home = Home::populated();
    let args = [
        "usage",
        "skills",
        "--harness",
        " CODEX ,Antigravity",
        "--harness=agy",
        "--since",
        "2026-11-03T10:00:00Z",
        "--limit",
        "1",
        "--by-harness=true",
    ];
    assert_eq!(stdout(&home, &args, true), "bun\tcodex\t0\t1\t1\n");
    let args = ["usage", "skills", "--limit=1", "--json=false"];
    let report = stdout(&home, &args, false);
    assert!(report.contains("11 invocations of 4 skills across 7 sessions."));
    assert!(!report.contains("docs-writer"));
}

#[test]
fn parses_local_dates_and_relative_since_values_in_the_cli() {
    let home = Home::populated();
    let all = stdout(&home, &["usage", "skills"], true);
    assert_eq!(
        stdout(&home, &["usage", "skills", "--since", "1970-01-01"], true),
        all
    );
    for since in ["30d", "24h"] {
        let output = run(&home, &["usage", "skills", "--since", since], true);
        assert!(output.status.success(), "{output:?}");
        assert!(output.stderr.is_empty(), "{output:?}");
    }
}

#[test]
fn handles_empty_reports_in_all_modes() {
    let home = Home::empty();
    let args = ["usage", "skills"];
    assert_eq!(stdout(&home, &args, true), "");
    assert_eq!(
        stdout(&home, &args, false),
        "[INFO] No skill invocations found in 0 sessions.\n"
    );
    assert_eq!(stdout(&home, &["usage", "skills", "--json"], false), "[]\n");
}

#[test]
fn warns_about_unreadable_data_and_skips_undated_forks() {
    let home = Home::populated();
    home.write(fixture::BAD_CODEX, "invalid archive");
    home.write(
        ".local/share/opencode/opencode-broken.db",
        "invalid database",
    );
    home.write(
        ".claude/projects/-repo/s1/subagents/agent-undated.meta.json",
        r#"{"isFork":true,"toolUseId":"missing"}"#,
    );
    home.write(
        ".claude/projects/-repo/s1/subagents/agent-undated.jsonl",
        fixture::CLAUDE_MODEL,
    );
    let output = run(&home, &["usage", "skills", "--json"], true);
    assert!(output.status.success(), "{output:?}");
    let warnings = str::from_utf8(&output.stderr).unwrap();
    assert!(warnings.contains(&format!(
        "WARN: skipped unreadable data: ~/{}",
        fixture::BAD_CODEX
    )));
    assert!(warnings.contains("opencode-broken.db"));
    assert!(warnings.contains("fork creation time is unknown"));
    assert!(!warnings.contains(home.path().to_str().unwrap()));
    let rows = serde_json::from_slice::<serde_json::Value>(&output.stdout).unwrap();
    assert_eq!(
        rows[1],
        json!({"skill":"docs-writer","model":2,"user":1,"total":3})
    );
}

#[test]
fn exposes_help_skill_version_and_completion() {
    let home = Home::empty();
    assert_eq!(
        stdout(&home, &["-v"], false),
        concat!(env!("CARGO_PKG_VERSION"), "\n")
    );
    assert_eq!(stdout(&home, &[], false), stdout(&home, &["--help"], false));
    assert!(stdout(&home, &["usage"], false).contains("skills"));
    assert!(stdout(&home, &["help", "usage", "skills"], true).contains("--by-harness"));
    assert!(stdout(&home, &["skill"], true).contains("alexgorbatchev/agent-stats-cli"));
    for shell in ["bash", "zsh", "fish", "powershell"] {
        assert!(stdout(&home, &["completion", shell], false).contains("agent-stats"));
    }
    assert!(
        stdout(&home, &["completion", "fish", "--no-descriptions"], false).contains("agent-stats")
    );
    assert!(stdout(&home, &["completion"], false).contains("powershell"));
}

#[test]
fn reports_invalid_arguments_on_stderr() {
    let home = Home::empty();
    for (flags, expected) in [
        (&["--harness", "nope"][..], "unknown harness \"nope\""),
        (&["--since", "-1h"][..], "the duration must not be negative"),
        (&["--since", "nonsense"][..], "invalid --since \"nonsense\""),
        (
            &["--since", "999999999999999999999d"][..],
            "days are out of range",
        ),
        (&["--since", "99999999999d"][..], "date is out of range"),
        (
            &["--limit", "-1"][..],
            "--limit must be zero or more, got -1",
        ),
        (&["--invalid"][..], "--invalid"),
    ] {
        let mut args = vec!["usage", "skills"];
        args.extend(flags);
        let result = run(&home, &args, true);
        assert_eq!(result.status.code(), Some(1));
        assert!(result.stdout.is_empty(), "{result:?}");
        let error = String::from_utf8(result.stderr).unwrap();
        assert!(
            error.starts_with("ERR: ") && error.contains(expected),
            "{error}"
        );
    }
}
