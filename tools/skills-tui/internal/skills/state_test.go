package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func assertSkillState(t *testing.T, cfg Config, skill Skill, want State) {
	t.Helper()
	if got := cfg.SkillState(skill); got != want {
		t.Fatalf("expected state '%s', got '%s'", want, got)
	}
}

// Port of test_state_not_installed.
func TestStateNotInstalled(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)

	assertSkillState(t, cfg, Skill{Kind: KindFirst, Name: "commit", Source: filepath.Join(repo, "skills/commit")}, StateNotInstalled)
}

// Port of test_state_installed_when_linked.
func TestStateInstalledWhenLinked(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	skill := Skill{Kind: KindFirst, Name: "commit", Source: filepath.Join(repo, "skills/commit")}

	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}
	assertSkillState(t, cfg, skill, StateInstalled)
}

// Port of test_state_upgrade_when_copy_differs.
func TestStateUpgradeWhenCopyDiffers(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	writeFile(t, filepath.Join(src, "SKILL.md"), "v2\n")

	for _, root := range []string{".agents", ".claude", ".cursor"} {
		writeFile(t, filepath.Join(cfg.Home, root, "skills/commit/SKILL.md"), "v1\n")
	}

	assertSkillState(t, cfg, Skill{Kind: KindFirst, Name: "commit", Source: src}, StateUpgrade)
}

// Port of test_state_installed_when_copy_identical.
func TestStateInstalledWhenCopyIdentical(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	writeFile(t, filepath.Join(src, "SKILL.md"), "same\n")

	for _, root := range []string{".agents", ".claude", ".cursor"} {
		writeFile(t, filepath.Join(cfg.Home, root, "skills/commit/SKILL.md"), "same\n")
	}

	assertSkillState(t, cfg, Skill{Kind: KindFirst, Name: "commit", Source: src}, StateInstalled)
}

// Port of test_state_partial_when_one_root_missing.
func TestStatePartialWhenOneRootMissing(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	skill := Skill{Kind: KindFirst, Name: "commit", Source: filepath.Join(repo, "skills/commit")}

	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(cfg.Home, ".agents/skills/commit")); err != nil {
		t.Fatal(err)
	}

	assertSkillState(t, cfg, skill, StatePartial)
}

// Port of test_existing_repo_symlinks_migrate_to_staged_symlinks (state half):
// legacy repo-pointing symlinks read as foreign, so the skill is upgradeable.
// The apply half lives in TestExistingRepoSymlinksMigrateToStagedSymlinks.
func TestStateUpgradeForLegacyRepoSymlinks(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")

	for _, root := range []string{".agents", ".claude", ".cursor"} {
		link := filepath.Join(cfg.Home, root, "skills/commit")
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(src, link); err != nil {
			t.Fatal(err)
		}
	}

	assertSkillState(t, cfg, Skill{Kind: KindFirst, Name: "commit", Source: src}, StateUpgrade)
}

// Port of test_chmod_only_repo_update_marks_staged_copy_upgrade (state half).
func TestChmodOnlyRepoUpdateMarksStagedCopyUpgrade(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")
	skill := Skill{Kind: KindFirst, Name: "commit", Source: src}

	writeFile(t, filepath.Join(src, "helper.sh"), "echo helper\n")
	if err := os.Chmod(filepath.Join(src, "helper.sh"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(filepath.Join(src, "helper.sh"), 0o755); err != nil {
		t.Fatal(err)
	}

	assertSkillState(t, cfg, skill, StateUpgrade)
	res := cfg.ApplySkill(skill, DesiredInstall, false)
	if res.Outcome != OutcomeUpgraded {
		t.Fatalf("expected upgraded outcome, got %+v", res)
	}
	info, err := os.Stat(filepath.Join(staged, "helper.sh"))
	if err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Fatal("upgrade did not refresh helper executable bit")
	}
}

// Port of test_staged_root_permission_drift_marks_upgrade.
func TestStagedRootPermissionDriftMarksUpgrade(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")
	skill := Skill{Kind: KindFirst, Name: "commit", Source: src}

	if err := os.Chmod(src, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		t.Fatal(err)
	}

	assertSkillState(t, cfg, skill, StateUpgrade)
}

// Team skills are skipped when none of their runtime roots is targeted.
func TestTeamStateSkippedWithoutClaudeTarget(t *testing.T) {
	cfg := stageConfig(t)
	cfg.Targets = []Target{"agents", "cursor"}
	repo := makeRepo(t)
	skill := Skill{Kind: KindTeam, Name: "go-review", Source: filepath.Join(repo, "agent-teams/go-review-team")}

	assertSkillState(t, cfg, skill, StateSkipped)
}

// Cursor-only installs cannot manage a cursor-less forked skill (no overlay
// for the selected target). Report skipped — not not-installed — so --all
// does not attempt an impossible install and print blocked: not-installed
// (Codex review on #75).
func TestStateSkippedForCursorLessSkillWhenCursorOnly(t *testing.T) {
	cfg := stageConfig(t)
	cfg.Targets = []Target{TargetCursor}
	repo := makeRepo(t)
	src := makeCursorLessForkedSkill(t, repo, "cursor-less")
	skill := Skill{Kind: KindFirst, Name: "cursor-less", Source: src, Forked: true}

	assertSkillState(t, cfg, skill, StateSkipped)
	if got := PlanAction(StateSkipped, DesiredInstall); got != ActionNone {
		t.Fatalf("skipped skill must plan none, got %s", got)
	}

	res := cfg.ApplySkill(skill, DesiredInstall, false)
	if res.Outcome != OutcomeNone {
		t.Fatalf("cursor-only apply must not install cursor-less skill, got %+v", res)
	}
	assertNotExists(t, filepath.Join(cfg.Home, ".cursor/skills/cursor-less"),
		"cursor-only install must not create a cursor link for a cursor-less skill")
}
