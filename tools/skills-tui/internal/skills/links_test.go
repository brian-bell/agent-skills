package skills

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".cursor/skills/commit"), staged)
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

// Port of test_install_respects_skill_install_targets_cursor_only.
func TestInstallRespectsTargetsCursorOnly(t *testing.T) {
	cfg := stageConfig(t)
	cfg.Targets = []Target{"cursor"}
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")

	if err := cfg.InstallSkill(Skill{Kind: KindFirst, Name: "commit", Source: src}, false, false); err != nil {
		t.Fatal(err)
	}

	assertNotExists(t, filepath.Join(cfg.Home, ".agents/skills/commit"),
		"cursor-only install must not link into ~/.agents")
	assertNotExists(t, filepath.Join(cfg.Home, ".claude/skills/commit"),
		"cursor-only install must not link into ~/.claude")
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".cursor/skills/commit"), staged)
}

// Port of test_install_respects_skill_install_targets_without_cursor.
func TestInstallRespectsTargetsWithoutCursor(t *testing.T) {
	cfg := stageConfig(t)
	cfg.Targets = []Target{"agents", "claude"}
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")

	if err := cfg.InstallSkill(Skill{Kind: KindFirst, Name: "commit", Source: src}, false, false); err != nil {
		t.Fatal(err)
	}

	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".agents/skills/commit"), staged)
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/skills/commit"), staged)
	assertNotExists(t, filepath.Join(cfg.Home, ".cursor/skills/commit"),
		"agents,claude install must not link into ~/.cursor")
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

	for _, root := range []string{".agents", ".claude", ".cursor"} {
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

	for _, root := range []string{".agents", ".claude", ".cursor"} {
		writeFile(t, filepath.Join(cfg.Home, root, "skills/commit/SKILL.md"), "old\n")
	}

	// force + destroy required to overwrite a real directory.
	if err := cfg.InstallSkill(skill, true, true); err != nil {
		t.Fatal(err)
	}

	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".agents/skills/commit"), staged)
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/skills/commit"), staged)
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".cursor/skills/commit"), staged)
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

	for _, root := range []string{".claude", ".agents", ".cursor"} {
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

	for _, root := range []string{".agents", ".claude", ".cursor"} {
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

	for _, root := range []string{".agents", ".claude", ".cursor"} {
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
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".cursor/skills/commit"), staged)
	if _, err := os.Stat(filepath.Join(cfg.Home, ".claude/skills/commit/SKILL.md")); err != nil {
		t.Fatal("installed skill should still resolve through staged copy")
	}
}

func makeCursorLessForkedSkill(t *testing.T, repo, name string) string {
	t.Helper()
	src := filepath.Join(repo, "skills", name)
	writeFile(t, filepath.Join(src, "shared/scripts/helper.sh"), "echo shared\n")
	writeFile(t, filepath.Join(src, "runtimes/claude/SKILL.md"), "claude skill\n")
	writeFile(t, filepath.Join(src, "runtimes/codex/SKILL.md"), "codex skill\n")
	return src
}

func TestSkillLinksOmitsMissingCursorOverlay(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := makeCursorLessForkedSkill(t, repo, "cursor-less")
	skill := Skill{Kind: KindFirst, Name: "cursor-less", Source: src, Forked: true}

	links := cfg.SkillLinks(skill)

	for _, l := range links {
		if strings.Contains(l.Target, ".cursor/") {
			t.Fatalf("cursor-less skill must not emit a cursor link, got %s", l.Target)
		}
	}
	if len(links) != 2 {
		t.Fatalf("expected agents+claude links only, got %d: %v", len(links), links)
	}
}

func TestUninstallForkedFirstPartyRemovesAgentsAndClaudeLinks(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := makeCursorLessForkedSkill(t, repo, "cursor-less")
	skill := Skill{Kind: KindFirst, Name: "cursor-less", Source: src, Forked: true}

	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}
	if err := cfg.UninstallSkill(skill); err != nil {
		t.Fatal(err)
	}

	assertNotExists(t, filepath.Join(cfg.Home, ".agents/skills/cursor-less"),
		"uninstall should remove agents link")
	assertNotExists(t, filepath.Join(cfg.Home, ".claude/skills/cursor-less"),
		"uninstall should remove claude link")
}

