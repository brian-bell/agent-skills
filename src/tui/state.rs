use crate::{
    AgentEnablement, AgentEntries, AgentEntryStatus, RepositorySkillSelection, SkillAgent,
    SkillEntry, SkillInventory, SkillSource,
};

use crate::tui::action::{
    AppAction, AppImportSource, AppOperationRequest, ConfirmationOperation, SelectionDelta,
};
pub use crate::tui::action::{AppOperationResult, AppOperationStatus};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct AppState {
    inventory: SkillInventory,
    visible_indices: Vec<usize>,
    selected_visible: Option<usize>,
    active_target: SkillAgent,
    filter: String,
    latest_result: Option<AppOperationResult>,
    mode: AppInteractionMode,
    pending_request: Option<AppOperationRequest>,
    needs_refresh: bool,
    prompt_text: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum AppInteractionMode {
    Main,
    ImportPrompt {
        source: AppImportSource,
    },
    Confirm {
        operation: ConfirmationOperation,
        skill_name: String,
    },
    RepositorySelection {
        selection: RepositorySkillSelection,
        selected_candidate: usize,
    },
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SkillRow {
    pub name: String,
    pub description: Option<String>,
    pub selected: bool,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SkillDetail {
    pub name: String,
    pub description: Option<String>,
    pub source: SkillSource,
    pub enablement: AgentEnablement,
    pub agent_entries: AgentEntries,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CandidateView {
    pub name: String,
    pub description: Option<String>,
    pub relative_path: String,
    pub selected: bool,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct StatusView {
    pub operation: String,
    pub skill_name: Option<String>,
    pub message: String,
    pub success: bool,
}

const DEFAULT_ACTIVE_TARGET: SkillAgent = SkillAgent::Codex;

impl AppState {
    pub fn new(inventory: SkillInventory) -> Self {
        let mut state = Self {
            inventory,
            visible_indices: Vec::new(),
            selected_visible: None,
            // Codex is the default because this TUI is primarily launched from
            // Codex skill workflows; the target remains explicit and switchable.
            active_target: DEFAULT_ACTIVE_TARGET,
            filter: String::new(),
            latest_result: None,
            mode: AppInteractionMode::Main,
            pending_request: None,
            needs_refresh: false,
            prompt_text: String::new(),
        };
        state.recompute_visible();
        state
    }

    pub fn reduce(&mut self, action: AppAction) {
        match action {
            AppAction::FilterChanged(filter) => {
                self.filter = filter;
                self.recompute_visible();
            }
            AppAction::AppendFilter(character) => {
                self.filter.push(character);
                self.recompute_visible();
            }
            AppAction::DeleteFilterChar => {
                self.filter.pop();
                self.recompute_visible();
            }
            AppAction::MoveSelection(delta) => self.move_selection(delta),
            AppAction::SwitchTarget(target) => self.active_target = target,
            AppAction::OperationFinished(result) => self.latest_result = Some(result),
            AppAction::RepositorySelectionLoaded(selection) => {
                self.latest_result = None;
                self.pending_request = None;
                self.mode = AppInteractionMode::RepositorySelection {
                    selection,
                    selected_candidate: 0,
                };
            }
            AppAction::MoveRepositoryCandidate(delta) => self.move_repository_candidate(delta),
            AppAction::ChooseRepositoryCandidate => self.choose_repository_candidate(),
            AppAction::CancelRepositorySelection => {
                self.mode = AppInteractionMode::Main;
                self.pending_request = None;
                self.prompt_text.clear();
            }
            AppAction::CancelPrompt => {
                self.mode = AppInteractionMode::Main;
                self.pending_request = None;
                self.prompt_text.clear();
            }
            AppAction::CompletePendingOperation(result) => self.complete_pending(result),
            AppAction::CompleteOperation { request, result } => {
                self.complete_operation(request, result)
            }
            AppAction::BeginImportPrompt(source) => {
                self.mode = AppInteractionMode::ImportPrompt { source };
                self.prompt_text.clear();
            }
            AppAction::BeginConfirmation(operation) => {
                if let Some(skill) = self.selected_skill() {
                    self.mode = AppInteractionMode::Confirm {
                        operation,
                        skill_name: skill.name.clone(),
                    };
                    self.prompt_text.clear();
                }
            }
            AppAction::PromptChanged(input) => self.apply_prompt_input(&input),
            AppAction::DeletePromptChar => {
                self.prompt_text.pop();
            }
            AppAction::SubmitPrompt => self.submit_prompt(),
            AppAction::RequestEnableSelected => {
                if let Some(skill) = self.selected_skill() {
                    self.pending_request = Some(AppOperationRequest::EnableSkill {
                        skill_name: skill.name.clone(),
                        agent: self.active_target,
                    });
                }
            }
            AppAction::RequestDisableSelected => {
                if let Some(skill) = self.selected_skill() {
                    self.pending_request = Some(AppOperationRequest::DisableSkill {
                        skill_name: skill.name.clone(),
                        agent: self.active_target,
                    });
                }
            }
            AppAction::ConfirmPending => self.confirm_pending(),
            AppAction::ClearPendingRequest => self.pending_request = None,
        }
    }

    pub fn visible_skills(&self) -> Vec<SkillRow> {
        self.visible_indices
            .iter()
            .enumerate()
            .map(|(visible_index, inventory_index)| {
                let skill = &self.inventory.skills[*inventory_index];
                SkillRow {
                    name: skill.name.clone(),
                    description: skill.description.clone(),
                    selected: self.selected_visible == Some(visible_index),
                }
            })
            .collect()
    }

    pub fn selected_detail(&self) -> Option<SkillDetail> {
        self.selected_skill().map(|skill| SkillDetail {
            name: skill.name.clone(),
            description: skill.description.clone(),
            source: skill.source,
            enablement: skill.enablement,
            agent_entries: skill.agent_entries.clone(),
        })
    }

    pub fn repository_candidates(&self) -> Vec<CandidateView> {
        match &self.mode {
            AppInteractionMode::RepositorySelection {
                selection,
                selected_candidate,
            } => selection
                .skills
                .iter()
                .enumerate()
                .map(|(index, candidate)| CandidateView {
                    name: candidate.name.clone(),
                    description: candidate.description.clone(),
                    relative_path: candidate.relative_path.clone(),
                    selected: index == *selected_candidate,
                })
                .collect(),
            _ => Vec::new(),
        }
    }

    pub fn action_hints(&self) -> Vec<String> {
        match &self.mode {
            AppInteractionMode::Main => vec![
                "j/k move".to_string(),
                format!("e enable {}", agent_label(self.active_target)),
                format!("d disable {}", agent_label(self.active_target)),
                "c Claude".to_string(),
                "x Codex".to_string(),
                "p promote".to_string(),
                "r delete".to_string(),
                "u URL".to_string(),
                "f path".to_string(),
                "m markdown".to_string(),
                "g repo".to_string(),
                "q quit".to_string(),
            ],
            AppInteractionMode::RepositorySelection { .. } => vec![
                "j/k candidate".to_string(),
                "enter import".to_string(),
                "esc cancel".to_string(),
            ],
            AppInteractionMode::ImportPrompt { .. } => {
                vec!["enter submit".to_string(), "esc cancel".to_string()]
            }
            AppInteractionMode::Confirm { operation, .. } => vec![
                format!("enter confirm {}", confirmation_label(*operation)),
                "esc cancel".to_string(),
            ],
        }
    }

    pub fn status_view(&self) -> Option<StatusView> {
        self.latest_result
            .as_ref()
            .map(|result| match &result.status {
                AppOperationStatus::Success { action_count } => StatusView {
                    operation: result.operation.clone(),
                    skill_name: result.skill_name.clone(),
                    message: format!("success: {action_count} actions"),
                    success: true,
                },
                AppOperationStatus::Failure { reason } => StatusView {
                    operation: result.operation.clone(),
                    skill_name: result.skill_name.clone(),
                    message: format!("failed: {reason}"),
                    success: false,
                },
            })
    }

    pub fn active_target(&self) -> SkillAgent {
        self.active_target
    }

    pub fn filter(&self) -> &str {
        &self.filter
    }

    pub fn latest_result(&self) -> Option<&AppOperationResult> {
        self.latest_result.as_ref()
    }

    pub fn mode(&self) -> &AppInteractionMode {
        &self.mode
    }

    pub fn pending_request(&self) -> Option<&AppOperationRequest> {
        self.pending_request.as_ref()
    }

    pub fn take_pending_request(&mut self) -> Option<AppOperationRequest> {
        self.pending_request.take()
    }

    pub fn needs_refresh(&self) -> bool {
        self.needs_refresh
    }

    pub fn prompt_text(&self) -> &str {
        &self.prompt_text
    }

    pub fn update_inventory(&mut self, inventory: SkillInventory) {
        self.inventory = inventory;
        self.recompute_visible();
        self.needs_refresh = false;
    }

    pub(crate) fn clear_refresh_needed(&mut self) {
        self.needs_refresh = false;
    }

    fn recompute_visible(&mut self) {
        let previous_selected_name = self.selected_skill().map(|skill| skill.name.clone());
        self.visible_indices = self
            .inventory
            .skills
            .iter()
            .enumerate()
            .filter_map(|(index, skill)| {
                if skill_matches_filter(skill, &self.filter) {
                    Some(index)
                } else {
                    None
                }
            })
            .collect();

        self.selected_visible = if self.visible_indices.is_empty() {
            None
        } else if let Some(previous_selected_name) = previous_selected_name {
            self.visible_indices
                .iter()
                .position(|index| self.inventory.skills[*index].name == previous_selected_name)
                .or(Some(0))
        } else {
            Some(0)
        };
    }

    fn selected_skill(&self) -> Option<&SkillEntry> {
        let visible_index = self.selected_visible?;
        let inventory_index = *self.visible_indices.get(visible_index)?;
        self.inventory.skills.get(inventory_index)
    }

    fn move_selection(&mut self, delta: SelectionDelta) {
        let Some(selected) = self.selected_visible else {
            return;
        };
        self.selected_visible = Some(match delta {
            SelectionDelta::Previous => selected.saturating_sub(1),
            SelectionDelta::Next => (selected + 1).min(self.visible_indices.len() - 1),
        });
    }

    fn move_repository_candidate(&mut self, delta: SelectionDelta) {
        let AppInteractionMode::RepositorySelection {
            selection,
            selected_candidate,
        } = &mut self.mode
        else {
            return;
        };
        if selection.skills.is_empty() {
            return;
        }
        *selected_candidate = match delta {
            SelectionDelta::Previous => selected_candidate.saturating_sub(1),
            SelectionDelta::Next => (*selected_candidate + 1).min(selection.skills.len() - 1),
        };
    }

    fn choose_repository_candidate(&mut self) {
        if let AppInteractionMode::RepositorySelection {
            selection,
            selected_candidate,
        } = &self.mode
        {
            let Some(candidate) = selection.skills.get(*selected_candidate) else {
                return;
            };
            self.pending_request = Some(AppOperationRequest::RepositoryImport {
                repository: selection.repository.clone(),
                selected_skill_path: Some(candidate.relative_path.clone()),
            });
        }
    }

    fn complete_pending(&mut self, result: Result<AppOperationResult, String>) {
        let request = self.pending_request.take();
        self.complete_operation(request, result);
    }

    fn complete_operation(
        &mut self,
        request: Option<AppOperationRequest>,
        result: Result<AppOperationResult, String>,
    ) {
        match result {
            Ok(result) => {
                self.latest_result = Some(result);
                self.needs_refresh = true;
                self.mode = AppInteractionMode::Main;
            }
            Err(reason) => {
                let (operation, skill_name) = failure_context(request.as_ref(), &self.mode);
                self.latest_result =
                    Some(AppOperationResult::failure(operation, skill_name, reason));
            }
        }
    }

    fn apply_prompt_input(&mut self, input: &str) {
        self.prompt_text.push_str(input);
    }

    fn submit_prompt(&mut self) {
        match &self.mode {
            AppInteractionMode::ImportPrompt { source } => {
                let request = match source {
                    AppImportSource::Markdown => AppOperationRequest::ImportMarkdown {
                        markdown: self.prompt_text.clone(),
                    },
                    AppImportSource::Path => AppOperationRequest::ImportPath {
                        path: self.prompt_text.clone().into(),
                    },
                    AppImportSource::Url => AppOperationRequest::ImportUrl {
                        url: self.prompt_text.clone(),
                    },
                    AppImportSource::Repository => AppOperationRequest::RepositoryImport {
                        repository: self.prompt_text.clone(),
                        selected_skill_path: None,
                    },
                };
                self.pending_request = Some(request);
            }
            AppInteractionMode::Confirm { .. } => self.confirm_pending(),
            _ => {}
        }
    }

    fn confirm_pending(&mut self) {
        if let AppInteractionMode::Confirm {
            operation,
            skill_name,
        } = &self.mode
        {
            self.pending_request = Some(match operation {
                ConfirmationOperation::Promote => AppOperationRequest::PromoteSkill {
                    skill_name: skill_name.clone(),
                },
                ConfirmationOperation::Delete => AppOperationRequest::DeleteImport {
                    skill_name: skill_name.clone(),
                },
            });
        }
    }
}

fn failure_context(
    request: Option<&AppOperationRequest>,
    mode: &AppInteractionMode,
) -> (&'static str, Option<String>) {
    match request {
        Some(AppOperationRequest::EnableSkill { skill_name, .. }) => {
            ("enable", Some(skill_name.clone()))
        }
        Some(AppOperationRequest::DisableSkill { skill_name, .. }) => {
            ("disable", Some(skill_name.clone()))
        }
        Some(AppOperationRequest::PromoteSkill { skill_name }) => {
            ("promote", Some(skill_name.clone()))
        }
        Some(AppOperationRequest::DeleteImport { skill_name }) => {
            ("delete", Some(skill_name.clone()))
        }
        Some(AppOperationRequest::ImportMarkdown { .. }) => ("import markdown", None),
        Some(AppOperationRequest::ImportPath { .. }) => ("import path", None),
        Some(AppOperationRequest::ImportUrl { .. }) => ("import url", None),
        Some(AppOperationRequest::RepositoryImport { .. }) => ("repository import", None),
        None if matches!(mode, AppInteractionMode::RepositorySelection { .. }) => {
            ("repository import", None)
        }
        None => ("operation", None),
    }
}

fn skill_matches_filter(skill: &SkillEntry, filter: &str) -> bool {
    if filter.is_empty() {
        return true;
    }
    let filter = filter.to_lowercase();
    skill.name.to_lowercase().contains(&filter)
        || skill
            .description
            .as_deref()
            .unwrap_or_default()
            .to_lowercase()
            .contains(&filter)
}

pub fn source_label(source: SkillSource) -> &'static str {
    match source {
        SkillSource::Canonical => "canonical",
        SkillSource::Imported => "imported",
        SkillSource::AgentOnly => "agent only",
    }
}

pub fn enablement_label(enablement: AgentEnablement) -> &'static str {
    match enablement {
        AgentEnablement::Neither => "disabled",
        AgentEnablement::ClaudeCode => "Claude Code",
        AgentEnablement::Codex => "Codex",
        AgentEnablement::Both => "Claude Code + Codex",
    }
}

pub fn entry_status_label(status: AgentEntryStatus) -> &'static str {
    match status {
        AgentEntryStatus::Missing => "missing",
        AgentEntryStatus::SkillDirectory => "skill directory",
        AgentEntryStatus::CanonicalSymlink => "canonical symlink",
        AgentEntryStatus::ImportedSymlink => "imported symlink",
        AgentEntryStatus::ExternalSymlink => "external symlink",
        AgentEntryStatus::BrokenSymlink => "broken symlink",
    }
}

pub fn agent_label(agent: SkillAgent) -> &'static str {
    match agent {
        SkillAgent::ClaudeCode => "Claude Code",
        SkillAgent::Codex => "Codex",
    }
}

pub fn confirmation_label(operation: ConfirmationOperation) -> &'static str {
    match operation {
        ConfirmationOperation::Promote => "promote",
        ConfirmationOperation::Delete => "delete",
    }
}
