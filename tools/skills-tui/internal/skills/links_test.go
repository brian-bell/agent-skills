package skills

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func assertSymlinkTarget(t *testing.T, path, target string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected %s to be a symlink: %v", path, err)
	}
	got, err := os.Readlink(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != target {
		t.Fatalf("expected %s -> %s, got %s", path, target, got)
	}
}

func assertNotExists(t *testing.T, path, msg string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("%s: %s exists (%v)", msg, path, err)
	}
}

// Port of test_install_first_party_links_all_roots.
func TestInstallFirstPartyLinksAllRoots(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")

	if err := cfg.InstallSkill(Skill{Kind: KindFirst, Name: "commit", Source: src}, false, false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(staged, "SKILL.md")); err != nil {
		t.Fatalf("expected staged skill copy at %s: %v", staged, err)
	}
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".agents/skills/commit"), staged)
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/skills/commit"), staged)
}

func TestInstallForkedFirstPartyAssemblesRuntimeStagedTrees(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := makeForkedSkill(t, repo, "runtime-demo")
	skill := Skill{Kind: KindFirst, Name: "runtime-demo", Source: src, Forked: true}

	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}

	codexStaged := filepath.Join(cfg.StageDir, "runtimes/codex/skills/runtime-demo")
	claudeStaged := filepath.Join(cfg.StageDir, "runtimes/claude/skills/runtime-demo")

	for runtime, staged := range map[string]string{
		"codex":  codexStaged,
		"claude": claudeStaged,
	} {
		data, err := os.ReadFile(filepath.Join(staged, "SKILL.md"))
		if err != nil {
			t.Fatalf("%s staged SKILL.md missing: %v", runtime, err)
		}
		if string(data) != runtime+" skill\n" {
			t.Fatalf("%s staged SKILL.md = %q", runtime, data)
		}
		if _, err := os.Stat(filepath.Join(staged, "scripts/helper.sh")); err != nil {
			t.Fatalf("%s staged shared script missing: %v", runtime, err)
		}
	}

	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".agents/skills/runtime-demo"), codexStaged)
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/skills/runtime-demo"), claudeStaged)
}

func TestInstallForkedFirstPartyRepointsLegacyStagedSymlinkWithoutForce(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := makeForkedSkill(t, repo, "runtime-demo")
	skill := Skill{Kind: KindFirst, Name: "runtime-demo", Source: src, Forked: true}
	legacyStaged := cfg.LegacyStagedPath("runtime-demo")
	claudeTarget := filepath.Join(cfg.Home, ".claude/skills/runtime-demo")
	claudeStaged := filepath.Join(cfg.StageDir, "runtimes/claude/skills/runtime-demo")

	writeFile(t, filepath.Join(legacyStaged, "SKILL.md"), "legacy staged\n")
	if err := os.MkdirAll(filepath.Dir(claudeTarget), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(legacyStaged, claudeTarget); err != nil {
		t.Fatal(err)
	}

	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}

	assertSymlinkTarget(t, claudeTarget, claudeStaged)
	if _, err := os.Stat(filepath.Join(legacyStaged, "SKILL.md")); err != nil {
		t.Fatal("legacy staged directory should remain for explicit future cleanup")
	}
}

func TestForceInstallForkedFirstPartySyncsMatchingRealCopyBeforeRelink(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := makeForkedSkill(t, repo, "runtime-demo")
	skill := Skill{Kind: KindFirst, Name: "runtime-demo", Source: src, Forked: true}
	claudeTarget := filepath.Join(cfg.Home, ".claude/skills/runtime-demo")
	claudeStaged := filepath.Join(cfg.StageDir, "runtimes/claude/skills/runtime-demo")

	writeFile(t, filepath.Join(claudeTarget, "SKILL.md"), "claude skill\n")
	writeFile(t, filepath.Join(claudeTarget, "scripts/helper.sh"), "echo shared\n")

	if err := cfg.InstallSkill(skill, true, true); err != nil {
		t.Fatal(err)
	}

	assertSymlinkTarget(t, claudeTarget, claudeStaged)
	data, err := os.ReadFile(filepath.Join(claudeStaged, "SKILL.md"))
	if err != nil || string(data) != "claude skill\n" {
		t.Fatalf("force install should sync matching real copy before relink, got %q, %v", data, err)
	}
}