// seedLegacyTeamInstall recreates what a pre-as-77n install of a review team
// left on disk: a staged whole-team copy plus installer-owned agent symlinks
// pointing into it.
func seedLegacyTeamInstall(t *testing.T, cfg Config, teamdir string, agentFiles ...string) string {
	t.Helper()
	staged := filepath.Join(cfg.StageDir, "agent-teams", teamdir)
	agentsDir := filepath.Join(cfg.Home, ".claude/agents", teamdir)
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range agentFiles {
		writeFile(t, filepath.Join(staged, name), "legacy agent\n")
		if err := os.Symlink(filepath.Join(staged, name), filepath.Join(agentsDir, name)); err != nil {
			t.Fatal(err)
		}
	}
	return staged
}

// A migrated skill must clear its own pre-migration team install. Without
// this, the repo dir is gone, the team is never discovered, its uninstall path
// never runs, and the deleted lead stays registered and invocable.
func TestInstallPrunesLegacyTeamRegistrations(t *testing.T) {
	for skillName, teamdir := range legacyTeamDirs {
		t.Run(skillName, func(t *testing.T) {
			cfg := stageConfig(t)
			repo := t.TempDir()
			src := filepath.Join(repo, "skills", skillName)
			writeFile(t, filepath.Join(src, "shared/roles/a-reviewer.md"), "role\n")
			writeFile(t, filepath.Join(src, "runtimes/claude/SKILL.md"), "claude\n")
			writeFile(t, filepath.Join(src, "runtimes/codex/SKILL.md"), "codex\n")

			staged := seedLegacyTeamInstall(t, cfg, teamdir, "lead.md", "a-reviewer.md")
			agentsDir := filepath.Join(cfg.Home, ".claude/agents", teamdir)

			s := Skill{Kind: KindFirst, Name: skillName, Source: src, Forked: true}
			if err := cfg.InstallSkill(s, false, false); err != nil {
				t.Fatal(err)
			}

			assertNotExists(t, agentsDir, "legacy agent dir should be pruned")
			assertNotExists(t, staged, "legacy staged team copy should be pruned")
			// The migration must still install the skill itself.
			assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/skills", skillName),
				cfg.RuntimeStagedSource(skillName, RuntimeClaude))
		})
	}
}

// `--none` must clear the legacy install too, or uninstall reports success
// while the old agents stay registered.
func TestUninstallPrunesLegacyTeamRegistrations(t *testing.T) {
	cfg := stageConfig(t)
	repo := t.TempDir()
	src := filepath.Join(repo, "skills/go-review")
	writeFile(t, filepath.Join(src, "shared/roles/a-reviewer.md"), "role\n")
	writeFile(t, filepath.Join(src, "runtimes/claude/SKILL.md"), "claude\n")
	writeFile(t, filepath.Join(src, "runtimes/codex/SKILL.md"), "codex\n")

	staged := seedLegacyTeamInstall(t, cfg, "go-review-team", "review-lead.md")
	agentsDir := filepath.Join(cfg.Home, ".claude/agents/go-review-team")

	s := Skill{Kind: KindFirst, Name: "go-review", Source: src, Forked: true}
	if err := cfg.UninstallSkill(s); err != nil {
		t.Fatal(err)
	}
	assertNotExists(t, agentsDir, "uninstall should prune the legacy agent dir")
	assertNotExists(t, staged, "uninstall should prune the legacy staged team copy")
}

