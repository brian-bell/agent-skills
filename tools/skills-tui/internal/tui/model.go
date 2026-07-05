// Package tui implements the interactive layer of skills-tui: a
// behavior-identical port of the bash script's render/read_key/run_tui
// functions on top of the internal/skills engine.
package tui

import (
	"io"

	"agent-skills/tools/skills-tui/internal/skills"
)

// Row is one selectable line: a discovered skill, its last-refreshed state,
// and the user's desired selection (bash TSTATE/TDESIRED entries).
type Row struct {
	Skill   skills.Skill
	State   skills.State
	Desired skills.Desired
}

// Model is the whole TUI state: rows, cursor index, and an optional one-shot
// message rendered below the list.
type Model struct {
	Rows    []Row
	Cursor  int
	Message string
}

// LoadSkills discovers the repo's skills and seeds each row's state and
// desired selection, mirroring bash load_skills + refresh_states.
func LoadSkills(cfg skills.Config) (*Model, error) {
	list, err := skills.Discover(cfg.RepoDir)
	if err != nil {
		return nil, err
	}
	m := &Model{}
	for _, s := range list {
		m.Rows = append(m.Rows, Row{Skill: s, Desired: skills.DesiredRemove})
	}
	m.RefreshStates(cfg)
	return m, nil
}

// RefreshStates recomputes the on-disk state of every row and seeds desired
// from it, mirroring bash refresh_states: installed, partial, and upgrade
// rows default to selected; everything else to deselected.
func (m *Model) RefreshStates(cfg skills.Config) {
	for i := range m.Rows {
		st := cfg.SkillState(m.Rows[i].Skill)
		m.Rows[i].State = st
		m.Rows[i].Desired = skills.DefaultDesired(st)
	}
}

// MoveUp moves the cursor up with wrap-around.
func (m *Model) MoveUp() {
	n := len(m.Rows)
	if n == 0 {
		return
	}
	m.Cursor = (m.Cursor - 1 + n) % n
}

// MoveDown moves the cursor down with wrap-around.
func (m *Model) MoveDown() {
	n := len(m.Rows)
	if n == 0 {
		return
	}
	m.Cursor = (m.Cursor + 1) % n
}

// Toggle advances the cursor row's selection via the engine's ToggleDesired.
func (m *Model) Toggle() {
	if m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return
	}
	r := &m.Rows[m.Cursor]
	r.Desired = skills.ToggleDesired(r.State, r.Desired)
}

// SelectAll marks every row for install (bash 'a').
func (m *Model) SelectAll() {
	for i := range m.Rows {
		m.Rows[i].Desired = skills.DesiredInstall
	}
}

// SelectNone marks every row for removal (bash 'n').
func (m *Model) SelectNone() {
	for i := range m.Rows {
		m.Rows[i].Desired = skills.DesiredRemove
	}
}

// ApplyChanges applies every pending action and prints the same status block
// as bash apply_changes: each action's status line indented two spaces, or
// "  nothing to do" when no action ran, then a state refresh. It shares the
// engine's ApplyAll with the non-interactive path so the two cannot drift.
func (m *Model) ApplyChanges(cfg skills.Config, w io.Writer) {
	plans := make([]skills.ApplyPlan, len(m.Rows))
	for i, r := range m.Rows {
		plans[i] = skills.ApplyPlan{Skill: r.Skill, State: r.State, Desired: r.Desired}
	}
	cfg.ApplyAll(plans, w)
	m.RefreshStates(cfg)
}
