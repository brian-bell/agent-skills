package skills

import "testing"

// TestLifecycleAction ports TestPlanActionMatrix onto the Lifecycle engine API:
// the (State, Desired) pair decides what action to run.
func TestLifecycleAction(t *testing.T) {
	cases := []struct {
		state   State
		desired Desired
		want    Action
	}{
		{StateNotInstalled, DesiredInstall, ActionInstall},
		{StateUpgrade, DesiredInstall, ActionUpgrade},
		{StateUpgrade, DesiredHold, ActionNone},
		{StatePartial, DesiredInstall, ActionInstall},
		{StateInstalled, DesiredInstall, ActionNone},
		{StateInstalled, DesiredRemove, ActionRemove},
		{StateUpgrade, DesiredRemove, ActionRemove},
		{StatePartial, DesiredRemove, ActionRemove},
		{StateNotInstalled, DesiredRemove, ActionNone},
		{StateSkipped, DesiredInstall, ActionNone},
		{StateSkipped, DesiredRemove, ActionNone},
	}
	for _, c := range cases {
		if got := (Lifecycle{c.state, c.desired}).Action(); got != c.want {
			t.Errorf("Lifecycle{%s,%v}.Action() = %s, want %s", c.state, c.desired, got, c.want)
		}
	}
}
