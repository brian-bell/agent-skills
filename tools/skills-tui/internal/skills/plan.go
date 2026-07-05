package skills

import (
	"fmt"
	"io"
)

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
// A no-op action yields an empty string (bash prints nothing). The skill name
// is sanitized because these lines are printed while the TUI terminal is still
// in raw mode.
func (r ApplyResult) StatusLine() string {
	name := SanitizeLabel(r.Name)
	switch r.Outcome {
	case OutcomeInstalled:
		return "+ installed " + name
	case OutcomeUpgraded:
		return "^ upgraded " + name
	case OutcomeRemoved:
		return "- removed " + name
	case OutcomePartial:
		return fmt.Sprintf("~ %s partially applied (some targets need --force)", name)
	case OutcomeBlocked:
		return fmt.Sprintf("! %s blocked: %s (use --force to overwrite)", name, r.State)
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
		// The known "Refusing to overwrite" refusals are expected and judged
		// from the resulting state below; any *other* error is unexpected and
		// logged so an unexplained blocked/partial is diagnosable.
		if err := c.InstallSkill(s, force, destroy); err != nil && !isExpectedRefusal(err) && c.WarnW != nil {
			fmt.Fprintln(c.WarnW, err)
		}
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
		// Bash uninstall_skill always succeeds (unlink_owned hardcodes a 0
		// return after `rm -f`, and the loop/rmdir bodies end in `|| true`), so
		// bash always prints "- removed". Preserve that stdout parity. The
		// substance of the review finding was that UnlinkOwned SWALLOWED real
		// errors; those are now surfaced via UninstallSkill and logged to WarnW
		// (stderr) for diagnosability, rather than reported on stdout as a
		// spurious "blocked" whose "--force to overwrite" advice is meaningless
		// for a removal.
		if err := c.UninstallSkill(s); err != nil && c.WarnW != nil {
			fmt.Fprintln(c.WarnW, err)
		}
		res.Outcome = OutcomeRemoved
	}
	return res
}

// ApplyPlan is one skill's pre-change snapshot for a batch apply: the state
// read before any changes plus the user's desired selection.
type ApplyPlan struct {
	Skill   Skill
	State   State
	Desired Desired
}

// ApplyAll applies each planned action against its pre-snapshot state and
// prints the same status block as bash apply_changes: each action's status
// line indented two spaces, or "  nothing to do" when no action ran. Gating on
// the pre-snapshot (not a freshly recomputed state) mirrors bash: an earlier
// install in the same batch cannot re-trigger a later skill. It reports
// whether any action ran.
func (c Config) ApplyAll(plans []ApplyPlan, w io.Writer) bool {
	changed := false
	for _, p := range plans {
		if PlanAction(p.State, p.Desired) == ActionNone {
			continue
		}
		res := c.ApplySkill(p.Skill, p.Desired, false)
		if line := res.StatusLine(); line != "" {
			fmt.Fprintf(w, "  %s\n", line)
		}
		changed = true
	}
	if !changed {
		fmt.Fprintln(w, "  nothing to do")
	}
	return changed
}
