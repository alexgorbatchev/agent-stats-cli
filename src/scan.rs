use agent_parser::{
    AgyParser, ClaudeCodeParser, CodexParser, DiscoverOptions, Harness, OpencodeDb, ParsedEvent,
    SessionSource, SkillSource, discover_sessions_until, open_transcript, parse_pi_line,
};
use rayon::prelude::*;
use std::{
    collections::HashMap,
    io::{self, BufRead, Read},
    path::{Path, PathBuf},
    sync::{
        Arc, Mutex,
        atomic::{AtomicBool, Ordering},
    },
};

const MAX_LINE_BYTES: usize = 256 << 20;
const UNKNOWN_FORK: &str =
    "fork creation time is unknown, so its replayed parent history cannot be told apart";

#[derive(Default)]
pub struct Tally {
    pub model: usize,
    pub user: usize,
}

impl Tally {
    pub fn total(&self) -> usize {
        self.model + self.user
    }

    fn merge(&mut self, other: Self) {
        self.model += other.model;
        self.user += other.user;
    }
}

#[derive(Default)]
pub struct Report {
    counts: HashMap<(String, &'static str), Tally>,
    pub sessions: usize,
    pub problems: Vec<Problem>,
}

pub struct Problem {
    pub path: Option<PathBuf>,
    pub message: String,
}

impl Report {
    pub fn rows(self, by_harness: bool) -> Vec<(String, &'static str, Tally)> {
        let mut rows = HashMap::<_, Tally>::new();
        for ((skill, harness), tally) in self.counts {
            let harness = if by_harness { harness } else { "" };
            rows.entry((skill, harness)).or_default().merge(tally);
        }
        let mut rows: Vec<_> = rows
            .into_iter()
            .map(|((skill, harness), tally)| (skill, harness, tally))
            .collect();
        rows.sort_by(|a, b| {
            b.2.total()
                .cmp(&a.2.total())
                .then_with(|| a.0.cmp(&b.0))
                .then_with(|| a.1.cmp(b.1))
        });
        rows
    }

    fn merge(mut self, other: Self) -> Self {
        self.sessions += other.sessions;
        self.problems.extend(other.problems);
        for (key, tally) in other.counts {
            self.counts.entry(key).or_default().merge(tally);
        }
        self
    }
}

pub fn scan(
    options: &DiscoverOptions,
    since: i64,
    canceled: &AtomicBool,
    indicator: Option<&indicatif::ProgressBar>,
) -> Result<Report, &'static str> {
    check_canceled(canceled)?;
    let discovery = discover_sessions_until(options, || canceled.load(Ordering::Relaxed));
    check_canceled(canceled)?;
    if let Some(bar) = indicator {
        bar.set_message("Reading sessions");
        bar.set_length(discovery.sources.len() as u64);
    }

    let dbs = Databases::default();
    let mut report = discovery
        .sources
        .into_par_iter()
        .fold(Report::default, |mut report, source| {
            if !canceled.load(Ordering::Relaxed) {
                let mut counts = HashMap::new();
                match count_source(&source, since, canceled, &dbs, &mut counts) {
                    Ok(()) => report.sessions += 1,
                    Err(message) => report.problems.push(Problem {
                        path: Some(source.path),
                        message,
                    }),
                }
                for (skill, tally) in counts {
                    report
                        .counts
                        .entry((skill, source.harness.as_str()))
                        .or_default()
                        .merge(tally);
                }
                if let Some(bar) = indicator {
                    bar.inc(1);
                }
            }
            report
        })
        .reduce(Report::default, Report::merge);
    check_canceled(canceled)?;
    if !discovery.errors.is_empty() {
        let message = discovery
            .errors
            .iter()
            .map(ToString::to_string)
            .collect::<Vec<_>>()
            .join("\n");
        report.problems.insert(
            0,
            Problem {
                path: None,
                message,
            },
        );
    }
    Ok(report)
}

fn check_canceled(canceled: &AtomicBool) -> Result<(), &'static str> {
    if canceled.load(Ordering::Relaxed) {
        Err("context canceled")
    } else {
        Ok(())
    }
}

#[derive(Default)]
struct Databases {
    by_path: Mutex<HashMap<PathBuf, Arc<Mutex<OpencodeDb>>>>,
}

impl Databases {
    fn open(&self, path: &Path) -> Result<Arc<Mutex<OpencodeDb>>, agent_parser::Error> {
        let mut by_path = self.by_path.lock().expect("database cache lock");
        if let Some(db) = by_path.get(path) {
            return Ok(Arc::clone(db));
        }
        let db = Arc::new(Mutex::new(OpencodeDb::open(path)?));
        by_path.insert(path.to_owned(), Arc::clone(&db));
        Ok(db)
    }
}

