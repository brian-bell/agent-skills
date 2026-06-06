use ratatui::{Terminal, backend::TestBackend};
use skill_importer::{
    AgentEnablement, AgentEntries, AgentEntryStatus, RepositorySkillCandidate, SkillAgent,
    SkillEntry, SkillInventory, SkillSource,
    tui::{
        AppAction, AppImportSource, AppOperationResult, AppState, ConfirmationOperation,
        SelectionDelta, render_app,
    },
};

#[test]
fn main_screen_renders_user_visible_sections() {
    let mut state = AppState::new(inventory(vec![
        skill("alpha", "First skill", SkillSource::Canonical),
        skill("beta", "Second skill", SkillSource::Imported),
    ]));
    state.reduce(AppAction::MoveSelection(SelectionDelta::Next));
    state.reduce(AppAction::SwitchTarget(SkillAgent::ClaudeCode));
    state.reduce(AppAction::OperationFinished(AppOperationResult::success(
        "enable",
        Some("beta".to_string()),
        2,
    )));

    let text = render_text(&state, 90, 24);

    for expected in [
        "Skill Importer TUI",
        "Skill list",
        "Selected detail",
        "Active target: Claude Code",
        "Keyboard hints",
        "Status: enable (beta) - success: 2 actions",
        "beta",
        "Source:",
        "Enablement:",
    ] {
        assert!(text.contains(expected), "missing `{expected}` in:\n{text}");
    }
}

#[test]
fn repository_selection_render_shows_candidates_and_confirm_cancel_hints() {
    let mut state = AppState::new(inventory(Vec::new()));
    state.reduce(AppAction::RepositorySelectionLoaded(
        skill_importer::RepositorySkillSelection {
            repository: "https://example.test/repo.git".to_string(),
            skills: vec![
                candidate("repo-alpha", "First repo", "skills/repo-alpha"),
                candidate("repo-beta", "Second repo", "skills/repo-beta"),
            ],
        },
    ));

    let text = render_text(&state, 90, 20);

    for expected in [
        "Repository selection",
        "repo-alpha",
        "skills/repo-alpha",
        "repo-beta",
        "enter import",
        "esc cancel",
    ] {
        assert!(text.contains(expected), "missing `{expected}` in:\n{text}");
    }
}

#[test]
fn success_and_failure_status_states_are_visible_without_stale_text() {
    let mut state = AppState::new(inventory(vec![skill(
        "alpha",
        "First",
        SkillSource::Canonical,
    )]));
    state.reduce(AppAction::OperationFinished(AppOperationResult::success(
        "enable",
        Some("alpha".to_string()),
        3,
    )));
    assert!(render_text(&state, 80, 20).contains("success: 3 actions"));

    state.reduce(AppAction::OperationFinished(AppOperationResult::failure(
        "disable",
        Some("alpha".to_string()),
        "unsafe entry",
    )));
    let text = render_text(&state, 80, 20);
    assert!(text.contains("failed: unsafe entry"));
    assert!(!text.contains("success: 3 actions"));
}

#[test]
fn pending_request_status_uses_human_label_instead_of_debug_struct() {
    let mut state = AppState::new(inventory(vec![skill(
        "alpha",
        "First",
        SkillSource::Canonical,
    )]));
    state.reduce(AppAction::RequestEnableSelected);

    let text = render_text(&state, 80, 20);

    assert!(text.contains("Status: pending enable (alpha)"));
    assert!(!text.contains("EnableSkill"));

    let mut import_state = AppState::new(inventory(Vec::new()));
    import_state.reduce(AppAction::BeginImportPrompt(AppImportSource::Url));
    import_state.reduce(AppAction::PromptChanged(
        "https://example.test/skill.md".to_string(),
    ));
    import_state.reduce(AppAction::SubmitPrompt);
    let import_text = render_text(&import_state, 80, 20);
    assert!(import_text.contains("Status: pending import url"));
    assert!(!import_text.contains("ImportUrl"));

    let mut delete_state = AppState::new(inventory(vec![skill(
        "alpha",
        "First",
        SkillSource::Imported,
    )]));
    delete_state.reduce(AppAction::BeginConfirmation(ConfirmationOperation::Delete));
    delete_state.reduce(AppAction::ConfirmPending);
    let delete_text = render_text(&delete_state, 80, 20);
    assert!(delete_text.contains("Status: pending delete (alpha)"));
    assert!(!delete_text.contains("DeleteImport"));
}

#[test]
fn pending_request_status_takes_precedence_over_previous_result() {
    let mut state = AppState::new(inventory(vec![skill(
        "alpha",
        "First",
        SkillSource::Canonical,
    )]));
    state.reduce(AppAction::OperationFinished(AppOperationResult::success(
        "disable",
        Some("alpha".to_string()),
        1,
    )));
    state.reduce(AppAction::RequestEnableSelected);

    let text = render_text(&state, 80, 20);

    assert!(text.contains("Status: pending enable (alpha)"));
    assert!(!text.contains("Status: disable (alpha) - success: 1 actions"));
}

#[test]
fn constrained_terminal_render_does_not_panic_and_preserves_essential_labels() {
    let state = AppState::new(inventory(vec![skill(
        "alpha",
        "First",
        SkillSource::Canonical,
    )]));

    let text = render_text(&state, 42, 9);

    assert!(text.contains("Skill"));
    assert!(text.contains("Status"));
}

fn render_text(state: &AppState, width: u16, height: u16) -> String {
    let backend = TestBackend::new(width, height);
    let mut terminal = Terminal::new(backend).expect("terminal");
    terminal
        .draw(|frame| render_app(frame, state))
        .expect("draw");
    terminal
        .backend()
        .buffer()
        .content()
        .iter()
        .map(|cell| cell.symbol())
        .collect::<String>()
}

fn inventory(skills: Vec<SkillEntry>) -> SkillInventory {
    SkillInventory { skills }
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

fn candidate(name: &str, description: &str, relative_path: &str) -> RepositorySkillCandidate {
    RepositorySkillCandidate {
        name: name.to_string(),
        description: Some(description.to_string()),
        relative_path: relative_path.to_string(),
    }
}
