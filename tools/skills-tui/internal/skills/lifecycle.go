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
