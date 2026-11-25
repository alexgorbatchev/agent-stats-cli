mod cli;
mod output;
mod scan;
mod since;

use cli_command_kit::{Mode, fold_message};
use std::{
    io::{self, IsTerminal, Write},
    path::PathBuf,
    process::ExitCode,
    sync::{
        Arc,
        atomic::{AtomicBool, Ordering},
    },
};

fn main() -> ExitCode {
    let mode = Mode::from_env();
    let home = home_dir();
    let canceled = Arc::new(AtomicBool::new(false));
    let interrupt = Arc::clone(&canceled);
    let mut stdout = io::stdout().lock();
    let mut stderr = io::stderr();
    let runtime = cli::Runtime {
        home: &home,
        mode,
        canceled: &canceled,
        stderr_is_terminal: stderr.is_terminal() && std::env::var("TERM").as_deref() != Ok("dumb"),
    };
    let result = ctrlc::set_handler(move || interrupt.store(true, Ordering::Relaxed))
        .map_err(|error| -> Box<dyn std::error::Error> { Box::new(error) })
        .and_then(|()| cli::run(std::env::args_os(), runtime, &mut stdout, &mut stderr))
        .and_then(|()| stdout.flush().map_err(Into::into));
    if let Err(error) = result {
        let prefix = if mode == Mode::Agent {
            "ERR: "
        } else {
            "[ERROR] "
        };
        let message = fold_message(&error.to_string(), &home);
        let _ = writeln!(stderr, "{prefix}{}", message.trim_end());
        return ExitCode::FAILURE;
    }
    ExitCode::SUCCESS
}

fn home_dir() -> PathBuf {
    if cfg!(windows)
        && let Some(home) = std::env::var_os("USERPROFILE").filter(|path| !path.is_empty())
    {
        return home.into();
    }
    dirs::home_dir().unwrap_or_default()
}
