use skill_importer::{
    RepositorySkillCandidate, SkillAgent,
    tui::{
        AppAction, AppImportSource, AppInput, AppInteractionMode, AppOperationRequest, AppState,
        InputOutcome, SelectionDelta, action_for_input,
    },
};

#[test]
fn main_mode_keys_map_to_navigation_target_filter_and_quit() {
    let mode = AppInteractionMode::Main;

    assert_eq!(
        action_for_input(&mode, AppInput::Down),
        InputOutcome::Action(AppAction::MoveSelection(SelectionDelta::Next))
    );
    assert_eq!(
        action_for_input(&mode, AppInput::Char('k')),
        InputOutcome::Action(AppAction::MoveSelection(SelectionDelta::Previous))
    );
    assert_eq!(
        action_for_input(&mode, AppInput::Char('c')),
        InputOutcome::Action(AppAction::SwitchTarget(SkillAgent::ClaudeCode))
    );
    assert_eq!(
        action_for_input(&mode, AppInput::Char('x')),
        InputOutcome::Action(AppAction::SwitchTarget(SkillAgent::Codex))
    );
    assert_eq!(
        action_for_input(&mode, AppInput::Char('m')),
        InputOutcome::Action(AppAction::BeginImportPrompt(AppImportSource::Markdown))
    );
    assert_eq!(
        action_for_input(&mode, AppInput::Char('f')),
        InputOutcome::Action(AppAction::BeginImportPrompt(AppImportSource::Path))
    );
    assert_eq!(
        action_for_input(&mode, AppInput::Char('u')),
        InputOutcome::Action(AppAction::BeginImportPrompt(AppImportSource::Url))
    );
    assert_eq!(
        action_for_input(&mode, AppInput::Char('g')),
        InputOutcome::Action(AppAction::BeginImportPrompt(AppImportSource::Repository))
    );
    assert_eq!(
        action_for_input(&mode, AppInput::Char('a')),
        InputOutcome::Action(AppAction::AppendFilter('a'))
    );
    assert_eq!(
        action_for_input(&mode, AppInput::Char('q')),
        InputOutcome::Quit
    );
}

#[test]
fn escape_leaves_prompt_before_quit_and_prompt_text_maps_to_request() {
    let mut state = AppState::new(skill_importer::SkillInventory { skills: Vec::new() });
    state.reduce(AppAction::BeginImportPrompt(AppImportSource::Url));

    assert_eq!(
        action_for_input(state.mode(), AppInput::Escape),
        InputOutcome::Action(AppAction::CancelPrompt)
    );
    assert_ne!(
        action_for_input(state.mode(), AppInput::Escape),
        InputOutcome::Quit
    );
    assert_eq!(
        action_for_input(state.mode(), AppInput::Backspace),
        InputOutcome::Action(AppAction::DeletePromptChar)
    );

    state.reduce(AppAction::PromptChanged(
        "https://example.test/skill.md".to_string(),
    ));
    state.reduce(AppAction::SubmitPrompt);
    assert_eq!(
        state.pending_request(),
        Some(&AppOperationRequest::ImportUrl {
            url: "https://example.test/skill.md".to_string()
        })
    );

    let confirm_mode = AppInteractionMode::Confirm {
        operation: skill_importer::tui::ConfirmationOperation::Promote,
        skill_name: "alpha".to_string(),
    };
    assert_eq!(
        action_for_input(&confirm_mode, AppInput::Escape),
        InputOutcome::Action(AppAction::CancelPrompt)
    );
    assert_eq!(
        action_for_input(&confirm_mode, AppInput::Backspace),
        InputOutcome::Action(AppAction::DeletePromptChar)
    );
}

#[test]
fn repository_selection_keys_move_confirm_cancel_and_ignore_text() {
    let mode = AppInteractionMode::RepositorySelection {
        selection: skill_importer::RepositorySkillSelection {
            repository: "https://example.test/repo.git".to_string(),
            skills: vec![candidate("a", "A", "a"), candidate("b", "B", "b")],
        },
        selected_candidate: 0,
    };

    assert_eq!(
        action_for_input(&mode, AppInput::Char('j')),
        InputOutcome::Action(AppAction::MoveRepositoryCandidate(SelectionDelta::Next))
    );
    assert_eq!(
        action_for_input(&mode, AppInput::Up),
        InputOutcome::Action(AppAction::MoveRepositoryCandidate(SelectionDelta::Previous))
    );
    assert_eq!(
        action_for_input(&mode, AppInput::Enter),
        InputOutcome::Action(AppAction::ChooseRepositoryCandidate)
    );
    assert_eq!(
        action_for_input(&mode, AppInput::Escape),
        InputOutcome::Action(AppAction::CancelRepositorySelection)
    );
    assert_eq!(
        action_for_input(&mode, AppInput::Char('x')),
        InputOutcome::Ignored
    );
}

fn candidate(name: &str, description: &str, relative_path: &str) -> RepositorySkillCandidate {
    RepositorySkillCandidate {
        name: name.to_string(),
        description: Some(description.to_string()),
        relative_path: relative_path.to_string(),
    }
}