fn count_source(
    source: &SessionSource,
    since: i64,
    canceled: &AtomicBool,
    dbs: &Databases,
    counts: &mut HashMap<String, Tally>,
) -> Result<(), String> {
    let forked_at = if source.is_fork {
        source.forked_at.ok_or(UNKNOWN_FORK)?
    } else {
        0
    };
    if source.harness == Harness::Opencode {
        let db = dbs.open(&source.path).map_err(|error| error.to_string())?;
        let connection = db.lock().expect("database connection lock");
        check_canceled(canceled)?;
        let events = connection
            .reconstruct_session_events(&source.session_id)
            .map_err(|error| format!("reading OpenCode session {}: {error}", source.session_id))?;
        add_skills(events, since, 0, counts);
        return Ok(());
    }
    let mut reader = open_transcript(&source.path).map_err(|error| error.to_string())?;
    count_transcript(
        &mut reader,
        source.harness,
        since,
        forked_at,
        canceled,
        counts,
    )
    .map_err(|error| format!("reading transcript: {error}"))
}

type LineParser = Box<dyn FnMut(&[u8]) -> Result<Vec<ParsedEvent>, agent_parser::Error>>;

fn line_parser(harness: Harness) -> LineParser {
    // Each JSONL parser maintains its own turn state for one transcript.
    match harness {
        Harness::ClaudeCode => {
            let mut parser = ClaudeCodeParser::new();
            Box::new(move |line| parser.parse_line(line))
        }
        Harness::Codex => {
            let mut parser = CodexParser::new();
            Box::new(move |line| parser.parse_line(line))
        }
        Harness::Agy => {
            let mut parser = AgyParser::new();
            Box::new(move |line| parser.parse_line(line))
        }
        _ => Box::new(|line| parse_pi_line(line, "")),
    }
}

fn count_transcript(
    reader: &mut impl BufRead,
    harness: Harness,
    since: i64,
    forked_at: i64,
    canceled: &AtomicBool,
    counts: &mut HashMap<String, Tally>,
) -> io::Result<()> {
    let mut parse = line_parser(harness);
    let mut line = Vec::new();
    while !canceled.load(Ordering::Relaxed) {
        line.clear();
        let size = reader
            .take(MAX_LINE_BYTES as u64)
            .read_until(b'\n', &mut line)?;
        if size == 0 {
            break;
        }
        if size == MAX_LINE_BYTES && !line.ends_with(b"\n") {
            return Err(io::Error::new(
                io::ErrorKind::InvalidData,
                "transcript line exceeds 256 MiB",
            ));
        }
        let line = line.strip_suffix(b"\n").unwrap_or(&line);
        let line = line.strip_suffix(b"\r").unwrap_or(line);
        if let Ok(events) = parse(line) {
            add_skills(events, since, forked_at, counts);
        }
    }
    Ok(())
}

fn add_skills(
    events: Vec<ParsedEvent>,
    since: i64,
    forked_at: i64,
    counts: &mut HashMap<String, Tally>,
) {
    for event in events {
        if (since > 0 && event.timestamp < since) || (forked_at > 0 && event.timestamp <= forked_at)
        {
            continue;
        }
        for skill in event.data.skills {
            let count = counts.entry(skill.name).or_default();
            match skill.source {
                SkillSource::Model => count.model += 1,
                SkillSource::User => count.user += 1,
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Cursor;

    #[test]
    fn preserves_a_partial_count_when_a_reader_fails() {
        let line = br#"{"type":"message","timestamp":"2025-11-03T10:00:01Z","message":{"role":"assistant","content":[{"type":"toolCall","id":"c1","name":"read","arguments":{"path":"/x/skills/golang/SKILL.md"}}]}}"#;
        let mut reader = Cursor::new(line)
            .chain(io::repeat(b'\n').take(1))
            .chain(BrokenReader);
        let mut counts = HashMap::new();
        let error = count_transcript(
            &mut io::BufReader::new(&mut reader),
            Harness::Pi,
            0,
            0,
            &AtomicBool::new(false),
            &mut counts,
        )
        .unwrap_err();
        assert_eq!(error.kind(), io::ErrorKind::TimedOut);
        assert_eq!(counts["golang"].model, 1);
    }

    struct BrokenReader;

    impl Read for BrokenReader {
        fn read(&mut self, _: &mut [u8]) -> io::Result<usize> {
            Err(io::ErrorKind::TimedOut.into())
        }
    }

    #[test]
    fn stops_before_discovery_when_interrupted() {
        let result = scan(
            &DiscoverOptions {
                home: PathBuf::new(),
                harnesses: Vec::new(),
            },
            0,
            &AtomicBool::new(true),
            None,
        );
        assert!(matches!(result, Err("context canceled")));
    }

    #[test]
    fn reuses_database_handles() {
        let temp = tempfile::tempdir().unwrap();
        let path = temp.path().join("opencode.db");
        let db = rusqlite::Connection::open(&path).unwrap();
        db.execute("CREATE TABLE session (id TEXT)", []).unwrap();
        let dbs = Databases::default();
        let first = dbs.open(&path).unwrap();
        let second = dbs.open(&path).unwrap();
        assert!(Arc::ptr_eq(&first, &second));
    }
}