// The prune is ownership-checked: only symlinks into StageDir are ours. A
// user's own file and a symlink pointing elsewhere both survive, and their
// presence keeps the directory (rmdir semantics).
func TestPruneLegacyTeamLeavesForeignEntries(t *testing.T) {
	cfg := stageConfig(t)
	repo := t.TempDir()
	src := filepath.Join(repo, "skills/go-review")
	writeFile(t, filepath.Join(src, "shared/roles/a-reviewer.md"), "role\n")
	writeFile(t, filepath.Join(src, "runtimes/claude/SKILL.md"), "claude\n")
	writeFile(t, filepath.Join(src, "runtimes/codex/SKILL.md"), "codex\n")

	staged := seedLegacyTeamInstall(t, cfg, "go-review-team", "review-lead.md")
	agentsDir := filepath.Join(cfg.Home, ".claude/agents/go-review-team")

	// A hand-written agent and a symlink to somewhere we do not own.
	writeFile(t, filepath.Join(agentsDir, "my-own.md"), "mine\n")
	elsewhere := filepath.Join(repo, "elsewhere.md")
	writeFile(t, elsewhere, "foreign\n")
	foreign := filepath.Join(agentsDir, "foreign.md")
	if err := os.Symlink(elsewhere, foreign); err != nil {
		t.Fatal(err)
	}

	s := Skill{Kind: KindFirst, Name: "go-review", Source: src, Forked: true}
	if err := cfg.InstallSkill(s, false, false); err != nil {
		t.Fatal(err)
	}

	assertNotExists(t, filepath.Join(agentsDir, "review-lead.md"), "owned link should be pruned")
	if _, err := os.Stat(filepath.Join(agentsDir, "my-own.md")); err != nil {
		t.Fatalf("a user's own agent file must survive: %v", err)
	}
	assertSymlinkTarget(t, foreign, elsewhere)
	if _, err := os.Stat(agentsDir); err != nil {
		t.Fatalf("dir must survive while foreign entries remain: %v", err)
	}
	assertNotExists(t, staged, "legacy staged team copy should still be pruned")
}

// A skill that never was a team must not trip the prune.
func TestPruneLegacyTeamIgnoresUnrelatedSkills(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	agentsDir := filepath.Join(cfg.Home, ".claude/agents/go-review-team")
	staged := seedLegacyTeamInstall(t, cfg, "go-review-team", "review-lead.md")

	s := Skill{Kind: KindFirst, Name: "commit", Source: filepath.Join(repo, "skills/commit")}
	if err := cfg.InstallSkill(s, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(agentsDir, "review-lead.md")); err != nil {
		t.Fatalf("installing an unrelated skill must not prune: %v", err)
	}
	if _, err := os.Stat(staged); err != nil {
		t.Fatalf("installing an unrelated skill must not prune staged copy: %v", err)
	}
}

// makeMigratedSkill builds a repo dir for one of the migrated review skills.
func makeMigratedSkill(t *testing.T, repo, name string) string {
	t.Helper()
	src := filepath.Join(repo, "skills", name)
	writeFile(t, filepath.Join(src, "shared/roles/a-reviewer.md"), "role\n")
	writeFile(t, filepath.Join(src, "runtimes/claude/SKILL.md"), "claude\n")
	writeFile(t, filepath.Join(src, "runtimes/codex/SKILL.md"), "codex\n")
	return src
}

// The real upgrade shape: a pre-migration install also pointed the SKILL
// links at the staged team tree. Those must be recognised as ours and
// repointed, or LinkPath reports a foreign target and refuses without
// --force, and the prune then strands them.
func TestInstallMigratesLegacySkillLinksFromTeamStage(t *testing.T) {
	// go-review installed flat; feature-review installed forked. A given
	// machine has whichever shape it last installed.
	cases := []struct {
		skill string
		// legacyFor returns the staged team path the old link pointed at.
		legacyFor func(cfg Config, teamdir string, runtime Runtime) string
	}{
		{"go-review", func(cfg Config, teamdir string, _ Runtime) string {
			return filepath.Join(cfg.StageDir, "agent-teams", teamdir)
		}},
		{"feature-review", func(cfg Config, teamdir string, rt Runtime) string {
			return cfg.RuntimeTeamStagedSource(teamdir, rt)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.skill, func(t *testing.T) {
			cfg := stageConfig(t)
			repo := t.TempDir()
			src := makeMigratedSkill(t, repo, tc.skill)
			teamdir := legacyTeamDirs[tc.skill]

			// Seed old skill links pointing at the legacy staged team.
			for _, rt := range []struct {
				root    string
				runtime Runtime
			}{{".agents", RuntimeCodex}, {".claude", RuntimeClaude}} {
				legacy := tc.legacyFor(cfg, teamdir, rt.runtime)
				writeFile(t, filepath.Join(legacy, "SKILL.md"), "legacy\n")
				link := filepath.Join(cfg.Home, rt.root, "skills", tc.skill)
				if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(legacy, link); err != nil {
					t.Fatal(err)
				}
			}

			s := Skill{Kind: KindFirst, Name: tc.skill, Source: src, Forked: true}
			// force=false: an upgrade must not require --force.
			if err := cfg.InstallSkill(s, false, false); err != nil {
				t.Fatal(err)
			}

			assertSymlinkTarget(t, filepath.Join(cfg.Home, ".agents/skills", tc.skill),
				cfg.RuntimeStagedSource(tc.skill, RuntimeCodex))
			assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/skills", tc.skill),
				cfg.RuntimeStagedSource(tc.skill, RuntimeClaude))
			for _, legacy := range cfg.legacyTeamOwnedPaths(tc.skill) {
				assertNotExists(t, legacy, "legacy staged team tree should be pruned")
			}
		})
	}
}

