package tui

import (
	"regexp"
	"strings"
	"testing"

	"agent-skills/tools/skills-tui/internal/skills"
)

// row builds a Model row for render tests without touching the filesystem.
func row(kind skills.Kind, name string, state skills.State, desired skills.Desired) Row {
	return Row{
		Skill:   skills.Skill{Kind: kind, Name: name},
		State:   state,
		Desired: desired,
	}
}

// Port of bash test_render_oversized_list_uses_viewport_without_full_clear.
func TestRenderOversizedListUsesViewportWithoutFullClear(t *testing.T) {
	names := []string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"}
	m := Model{Cursor: 6}
	for _, n := range names {
		m.Rows = append(m.Rows, row(skills.KindFirst, n, skills.StateNotInstalled, skills.DesiredRemove))
	}

	out := Render(m, 8)

	if strings.Contains(out, "\x1b[2J") {
		t.Fatal("oversized render should not full-clear on cursor movement")
	}
	if lines := strings.Count(out, "\n") + 1; lines > 8 {
		t.Fatalf("expected oversized render to fit in 8 rows, got %d", lines)
	}
	if !strings.Contains(out, "seven") {
		t.Fatal("selected item should stay visible")
	}
}

// Port of bash test_upgrade_rows_default_to_update_and_can_be_held.
func TestUpgradeRowsDefaultToUpdateAndCanBeHeld(t *testing.T) {
	m := Model{Rows: []Row{row(skills.KindFirst, "commit", skills.StateUpgrade, skills.DesiredInstall)}}

	out := Render(m, 8)
	if !regexp.MustCompile(`\[x\].*will be updated`).MatchString(out) {
		t.Fatal("selected upgrade should render as checked and will be updated")
	}

	m.Toggle()

	if got := m.Rows[0].Desired; got != skills.DesiredHold {
		t.Fatalf("space toggle should hold an upgradeable skill, got %v", got)
	}
	if got := skills.PlanAction(skills.StateUpgrade, m.Rows[0].Desired); got != skills.ActionNone {
		t.Fatalf("held upgrade should not apply any action, got %v", got)
	}

	out = Render(m, 8)
	if !regexp.MustCompile(`\[-\].*upgrade available`).MatchString(out) {
		t.Fatal("held upgrade should render with '-' and upgrade available")
	}
}

// Port of bash test_installed_rows_selected_for_uninstall_show_removed_label.
func TestInstalledRowsSelectedForUninstallShowRemovedLabel(t *testing.T) {
	m := Model{Rows: []Row{row(skills.KindFirst, "commit", skills.StateInstalled, skills.DesiredRemove)}}

	out := Render(m, 8)

	if !regexp.MustCompile(`\[ \].*will be removed`).MatchString(out) {
		t.Fatal("installed skill selected for uninstall should render as will be removed")
	}
}

// Full-frame golden test: two kinds, three skills, cursor on the first row.
// Asserts the exact frame bytes bash render() would emit, ANSI codes and all.
func TestRenderGoldenFrame(t *testing.T) {
	m := Model{
		Rows: []Row{
			row(skills.KindFirst, "commit", skills.StateInstalled, skills.DesiredInstall),
			row(skills.KindFirst, "tdd", skills.StateNotInstalled, skills.DesiredRemove),
			row(skills.KindThird, "grill-me", skills.StateUpgrade, skills.DesiredInstall),
		},
		Cursor: 0,
	}

	pad := func(name string) string { return name + strings.Repeat(" ", 32-len(name)) }
	want := "\x1b[H" +
		"\x1b[1m  agent-skills · manage skills\x1b[0m\x1b[K\n" +
		"\x1b[2m  ↑↓ move · space toggle · a all · n none · enter apply · q quit\x1b[0m\x1b[K\n" +
		"  \x1b[1mfirst-party\x1b[0m\x1b[K\n" +
		"  \x1b[1m>\x1b[0m [x] " + pad("commit") + " \x1b[32minstalled\x1b[0m\x1b[K\n" +
		"    [ ] " + pad("tdd") + " \x1b[2mnot installed\x1b[0m\x1b[K\n" +
		"  \x1b[1mthird-party\x1b[0m\x1b[K\n" +
		"    [x] " + pad("grill-me") + " \x1b[33mwill be updated\x1b[0m\x1b[K" +
		"\x1b[J"

	got := Render(m, 24)
	if got != want {
		t.Fatalf("golden frame mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

// The message block renders as a blank line plus the indented message before
// the ESC[J tail, exactly like bash.
func TestRenderMessageBlock(t *testing.T) {
	m := Model{
		Rows:    []Row{row(skills.KindFirst, "commit", skills.StateInstalled, skills.DesiredInstall)},
		Message: "hello",
	}

	out := Render(m, 24)

	wantTail := "\x1b[K\n  hello\x1b[K\x1b[J"
	if !strings.HasSuffix(out, wantTail) {
		t.Fatalf("message tail mismatch\ngot:  %q\nwant suffix: %q", out, wantTail)
	}
}

func TestRenderSkippedAndPartialLabels(t *testing.T) {
	m := Model{
		Rows: []Row{
			row(skills.KindTeam, "go-review", skills.StateSkipped, skills.DesiredRemove),
			row(skills.KindFirst, "commit", skills.StatePartial, skills.DesiredInstall),
		},
	}

	out := Render(m, 24)

	if !strings.Contains(out, "\x1b[2mskipped (claude not in targets)\x1b[0m") {
		t.Fatal("skipped team should render dim skipped label")
	}
	if !strings.Contains(out, "\x1b[36m~ partial\x1b[0m") {
		t.Fatal("partial skill should render cyan ~ partial label")
	}
	if !strings.Contains(out, "\x1b[1magent-teams (claude only)\x1b[0m") {
		t.Fatal("team kind header missing")
	}
}
