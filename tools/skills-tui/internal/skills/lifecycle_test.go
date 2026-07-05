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

// TestLifecycleStatus pins the display-independent status decision that was
// previously untested in isolation (only exercised transitively through the
// render golden frame). It is derived from the current stateLabel branches.
func TestLifecycleStatus(t *testing.T) {
	cases := []struct {
		state   State
		desired Desired
		want    Status
	}{
		// desired==Remove AND state installed/partial/upgrade -> will be removed
		{StateInstalled, DesiredRemove, StatusWillBeRemoved},
		{StatePartial, DesiredRemove, StatusWillBeRemoved},
		{StateUpgrade, DesiredRemove, StatusWillBeRemoved},
		// otherwise, by state:
		{StateInstalled, DesiredInstall, StatusInstalled},
		{StateNotInstalled, DesiredRemove, StatusNotInstalled},
		{StateNotInstalled, DesiredInstall, StatusNotInstalled},
		{StateUpgrade, DesiredInstall, StatusWillBeUpdated},
		{StateUpgrade, DesiredHold, StatusUpgradeAvailable},
		{StatePartial, DesiredInstall, StatusPartial},
		{StateSkipped, DesiredInstall, StatusSkipped},
		{StateSkipped, DesiredRemove, StatusSkipped},
	}
	for _, c := range cases {
		if got := (Lifecycle{c.state, c.desired}).Status(); got != c.want {
			t.Errorf("Lifecycle{%s,%v}.Status() = %q, want %q", c.state, c.desired, got, c.want)
		}
	}
}

// TestStatusLabel pins the exact bash strings (incl. the "~"/"⬆" glyphs) in the
// engine, not only in the render golden frame.
func TestStatusLabel(t *testing.T) {
	cases := []struct {
		status Status
		want   string
	}{
		{StatusNone, ""},
		{StatusInstalled, "installed"},
		{StatusNotInstalled, "not installed"},
		{StatusWillBeRemoved, "will be removed"},
		{StatusWillBeUpdated, "will be updated"},
		{StatusUpgradeAvailable, "⬆ upgrade available"},
		{StatusPartial, "~ partial"},
		{StatusSkipped, "skipped (claude not in targets)"},
	}
	for _, c := range cases {
		if got := c.status.Label(); got != c.want {
			t.Errorf("%v.Label() = %q, want %q", c.status, got, c.want)
		}
	}
}

// TestDefaultDesired pins the selection seeded for a freshly observed state,
// mirroring the switch in Model.RefreshStates: installed/partial/upgrade
// default selected; everything else deselected.
func TestDefaultDesired(t *testing.T) {
	cases := []struct {
		s    State
		want Desired
	}{
		{StateInstalled, DesiredInstall},
		{StatePartial, DesiredInstall},
		{StateUpgrade, DesiredInstall},
		{StateNotInstalled, DesiredRemove},
		{StateSkipped, DesiredRemove},
	}
	for _, c := range cases {
		if got := DefaultDesired(c.s); got != c.want {
			t.Errorf("DefaultDesired(%s) = %v, want %v", c.s, got, c.want)
		}
	}
}
