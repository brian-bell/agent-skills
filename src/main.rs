use std::env;
use std::ffi::OsString;
use std::io::{self, Read, Write};
use std::path::PathBuf;
use std::process::ExitCode;

use skill_importer::{
    DiscoveryRoots, ImportLocalPathRequest, ImportMarkdownRequest, discover_skills,
    import_local_path_skill, import_markdown_skill, inventory_to_json,
};

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
        Command::ImportMarkdown {
            roots,
            source_location,
        } => {
            let mut markdown = String::new();
            io::stdin()
                .read_to_string(&mut markdown)
                .map_err(|error| format!("failed to read Markdown from stdin: {error}"))?;
            let import = import_markdown_skill(
                &roots,
                ImportMarkdownRequest {
                    markdown: &markdown,
                    source_location: source_location.as_deref(),
                },
            )
            .map_err(|error| format!("failed to import Markdown: {error}"))?;
            serde_json::to_writer_pretty(&mut stdout, &import)
                .map_err(|error| format!("failed to write JSON: {error}"))?;
            writeln!(stdout).map_err(|error| format!("failed to write JSON: {error}"))?;
            Ok(())
        }
        Command::ImportPath { roots, path } => {
            let import = import_local_path_skill(
                &roots,
                ImportLocalPathRequest {
                    path: path.as_path(),
                },
            )
            .map_err(|error| format!("failed to import path: {error}"))?;
            serde_json::to_writer_pretty(&mut stdout, &import)
                .map_err(|error| format!("failed to write JSON: {error}"))?;
            writeln!(stdout).map_err(|error| format!("failed to write JSON: {error}"))?;
            Ok(())
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
enum Command {
    List {
        roots: DiscoveryRoots,
    },
    ImportMarkdown {
        roots: DiscoveryRoots,
        source_location: Option<String>,
    },
    ImportPath {
        roots: DiscoveryRoots,
        path: PathBuf,
    },
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
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

        match command.to_str() {
            Some("list") => parse_list_command(args),
            Some("import") => parse_import_command(args),
            _ => Err(format!(
                "unknown command `{}`\n{}",
                display_arg(command),
                usage()
            )),
        }
    }
}

fn parse_list_command(mut args: impl Iterator<Item = OsString>) -> Result<Command, String> {
    let mut saw_json = false;
    let mut roots = RootArgs::default();

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

    Ok(Command::List {
        roots: roots.into_discovery_roots()?,
    })
}

fn parse_import_command(mut args: impl Iterator<Item = OsString>) -> Result<Command, String> {
    let Some(import_kind) = args.next() else {
        return Err(format!("import requires a kind\n{}", usage()));
    };

    match import_kind.to_str() {
        Some("markdown") => parse_import_markdown_command(args),
        Some("path") => parse_import_path_command(args),
        _ => Err(format!(
            "unknown import kind `{}`\n{}",
            display_arg(import_kind),
            usage()
        )),
    }
}

fn parse_import_markdown_command(
    mut args: impl Iterator<Item = OsString>,
) -> Result<Command, String> {
    let mut saw_json = false;
    let mut roots = RootArgs::default();
    let mut source_location = None;

    while let Some(arg) = args.next() {
        match arg.to_str() {
            Some("--json") => saw_json = true,
            Some("--source-location") => {
                source_location = Some(next_string(&mut args, "--source-location")?);
            }
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
        return Err("import markdown currently requires --json".to_string());
    }

    Ok(Command::ImportMarkdown {
        roots: roots.into_discovery_roots()?,
        source_location,
    })
}

fn parse_import_path_command(mut args: impl Iterator<Item = OsString>) -> Result<Command, String> {
    let mut saw_json = false;
    let mut roots = RootArgs::default();
    let mut path = None;

    while let Some(arg) = args.next() {
        match arg.to_str() {
            Some("--json") => saw_json = true,
            Some("--path") => {
                path = Some(next_path(&mut args, "--path")?);
            }
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
        return Err("import path currently requires --json".to_string());
    }

    Ok(Command::ImportPath {
        roots: roots.into_discovery_roots()?,
        path: path.ok_or_else(|| "import path requires --path".to_string())?,
    })
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

fn next_string(
    args: &mut impl Iterator<Item = OsString>,
    flag: &'static str,
) -> Result<String, String> {
    args.next()
        .map(display_arg)
        .ok_or_else(|| format!("{flag} requires a value"))
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
    "usage: skill-importer list --json [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]\n       skill-importer import markdown --json [--source-location VALUE] [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]\n       skill-importer import path --json --path PATH [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]".to_string()
}