// Ownership is scoped to this team's legacy stage trees, not to StageDir as a
// whole: a user's symlink to some other staged skill must survive.
func TestPruneLegacyTeamLeavesUnrelatedStagedSymlink(t *testing.T) {
	cfg := stageConfig(t)
	repo := t.TempDir()
	src := makeMigratedSkill(t, repo, "go-review")

	seedLegacyTeamInstall(t, cfg, "go-review-team", "review-lead.md")
	agentsDir := filepath.Join(cfg.Home, ".claude/agents/go-review-team")

	// A user-managed symlink into staging that is nothing to do with the team.
	otherStaged := filepath.Join(cfg.StageDir, "skills/some-other-skill/helper.md")
	writeFile(t, otherStaged, "helper\n")
	mine := filepath.Join(agentsDir, "my-helper.md")
	if err := os.Symlink(otherStaged, mine); err != nil {
		t.Fatal(err)
	}

	s := Skill{Kind: KindFirst, Name: "go-review", Source: src, Forked: true}
	if err := cfg.InstallSkill(s, false, false); err != nil {
		t.Fatal(err)
	}

	assertNotExists(t, filepath.Join(agentsDir, "review-lead.md"), "owned link should be pruned")
	assertSymlinkTarget(t, mine, otherStaged)
}

// A symlinked agents dir must not be followed: pruning through it would
// delete inside a directory the installer does not own.
func TestPruneLegacyTeamDoesNotFollowSymlinkedAgentsDir(t *testing.T) {
	cfg := stageConfig(t)
	repo := t.TempDir()
	src := makeMigratedSkill(t, repo, "go-review")

	staged := filepath.Join(cfg.StageDir, "agent-teams/go-review-team")
	writeFile(t, filepath.Join(staged, "review-lead.md"), "legacy\n")

	elsewhere := filepath.Join(repo, "my-agents")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(staged, "review-lead.md"), filepath.Join(elsewhere, "review-lead.md")); err != nil {
		t.Fatal(err)
	}
	agentsParent := filepath.Join(cfg.Home, ".claude/agents")
	if err := os.MkdirAll(agentsParent, 0o755); err != nil {
		t.Fatal(err)
	}
	agentsDir := filepath.Join(agentsParent, "go-review-team")
	if err := os.Symlink(elsewhere, agentsDir); err != nil {
		t.Fatal(err)
	}

	s := Skill{Kind: KindFirst, Name: "go-review", Source: src, Forked: true}
	if err := cfg.InstallSkill(s, false, false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Lstat(filepath.Join(elsewhere, "review-lead.md")); err != nil {
		t.Fatalf("must not prune through a symlinked agents dir: %v", err)
	}
	assertSymlinkTarget(t, agentsDir, elsewhere)
}

