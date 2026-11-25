use crate::{output, scan, since};
use agent_parser::{DiscoverOptions, Harness};
use chrono::Local;
use clap::{Arg, ArgAction, ArgMatches, Command, error::ErrorKind};
use cli_command_kit::{HelpOptions, Mode, SkillSource, Spec, TechInfo, TreeOptions};
use indicatif::{ProgressBar, ProgressStyle};
use std::{ffi::OsString, io::Write, path::Path, sync::atomic::AtomicBool, time::Duration};

const NAME: &str = "agent-stats";
const VERSION: &str = env!("CARGO_PKG_VERSION");
const SHELLS: [clap_complete::Shell; 4] = [
    clap_complete::Shell::Bash,
    clap_complete::Shell::Zsh,
    clap_complete::Shell::Fish,
    clap_complete::Shell::PowerShell,
];

fn command(mode: Mode) -> Command {
    Command::new(NAME)
        .version(VERSION)
        .override_usage("agent-stats [flags] [command]")
        .disable_version_flag(true)
        .disable_help_subcommand(true)
        .args_override_self(true)
        .about("Report how AI coding agents are used on this machine")
        .long_about(
            "agent-stats reads the session history that Claude Code, Codex, Pi, OpenCode, and Antigravity (agy) keep on this machine and reports usage statistics, such as which skills are used most.",
        )
        .arg(
            bool_flag("version", "version for agent-stats")
                .short('v')
        )
        .subcommand(
            Command::new("usage")
                .about("Report what agents used across stored sessions")
                .override_usage("agent-stats usage [flags] [command]")
                .subcommand(skills_command()),
        )
        .subcommand(
            Command::new("completion")
                .about("Generate the autocompletion script for the specified shell")
                .hide(mode == Mode::Human)
                .subcommands(SHELLS.map(|shell| {
                    Command::new(shell.to_string())
                        .args_override_self(true)
                        .about(format!("Generate the autocompletion script for {shell}"))
                        .arg(bool_flag("no-descriptions", "Omit completion descriptions"))
                })),
        )
        .subcommand(
            Command::new("help")
                .hide(true)
                .about("Print help for a command")
                .arg(Arg::new("command").num_args(0..).action(ArgAction::Append)),
        )
}

fn skills_command() -> Command {
    Command::new("skills")
        .args_override_self(true)
        .override_usage("agent-stats usage skills [flags]")
        .about("Show which skills agents use most")
        .long_about("Counts every skill invocation in the sessions Claude Code, Codex, Pi, OpenCode, and Antigravity (agy) keep on this machine, whether the agent loaded the skill on its own or you asked for it, and lists skills from most to least used.")
        .arg(
            Arg::new("harness")
                .long("harness")
                .value_name("names")
                .action(ArgAction::Append)
                .help("Only count these harnesses: claude-code, codex, pi, opencode, agy (alias antigravity; repeatable or comma-separated)"),
        )
        .arg(bool_flag("by-harness", "Show a separate row for each skill and harness"))
        .arg(
            Arg::new("since")
                .long("since")
                .value_name("time")
                .allow_negative_numbers(true)
                .help("Only count invocations since a time: a duration such as 30d or 12h, or a date such as 2026-10-01")
                .allow_hyphen_values(true),
        )
        .arg(
            Arg::new("limit")
                .long("limit")
                .value_name("n")
                .allow_negative_numbers(true)
                .default_value("0")
                .value_parser(clap::value_parser!(i64))
                .help("Show at most this many rows; 0 shows all"),
        )
        .arg(bool_flag("json", "Print the rows as a JSON array"))
}

fn bool_flag(name: &'static str, help: &'static str) -> Arg {
    Arg::new(name)
        .long(name)
        .num_args(0..=1)
        .require_equals(true)
        .default_missing_value("true")
        .default_value("false")
        .value_parser(parse_bool)
        .help(help)
}

fn parse_bool(value: &str) -> Result<bool, &'static str> {
    match value {
        "1" | "t" | "T" | "true" | "TRUE" | "True" => Ok(true),
        "0" | "f" | "F" | "false" | "FALSE" | "False" => Ok(false),
        _ => Err("use true or false"),
    }
}

