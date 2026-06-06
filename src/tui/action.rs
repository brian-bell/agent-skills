use std::path::PathBuf;

use crate::{ImportResult, RepositoryImportResult, SkillAgent, SkillOperationResult};

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum AppAction {
    FilterChanged(String),
    AppendFilter(char),
    DeleteFilterChar,
    MoveSelection(SelectionDelta),
    SwitchTarget(SkillAgent),
    OperationFinished(AppOperationResult),
    RepositorySelectionLoaded(crate::RepositorySkillSelection),
    MoveRepositoryCandidate(SelectionDelta),
    ChooseRepositoryCandidate,
    CancelRepositorySelection,
    CancelPrompt,
    CompletePendingOperation(Result<AppOperationResult, String>),
    CompleteOperation {
        request: Option<AppOperationRequest>,
        result: Result<AppOperationResult, String>,
    },
    BeginImportPrompt(AppImportSource),
    BeginConfirmation(ConfirmationOperation),
    PromptChanged(String),
    DeletePromptChar,
    SubmitPrompt,
    RequestEnableSelected,
    RequestDisableSelected,
    ConfirmPending,
    ClearPendingRequest,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SelectionDelta {
    Previous,
    Next,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ConfirmationOperation {
    Promote,
    Delete,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum AppImportSource {
    Markdown,
    Path,
    Url,
    Repository,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum AppOperationRequest {
    EnableSkill {
        skill_name: String,
        agent: SkillAgent,
    },
    DisableSkill {
        skill_name: String,
        agent: SkillAgent,
    },
    PromoteSkill {
        skill_name: String,
    },
    DeleteImport {
        skill_name: String,
    },
    ImportMarkdown {
        markdown: String,
    },
    ImportPath {
        path: PathBuf,
    },
    ImportUrl {
        url: String,
    },
    RepositoryImport {
        repository: String,
        selected_skill_path: Option<String>,
    },
}

impl AppOperationRequest {
    pub fn status_label(&self) -> &'static str {
        match self {
            AppOperationRequest::EnableSkill { .. } => "enable",
            AppOperationRequest::DisableSkill { .. } => "disable",
            AppOperationRequest::PromoteSkill { .. } => "promote",
            AppOperationRequest::DeleteImport { .. } => "delete",
            AppOperationRequest::ImportMarkdown { .. } => "import markdown",
            AppOperationRequest::ImportPath { .. } => "import path",
            AppOperationRequest::ImportUrl { .. } => "import url",
            AppOperationRequest::RepositoryImport { .. } => "repository import",
        }
    }

    pub fn skill_name(&self) -> Option<&str> {
        match self {
            AppOperationRequest::EnableSkill { skill_name, .. }
            | AppOperationRequest::DisableSkill { skill_name, .. }
            | AppOperationRequest::PromoteSkill { skill_name }
            | AppOperationRequest::DeleteImport { skill_name } => Some(skill_name.as_str()),
            AppOperationRequest::ImportMarkdown { .. }
            | AppOperationRequest::ImportPath { .. }
            | AppOperationRequest::ImportUrl { .. }
            | AppOperationRequest::RepositoryImport { .. } => None,
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum AppInput {
    Up,
    Down,
    Char(char),
    Backspace,
    Enter,
    Escape,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum InputOutcome {
    Action(AppAction),
    Quit,
    Ignored,
}

pub fn action_for_input(
    mode: &crate::tui::state::AppInteractionMode,
    input: AppInput,
) -> InputOutcome {
    use crate::tui::state::AppInteractionMode;

    match mode {
        AppInteractionMode::Main => match input {
            AppInput::Up | AppInput::Char('k') => {
                InputOutcome::Action(AppAction::MoveSelection(SelectionDelta::Previous))
            }
            AppInput::Down | AppInput::Char('j') => {
                InputOutcome::Action(AppAction::MoveSelection(SelectionDelta::Next))
            }
            AppInput::Char('m') => {
                InputOutcome::Action(AppAction::BeginImportPrompt(AppImportSource::Markdown))
            }
            AppInput::Char('f') => {
                InputOutcome::Action(AppAction::BeginImportPrompt(AppImportSource::Path))
            }
            AppInput::Char('u') => {
                InputOutcome::Action(AppAction::BeginImportPrompt(AppImportSource::Url))
            }
            AppInput::Char('g') => {
                InputOutcome::Action(AppAction::BeginImportPrompt(AppImportSource::Repository))
            }
            AppInput::Char('c') => {
                InputOutcome::Action(AppAction::SwitchTarget(crate::SkillAgent::ClaudeCode))
            }
            AppInput::Char('x') => {
                InputOutcome::Action(AppAction::SwitchTarget(crate::SkillAgent::Codex))
            }
            AppInput::Char('e') => InputOutcome::Action(AppAction::RequestEnableSelected),
            AppInput::Char('d') => InputOutcome::Action(AppAction::RequestDisableSelected),
            AppInput::Char('p') => {
                InputOutcome::Action(AppAction::BeginConfirmation(ConfirmationOperation::Promote))
            }
            AppInput::Char('r') => {
                InputOutcome::Action(AppAction::BeginConfirmation(ConfirmationOperation::Delete))
            }
            AppInput::Char('q') => InputOutcome::Quit,
            AppInput::Char(character) => InputOutcome::Action(AppAction::AppendFilter(character)),
            AppInput::Backspace => InputOutcome::Action(AppAction::DeleteFilterChar),
            _ => InputOutcome::Ignored,
        },
        AppInteractionMode::ImportPrompt { .. } | AppInteractionMode::Confirm { .. } => match input
        {
            AppInput::Escape => InputOutcome::Action(AppAction::CancelPrompt),
            AppInput::Enter => InputOutcome::Action(AppAction::SubmitPrompt),
            AppInput::Backspace => InputOutcome::Action(AppAction::DeletePromptChar),
            AppInput::Char(character) => {
                InputOutcome::Action(AppAction::PromptChanged(character.to_string()))
            }
            _ => InputOutcome::Ignored,
        },
        AppInteractionMode::RepositorySelection { .. } => match input {
            AppInput::Up | AppInput::Char('k') => {
                InputOutcome::Action(AppAction::MoveRepositoryCandidate(SelectionDelta::Previous))
            }
            AppInput::Down | AppInput::Char('j') => {
                InputOutcome::Action(AppAction::MoveRepositoryCandidate(SelectionDelta::Next))
            }
            AppInput::Enter => InputOutcome::Action(AppAction::ChooseRepositoryCandidate),
            AppInput::Escape => InputOutcome::Action(AppAction::CancelRepositorySelection),
            AppInput::Char('q') => InputOutcome::Quit,
            _ => InputOutcome::Ignored,
        },
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct AppOperationResult {
    pub operation: String,
    pub skill_name: Option<String>,
    pub status: AppOperationStatus,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum AppOperationStatus {
    Success { action_count: usize },
    Failure { reason: String },
}

impl AppOperationResult {
    pub fn success(
        operation: impl Into<String>,
        skill_name: Option<String>,
        action_count: usize,
    ) -> Self {
        Self {
            operation: operation.into(),
            skill_name,
            status: AppOperationStatus::Success { action_count },
        }
    }

    pub fn failure(
        operation: impl Into<String>,
        skill_name: Option<String>,
        reason: impl Into<String>,
    ) -> Self {
        Self {
            operation: operation.into(),
            skill_name,
            status: AppOperationStatus::Failure {
                reason: reason.into(),
            },
        }
    }

    pub fn from_import(operation: impl Into<String>, import: &ImportResult) -> Self {
        Self::success(
            operation,
            Some(import.skill_name.clone()),
            import.actions.len(),
        )
    }

    pub fn from_repository_import(result: &RepositoryImportResult) -> Option<Self> {
        match result {
            RepositoryImportResult::Imported(import) => {
                Some(Self::from_import("repository import", import))
            }
            RepositoryImportResult::Selection(_) => None,
        }
    }

    pub fn from_skill_operation(
        operation: impl Into<String>,
        result: &SkillOperationResult,
    ) -> Self {
        Self::success(
            operation,
            Some(result.skill_name.clone()),
            result.actions.len(),
        )
    }
}
