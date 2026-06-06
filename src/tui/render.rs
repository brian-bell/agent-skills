use ratatui::{
    Frame,
    layout::{Constraint, Direction, Layout, Rect},
    style::{Modifier, Style},
    text::{Line, Span},
    widgets::{Block, Borders, List, ListItem, Paragraph, Wrap},
};

use crate::tui::state::{
    AppInteractionMode, AppState, agent_label, enablement_label, entry_status_label, source_label,
};

pub fn render_app(frame: &mut Frame<'_>, state: &AppState) {
    let area = frame.area();
    let rows = if area.height < 12 {
        Layout::default()
            .direction(Direction::Vertical)
            .constraints([
                Constraint::Length(3),
                Constraint::Min(1),
                Constraint::Length(3),
            ])
            .split(area)
    } else {
        Layout::default()
            .direction(Direction::Vertical)
            .constraints([
                Constraint::Length(3),
                Constraint::Min(6),
                Constraint::Length(3),
                Constraint::Length(3),
            ])
            .split(area)
    };

    render_header(frame, state, rows[0]);
    if matches!(state.mode(), AppInteractionMode::RepositorySelection { .. }) {
        render_repository_selection(frame, state, rows[1]);
    } else {
        render_main(frame, state, rows[1]);
    }
    render_status(frame, state, rows[2]);
    if rows.len() > 3 {
        render_help(frame, state, rows[3]);
    }
}

fn render_header(frame: &mut Frame<'_>, state: &AppState, area: Rect) {
    let text = format!(
        "Skill Importer TUI | Filter: {} | Active target: {}",
        if state.filter().is_empty() {
            "(none)"
        } else {
            state.filter()
        },
        agent_label(state.active_target())
    );
    frame.render_widget(
        Paragraph::new(text).block(Block::default().title("Skills").borders(Borders::ALL)),
        area,
    );
}

fn render_main(frame: &mut Frame<'_>, state: &AppState, area: Rect) {
    let columns = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([Constraint::Percentage(42), Constraint::Percentage(58)])
        .split(area);

    let rows = state.visible_skills();
    let items = if rows.is_empty() {
        vec![ListItem::new("No skills match the filter")]
    } else {
        rows.iter()
            .map(|skill| {
                let marker = if skill.selected { "> " } else { "  " };
                ListItem::new(format!("{marker}{}", skill.name))
            })
            .collect()
    };
    frame.render_widget(
        List::new(items).block(Block::default().title("Skill list").borders(Borders::ALL)),
        columns[0],
    );

    let detail = if let Some(detail) = state.selected_detail() {
        vec![
            Line::from(vec![Span::raw("Name: "), Span::raw(detail.name)]),
            Line::from(vec![
                Span::raw("Description: "),
                Span::raw(detail.description.unwrap_or_else(|| "(none)".to_string())),
            ]),
            Line::from(vec![
                Span::raw("Source: "),
                Span::raw(source_label(detail.source)),
            ]),
            Line::from(vec![
                Span::raw("Enablement: "),
                Span::raw(enablement_label(detail.enablement)),
            ]),
            Line::from(vec![
                Span::raw("Claude Code: "),
                Span::raw(entry_status_label(detail.agent_entries.claude_code)),
            ]),
            Line::from(vec![
                Span::raw("Codex: "),
                Span::raw(entry_status_label(detail.agent_entries.codex)),
            ]),
        ]
    } else {
        vec![Line::from("No selected skill")]
    };
    frame.render_widget(
        Paragraph::new(detail).wrap(Wrap { trim: true }).block(
            Block::default()
                .title("Selected detail")
                .borders(Borders::ALL),
        ),
        columns[1],
    );
}

fn render_repository_selection(frame: &mut Frame<'_>, state: &AppState, area: Rect) {
    let candidates = state.repository_candidates();
    let items = if candidates.is_empty() {
        vec![ListItem::new("No repository candidates")]
    } else {
        candidates
            .iter()
            .map(|candidate| {
                let marker = if candidate.selected { "> " } else { "  " };
                let description = candidate.description.as_deref().unwrap_or("");
                ListItem::new(format!(
                    "{marker}{} | {} | {}",
                    candidate.name, candidate.relative_path, description
                ))
            })
            .collect()
    };
    frame.render_widget(
        List::new(items).block(
            Block::default()
                .title("Repository selection")
                .borders(Borders::ALL),
        ),
        area,
    );
}

fn render_status(frame: &mut Frame<'_>, state: &AppState, area: Rect) {
    let text = if let Some(request) = state.pending_request() {
        let skill = request
            .skill_name()
            .map(|name| format!(" ({name})"))
            .unwrap_or_default();
        format!("Status: pending {}{}", request.status_label(), skill)
    } else if let Some(status) = state.status_view() {
        let skill = status
            .skill_name
            .map(|name| format!(" ({name})"))
            .unwrap_or_default();
        format!("Status: {}{} - {}", status.operation, skill, status.message)
    } else if matches!(state.mode(), AppInteractionMode::ImportPrompt { .. }) {
        format!("Status: prompt {}", state.prompt_text())
    } else {
        "Status: ready".to_string()
    };
    frame.render_widget(
        Paragraph::new(text).block(Block::default().title("Result").borders(Borders::ALL)),
        area,
    );
}

fn render_help(frame: &mut Frame<'_>, state: &AppState, area: Rect) {
    let hints = state.action_hints().join(" | ");
    frame.render_widget(
        Paragraph::new(hints)
            .style(Style::default().add_modifier(Modifier::DIM))
            .block(
                Block::default()
                    .title("Keyboard hints")
                    .borders(Borders::ALL),
            ),
        area,
    );
}
