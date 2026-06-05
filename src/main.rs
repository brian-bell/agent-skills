use std::env;
use std::ffi::OsString;
use std::io::{self, Write};
use std::path::PathBuf;
use std::process::ExitCode;

use skill_importer::{DiscoveryRoots, discover_skills, inventory_to_json};

fn main() -> ExitCode {
    match run(env::args_os().skip(1), io::stdout()) {
        Ok(()) => ExitCode::SUCCESS,
        Err(error) => {
            eprintln!("skill-importer: {error}");
            ExitCode::FAILURE
        }
    }
}

fn run(args: impl IntoIterator<Item = OsString>, mut stdout: impl Write) -> Result<(), String> {
    let command = Command::parse(args)?;

    match command {
        Command::List { roots } => {
            let inventory = discover_skills(&roots)
                .map_err(|error| format!("failed to discover skills: {error}"))?;
            let json = inventory_to_json(&inventory);
            serde_json::to_writer_pretty(&mut stdout, &json)
                .map_err(|error| format!("failed to write JSON: {error}"))?;
            writeln!(stdout).map_err(|error| format!("failed to write JSON: {error}"))?;
            Ok(())
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
enum Command {
    List { roots: DiscoveryRoots },
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct RootArgs {
    canonical_root: Option<PathBuf>,
    imports_root: Option<PathBuf>,
    claude_code_root: Option<PathBuf>,
    codex_root: Option<PathBuf>,
}

impl Command {
    fn parse(args: impl IntoIterator<Item = OsString>) -> Result<Self, String> {
        let mut args = args.into_iter();
        let Some(command) = args.next() else {
            return Err(usage());
        };

        if command != "list" {
            return Err(format!(
                "unknown command `{}`\n{}",
                display_arg(command),
                usage()
            ));
        }

        let mut saw_json = false;
        let mut roots = RootArgs {
            canonical_root: None,
            imports_root: None,
            claude_code_root: None,
            codex_root: None,
        };

        while let Some(arg) = args.next() {
            match arg.to_str() {
                Some("--json") => saw_json = true,
                Some("--canonical-root") => {
                    roots.canonical_root = Some(next_path(&mut args, "--canonical-root")?);
                }
                Some("--imports-root") => {
                    roots.imports_root = Some(next_path(&mut args, "--imports-root")?);
                }
                Some("--claude-code-root") => {
                    roots.claude_code_root = Some(next_path(&mut args, "--claude-code-root")?);
                }
                Some("--codex-root") => {
                    roots.codex_root = Some(next_path(&mut args, "--codex-root")?);
                }
                _ => {
                    return Err(format!(
                        "unknown argument `{}`\n{}",
                        display_arg(arg),
                        usage()
                    ));
                }
            }
        }

        if !saw_json {
            return Err("list currently requires --json".to_string());
        }

        Ok(Self::List {
            roots: roots.into_discovery_roots()?,
        })
    }
}

impl RootArgs {
    fn into_discovery_roots(self) -> Result<DiscoveryRoots, String> {
        let current_dir = env::current_dir()
            .map_err(|error| format!("failed to read current directory: {error}"))?;
        let home = home_dir();

        Ok(DiscoveryRoots {
            canonical_root: self.canonical_root.unwrap_or_else(|| current_dir.clone()),
            imports_root: self
                .imports_root
                .unwrap_or_else(|| current_dir.join(".skill-importer").join("imports")),
            claude_code_root: self
                .claude_code_root
                .unwrap_or_else(|| home.join(".claude").join("skills")),
            codex_root: self
                .codex_root
                .unwrap_or_else(|| home.join(".agents").join("skills")),
        })
    }
}

fn next_path(
    args: &mut impl Iterator<Item = OsString>,
    flag: &'static str,
) -> Result<PathBuf, String> {
    args.next()
        .map(PathBuf::from)
        .ok_or_else(|| format!("{flag} requires a path"))
}

fn home_dir() -> PathBuf {
    env::var_os("HOME")
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("."))
}

fn display_arg(arg: OsString) -> String {
    arg.to_string_lossy().into_owned()
}

fn usage() -> String {
    "usage: skill-importer list --json [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]".to_string()
}
