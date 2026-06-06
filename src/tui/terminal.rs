use std::{io, path::Path, process::Command};

use crossterm::{
    event::{self, Event, KeyCode},
    execute,
    terminal::{EnterAlternateScreen, LeaveAlternateScreen, disable_raw_mode, enable_raw_mode},
};
use ratatui::{
    Terminal,
    backend::{Backend, CrosstermBackend},
};

use crate::{
    DeleteImportRequest, DisableSkillRequest, DiscoveryRoots, EnableSkillRequest,
    ImportLocalPathRequest, ImportMarkdownRequest, ImportRepositoryRequest, ImportUrlRequest,
    PromoteSkillRequest, RepositoryImportResult, SkillRepositoryCheckout,
    SkillRepositoryFetchError, SkillRepositoryProvider, SkillUrlFetcher, delete_unpromoted_import,
    disable_skill, discover_skills, enable_skill, import_local_path_skill, import_markdown_skill,
    import_repository_skill, import_url_skill, promote_imported_skill,
};

use super::{
    AppAction, AppInput, AppOperationRequest, AppOperationResult, AppState, InputOutcome,
    action_for_input, render_app,
};

pub fn run_tui(
    roots: &DiscoveryRoots,
    url_fetcher: &impl SkillUrlFetcher,
) -> Result<(), io::Error> {
    let repository_provider = GitRepositoryProvider;
    run_tui_with_services(roots, url_fetcher, &repository_provider)
}

fn run_tui_with_services(
    roots: &DiscoveryRoots,
    url_fetcher: &impl SkillUrlFetcher,
    repository_provider: &impl SkillRepositoryProvider,
) -> Result<(), io::Error> {
    let inventory = discover_skills(roots)?;
    let mut state = AppState::new(inventory);
    let mut stdout = io::stdout();
    let _cleanup = TerminalCleanup::enter(&mut stdout)?;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;
    let result = run_event_loop(
        &mut terminal,
        roots,
        url_fetcher,
        repository_provider,
        &mut state,
    );
    result.and_then(|()| terminal.show_cursor())
}

fn run_event_loop(
    terminal: &mut Terminal<CrosstermBackend<io::Stdout>>,
    roots: &DiscoveryRoots,
    url_fetcher: &impl SkillUrlFetcher,
    repository_provider: &impl SkillRepositoryProvider,
    state: &mut AppState,
) -> Result<(), io::Error> {
    loop {
        refresh_inventory_if_needed(roots, state);
        terminal.draw(|frame| render_app(frame, state))?;
        if let Event::Key(key) = event::read()? {
            let Some(input) = input_from_key_code(key.code) else {
                continue;
            };
            match action_for_input(state.mode(), input) {
                InputOutcome::Action(action) => {
                    handle_action(terminal, state, action, |request| {
                        execute_operation_request(roots, url_fetcher, repository_provider, request)
                    })?;
                }
                InputOutcome::Quit => break,
                InputOutcome::Ignored => {}
            }
        }
    }
    Ok(())
}

fn handle_action<B: Backend>(
    terminal: &mut Terminal<B>,
    state: &mut AppState,
    action: AppAction,
    execute_request: impl FnOnce(AppOperationRequest) -> Result<TerminalOperationOutcome, String>,
) -> Result<(), <B as Backend>::Error> {
    handle_action_with_pending_observer(terminal, state, action, execute_request, || {})
}

fn handle_action_with_pending_observer<B: Backend>(
    terminal: &mut Terminal<B>,
    state: &mut AppState,
    action: AppAction,
    execute_request: impl FnOnce(AppOperationRequest) -> Result<TerminalOperationOutcome, String>,
    mut after_pending_draw: impl FnMut(),
) -> Result<(), <B as Backend>::Error> {
    state.reduce(action);
    if state.pending_request().is_some() {
        terminal.draw(|frame| render_app(frame, state))?;
        after_pending_draw();
    }
    if let Some(request) = state.take_pending_request() {
        let request_context = request.clone();
        match execute_request(request) {
            Ok(TerminalOperationOutcome::Completed(result)) => {
                state.reduce(AppAction::CompleteOperation {
                    request: Some(request_context),
                    result: Ok(result),
                });
            }
            Ok(TerminalOperationOutcome::RepositorySelection(selection)) => {
                state.reduce(AppAction::RepositorySelectionLoaded(selection));
            }
            Err(reason) => {
                state.reduce(AppAction::CompleteOperation {
                    request: Some(request_context),
                    result: Err(reason),
                });
            }
        }
    }
    Ok(())
}

