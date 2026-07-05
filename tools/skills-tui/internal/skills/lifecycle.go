package skills

// Lifecycle is the (State, Desired) pair — the complete input to every
// per-skill decision the installer makes. It gives the scattered
// State × Desired matrix (which action to run, how spacebar advances the
// selection, what status text to show, what selection to seed) one home.
type Lifecycle struct {
	State   State
	Desired Desired
}

// Action decides what to do given the current state and the desired
// selection, mirroring the bash plan_action matrix.
func (l Lifecycle) Action() Action {
	if l.State == StateSkipped {
		return ActionNone
	}
	if l.Desired == DesiredHold {
		return ActionNone
	}

	if l.Desired == DesiredInstall {
		switch l.State {
		case StateNotInstalled, StatePartial:
			return ActionInstall
		case StateUpgrade:
			return ActionUpgrade
		default:
			return ActionNone
		}
	}

	if l.State == StateNotInstalled {
		return ActionNone
	}
	return ActionRemove
}

// Toggle advances the selection on spacebar and returns the NEW desired,
// mirroring bash toggle_desired: upgrade rows cycle install -> hold -> remove
// -> install; all other rows flip install <-> remove.
func (l Lifecycle) Toggle() Desired {
	if l.State == StateUpgrade {
		switch l.Desired {
		case DesiredInstall:
			return DesiredHold
		case DesiredHold:
			return DesiredRemove
		default:
			return DesiredInstall
		}
	}

	if l.Desired == DesiredInstall {
		return DesiredRemove
	}
	return DesiredInstall
}

// Status is the display-independent status of a row. The engine decides WHICH
// status applies; the tui decides its colour. Label carries the exact bash
// strings incl. the "~"/"⬆" glyphs.
type Status string

const (
	StatusNone             Status = ""
	StatusInstalled        Status = "installed"
	StatusNotInstalled     Status = "not installed"
	StatusWillBeRemoved    Status = "will be removed"
	StatusWillBeUpdated    Status = "will be updated"
	StatusUpgradeAvailable Status = "⬆ upgrade available"
	StatusPartial          Status = "~ partial"
	StatusSkipped          Status = "skipped (claude not in targets)"
)

// Label is the plain (uncoloured) status text. The engine deliberately does
// not learn about ANSI: the tui maps Status → colour locally.
func (s Status) Label() string { return string(s) }

// Status decides which display-independent status applies to the row,
// mirroring the branch structure of the bash state_label (and the Go
// tui.stateLabel it was ported to) minus the colour.
func (l Lifecycle) Status() Status {
	if l.Desired == DesiredRemove {
		switch l.State {
		case StateInstalled, StatePartial, StateUpgrade:
			return StatusWillBeRemoved
		}
	}

	switch l.State {
	case StateInstalled:
		return StatusInstalled
	case StateNotInstalled:
		return StatusNotInstalled
	case StateUpgrade:
		if l.Desired == DesiredInstall {
			return StatusWillBeUpdated
		}
		return StatusUpgradeAvailable
	case StatePartial:
		return StatusPartial
	case StateSkipped:
		return StatusSkipped
	}
	return StatusNone
}
