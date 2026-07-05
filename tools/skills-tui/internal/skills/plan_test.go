package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Port of test_plan_action_matrix.
func TestPlanActionMatrix(t *testing.T) {
	cases := []struct {
		current State
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
		if got := PlanAction(c.current, c.desired); got != c.want {
			t.Errorf("PlanAction(%s, %v) = %s, want %s", c.current, c.desired, got, c.want)
		}
	}
}

// Port of the toggle half of test_upgrade_rows_default_to_update_and_can_be_held:
// upgrade rows cycle 1 -> hold -> 0 -> 1; all other rows flip 1 <-> 0.
func TestToggleDesired(t *testing.T) {
	if got := ToggleDesired(StateUpgrade, DesiredInstall); got != DesiredHold {
		t.Fatalf("space toggle should hold an upgradeable skill, got %v", got)
	}
	if got := PlanAction(StateUpgrade, DesiredHold); got != ActionNone {
		t.Fatal("held upgrade should not apply any action")
	}
	if got := ToggleDesired(StateUpgrade, DesiredHold); got != DesiredRemove {
		t.Fatalf("held upgrade should toggle to remove, got %v", got)
	}
	if got := ToggleDesired(StateUpgrade, DesiredRemove); got != DesiredInstall {
		t.Fatalf("removed upgrade should toggle back to install, got %v", got)
	}
	if got := ToggleDesired(StateInstalled, DesiredInstall); got != DesiredRemove {
		t.Fatalf("installed row should flip to remove, got %v", got)
	}
	if got := ToggleDesired(StateNotInstalled, DesiredRemove); got != DesiredInstall {
		t.Fatalf("not-installed row should flip to install, got %v", got)
	}
}

// Port of test_cursor_only_install_all_skips_team_skills: applying everything
// with only cursor targeted must not report team skills as blocked.
func TestCursorOnlyInstallAllSkipsTeamSkills(t *testing.T) {
	cfg := stageConfig(t)
	cfg.Targets = []Target{"cursor"}
	repo := makeRepo(t)

	all, err := Discover(repo)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, s := range all {
		res := cfg.ApplySkill(s, DesiredInstall, false)
		if line := res.StatusLine(); line != "" {
			lines = append(lines, line)
		}
	}

	out := strings.Join(lines, "\n")
	if strings.Contains(out, "blocked: not-installed") {
		t.Fatalf("cursor-only --all must not block on team skills: %s", out)
	}
	if strings.Contains(out, "blocked: skipped") {
		t.Fatalf("cursor-only --all must not block on skipped team skills: %s", out)
	}
}

// Port of test_apply_upgrade_keeps_real_dir_without_force (C2): an
// interactive apply (destroy=false) must never rm -rf a real directory.
func TestApplyUpgradeKeepsRealDirWithoutForce(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	skill := Skill{KindFirst, "commit", src}
	writeFile(t, filepath.Join(src, "SKILL.md"), "v2\n")

	writeFile(t, filepath.Join(cfg.Home, ".agents/skills/commit/SKILL.md"), "v1\n")
	writeFile(t, filepath.Join(cfg.Home, ".claude/skills/commit/NOTES.md"), "private\n")

	assertSkillState(t, cfg, skill, StateUpgrade)
	// desired=install, destroy=false (interactive apply): preserve the real dir.
	cfg.ApplySkill(skill, DesiredInstall, false)

	if _, err := os.Stat(filepath.Join(cfg.Home, ".claude/skills/commit/NOTES.md")); err != nil {
		t.Fatal("interactive apply destroyed a real user directory (data loss)")
	}

	// With destroy=true (--force) it relinks.
	cfg.ApplySkill(skill, DesiredInstall, true)
	assertSkillState(t, cfg, skill, StateInstalled)
}

// Port of test_foreign_symlink_upgrade_is_nondestructive (I1).
func TestForeignSymlinkUpgradeIsNondestructive(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	elsewhere := t.TempDir()
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")
	skill := Skill{KindFirst, "commit", src}
	writeFile(t, filepath.Join(elsewhere, "data.txt"), "keep\n")

	for _, root := range []string{".agents", ".claude", ".cursor"} {
		link := filepath.Join(cfg.Home, root, "skills/commit")
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(elsewhere, link); err != nil {
			t.Fatal(err)
		}
	}

	assertSkillState(t, cfg, skill, StateUpgrade)
	// Interactive apply (destroy=false) may relink a symlink (non-destructive).
	cfg.ApplySkill(skill, DesiredInstall, false)

	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/skills/commit"), staged)
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".cursor/skills/commit"), staged)
	if _, err := os.Stat(filepath.Join(elsewhere, "data.txt")); err != nil {
		t.Fatal("relinking a foreign symlink destroyed its data")
	}
}

