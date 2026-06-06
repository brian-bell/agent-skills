use std::env;
use std::ffi::OsString;
use std::io::{self, Read, Write};
use std::path::PathBuf;
use std::process::ExitCode;
use std::time::Duration;

use skill_importer::{
    DeleteImportRequest, DisableSkillRequest, DiscoveryRoots, EnableSkillRequest,
    ImportLocalPathRequest, ImportMarkdownRequest, ImportUrlRequest, PromoteSkillRequest,
    SkillAgent, SkillOperationResult, SkillUrlFetchError, SkillUrlFetcher,
    delete_unpromoted_import, disable_skill, discover_skills, enable_skill,
    import_local_path_skill, import_markdown_skill, import_url_skill, inventory_to_json,
    promote_imported_skill, tui::run_tui,
};

const MAX_SKILL_MARKDOWN_BYTES: u64 = 1024 * 1024;

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
    run_with_services(args, &mut stdout, &UreqUrlFetcher, &DefaultTuiRunner)
}

#[cfg(test)]
fn run_with_url_fetcher(
    args: impl IntoIterator<Item = OsString>,
    mut stdout: impl Write,
    url_fetcher: &impl SkillUrlFetcher,
) -> Result<(), String> {
    run_with_services(args, &mut stdout, url_fetcher, &DisabledTuiRunner)
}

fn run_with_services(
    args: impl IntoIterator<Item = OsString>,
    mut stdout: impl Write,
    url_fetcher: &impl SkillUrlFetcher,
    tui_runner: &impl TuiRunner,
) -> Result<(), String> {
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
        Command::ImportUrl { roots, url } => {
            let import =
                import_url_skill(&roots, ImportUrlRequest { url: url.as_str() }, url_fetcher)
                    .map_err(|error| format!("failed to import URL: {error}"))?;
            serde_json::to_writer_pretty(&mut stdout, &import)
                .map_err(|error| format!("failed to write JSON: {error}"))?;
            writeln!(stdout).map_err(|error| format!("failed to write JSON: {error}"))?;
            Ok(())
        }
        Command::Enable {
            roots,
            skill_name,
            agents,
        } => {
            let result = enable_skill(
                &roots,
                EnableSkillRequest {
                    skill_name: skill_name.as_str(),
                    agents: &agents,
                },
            )
            .map_err(|failure| format!("failed to enable skill: {}", failure.error))?;
            write_operation_json(&mut stdout, &result)
        }
        Command::Disable {
            roots,
            skill_name,
            agents,
        } => {
            let result = disable_skill(
                &roots,
                DisableSkillRequest {
                    skill_name: skill_name.as_str(),
                    agents: &agents,
                },
            )
            .map_err(|failure| format!("failed to disable skill: {}", failure.error))?;
            write_operation_json(&mut stdout, &result)
        }
        Command::Promote { roots, skill_name } => {
            let result = promote_imported_skill(
                &roots,
                PromoteSkillRequest {
                    skill_name: skill_name.as_str(),
                },
            )
            .map_err(|failure| format!("failed to promote skill: {}", failure.error))?;
            write_operation_json(&mut stdout, &result)
        }
        Command::Delete { roots, skill_name } => {
            let result = delete_unpromoted_import(
                &roots,
                DeleteImportRequest {
                    skill_name: skill_name.as_str(),
                },
            )
            .map_err(|failure| format!("failed to delete import: {}", failure.error))?;
            write_operation_json(&mut stdout, &result)
        }
        Command::Tui { roots } => tui_runner.run(&roots),
    }
}

trait TuiRunner {
    fn run(&self, roots: &DiscoveryRoots) -> Result<(), String>;
}

struct DefaultTuiRunner;

impl TuiRunner for DefaultTuiRunner {
    fn run(&self, roots: &DiscoveryRoots) -> Result<(), String> {
        run_tui(roots, &UreqUrlFetcher).map_err(|error| format!("failed to run TUI: {error}"))
    }
}

#[cfg(test)]
struct DisabledTuiRunner;

