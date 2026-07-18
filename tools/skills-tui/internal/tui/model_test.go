package tui

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-skills/tools/skills-tui/internal/skills"
)

// testConfig builds a Config over throwaway repo/home trees like the bash
// suite's make_repo + fake HOME.
func testConfig(t *testing.T) skills.Config {
	t.Helper()
	repo := t.TempDir()
	home := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("skills/commit/SKILL.md", "commit skill\n")
	write("skills/tdd/SKILL.md", "tdd skill\n")
	return skills.Config{
		RepoDir:  repo,
		Home:     home,
		StageDir: filepath.Join(home, ".skill-symlinks"),
		Targets:  skills.NormalizeTargets("", io.Discard),
		WarnW:    io.Discard,
		Now:      time.Now,
	}
}

// Port of bash test_refresh_states_selects_upgrades_by_default.
func TestRefreshStatesSelectsUpgradesByDefault(t *testing.T) {
	cfg := testConfig(t)
	src := filepath.Join(cfg.RepoDir, "skills", "commit")
	s := skills.Skill{Kind: skills.KindFirst, Name: "commit", Source: src}

	if err := cfg.InstallSkill(s, false, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("updated skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &Model{Rows: []Row{{Skill: s, Desired: skills.DesiredRemove}}}
	m.RefreshStates(cfg)

	if got := m.Rows[0].State; got != skills.StateUpgrade {
		t.Fatalf("expected refresh to mark changed staged copy as upgrade, got %v", got)
	}
	if got := m.Rows[0].Desired; got != skills.DesiredInstall {
		t.Fatalf("upgrade should be selected by default, got %v", got)
	}
}

// LoadSkills seeds desired from state: installed rows selected, missing not.
func TestLoadSkillsSeedsDesiredFromState(t *testing.T) {
	cfg := testConfig(t)
	src := filepath.Join(cfg.RepoDir, "skills", "commit")
	if err := cfg.InstallSkill(skills.Skill{Kind: skills.KindFirst, Name: "commit", Source: src}, false, false); err != nil {
		t.Fatal(err)
	}

	m, err := LoadSkills(cfg)
	if err != nil {
		t.Fatal(err)
	}

	byName := map[string]Row{}
	for _, r := range m.Rows {
		byName[r.Skill.Name] = r
	}
	if r := byName["commit"]; r.State != skills.StateInstalled || r.Desired != skills.DesiredInstall {
		t.Fatalf("commit should load installed+selected, got %v/%v", r.State, r.Desired)
	}
	if r := byName["tdd"]; r.State != skills.StateNotInstalled || r.Desired != skills.DesiredRemove {
		t.Fatalf("tdd should load not-installed+deselected, got %v/%v", r.State, r.Desired)
	}
}

func TestCursorWrapsAndBulkSelect(t *testing.T) {
	m := Model{Rows: []Row{
		row(skills.KindFirst, "commit", skills.StateNotInstalled, skills.DesiredRemove),
		row(skills.KindFirst, "tdd", skills.StateNotInstalled, skills.DesiredRemove),
	}}

	m.MoveUp()
	if m.Cursor != 1 {
		t.Fatalf("MoveUp from 0 should wrap to 1, got %d", m.Cursor)
	}
	m.MoveDown()
	if m.Cursor != 0 {
		t.Fatalf("MoveDown from 1 should wrap to 0, got %d", m.Cursor)
	}

	m.SelectAll()
	for i, r := range m.Rows {
		if r.Desired != skills.DesiredInstall {
			t.Fatalf("SelectAll left row %d at %v", i, r.Desired)
		}
	}
	m.SelectNone()
	for i, r := range m.Rows {
		if r.Desired != skills.DesiredRemove {
			t.Fatalf("SelectNone left row %d at %v", i, r.Desired)
		}
	}
}

// The event loop navigates with j/k and arrows and quits on q without
// touching the filesystem.
func TestRunLoopNavigatesAndQuits(t *testing.T) {
	cfg := testConfig(t)
	m, err := LoadSkills(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	kr := NewKeyReader(bytes.NewReader([]byte("j\x1b[Aq")))
	runLoop(cfg, m, kr, &out, 24)

	if frames := strings.Count(out.String(), "\x1b[H"); frames < 3 {
		t.Fatalf("expected at least 3 rendered frames, got %d", frames)
	}
	if m.Cursor != 0 {
		t.Fatalf("j then up-arrow should land back on row 0, got %d", m.Cursor)
	}
}

// Enter applies pending changes (here: none), prints the bash status block,
// and waits for a key; q at the prompt quits.
func TestRunLoopEnterAppliesNothingToDo(t *testing.T) {
	cfg := testConfig(t)
	m, err := LoadSkills(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	kr := NewKeyReader(bytes.NewReader([]byte("\rq")))
	runLoop(cfg, m, kr, &out, 24)

	got := out.String()
	for _, want := range []string{"\x1b[2J\x1b[H\n", "  Applying…\n", "  nothing to do\n", "  Done. Press any key to continue, q to quit.\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("apply output missing %q in %q", want, got)
		}
	}
}

// Enter with a selected skill installs it and prints the status line.
func TestRunLoopEnterInstallsSelected(t *testing.T) {
	cfg := testConfig(t)
	m, err := LoadSkills(cfg)
	if err != nil {
		t.Fatal(err)
	}
	m.SelectAll()

	var out bytes.Buffer
	kr := NewKeyReader(bytes.NewReader([]byte("\rq")))
	runLoop(cfg, m, kr, &out, 24)

	if !strings.Contains(out.String(), "  + installed commit\n") {
		t.Fatalf("expected install status line, got %q", out.String())
	}
	if _, err := os.Readlink(filepath.Join(cfg.Home, ".claude/skills/commit")); err != nil {
		t.Fatalf("commit not linked after apply: %v", err)
	}
	if got := m.Rows[0].State; got != skills.StateInstalled {
		t.Fatalf("apply should refresh states, got %v", got)
	}
}

func TestReloadAfterImportPreservesDesiredStateAndSelectsNewRowsWithoutInstalling(t *testing.T) {
	cfg := testConfig(t)
	writeThirdPartySkill := func(name string) {
		t.Helper()
		path := filepath.Join(cfg.RepoDir, "third-party", name, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("---\nname: "+name+"\ndescription: test\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeThirdPartySkill("existing")
	model, err := LoadSkills(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for i := range model.Rows {
		switch model.Rows[i].Skill.Name {
		case "commit":
			model.Rows[i].Desired = skills.DesiredRemove
		case "tdd":
			model.Rows[i].Desired = skills.DesiredInstall
		case "existing":
			model.Rows[i].Desired = skills.DesiredHold
		}
	}
	writeThirdPartySkill("alpha")
	writeThirdPartySkill("zeta")

	if err := model.ReloadAfterImport(cfg, []string{"alpha", "zeta"}); err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]Row, len(model.Rows))
	indices := make(map[string]int, len(model.Rows))
	for i, row := range model.Rows {
		byName[row.Skill.Name] = row
		indices[row.Skill.Name] = i
	}
	for name, want := range map[string]skills.Desired{
		"commit": skills.DesiredRemove, "tdd": skills.DesiredInstall, "existing": skills.DesiredHold,
		"alpha": skills.DesiredInstall, "zeta": skills.DesiredInstall,
	} {
		if got := byName[name].Desired; got != want {
			t.Fatalf("%s desired state = %v, want %v", name, got, want)
		}
	}
	if model.Cursor != indices["alpha"] {
		t.Fatalf("cursor should move to first imported row alpha, got %d want %d", model.Cursor, indices["alpha"])
	}
	for _, name := range []string{"alpha", "zeta"} {
		if byName[name].State != skills.StateNotInstalled {
			t.Fatalf("%s should be rediscovered but not installed, state=%v", name, byName[name].State)
		}
		if _, err := os.Lstat(filepath.Join(cfg.StageDir, "skills", name)); !os.IsNotExist(err) {
			t.Fatalf("%s was staged before Apply, stat error: %v", name, err)
		}
		if _, err := os.Lstat(filepath.Join(cfg.Home, ".agents", "skills", name)); !os.IsNotExist(err) {
			t.Fatalf("%s was linked before Apply, stat error: %v", name, err)
		}
	}
	var applyOutput bytes.Buffer
	model.ApplyChanges(cfg, &applyOutput)
	for _, name := range []string{"alpha", "zeta"} {
		if _, err := os.Stat(filepath.Join(cfg.StageDir, "skills", name)); err != nil {
			t.Fatalf("%s was not staged by Apply: %v", name, err)
		}
		if _, err := os.Readlink(filepath.Join(cfg.Home, ".agents", "skills", name)); err != nil {
			t.Fatalf("%s was not linked by Apply: %v", name, err)
		}
	}
}