// The prune must respect SKILL_INSTALL_TARGETS. An agents-only run has no
// business deleting Claude state, and removing a stage tree that an unmanaged
// root still links at would leave that root dangling.
func TestPruneLegacyTeamRespectsTargets(t *testing.T) {
	cfg := stageConfig(t)
	cfg.Targets = []Target{TargetAgents}
	repo := t.TempDir()
	src := makeMigratedSkill(t, repo, "go-review")

	flat := seedLegacyTeamInstall(t, cfg, "go-review-team", "review-lead.md")
	agentsDir := filepath.Join(cfg.Home, ".claude/agents/go-review-team")
	claudeRuntime := cfg.RuntimeTeamStagedSource("go-review-team", RuntimeClaude)
	writeFile(t, filepath.Join(claudeRuntime, "SKILL.md"), "legacy\n")

	// The unmanaged Claude root still points at the flat stage.
	claudeLink := filepath.Join(cfg.Home, ".claude/skills/go-review")
	if err := os.MkdirAll(filepath.Dir(claudeLink), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(flat, claudeLink); err != nil {
		t.Fatal(err)
	}

	s := Skill{Kind: KindFirst, Name: "go-review", Source: src, Forked: true}
	if err := cfg.InstallSkill(s, false, false); err != nil {
		t.Fatal(err)
	}

	// Managed root migrated.
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".agents/skills/go-review"),
		cfg.RuntimeStagedSource("go-review", RuntimeCodex))
	// Claude state untouched, and its link still resolves.
	if _, err := os.Stat(filepath.Join(agentsDir, "review-lead.md")); err != nil {
		t.Fatalf("agents-only run must not prune Claude agent registrations: %v", err)
	}
	if _, err := os.Stat(claudeRuntime); err != nil {
		t.Fatalf("agents-only run must not prune the Claude runtime stage: %v", err)
	}
	if _, err := os.Stat(claudeLink); err != nil {
		t.Fatalf("unmanaged Claude link must not be left dangling: %v", err)
	}
}