fn help_options() -> HelpOptions {
    let env = [
        ("AGENT", "1, true, or yes switches to compact agent output"),
        ("CLAUDE_CONFIG_DIR", "Claude Code configuration directory, searched before ~/.claude"),
        ("CODEX_HOME", "Codex home directory; sessions and archived_sessions live under it"),
        ("CODEX_SESSIONS_DIR", "Codex sessions directory, overriding CODEX_HOME/sessions"),
        ("PI_SESSIONS_DIR", "Pi sessions directory"),
        ("PI_CODING_AGENT_DIR", "Pi agent directory; sessions live under it when PI_SESSIONS_DIR is unset"),
        ("OPENCODE_DB_PATH", "OpenCode database file, overriding every other OpenCode location"),
        ("OPENCODE_DB", "OpenCode's own database setting: absolute, or relative to the OpenCode data directory"),
        ("XDG_DATA_HOME", "Data directory holding opencode/, defaulting to ~/.local/share"),
        ("AGY_SESSIONS_DIR", "Antigravity brain directory holding conversation transcripts"),
        ("GEMINI_HOME", "Gemini home directory; antigravity-cli/brain lives under it when AGY_SESSIONS_DIR is unset"),
    ]
    .into_iter()
    .map(|(name, description)| Spec { name: name.into(), description: description.into() })
    .collect();
    HelpOptions {
        catalog: [
            (
                NAME.into(),
                TechInfo {
                    env,
                    ..Default::default()
                },
            ),
            (
                "agent-stats usage skills".into(),
                TechInfo {
                    metadata: [
                        ("type:harness".into(), "stringSlice".into()),
                        ("type:limit".into(), "int".into()),
                    ]
                    .into(),
                    ..Default::default()
                },
            ),
        ]
        .into(),
        skill: Some(
            SkillSource::new("alexgorbatchev/agent-stats-cli", "main")
                .expect("valid GitHub skill source"),
        ),
        tree: TreeOptions {
            hide_generated_commands: true,
            ..Default::default()
        },
        ..Default::default()
    }
}

pub struct Runtime<'a> {
    pub home: &'a Path,
    pub mode: Mode,
    pub canceled: &'a AtomicBool,
    pub stderr_is_terminal: bool,
}

pub fn run(
    argv: impl IntoIterator<Item = impl Into<OsString> + Clone>,
    runtime: Runtime<'_>,
    mut out: &mut dyn Write,
    err: &mut dyn Write,
) -> Result<(), Box<dyn std::error::Error>> {
    let help = help_options();
    let mut command = cli_command_kit::setup(command(runtime.mode), runtime.mode, &help)?;
    let matches = match command.clone().try_get_matches_from(argv) {
        Ok(matches) => matches,
        Err(error) if error.kind() == ErrorKind::DisplayHelp => {
            write!(out, "{error}")?;
            return Ok(());
        }
        Err(error) => {
            let message = error.to_string();
            return Err(message
                .lines()
                .next()
                .unwrap_or("")
                .trim_start_matches("error: ")
                .into());
        }
    };
    if matches.get_flag("version") {
        writeln!(out, "{VERSION}")?;
    } else if cli_command_kit::write_skill(&command, &matches, &help, &mut out)? {
        return Ok(());
    } else {
        match matches.subcommand() {
            Some(("usage", usage)) => {
                if let Some(skills) = usage.subcommand_matches("skills") {
                    run_skills(skills, runtime, out, err)?;
                } else {
                    write!(
                        out,
                        "{}",
                        command
                            .find_subcommand_mut("usage")
                            .expect("usage")
                            .render_help()
                    )?;
                }
            }
            Some(("completion", completion)) => {
                if let Some((shell, flags)) = completion.subcommand() {
                    let shell = shell.parse::<clap_complete::Shell>()?;
                    let mut completion_command = if flags.get_flag("no-descriptions") {
                        without_descriptions(command)
                    } else {
                        command
                    };
                    let mut script = Vec::new();
                    clap_complete::generate(shell, &mut completion_command, NAME, &mut script);
                    out.write_all(&script)?;
                } else {
                    write!(
                        out,
                        "{}",
                        command
                            .find_subcommand_mut("completion")
                            .expect("completion")
                            .render_help()
                    )?;
                }
            }
            Some(("help", help)) => {
                let mut target = &mut command;
                for segment in help.get_many::<String>("command").into_iter().flatten() {
                    target = target
                        .find_subcommand_mut(segment)
                        .ok_or_else(|| format!("unknown help topic {segment:?}"))?;
                }
                write!(out, "{}", target.render_help())?;
            }
            _ => write!(out, "{}", command.render_help())?,
        }
    }
    Ok(())
}