// Port of test_apply_partial_links_missing_keeps_real_dir: a real matching
// dir on one root, missing on another. Apply links the missing root but never
// destroys the real dir.
func TestApplyPartialLinksMissingKeepsRealDir(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")
	skill := Skill{KindFirst, "commit", src}
	writeFile(t, filepath.Join(src, "SKILL.md"), "same\n")

	// claude root: real dir with matching content + a private file; agents: missing.
	writeFile(t, filepath.Join(cfg.Home, ".claude/skills/commit/SKILL.md"), "same\n")
	writeFile(t, filepath.Join(cfg.Home, ".claude/skills/commit/NOTES.md"), "private\n")

	cfg.ApplySkill(skill, DesiredInstall, false)

	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".agents/skills/commit"), staged)
	if _, err := os.Stat(filepath.Join(cfg.Home, ".claude/skills/commit/NOTES.md")); err != nil {
		t.Fatal("partial install destroyed the real dir on the other root")
	}
	if info, err := os.Lstat(filepath.Join(cfg.Home, ".claude/skills/commit")); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("partial install overwrote a real dir without --force")
	}
}

// Port of test_existing_repo_symlinks_migrate_to_staged_symlinks.
func TestExistingRepoSymlinksMigrateToStagedSymlinks(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")
	skill := Skill{KindFirst, "commit", src}

	for _, root := range []string{".agents", ".claude", ".cursor"} {
		link := filepath.Join(cfg.Home, root, "skills/commit")
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(src, link); err != nil {
			t.Fatal(err)
		}
	}

	assertSkillState(t, cfg, skill, StateUpgrade)
	cfg.ApplySkill(skill, DesiredInstall, false)

	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".agents/skills/commit"), staged)
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/skills/commit"), staged)
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".cursor/skills/commit"), staged)
	if _, err := os.Stat(filepath.Join(staged, "SKILL.md")); err != nil {
		t.Fatal("migration did not create staged copy")
	}
}

// Port of test_apply_upgrade_refreshes_staged_copy.
func TestApplyUpgradeRefreshesStagedCopy(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")
	skill := Skill{KindFirst, "commit", src}

	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(src, "SKILL.md"), "updated skill\n")

	assertSkillState(t, cfg, skill, StateUpgrade)
	res := cfg.ApplySkill(skill, DesiredInstall, false)
	if want := "^ upgraded commit"; res.StatusLine() != want {
		t.Fatalf("expected status %q, got %q", want, res.StatusLine())
	}

	assertSkillState(t, cfg, skill, StateInstalled)
	data, err := os.ReadFile(filepath.Join(staged, "SKILL.md"))
	if err != nil || !strings.Contains(string(data), "updated skill") {
		t.Fatalf("upgrade did not refresh staged copy: %q, %v", data, err)
	}

	backupParent := filepath.Join(cfg.StageDir, "backups/skills/commit")
	entries, err := os.ReadDir(backupParent)
	if err != nil || len(entries) == 0 {
		t.Fatalf("upgrade did not create a staged skill backup: %v", err)
	}
	backup := filepath.Join(backupParent, entries[0].Name())
	data, err = os.ReadFile(filepath.Join(backup, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "commit skill") {
		t.Fatalf("backup did not preserve the previous staged skill, got %q", data)
	}
	if strings.Contains(string(data), "updated skill") {
		t.Fatal("backup contains upgraded content instead of previous staged content")
	}
}

// The status-line strings must match bash apply_skill byte for byte.
func TestApplyStatusLines(t *testing.T) {
	cases := []struct {
		res  ApplyResult
		want string
	}{
		{ApplyResult{Name: "commit", Action: ActionInstall, Outcome: OutcomeInstalled}, "+ installed commit"},
		{ApplyResult{Name: "commit", Action: ActionUpgrade, Outcome: OutcomeUpgraded}, "^ upgraded commit"},
		{ApplyResult{Name: "commit", Action: ActionRemove, Outcome: OutcomeRemoved}, "- removed commit"},
		{ApplyResult{Name: "commit", Action: ActionInstall, Outcome: OutcomePartial}, "~ commit partially applied (some targets need --force)"},
		{ApplyResult{Name: "commit", Action: ActionUpgrade, Outcome: OutcomeBlocked, State: StateUpgrade}, "! commit blocked: upgrade (use --force to overwrite)"},
		{ApplyResult{Name: "commit", Action: ActionNone}, ""},
	}
	for _, c := range cases {
		if got := c.res.StatusLine(); got != c.want {
			t.Errorf("StatusLine(%+v) = %q, want %q", c.res, got, c.want)
		}
	}
}

// Removing an installed skill through ApplySkill reports "- removed <name>".
func TestApplyRemoveReportsRemoved(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	skill := Skill{KindFirst, "commit", filepath.Join(repo, "skills/commit")}

	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}
	res := cfg.ApplySkill(skill, DesiredRemove, false)

	if want := "- removed commit"; res.StatusLine() != want {
		t.Fatalf("expected status %q, got %q", want, res.StatusLine())
	}
	assertSkillState(t, cfg, skill, StateNotInstalled)
}