// A failed relink must not be followed by a prune: deleting the stage tree a
// still-legacy link points at converts a recoverable error into a dangling
// install.
func TestPruneLegacyTeamSkippedWhenLinkFails(t *testing.T) {
	cfg := stageConfig(t)
	cfg.Targets = []Target{TargetAgents, TargetClaude}
	repo := t.TempDir()
	src := makeMigratedSkill(t, repo, "go-review")

	flat := seedLegacyTeamInstall(t, cfg, "go-review-team", "review-lead.md")
	writeFile(t, filepath.Join(flat, "SKILL.md"), "legacy\n")

	// A real directory at the Claude skill target: LinkPath refuses to
	// replace it without force, so that link fails.
	claudeTarget := filepath.Join(cfg.Home, ".claude/skills/go-review")
	if err := os.MkdirAll(claudeTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	// The agents root still points at the legacy flat stage.
	agentsLink := filepath.Join(cfg.Home, ".agents/skills/go-review")
	if err := os.MkdirAll(filepath.Dir(agentsLink), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(flat, agentsLink); err != nil {
		t.Fatal(err)
	}

	s := Skill{Kind: KindFirst, Name: "go-review", Source: src, Forked: true}
	if err := cfg.InstallSkill(s, false, false); err == nil {
		t.Fatal("expected the blocked Claude link to report an error")
	}

	if _, err := os.Stat(flat); err != nil {
		t.Fatalf("legacy stage must survive a failed migration: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.Home, ".claude/agents/go-review-team/review-lead.md")); err != nil {
		t.Fatalf("legacy agent registrations must survive a failed migration: %v", err)
	}
}

// Old enough installs registered agents by pointing straight at the checkout,
// before staging existed. Those repo files are gone now, so the prune must
// claim that shape too or the registrations dangle.
func TestPruneLegacyTeamRemovesRepoPointingRegistrations(t *testing.T) {
	cfg := stageConfig(t)
	repo := t.TempDir()
	cfg.RepoDir = repo
	src := makeMigratedSkill(t, repo, "go-review")

	// A pre-staging registration: ~/.claude/agents/<team>/x.md -> repo file.
	repoTeamFile := filepath.Join(repo, "agent-teams/go-review-team/review-lead.md")
	writeFile(t, repoTeamFile, "lead\n")
	agentsDir := filepath.Join(cfg.Home, ".claude/agents/go-review-team")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	repoLink := filepath.Join(agentsDir, "review-lead.md")
	if err := os.Symlink(repoTeamFile, repoLink); err != nil {
		t.Fatal(err)
	}

	s := Skill{Kind: KindFirst, Name: "go-review", Source: src, Forked: true}
	if err := cfg.InstallSkill(s, false, false); err != nil {
		t.Fatal(err)
	}

	assertNotExists(t, repoLink, "repo-pointing legacy registration should be pruned")
	assertNotExists(t, agentsDir, "emptied legacy agent dir should be removed")
}

// StageDir is configurable (SKILL_SYMLINKS_DIR), so a directory sitting at a
// legacy team path may be the user's, not ours. A recursive delete needs
// evidence that something we migrated actually pointed at it.
func TestPruneLegacyTeamKeepsUnclaimedStageDir(t *testing.T) {
	cfg := stageConfig(t)
	repo := t.TempDir()
	src := makeMigratedSkill(t, repo, "go-review")

	// A directory at the legacy path that no installed link or registration
	// references — name collision only.
	unclaimed := filepath.Join(cfg.StageDir, "agent-teams/go-review-team")
	writeFile(t, filepath.Join(unclaimed, "my-notes.md"), "not the installer's\n")

	s := Skill{Kind: KindFirst, Name: "go-review", Source: src, Forked: true}
	if err := cfg.InstallSkill(s, false, false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(unclaimed, "my-notes.md")); err != nil {
		t.Fatalf("unclaimed directory at a legacy path must survive: %v", err)
	}
}

// Uninstall must recognise a repo-pointing legacy skill link too, or `--none`
// reports it removed the link while leaving it behind — dangling, since the
// migration deleted agent-teams/ from the checkout.
func TestUninstallRemovesRepoPointingLegacySkillLink(t *testing.T) {
	cfg := stageConfig(t)
	repo := t.TempDir()
	cfg.RepoDir = repo
	src := makeMigratedSkill(t, repo, "go-review")

	// Pre-staging install: the skill link points straight at the checkout.
	repoTeam := filepath.Join(repo, "agent-teams/go-review-team")
	writeFile(t, filepath.Join(repoTeam, "SKILL.md"), "legacy\n")
	for _, root := range []string{".agents", ".claude"} {
		link := filepath.Join(cfg.Home, root, "skills/go-review")
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(repoTeam, link); err != nil {
			t.Fatal(err)
		}
	}

	s := Skill{Kind: KindFirst, Name: "go-review", Source: src, Forked: true}
	if err := cfg.UninstallSkill(s); err != nil {
		t.Fatal(err)
	}

	for _, root := range []string{".agents", ".claude"} {
		assertNotExists(t, filepath.Join(cfg.Home, root, "skills/go-review"),
			"repo-pointing legacy skill link should be removed by uninstall")
	}
}

// A foreign symlink resting on a descendant of a legacy stage is not proof the
// tree is ours: UnlinkOwned would not remove that link, so pruning the tree
// would both destroy data and leave the link dangling.
func TestPruneLegacyTeamIgnoresDescendantLinkAsEvidence(t *testing.T) {
	cfg := stageConfig(t)
	repo := t.TempDir()
	src := makeMigratedSkill(t, repo, "go-review")

	legacy := filepath.Join(cfg.StageDir, "agent-teams/go-review-team")
	writeFile(t, filepath.Join(legacy, "custom/SKILL.md"), "user data\n")

	// Points INSIDE the legacy tree, not at its root.
	foreign := filepath.Join(cfg.Home, ".agents/skills/go-review")
	if err := os.MkdirAll(filepath.Dir(foreign), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(legacy, "custom"), foreign); err != nil {
		t.Fatal(err)
	}

	// Uninstall is the exposed path: UnlinkOwned leaves the foreign link
	// alone WITHOUT erroring, so the failed-link guard does not fire and the
	// prune runs.
	s := Skill{Kind: KindFirst, Name: "go-review", Source: src, Forked: true}
	if err := cfg.UninstallSkill(s); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(legacy, "custom/SKILL.md")); err != nil {
		t.Fatalf("descendant link must not license deleting the tree: %v", err)
	}
	assertSymlinkTarget(t, foreign, filepath.Join(legacy, "custom"))
}