#[cfg(test)]
impl TuiRunner for DisabledTuiRunner {
    fn run(&self, _roots: &DiscoveryRoots) -> Result<(), String> {
        Err("TUI runner was not configured".to_string())
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
    ImportUrl {
        roots: DiscoveryRoots,
        url: String,
    },
    Enable {
        roots: DiscoveryRoots,
        skill_name: String,
        agents: Vec<SkillAgent>,
    },
    Disable {
        roots: DiscoveryRoots,
        skill_name: String,
        agents: Vec<SkillAgent>,
    },
    Promote {
        roots: DiscoveryRoots,
        skill_name: String,
    },
    Delete {
        roots: DiscoveryRoots,
        skill_name: String,
    },
    Tui {
        roots: DiscoveryRoots,
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
            Some("enable") => parse_enable_disable_command(args, EnableDisableCommand::Enable),
            Some("disable") => parse_enable_disable_command(args, EnableDisableCommand::Disable),
            Some("promote") => parse_promote_command(args),
            Some("delete") => parse_delete_command(args),
            Some("tui") => parse_tui_command(args),
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
        Some("url") => parse_import_url_command(args),
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

fn parse_import_url_command(mut args: impl Iterator<Item = OsString>) -> Result<Command, String> {
    let mut saw_json = false;
    let mut roots = RootArgs::default();
    let mut url = None;

    while let Some(arg) = args.next() {
        match arg.to_str() {
            Some("--json") => saw_json = true,
            Some("--url") => {
                url = Some(next_string(&mut args, "--url")?);
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
        return Err("import url currently requires --json".to_string());
    }

    Ok(Command::ImportUrl {
        roots: roots.into_discovery_roots()?,
        url: url.ok_or_else(|| "import url requires --url".to_string())?,
    })
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum EnableDisableCommand {
    Enable,
    Disable,
}

fn parse_enable_disable_command(
    mut args: impl Iterator<Item = OsString>,
    command: EnableDisableCommand,
) -> Result<Command, String> {
    let mut saw_json = false;
    let mut roots = RootArgs::default();
    let mut skill_name = None;
    let mut agents = Vec::new();

    while let Some(arg) = args.next() {
        match arg.to_str() {
            Some("--json") => saw_json = true,
            Some("--skill") => {
                skill_name = Some(next_string(&mut args, "--skill")?);
            }
            Some("--agent") => {
                agents.push(parse_agent(&next_string(&mut args, "--agent")?)?);
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

    let command_name = match command {
        EnableDisableCommand::Enable => "enable",
        EnableDisableCommand::Disable => "disable",
    };
    if !saw_json {
        return Err(format!("{command_name} currently requires --json"));
    }
    if agents.is_empty() {
        return Err(format!("{command_name} requires at least one --agent"));
    }

    let roots = roots.into_discovery_roots()?;
    let skill_name = skill_name.ok_or_else(|| format!("{command_name} requires --skill"))?;
    match command {
        EnableDisableCommand::Enable => Ok(Command::Enable {
            roots,
            skill_name,
            agents,
        }),
        EnableDisableCommand::Disable => Ok(Command::Disable {
            roots,
            skill_name,
            agents,
        }),
    }
}

fn parse_agent(value: &str) -> Result<SkillAgent, String> {
    match value {
        "claude-code" => Ok(SkillAgent::ClaudeCode),
        "codex" => Ok(SkillAgent::Codex),
        _ => Err(format!(
            "unknown agent `{value}`; expected `claude-code` or `codex`"
        )),
    }
}

fn parse_promote_command(mut args: impl Iterator<Item = OsString>) -> Result<Command, String> {
    let mut saw_json = false;
    let mut roots = RootArgs::default();
    let mut skill_name = None;

    while let Some(arg) = args.next() {
        match arg.to_str() {
            Some("--json") => saw_json = true,
            Some("--skill") => {
                skill_name = Some(next_string(&mut args, "--skill")?);
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
        return Err("promote currently requires --json".to_string());
    }

    Ok(Command::Promote {
        roots: roots.into_discovery_roots()?,
        skill_name: skill_name.ok_or_else(|| "promote requires --skill".to_string())?,
    })
}

fn parse_delete_command(mut args: impl Iterator<Item = OsString>) -> Result<Command, String> {
    let mut saw_json = false;
    let mut roots = RootArgs::default();
    let mut skill_name = None;

    while let Some(arg) = args.next() {
        match arg.to_str() {
            Some("--json") => saw_json = true,
            Some("--skill") => {
                skill_name = Some(next_string(&mut args, "--skill")?);
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
        return Err("delete currently requires --json".to_string());
    }

    Ok(Command::Delete {
        roots: roots.into_discovery_roots()?,
        skill_name: skill_name.ok_or_else(|| "delete requires --skill".to_string())?,
    })
}

fn parse_tui_command(mut args: impl Iterator<Item = OsString>) -> Result<Command, String> {
    let mut roots = RootArgs::default();

    while let Some(arg) = args.next() {
        match arg.to_str() {
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

    Ok(Command::Tui {
        roots: roots.into_discovery_roots()?,
    })
}

fn write_operation_json(
    stdout: &mut impl Write,
    result: &SkillOperationResult,
) -> Result<(), String> {
    serde_json::to_writer_pretty(&mut *stdout, result)
        .map_err(|error| format!("failed to write JSON: {error}"))?;
    writeln!(stdout).map_err(|error| format!("failed to write JSON: {error}"))?;
    Ok(())
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

struct UreqUrlFetcher;

impl SkillUrlFetcher for UreqUrlFetcher {
    fn fetch_skill_markdown(&self, url: &str) -> Result<String, SkillUrlFetchError> {
        let agent: ureq::Agent = ureq::Agent::config_builder()
            .timeout_global(Some(Duration::from_secs(30)))
            .build()
            .into();
        let response: ureq::http::Response<ureq::Body> =
            agent.get(url).call().map_err(|error| SkillUrlFetchError {
                message: error.to_string(),
            })?;
        read_limited_skill_markdown(response.into_body().into_reader())
    }
}

fn read_limited_skill_markdown(reader: impl Read) -> Result<String, SkillUrlFetchError> {
    let mut bytes = Vec::new();
    reader
        .take(MAX_SKILL_MARKDOWN_BYTES + 1)
        .read_to_end(&mut bytes)
        .map_err(|error| SkillUrlFetchError {
            message: error.to_string(),
        })?;

    if bytes.len() as u64 > MAX_SKILL_MARKDOWN_BYTES {
        return Err(SkillUrlFetchError {
            message: format!(
                "skill Markdown response exceeds the {} byte limit",
                MAX_SKILL_MARKDOWN_BYTES
            ),
        });
    }

    String::from_utf8(bytes).map_err(|error| SkillUrlFetchError {
        message: format!("skill Markdown response is not valid UTF-8: {error}"),
    })
}

fn usage() -> String {
    "usage: skill-importer list --json [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]\n       skill-importer import markdown --json [--source-location VALUE] [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]\n       skill-importer import path --json --path PATH [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]\n       skill-importer import url --json --url URL [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]\n       skill-importer enable --json --skill NAME --agent claude-code|codex [--agent claude-code|codex] [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]\n       skill-importer disable --json --skill NAME --agent claude-code|codex [--agent claude-code|codex] [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]\n       skill-importer promote --json --skill NAME [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]\n       skill-importer delete --json --skill NAME [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]\n       skill-importer tui [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]".to_string()
}

#[cfg(test)]
mod tests {
    use super::*;
    use skill_importer::{SkillUrlFetchError, SkillUrlFetcher};
    use std::cell::RefCell;

    #[test]
    fn import_url_command_fetches_with_injected_fetcher_and_outputs_action_json() {
        let temp = tempfile::tempdir().expect("tempdir");
        let canonical_root = temp.path().join("canonical");
        let imports_root = temp.path().join("imports");
        let mut stdout = Vec::new();
        let fetcher = StaticFetcher {
            markdown: r#"---
name: command-url-import
description: Imported from a URL through the command.
---

# Command URL Import
"#,
        };

        run_with_url_fetcher(
            [
                OsString::from("import"),
                OsString::from("url"),
                OsString::from("--json"),
                OsString::from("--url"),
                OsString::from("https://example.test/command-url-import.md"),
                OsString::from("--canonical-root"),
                canonical_root.clone().into_os_string(),
                OsString::from("--imports-root"),
                imports_root.clone().into_os_string(),
            ],
            &mut stdout,
            &fetcher,
        )
        .expect("command succeeds");

        let json: serde_json::Value = serde_json::from_slice(&stdout).expect("valid json output");
        assert_eq!(json["skill_name"], "command-url-import");
        assert_eq!(json["manifest"]["source_type"], "url");
        assert_eq!(
            json["manifest"]["source_location"],
            "https://example.test/command-url-import.md"
        );
        assert!(
            imports_root
                .join("command-url-import")
                .join("SKILL.md")
                .exists()
        );
    }

    #[test]
    fn import_url_command_reports_fetch_failures_without_partial_storage() {
        let temp = tempfile::tempdir().expect("tempdir");
        let canonical_root = temp.path().join("canonical");
        let imports_root = temp.path().join("imports");
        let mut stdout = Vec::new();

        let error = run_with_url_fetcher(
            [
                OsString::from("import"),
                OsString::from("url"),
                OsString::from("--json"),
                OsString::from("--url"),
                OsString::from("https://example.test/missing.md"),
                OsString::from("--canonical-root"),
                canonical_root.into_os_string(),
                OsString::from("--imports-root"),
                imports_root.clone().into_os_string(),
            ],
            &mut stdout,
            &FailingFetcher,
        )
        .expect_err("command fails");

        assert!(
            error.contains("failed to import URL"),
            "error should name the failing operation: {error}"
        );
        assert!(
            error.contains("https://example.test/missing.md"),
            "error should include the URL: {error}"
        );
        assert!(
            error.contains("HTTP 404"),
            "error should include the fetch failure: {error}"
        );
        assert!(
            !imports_root.exists(),
            "failed URL command should not create storage"
        );
    }

    #[test]
    fn tui_command_routes_to_injected_runner_and_bare_command_remains_usage() {
        let temp = tempfile::tempdir().expect("tempdir");
        let canonical_root = temp.path().join("canonical");
        let imports_root = temp.path().join("imports");
        let claude_root = temp.path().join("claude");
        let codex_root = temp.path().join("codex");
        let runner = RecordingTuiRunner::default();
        let mut stdout = Vec::new();

        run_with_services(
            [
                OsString::from("tui"),
                OsString::from("--canonical-root"),
                canonical_root.clone().into_os_string(),
                OsString::from("--imports-root"),
                imports_root.clone().into_os_string(),
                OsString::from("--claude-code-root"),
                claude_root.clone().into_os_string(),
                OsString::from("--codex-root"),
                codex_root.clone().into_os_string(),
            ],
            &mut stdout,
            &StaticFetcher { markdown: "" },
            &runner,
        )
        .expect("tui command routes to runner");

        assert_eq!(
            runner.roots.borrow().as_ref(),
            Some(&DiscoveryRoots {
                canonical_root,
                imports_root,
                claude_code_root: claude_root,
                codex_root,
            })
        );
        assert!(stdout.is_empty(), "tui runner should own terminal output");

        let error = run_with_services(
            Vec::<OsString>::new(),
            &mut Vec::new(),
            &StaticFetcher { markdown: "" },
            &runner,
        )
        .expect_err("bare command still reports usage");
        assert!(error.contains("usage: skill-importer list --json"));
        assert!(error.contains("skill-importer tui"));
        assert_eq!(
            runner.calls(),
            1,
            "bare command must not launch the TUI runner"
        );
    }

    #[derive(Default)]
    struct RecordingTuiRunner {
        roots: RefCell<Option<DiscoveryRoots>>,
        calls: RefCell<usize>,
    }

    impl RecordingTuiRunner {
        fn calls(&self) -> usize {
            *self.calls.borrow()
        }
    }

    impl TuiRunner for RecordingTuiRunner {
        fn run(&self, roots: &DiscoveryRoots) -> Result<(), String> {
            *self.calls.borrow_mut() += 1;
            *self.roots.borrow_mut() = Some(roots.clone());
            Ok(())
        }
    }

    struct StaticFetcher {
        markdown: &'static str,
    }

    impl SkillUrlFetcher for StaticFetcher {
        fn fetch_skill_markdown(&self, _url: &str) -> Result<String, SkillUrlFetchError> {
            Ok(self.markdown.to_string())
        }
    }

    struct FailingFetcher;

    impl SkillUrlFetcher for FailingFetcher {
        fn fetch_skill_markdown(&self, _url: &str) -> Result<String, SkillUrlFetchError> {
            Err(SkillUrlFetchError {
                message: "HTTP 404".to_string(),
            })
        }
    }
}