func TestUninstallForkedFirstPartyRemovesLegacyStagedSymlink(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := makeForkedSkill(t, repo, "runtime-demo")
	skill := Skill{Kind: KindFirst, Name: "runtime-demo", Source: src, Forked: true}
	legacyStaged := cfg.LegacyStagedPath("runtime-demo")
	claudeTarget := filepath.Join(cfg.Home, ".claude/skills/runtime-demo")

	writeFile(t, filepath.Join(legacyStaged, "SKILL.md"), "legacy staged\n")
	if err := os.MkdirAll(filepath.Dir(claudeTarget), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(legacyStaged, claudeTarget); err != nil {
		t.Fatal(err)
	}

	if err := cfg.UninstallSkill(skill); err != nil {
		t.Fatal(err)
	}

	assertNotExists(t, claudeTarget, "uninstall should remove owned legacy staged symlink")
	if _, err := os.Stat(filepath.Join(legacyStaged, "SKILL.md")); err != nil {
		t.Fatal("uninstall should leave legacy staged directory contents alone")
	}
}

// Port of test_uninstall_removes_owned_links.
func TestUninstallRemovesOwnedLinks(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	skill := Skill{Kind: KindFirst, Name: "commit", Source: src}

	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}
	if err := cfg.UninstallSkill(skill); err != nil {
		t.Fatal(err)
	}

	for _, root := range []string{".agents", ".claude"} {
		assertNotExists(t, filepath.Join(cfg.Home, root, "skills/commit"),
			"expected commit link removed from "+root)
	}
}

// Port of test_uninstall_leaves_real_dir_untouched.
func TestUninstallLeavesRealDirUntouched(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")

	writeFile(t, filepath.Join(cfg.Home, ".claude/skills/commit/local.txt"), "precious\n")
	if err := os.MkdirAll(filepath.Join(cfg.Home, ".agents/skills/commit"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg.UninstallSkill(Skill{Kind: KindFirst, Name: "commit", Source: src})

	if _, err := os.Stat(filepath.Join(cfg.Home, ".claude/skills/commit/local.txt")); err != nil {
		t.Fatal("uninstall must not delete a real directory")
	}
}

// Port of test_uninstall_leaves_foreign_symlink_untouched.
func TestUninstallLeavesForeignSymlinkUntouched(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	elsewhere := t.TempDir()
	src := filepath.Join(repo, "skills/commit")
	link := filepath.Join(cfg.Home, ".claude/skills/commit")

	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, link); err != nil {
		t.Fatal(err)
	}

	cfg.UninstallSkill(Skill{Kind: KindFirst, Name: "commit", Source: src})

	assertSymlinkTarget(t, link, elsewhere)
}

// Port of test_force_install_relinks_stale_copy.
func TestForceInstallRelinksStaleCopy(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")
	skill := Skill{Kind: KindFirst, Name: "commit", Source: src}

	for _, root := range []string{".agents", ".claude"} {
		writeFile(t, filepath.Join(cfg.Home, root, "skills/commit/SKILL.md"), "old\n")
	}

	// force + destroy required to overwrite a real directory.
	if err := cfg.InstallSkill(skill, true, true); err != nil {
		t.Fatal(err)
	}

	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".agents/skills/commit"), staged)
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/skills/commit"), staged)
	if got := cfg.SkillState(skill); got != StateInstalled {
		t.Fatalf("expected state installed, got %s", got)
	}
}

