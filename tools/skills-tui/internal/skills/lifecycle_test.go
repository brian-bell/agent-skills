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

// TestLifecycleToggle ports TestToggleDesired onto the engine API: upgrade rows
// cycle install -> hold -> remove -> install; all other rows flip install <-> remove.
func TestLifecycleToggle(t *testing.T) {
	if got := (Lifecycle{StateUpgrade, DesiredInstall}).Toggle(); got != DesiredHold {
		t.Fatalf("space toggle should hold an upgradeable skill, got %v", got)
	}
	if got := (Lifecycle{StateUpgrade, DesiredHold}).Action(); got != ActionNone {
		t.Fatal("held upgrade should not apply any action")
	}
	if got := (Lifecycle{StateUpgrade, DesiredHold}).Toggle(); got != DesiredRemove {
		t.Fatalf("held upgrade should toggle to remove, got %v", got)
	}
	if got := (Lifecycle{StateUpgrade, DesiredRemove}).Toggle(); got != DesiredInstall {
		t.Fatalf("removed upgrade should toggle back to install, got %v", got)
	}
	if got := (Lifecycle{StateInstalled, DesiredInstall}).Toggle(); got != DesiredRemove {
		t.Fatalf("installed row should flip to remove, got %v", got)
	}
	if got := (Lifecycle{StateNotInstalled, DesiredRemove}).Toggle(); got != DesiredInstall {
		t.Fatalf("not-installed row should flip to install, got %v", got)
	}
}