fn input_from_key_code(code: KeyCode) -> Option<AppInput> {
    match code {
        KeyCode::Up => Some(AppInput::Up),
        KeyCode::Down => Some(AppInput::Down),
        KeyCode::Enter => Some(AppInput::Enter),
        KeyCode::Esc => Some(AppInput::Escape),
        KeyCode::Backspace => Some(AppInput::Backspace),
        KeyCode::Char(character) => Some(AppInput::Char(character)),
        _ => None,
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
enum TerminalOperationOutcome {
    Completed(AppOperationResult),
    RepositorySelection(crate::RepositorySkillSelection),
}

fn execute_operation_request(
    roots: &DiscoveryRoots,
    url_fetcher: &impl SkillUrlFetcher,
    repository_provider: &impl SkillRepositoryProvider,
    request: AppOperationRequest,
) -> Result<TerminalOperationOutcome, String> {
    match request {
        AppOperationRequest::EnableSkill { skill_name, agent } => enable_skill(
            roots,
            EnableSkillRequest {
                skill_name: &skill_name,
                agents: &[agent],
            },
        )
        .map(|result| {
            TerminalOperationOutcome::Completed(AppOperationResult::from_skill_operation(
                "enable", &result,
            ))
        })
        .map_err(|failure| failure.error.to_string()),
        AppOperationRequest::DisableSkill { skill_name, agent } => disable_skill(
            roots,
            DisableSkillRequest {
                skill_name: &skill_name,
                agents: &[agent],
            },
        )
        .map(|result| {
            TerminalOperationOutcome::Completed(AppOperationResult::from_skill_operation(
                "disable", &result,
            ))
        })
        .map_err(|failure| failure.error.to_string()),
        AppOperationRequest::PromoteSkill { skill_name } => promote_imported_skill(
            roots,
            PromoteSkillRequest {
                skill_name: &skill_name,
            },
        )
        .map(|result| {
            TerminalOperationOutcome::Completed(AppOperationResult::from_skill_operation(
                "promote", &result,
            ))
        })
        .map_err(|failure| failure.error.to_string()),
        AppOperationRequest::DeleteImport { skill_name } => delete_unpromoted_import(
            roots,
            DeleteImportRequest {
                skill_name: &skill_name,
            },
        )
        .map(|result| {
            TerminalOperationOutcome::Completed(AppOperationResult::from_skill_operation(
                "delete", &result,
            ))
        })
        .map_err(|failure| failure.error.to_string()),
        AppOperationRequest::ImportMarkdown { markdown } => import_markdown_skill(
            roots,
            ImportMarkdownRequest {
                markdown: &markdown,
                source_location: Some("tui"),
            },
        )
        .map(|import| {
            TerminalOperationOutcome::Completed(AppOperationResult::from_import(
                "import markdown",
                &import,
            ))
        })
        .map_err(|error| error.to_string()),
        AppOperationRequest::ImportPath { path } => import_local_path_skill(
            roots,
            ImportLocalPathRequest {
                path: path.as_path(),
            },
        )
        .map(|import| {
            TerminalOperationOutcome::Completed(AppOperationResult::from_import(
                "import path",
                &import,
            ))
        })
        .map_err(|error| error.to_string()),
        AppOperationRequest::ImportUrl { url } => {
            import_url_skill(roots, ImportUrlRequest { url: &url }, url_fetcher)
                .map(|import| {
                    TerminalOperationOutcome::Completed(AppOperationResult::from_import(
                        "import url",
                        &import,
                    ))
                })
                .map_err(|error| error.to_string())
        }
        AppOperationRequest::RepositoryImport {
            repository,
            selected_skill_path,
        } => import_repository_skill(
            roots,
            ImportRepositoryRequest {
                repository: &repository,
                selected_skill_path: selected_skill_path.as_deref(),
            },
            repository_provider,
        )
        .map(|result| match result {
            RepositoryImportResult::Imported(import) => TerminalOperationOutcome::Completed(
                AppOperationResult::from_import("repository import", &import),
            ),
            RepositoryImportResult::Selection(selection) => {
                TerminalOperationOutcome::RepositorySelection(selection)
            }
        })
        .map_err(|error| error.to_string()),
    }
}

fn refresh_inventory_if_needed(roots: &DiscoveryRoots, state: &mut AppState) {
    if !state.needs_refresh() {
        return;
    }

    match discover_skills(roots) {
        Ok(inventory) => state.update_inventory(inventory),
        Err(error) => {
            state.reduce(AppAction::OperationFinished(AppOperationResult::failure(
                "refresh inventory",
                None,
                error.to_string(),
            )));
            state.clear_refresh_needed();
        }
    }
}

struct TerminalCleanup {
    raw_mode: bool,
    alternate_screen: bool,
}

impl TerminalCleanup {
    fn enter(stdout: &mut io::Stdout) -> Result<Self, io::Error> {
        enable_raw_mode()?;
        let mut cleanup = Self {
            raw_mode: true,
            alternate_screen: false,
        };
        execute!(stdout, EnterAlternateScreen)?;
        cleanup.alternate_screen = true;
        Ok(cleanup)
    }
}

impl Drop for TerminalCleanup {
    fn drop(&mut self) {
        if self.alternate_screen {
            let mut stdout = io::stdout();
            let _ = execute!(stdout, LeaveAlternateScreen);
        }
        if self.raw_mode {
            let _ = disable_raw_mode();
        }
    }
}

struct GitRepositoryProvider;

struct GitRepositoryCheckout {
    _temp_dir: tempfile::TempDir,
    path: std::path::PathBuf,
}

impl SkillRepositoryCheckout for GitRepositoryCheckout {
    fn path(&self) -> &Path {
        &self.path
    }
}

impl SkillRepositoryProvider for GitRepositoryProvider {
    type Checkout = GitRepositoryCheckout;

    fn fetch_repository(
        &self,
        repository: &str,
    ) -> Result<Self::Checkout, SkillRepositoryFetchError> {
        let temp_dir = tempfile::tempdir().map_err(|error| SkillRepositoryFetchError {
            message: error.to_string(),
        })?;
        let checkout_path = temp_dir.path().join("checkout");
        let output = Command::new("git")
            .args(["clone", "--depth", "1", repository])
            .arg(&checkout_path)
            .output()
            .map_err(|error| SkillRepositoryFetchError {
                message: format!("failed to run git clone: {error}"),
            })?;

        if !output.status.success() {
            return Err(SkillRepositoryFetchError {
                message: String::from_utf8_lossy(&output.stderr).trim().to_string(),
            });
        }

        Ok(GitRepositoryCheckout {
            _temp_dir: temp_dir,
            path: checkout_path,
        })
    }
}

#[cfg(test)]
mod tests {
    use std::{
        cell::Cell,
        fs,
        path::{Path, PathBuf},
        rc::Rc,
    };

    use ratatui::backend::TestBackend;

    use crate::{
        AgentEnablement, AgentEntries, AgentEntryStatus, SkillAgent, SkillEntry, SkillInventory,
        SkillRepositoryCheckout, SkillRepositoryFetchError, SkillRepositoryProvider, SkillSource,
        SkillUrlFetchError, SkillUrlFetcher,
    };

    use super::*;

    #[test]
    fn repository_import_request_uses_core_provider_boundary_for_selection_and_import() {
        let temp = tempfile::tempdir().expect("tempdir");
        let roots = roots(temp.path());
        let repository = temp.path().join("repo");
        write_skill(&repository, "repo-alpha", "First repo skill.");
        write_skill(&repository, "repo-beta", "Second repo skill.");
        let provider = StaticRepositoryProvider {
            repository_path: repository,
        };

        let selection = execute_operation_request(
            &roots,
            &UnusedFetcher,
            &provider,
            AppOperationRequest::RepositoryImport {
                repository: "https://example.test/repo.git".to_string(),
                selected_skill_path: None,
            },
        )
        .expect("repository selection succeeds");

        match selection {
            TerminalOperationOutcome::RepositorySelection(selection) => {
                assert_eq!(selection.repository, "https://example.test/repo.git");
                assert_eq!(selection.skills.len(), 2);
                assert_eq!(selection.skills[0].name, "repo-alpha");
            }
            TerminalOperationOutcome::Completed(result) => {
                panic!("expected selection, got completed result {result:?}")
            }
        }
        assert!(
            !roots.imports_root.exists(),
            "selection should not mutate import storage"
        );

        let imported = execute_operation_request(
            &roots,
            &UnusedFetcher,
            &provider,
            AppOperationRequest::RepositoryImport {
                repository: "https://example.test/repo.git".to_string(),
                selected_skill_path: Some("repo-beta".to_string()),
            },
        )
        .expect("selected repository import succeeds");

        match imported {
            TerminalOperationOutcome::Completed(result) => {
                assert_eq!(result.operation, "repository import");
                assert_eq!(result.skill_name.as_deref(), Some("repo-beta"));
            }
            TerminalOperationOutcome::RepositorySelection(selection) => {
                panic!(
                    "expected completed import, got {} choices",
                    selection.skills.len()
                )
            }
        }
        assert!(
            roots
                .imports_root
                .join("repo-beta")
                .join("SKILL.md")
                .exists()
        );
    }

    #[test]
    fn pending_frame_is_drawn_before_executing_operation_request() {
        let mut state = AppState::new(SkillInventory {
            skills: vec![skill("alpha", "First", SkillSource::Canonical)],
        });
        let backend = TestBackend::new(80, 20);
        let mut terminal = Terminal::new(backend).expect("terminal");
        let pending_drawn = Rc::new(Cell::new(false));
        let request_executed = Rc::new(Cell::new(false));

        handle_action_with_pending_observer(
            &mut terminal,
            &mut state,
            AppAction::RequestEnableSelected,
            {
                let pending_drawn = Rc::clone(&pending_drawn);
                let request_executed = Rc::clone(&request_executed);
                move |request| {
                    assert!(
                        pending_drawn.get(),
                        "pending frame should be drawn before execution starts"
                    );
                    request_executed.set(true);
                    assert_eq!(
                        request,
                        AppOperationRequest::EnableSkill {
                            skill_name: "alpha".to_string(),
                            agent: SkillAgent::Codex,
                        }
                    );
                    Ok(TerminalOperationOutcome::Completed(
                        AppOperationResult::success("enable", Some("alpha".to_string()), 1),
                    ))
                }
            },
            {
                let pending_drawn = Rc::clone(&pending_drawn);
                move || pending_drawn.set(true)
            },
        )
        .expect("handle action");

        assert!(request_executed.get());
        let rendered = terminal_text(&terminal);
        assert!(rendered.contains("Status: pending enable (alpha)"));
        assert!(state.status_view().expect("completion status").success);
    }

    #[test]
    fn pending_frame_is_drawn_before_executing_url_import_request() {
        let mut state = AppState::new(SkillInventory { skills: Vec::new() });
        state.reduce(AppAction::BeginImportPrompt(
            crate::tui::AppImportSource::Url,
        ));
        state.reduce(AppAction::PromptChanged(
            "https://example.test/skill.md".to_string(),
        ));
        let backend = TestBackend::new(80, 20);
        let mut terminal = Terminal::new(backend).expect("terminal");
        let pending_drawn = Rc::new(Cell::new(false));
        let request_executed = Rc::new(Cell::new(false));

        handle_action_with_pending_observer(
            &mut terminal,
            &mut state,
            AppAction::SubmitPrompt,
            {
                let pending_drawn = Rc::clone(&pending_drawn);
                let request_executed = Rc::clone(&request_executed);
                move |request| {
                    assert!(
                        pending_drawn.get(),
                        "pending frame should be drawn before URL import starts"
                    );
                    request_executed.set(true);
                    assert_eq!(
                        request,
                        AppOperationRequest::ImportUrl {
                            url: "https://example.test/skill.md".to_string(),
                        }
                    );
                    Err("network skipped".to_string())
                }
            },
            {
                let pending_drawn = Rc::clone(&pending_drawn);
                move || pending_drawn.set(true)
            },
        )
        .expect("handle action");

        assert!(request_executed.get());
        let rendered = terminal_text(&terminal);
        assert!(rendered.contains("Status: pending import url"));
        assert_eq!(
            state.status_view().expect("failure status").message,
            "failed: network skipped"
        );
    }

    struct UnusedFetcher;

    impl SkillUrlFetcher for UnusedFetcher {
        fn fetch_skill_markdown(&self, _url: &str) -> Result<String, SkillUrlFetchError> {
            panic!("repository test should not fetch URLs")
        }
    }

    struct StaticRepositoryProvider {
        repository_path: PathBuf,
    }

    impl SkillRepositoryProvider for StaticRepositoryProvider {
        type Checkout = StaticRepositoryCheckout;

        fn fetch_repository(
            &self,
            _repository: &str,
        ) -> Result<Self::Checkout, SkillRepositoryFetchError> {
            Ok(StaticRepositoryCheckout {
                path: self.repository_path.clone(),
            })
        }
    }

    struct StaticRepositoryCheckout {
        path: PathBuf,
    }

    impl SkillRepositoryCheckout for StaticRepositoryCheckout {
        fn path(&self) -> &Path {
            &self.path
        }
    }

    fn roots(base: &Path) -> DiscoveryRoots {
        DiscoveryRoots {
            canonical_root: base.join("canonical"),
            imports_root: base.join("imports"),
            claude_code_root: base.join("claude"),
            codex_root: base.join("codex"),
        }
    }

    fn write_skill(root: &Path, name: &str, description: &str) {
        let skill_path = root.join(name);
        fs::create_dir_all(&skill_path).expect("skill dir");
        fs::write(
            skill_path.join("SKILL.md"),
            format!(
                r#"---
name: {name}
description: {description}
---

# {name}
"#
            ),
        )
        .expect("skill file");
    }

    fn skill(name: &str, description: &str, source: SkillSource) -> SkillEntry {
        SkillEntry {
            name: name.to_string(),
            description: Some(description.to_string()),
            source,
            enablement: AgentEnablement::Neither,
            agent_entries: AgentEntries {
                claude_code: AgentEntryStatus::Missing,
                codex: AgentEntryStatus::Missing,
            },
        }
    }

    fn terminal_text(terminal: &Terminal<TestBackend>) -> String {
        terminal
            .backend()
            .buffer()
            .content()
            .iter()
            .map(|cell| cell.symbol())
            .collect::<String>()
    }
}
