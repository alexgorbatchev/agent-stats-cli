use crate::scan::{Report, Tally};
use cli_command_kit::{Mode, fold_home, fold_message, terminal_width};
use comfy_table::{ContentArrangement, Table, TableComponent, presets::UTF8_FULL_CONDENSED};
use serde::Serialize;
use std::{
    collections::HashSet,
    io::{self, Write},
    path::Path,
};

#[derive(Serialize)]
struct Row<'a> {
    skill: &'a str,
    #[serde(skip_serializing_if = "str::is_empty")]
    harness: &'a str,
    model: usize,
    user: usize,
    total: usize,
}

impl<'a> From<&'a (String, &'static str, Tally)> for Row<'a> {
    fn from((skill, harness, count): &'a (String, &'static str, Tally)) -> Self {
        Self {
            skill,
            harness,
            model: count.model,
            user: count.user,
            total: count.total(),
        }
    }
}

impl Row<'_> {
    fn cells(&self, by_harness: bool) -> Vec<String> {
        let mut cells = vec![self.skill.to_string()];
        if by_harness {
            cells.push(self.harness.to_string());
        }
        cells.extend([self.model, self.user, self.total].map(|count| count.to_string()));
        cells
    }
}

pub struct Options {
    pub mode: Mode,
    pub by_harness: bool,
    pub json: bool,
    pub limit: usize,
}

pub fn report(
    out: &mut dyn Write,
    err: &mut dyn Write,
    report: Report,
    home: &Path,
    Options {
        mode,
        by_harness,
        json,
        limit,
    }: Options,
) -> io::Result<()> {
    for problem in &report.problems {
        let prefix = if mode == Mode::Agent {
            "WARN: "
        } else {
            "[WARN] "
        };
        write!(err, "{prefix}skipped unreadable data: ")?;
        if let Some(path) = &problem.path {
            write!(err, "{}: ", fold_home(path, home))?;
        }
        writeln!(err, "{}", fold_message(&problem.message, home))?;
    }
    let sessions = report.sessions;
    let counts = report.rows(by_harness);
    let rows: Vec<_> = counts
        .iter()
        .take(if limit == 0 { counts.len() } else { limit })
        .map(Row::from)
        .collect();
    if json {
        let value = match mode {
            Mode::Agent => serde_json::to_string(&rows),
            Mode::Human => serde_json::to_string_pretty(&rows),
        }?;
        // Go's JSON encoder protects HTML-sensitive characters and line separators.
        return write_json(out, &value);
    }
    if mode == Mode::Agent {
        for row in rows {
            writeln!(out, "{}", row.cells(by_harness).join("\t"))?;
        }
        return Ok(());
    }
    if rows.is_empty() {
        return writeln!(
            out,
            "[INFO] No skill invocations found in {}.",
            plural(sessions, "session")
        );
    }

    let mut table = Table::new();
    table
        .load_preset(UTF8_FULL_CONDENSED)
        .set_style(TableComponent::LeftHeaderIntersection, '├')
        .set_style(TableComponent::HeaderLines, '─')
        .set_style(TableComponent::MiddleHeaderIntersections, '┼')
        .set_style(TableComponent::RightHeaderIntersection, '┤')
        .set_style(TableComponent::VerticalLines, '│')
        .set_header(if by_harness {
            vec!["Skill", "Harness", "Model", "User", "Total"]
        } else {
            vec!["Skill", "Model", "User", "Total"]
        });
    for row in rows {
        table.add_row(row.cells(by_harness));
    }
    if let Ok(width @ 1..) = u16::try_from(terminal_width()) {
        table
            .set_width(width)
            .set_content_arrangement(ContentArrangement::Dynamic);
    }
    let invocations = counts.iter().map(|(_, _, count)| count.total()).sum();
    let skills = counts
        .iter()
        .map(|(name, _, _)| name)
        .collect::<HashSet<_>>()
        .len();
    writeln!(out, "{table}")?;
    writeln!(
        out,
        "{} of {} across {}. Model: the agent loaded the skill itself. User: you invoked it.",
        plural(invocations, "invocation"),
        plural(skills, "skill"),
        plural(sessions, "session"),
    )
}

fn plural(count: usize, noun: &str) -> String {
    let suffix = if count == 1 { "" } else { "s" };
    format!("{count} {noun}{suffix}")
}

fn write_json(out: &mut dyn Write, mut json: &str) -> io::Result<()> {
    while let Some(index) = json.find(['<', '>', '&', '\u{2028}', '\u{2029}']) {
        let (head, rest) = json.split_at(index);
        out.write_all(head.as_bytes())?;
        let special = rest.chars().next().expect("found a character");
        let escape = match special {
            '<' => b"\\u003c",
            '>' => b"\\u003e",
            '&' => b"\\u0026",
            '\u{2028}' => b"\\u2028",
            _ => b"\\u2029",
        };
        out.write_all(escape)?;
        json = &rest[special.len_utf8()..];
    }
    writeln!(out, "{json}")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn escapes_html_and_line_separators_in_json() {
        let mut out = Vec::new();
        write_json(&mut out, "\"<>&\u{2028}\u{2029}\"").unwrap();
        assert_eq!(out, b"\"\\u003c\\u003e\\u0026\\u2028\\u2029\"\n");
    }
}