fn without_descriptions(command: Command) -> Command {
    command
        .about(None)
        .long_about(None)
        .mut_args(|arg| arg.help(None).long_help(None))
        .mut_subcommands(without_descriptions)
}

fn harnesses(matches: &ArgMatches) -> Result<Vec<Harness>, Box<dyn std::error::Error>> {
    let mut selected = Vec::new();
    for value in matches.get_many::<String>("harness").into_iter().flatten() {
        let mut csv = csv::ReaderBuilder::new()
            .has_headers(false)
            .from_reader(value.as_bytes());
        if let Some(record) = csv.records().next().transpose()? {
            for value in &record {
                let normalized = value.trim().to_ascii_lowercase();
                let harness = if normalized == "antigravity" {
                    "agy"
                } else {
                    &normalized
                };
                let harness = harness.parse().map_err(|_| {
                    format!("unknown harness {value:?}: use one of claude-code, codex, pi, opencode, agy")
                })?;
                if !selected.contains(&harness) {
                    selected.push(harness);
                }
            }
        }
    }
    Ok(selected)
}

fn run_skills(
    matches: &ArgMatches,
    runtime: Runtime<'_>,
    out: &mut dyn Write,
    err: &mut dyn Write,
) -> Result<(), Box<dyn std::error::Error>> {
    let harnesses = harnesses(matches)?;
    let since = since::parse(
        matches
            .get_one::<String>("since")
            .map_or("", String::as_str),
        Local::now(),
    )?;
    let limit = *matches.get_one::<i64>("limit").expect("limit default");
    let limit =
        usize::try_from(limit).map_err(|_| format!("--limit must be zero or more, got {limit}"))?;
    let json = matches.get_flag("json");
    let indicator = indicator(runtime.stderr_is_terminal, runtime.mode, json);
    let report = scan::scan(
        &DiscoverOptions {
            home: runtime.home.to_owned(),
            harnesses,
        },
        since,
        runtime.canceled,
        indicator.as_ref(),
    );
    if let Some(bar) = indicator {
        bar.finish_and_clear();
    }
    output::report(
        out,
        err,
        report?,
        runtime.home,
        output::Options {
            mode: runtime.mode,
            by_harness: matches.get_flag("by-harness"),
            json,
            limit,
        },
    )?;
    Ok(())
}

fn indicator(stderr_is_terminal: bool, mode: Mode, json: bool) -> Option<ProgressBar> {
    if !stderr_is_terminal || mode == Mode::Agent || json {
        return None;
    }
    let bar = ProgressBar::new_spinner();
    bar.set_style(
        ProgressStyle::with_template("{spinner} {msg} {pos}/{len}")
            .expect("valid progress template"),
    );
    bar.set_message("Finding sessions");
    bar.tick();
    bar.enable_steady_tick(Duration::from_millis(100));
    Some(bar)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::{self, Cursor};

    #[test]
    fn bundled_skill_covers_cli() {
        let skill = include_str!("../skills/agent-stats/SKILL.md");
        assert_eq!(
            cli_command_kit::skill_omissions(&command(Mode::Agent), &help_options(), skill),
            Vec::<String>::new()
        );
    }

    #[test]
    fn write_failures_propagate() {
        for args in [
            &["agent-stats", "--help"][..],
            &["agent-stats", "--version"],
            &["agent-stats", "usage"],
            &["agent-stats", "completion", "bash"],
            &["agent-stats", "skill"],
        ] {
            let result = run(
                args,
                Runtime {
                    home: Path::new("/unused"),
                    mode: Mode::Human,
                    canceled: &AtomicBool::new(false),
                    stderr_is_terminal: false,
                },
                &mut Cursor::new(&mut [][..]),
                &mut io::sink(),
            );
            assert!(result.is_err(), "{args:?}");
        }
    }

    #[test]
    fn shows_progress_only_for_human_terminal_output() {
        assert!(indicator(false, Mode::Human, false).is_none());
        assert!(indicator(true, Mode::Agent, false).is_none());
        assert!(indicator(true, Mode::Human, true).is_none());
        let bar = indicator(true, Mode::Human, false).expect("terminal progress");
        let home = tempfile::tempdir().unwrap();
        scan::scan(
            &DiscoverOptions {
                home: home.path().into(),
                harnesses: Vec::new(),
            },
            0,
            &AtomicBool::new(false),
            Some(&bar),
        )
        .unwrap();
        bar.finish_and_clear();
    }
}
