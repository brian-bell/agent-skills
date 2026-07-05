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

	if err := cfg.InstallSkill(Skill{KindFirst, "commit", src}, false, false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(staged, "SKILL.md")); err != nil {
		t.Fatalf("expected staged skill copy at %s: %v", staged, err)
	}
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".agents/skills/commit"), staged)
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/skills/commit"), staged)
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".cursor/skills/commit"), staged)
}

// Port of test_install_respects_skill_install_targets_cursor_only.
func TestInstallRespectsTargetsCursorOnly(t *testing.T) {
	cfg := stageConfig(t)
	cfg.Targets = []string{"cursor"}
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")

	if err := cfg.InstallSkill(Skill{KindFirst, "commit", src}, false, false); err != nil {
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
	cfg.Targets = []string{"agents", "claude"}
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")

	if err := cfg.InstallSkill(Skill{KindFirst, "commit", src}, false, false); err != nil {
		t.Fatal(err)
	}

	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".agents/skills/commit"), staged)
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/skills/commit"), staged)
	assertNotExists(t, filepath.Join(cfg.Home, ".cursor/skills/commit"),
		"agents,claude install must not link into ~/.cursor")
}

// Port of test_install_team_links_skill_and_agents.
func TestInstallTeamLinksSkillAndAgents(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "agent-teams/go-review-team")
	staged := filepath.Join(cfg.StageDir, "agent-teams/go-review-team")

	if err := cfg.InstallSkill(Skill{KindTeam, "go-review", src}, false, false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(staged, "review-lead.md")); err != nil {
		t.Fatalf("expected staged team copy at %s: %v", staged, err)
	}
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/skills/go-review"), staged)
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/agents/go-review-team/review-lead.md"),
		filepath.Join(staged, "review-lead.md"))
	assertNotExists(t, filepath.Join(cfg.Home, ".agents/skills/go-review"),
		"team skills must not link into ~/.agents")
	assertNotExists(t, filepath.Join(cfg.Home, ".cursor/skills/go-review"),
		"team skills must not link into ~/.cursor")
	assertNotExists(t, filepath.Join(cfg.Home, ".claude/agents/go-review-team/SKILL.md"),
		"SKILL.md is the manifest, not an agent; must not be linked")
}

// Port of test_install_hybrid_team_links_agents_skill_and_claude_agents.
func TestInstallHybridTeamLinksAgentsSkillAndClaudeAgents(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "agent-teams/hybrid-review-team")
	staged := filepath.Join(cfg.StageDir, "agent-teams/hybrid-review-team")

	if err := cfg.InstallSkill(Skill{KindTeamHybrid, "hybrid-review", src}, false, false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(staged, "hybrid-lead.md")); err != nil {
		t.Fatalf("expected staged hybrid team copy at %s: %v", staged, err)
	}
	if _, err := os.Stat(filepath.Join(staged, "agents/openai.yaml")); err != nil {
		t.Fatal("expected Codex metadata in staged hybrid team copy")
	}
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".agents/skills/hybrid-review"), staged)
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/skills/hybrid-review"), staged)
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/agents/hybrid-review-team/hybrid-lead.md"),
		filepath.Join(staged, "hybrid-lead.md"))
	assertNotExists(t, filepath.Join(cfg.Home, ".claude/agents/hybrid-review-team/SKILL.md"),
		"SKILL.md is the manifest, not an agent; must not be linked")
}

// bash iterates "$source"/*.md, which never matches leading-dot files, so
// hidden markdown files (including a literal ".md") are not agent links.
func TestSkillLinksSkipHiddenTeamAgentFiles(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "agent-teams/go-review-team")
	writeFile(t, filepath.Join(src, ".draft-reviewer.md"), "draft\n")
	writeFile(t, filepath.Join(src, ".md"), "dot\n")

	links := cfg.SkillLinks(Skill{KindTeam, "go-review", src})

	sawLead := false
	for _, l := range links {
		switch filepath.Base(l.Target) {
		case ".draft-reviewer.md", ".md":
			t.Fatalf("hidden markdown %s must not be linked as an agent", l.Target)
		case "review-lead.md":
			sawLead = true
		}
	}
	if !sawLead {
		t.Fatal("expected review-lead.md agent link")
	}
}