// Port of test_install_without_force_keeps_foreign_target.
func TestInstallWithoutForceKeepsForeignTarget(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")

	writeFile(t, filepath.Join(cfg.Home, ".claude/skills/commit/SKILL.md"), "mine\n")

	err := cfg.InstallSkill(Skill{Kind: KindFirst, Name: "commit", Source: src}, false, false)
	if err == nil {
		t.Fatal("install without force should report failure over a real dir")
	}
	var refused *RefusedRealPathError
	if !errors.As(err, &refused) {
		t.Fatalf("expected RefusedRealPathError, got: %v", err)
	}
	want := "Refusing to overwrite real path: " +
		filepath.Join(cfg.Home, ".claude/skills/commit") + " (use --force)"
	if refused.Error() != want {
		t.Fatalf("expected bash refusal message %q, got %q", want, refused.Error())
	}
	if _, err := os.Stat(filepath.Join(cfg.Home, ".claude/skills/commit/SKILL.md")); err != nil {
		t.Fatal("real dir clobbered without force")
	}
}

// LinkPath must refuse to replace a foreign symlink without force, using the
// exact bash refusal message.
func TestLinkPathRefusesForeignSymlinkWithoutForce(t *testing.T) {
	home := t.TempDir()
	elsewhere := t.TempDir()
	target := filepath.Join(home, "link")
	if err := os.Symlink(elsewhere, target); err != nil {
		t.Fatal(err)
	}

	err := LinkPath(filepath.Join(home, "src"), target, false, false)
	var refused *RefusedSymlinkError
	if !errors.As(err, &refused) {
		t.Fatalf("expected RefusedSymlinkError, got: %v", err)
	}
	want := "Refusing to replace existing symlink: " + target + " (use --force)"
	if refused.Error() != want {
		t.Fatalf("expected bash refusal message %q, got %q", want, refused.Error())
	}
	assertSymlinkTarget(t, target, elsewhere)
}

// Port of test_uninstall_last_skill_keeps_shared_roots (C1).
func TestUninstallLastSkillKeepsSharedRoots(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	skill := Skill{Kind: KindFirst, Name: "commit", Source: src}

	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}
	cfg.UninstallSkill(skill)

	for _, root := range []string{".claude", ".agents"} {
		if info, err := os.Stat(filepath.Join(cfg.Home, root, "skills")); err != nil || !info.IsDir() {
			t.Fatalf("uninstall removed shared ~/%s/skills root", root)
		}
	}
	assertNotExists(t, filepath.Join(cfg.Home, ".claude/skills/commit"), "commit link not removed")
}

// Port of test_uninstall_removes_existing_repo_symlinks: legacy symlinks that
// point at the repo source (not the staged copy) are still owned and removed.
func TestUninstallRemovesExistingRepoSymlinks(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	skill := Skill{Kind: KindFirst, Name: "commit", Source: src}

	for _, root := range []string{".agents", ".claude"} {
		link := filepath.Join(cfg.Home, root, "skills/commit")
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(src, link); err != nil {
			t.Fatal(err)
		}
	}

	if got := cfg.SkillState(skill); got != StateUpgrade {
		t.Fatalf("expected state upgrade, got %s", got)
	}
	cfg.UninstallSkill(skill)

	for _, root := range []string{".agents", ".claude"} {
		assertNotExists(t, filepath.Join(cfg.Home, root, "skills/commit"),
			"uninstall left legacy repo symlink in ~/"+root)
	}
}

// Port of test_installed_skill_survives_repo_source_removal.
func TestInstalledSkillSurvivesRepoSourceRemoval(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")

	if err := cfg.InstallSkill(Skill{Kind: KindFirst, Name: "commit", Source: src}, false, false); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(src); err != nil {
		t.Fatal(err)
	}

	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/skills/commit"), staged)
	if _, err := os.Stat(filepath.Join(cfg.Home, ".claude/skills/commit/SKILL.md")); err != nil {
		t.Fatal("installed skill should still resolve through staged copy")
	}
}

func TestUninstallForkedFirstPartyRemovesAgentsAndClaudeLinks(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := makeForkedSkill(t, repo, "runtime-demo")
	skill := Skill{Kind: KindFirst, Name: "runtime-demo", Source: src, Forked: true}

	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}
	if err := cfg.UninstallSkill(skill); err != nil {
		t.Fatal(err)
	}

	assertNotExists(t, filepath.Join(cfg.Home, ".agents/skills/runtime-demo"),
		"uninstall should remove agents link")
	assertNotExists(t, filepath.Join(cfg.Home, ".claude/skills/runtime-demo"),
		"uninstall should remove claude link")
}
