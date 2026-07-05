package skills

import "fmt"

// Desired is the user's per-skill selection, mirroring the bash TDESIRED
// values 1 (install), 0 (remove), and "-" (hold an available upgrade).
type Desired int

const (
	DesiredRemove  Desired = 0  // bash 0
	DesiredInstall Desired = 1  // bash 1
	DesiredHold    Desired = -1 // bash "-"
)

// Action is a planned step for one skill, mirroring bash plan_action output.
type Action string

const (
	ActionInstall Action = "install"
	ActionUpgrade Action = "upgrade"
	ActionRemove  Action = "remove"
	ActionNone    Action = "none"
)

// PlanAction decides what to do given the current state and the desired
// selection, mirroring the bash plan_action matrix.
func PlanAction(current State, desired Desired) Action {
	if current == StateSkipped {
		return ActionNone
	}
	if desired == DesiredHold {
		return ActionNone
	}

	if desired == DesiredInstall {
		switch current {
		case StateNotInstalled, StatePartial:
			return ActionInstall
		case StateUpgrade:
			return ActionUpgrade
		default:
			return ActionNone
		}
	}

	if current == StateNotInstalled {
		return ActionNone
	}
	return ActionRemove
}

// ToggleDesired advances a row's selection on spacebar, mirroring bash
// toggle_desired: upgrade rows cycle install → hold → remove → install; all
// other rows flip install ↔ remove.
func ToggleDesired(state State, desired Desired) Desired {
	if state == StateUpgrade {
		switch desired {
		case DesiredInstall:
			return DesiredHold
		case DesiredHold:
			return DesiredRemove
		default:
			return DesiredInstall
		}
	}

	if desired == DesiredInstall {
		return DesiredRemove
	}
	return DesiredInstall
}

// Outcome is what actually happened when a planned action was applied,
// derived from the resulting on-disk state like bash apply_skill.
type Outcome string

const (
	OutcomeNone      Outcome = ""
	OutcomeInstalled Outcome = "installed"
	OutcomeUpgraded  Outcome = "upgraded"
	OutcomeRemoved   Outcome = "removed"
	OutcomePartial   Outcome = "partial"
	OutcomeBlocked   Outcome = "blocked"
)

// ApplyResult is the typed result of ApplySkill; StatusLine renders it as the
// exact bash status line.
type ApplyResult struct {
	Name    string
	Action  Action
	Outcome Outcome
	State   State // resulting state after an install/upgrade attempt
}

// StatusLine formats the result exactly like the bash apply_skill echoes.
// A no-op action yields an empty string (bash prints nothing).
func (r ApplyResult) StatusLine() string {
	switch r.Outcome {
	case OutcomeInstalled:
		return "+ installed " + r.Name
	case OutcomeUpgraded:
		return "^ upgraded " + r.Name
	case OutcomeRemoved:
		return "- removed " + r.Name
	case OutcomePartial:
		return fmt.Sprintf("~ %s partially applied (some targets need --force)", r.Name)
	case OutcomeBlocked:
		return fmt.Sprintf("! %s blocked: %s (use --force to overwrite)", r.Name, r.State)
	}
	return ""
}

// ApplySkill executes the planned action for one skill, mirroring bash
// apply_skill. destroy allows rm -rf of real dirs during an upgrade (set by
// --force). On install/upgrade, force is set for upgrades (and whenever
// destroy is set), the InstallSkill error is intentionally ignored — the
// known "Refusing to overwrite" failures are expected — and the outcome is
// derived from the RESULTING state, so partial outcomes and unexpected
// failures are reported accurately rather than always blaming --force.
func (c Config) ApplySkill(s Skill, desired Desired, destroy bool) ApplyResult {
	current := c.SkillState(s)
	action := PlanAction(current, desired)
	res := ApplyResult{Name: s.Name, Action: action}

	switch action {
	case ActionInstall, ActionUpgrade:
		force := action == ActionUpgrade
		// --force (destroy) implies overwriting symlinks too.
		if destroy {
			force = true
		}
		// Judge success from the resulting state, not the returned error.
		_ = c.InstallSkill(s, force, destroy)
		res.State = c.SkillState(s)
		switch res.State {
		case StateInstalled:
			if action == ActionUpgrade {
				res.Outcome = OutcomeUpgraded
			} else {
				res.Outcome = OutcomeInstalled
			}
		case StatePartial:
			res.Outcome = OutcomePartial
		default:
			res.Outcome = OutcomeBlocked
		}
	case ActionRemove:
		c.UninstallSkill(s)
		res.Outcome = OutcomeRemoved
	}
	return res
}