// Install half of test_team_skips_when_claude_not_in_skill_install_targets:
// a direct InstallSkill (as on the --force path) of a claude-only team with
// claude untargeted must create nothing — no ~/.claude links and no staged
// copy.
func TestInstallTeamWithoutClaudeTargetCreatesNothing(t *testing.T) {
	cfg := stageConfig(t)
	cfg.Targets = []string{"agents", "cursor"}
	repo := makeRepo(t)
	src := filepath.Join(repo, "agent-teams/go-review-team")

	if err := cfg.InstallSkill(Skill{KindTeam, "go-review", src}, true, true); err != nil {
		t.Fatalf("skipped team install should be a no-op, got: %v", err)
	}

	assertNotExists(t, filepath.Join(cfg.Home, ".claude/skills/go-review"),
		"claude-untargeted team install must not link the skill")
	assertNotExists(t, filepath.Join(cfg.Home, ".claude/agents/go-review-team"),
		"claude-untargeted team install must not link agents")
	assertNotExists(t, filepath.Join(cfg.StageDir, "agent-teams/go-review-team"),
		"claude-untargeted team install must not stage a copy")
}

// Uninstall half: with claude untargeted, uninstall_skill returns early and
// leaves existing claude links alone.
func TestUninstallTeamWithoutClaudeTargetLeavesLinks(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "agent-teams/go-review-team")
	skill := Skill{KindTeam, "go-review", src}
	staged := filepath.Join(cfg.StageDir, "agent-teams/go-review-team")

	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}

	limited := cfg
	limited.Targets = []string{"agents", "cursor"}
	limited.UninstallSkill(skill)

	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/skills/go-review"), staged)
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/agents/go-review-team/review-lead.md"),
		filepath.Join(staged, "review-lead.md"))
}

// Port of test_uninstall_removes_owned_links.
func TestUninstallRemovesOwnedLinks(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "agent-teams/go-review-team")
	skill := Skill{KindTeam, "go-review", src}

	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}
	cfg.UninstallSkill(skill)

	assertNotExists(t, filepath.Join(cfg.Home, ".claude/skills/go-review"),
		"expected go-review link removed")
	assertNotExists(t, filepath.Join(cfg.Home, ".claude/agents/go-review-team"),
		"expected empty team agent dir pruned")
}

// Port of test_uninstall_hybrid_team_removes_agents_and_claude_links.
func TestUninstallHybridTeamRemovesAgentsAndClaudeLinks(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "agent-teams/hybrid-review-team")
	skill := Skill{KindTeamHybrid, "hybrid-review", src}

	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}
	cfg.UninstallSkill(skill)

	assertNotExists(t, filepath.Join(cfg.Home, ".agents/skills/hybrid-review"),
		"expected hybrid agents skill link removed")
	assertNotExists(t, filepath.Join(cfg.Home, ".claude/skills/hybrid-review"),
		"expected hybrid Claude skill link removed")
	assertNotExists(t, filepath.Join(cfg.Home, ".claude/agents/hybrid-review-team"),
		"expected empty hybrid team agent dir pruned")
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

	cfg.UninstallSkill(Skill{KindFirst, "commit", src})

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

	cfg.UninstallSkill(Skill{KindFirst, "commit", src})

	assertSymlinkTarget(t, link, elsewhere)
}

// Port of test_force_install_relinks_stale_copy.
func TestForceInstallRelinksStaleCopy(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")
	skill := Skill{KindFirst, "commit", src}

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

	err := cfg.InstallSkill(Skill{KindFirst, "commit", src}, false, false)
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
	skill := Skill{KindFirst, "commit", src}

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

	if err := cfg.InstallSkill(Skill{KindFirst, "commit", src}, false, false); err != nil {
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

// Port of test_feature_review_team_discovered_and_installed (I2, fixture
// version): feature-review must be discovered, installed, SKILL.md excluded.
func TestFeatureReviewTeamDiscoveredAndInstalled(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "agent-teams/feature-review-team")
	staged := filepath.Join(cfg.StageDir, "agent-teams/feature-review-team")

	out, err := Discover(repo)
	if err != nil {
		t.Fatal(err)
	}
	skill, ok := findSkill(out, KindTeam, "feature-review")
	if !ok || skill.Source != src {
		t.Fatalf("feature-review not discovered: %v", out)
	}

	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/skills/feature-review"), staged)
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/agents/feature-review-team/acceptance-lead.md"),
		filepath.Join(staged, "acceptance-lead.md"))
	assertNotExists(t, filepath.Join(cfg.Home, ".claude/agents/feature-review-team/SKILL.md"),
		"feature-review SKILL.md must not be linked as an agent")
}
