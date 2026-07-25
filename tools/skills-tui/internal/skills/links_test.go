package skills

import (
	"errors"
	"io"
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

// Port of test_install_team_links_skill_and_agents.
func TestInstallTeamLinksSkillAndAgents(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "agent-teams/go-review-team")
	staged := filepath.Join(cfg.StageDir, "agent-teams/go-review-team")

	if err := cfg.InstallSkill(Skill{Kind: KindTeam, Name: "go-review", Source: src}, false, false); err != nil {
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

	if err := cfg.InstallSkill(Skill{Kind: KindTeamHybrid, Name: "hybrid-review", Source: src}, false, false); err != nil {
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

	links := cfg.SkillLinks(Skill{Kind: KindTeam, Name: "go-review", Source: src})

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
	cfg.Targets = []Target{"agents", "cursor"}
	repo := makeRepo(t)
	src := filepath.Join(repo, "agent-teams/go-review-team")

	if err := cfg.InstallSkill(Skill{Kind: KindTeam, Name: "go-review", Source: src}, true, true); err != nil {
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
	skill := Skill{Kind: KindTeam, Name: "go-review", Source: src}
	staged := filepath.Join(cfg.StageDir, "agent-teams/go-review-team")

	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}

	limited := cfg
	limited.Targets = []Target{"agents", "cursor"}
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
	skill := Skill{Kind: KindTeam, Name: "go-review", Source: src}

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
	skill := Skill{Kind: KindTeamHybrid, Name: "hybrid-review", Source: src}

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

// Forked teams link exactly two runtime trees (codex → ~/.agents, claude →
// ~/.claude) plus per-agent file links assembled from shared/ and the claude
// overlay. Cursor is deliberately absent from the team contract.
func TestForkedTeamSkillLinks(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := makeForkedTeam(t, repo, "forked-review-team", true)
	// Same name in shared/ and the claude overlay: the overlay wins in the
	// assembled tree, so the agent link must compare against the overlay file.
	writeFile(t, filepath.Join(src, "shared/team-lead.md"), "shared lead\n")

	skill := Skill{Kind: KindTeamHybrid, Name: "forked-review", Source: src, Forked: true}
	links := cfg.SkillLinks(skill)

	codexStaged := filepath.Join(cfg.StageDir, "runtimes/codex/agent-teams/forked-review-team")
	claudeStaged := filepath.Join(cfg.StageDir, "runtimes/claude/agent-teams/forked-review-team")

	byTarget := map[string]Link{}
	for _, l := range links {
		byTarget[l.Target] = l
	}

	agentsLink, ok := byTarget[filepath.Join(cfg.Home, ".agents/skills/forked-review")]
	if !ok {
		t.Fatalf("expected ~/.agents skills link, got %v", links)
	}
	if agentsLink.LinkSource != codexStaged {
		t.Fatalf("agents link source = %s, want %s", agentsLink.LinkSource, codexStaged)
	}
	if agentsLink.CompareShared != filepath.Join(src, "shared") ||
		agentsLink.CompareOverlay != filepath.Join(src, "runtimes/codex") {
		t.Fatalf("agents link compare = %q + %q", agentsLink.CompareShared, agentsLink.CompareOverlay)
	}

	claudeLink, ok := byTarget[filepath.Join(cfg.Home, ".claude/skills/forked-review")]
	if !ok {
		t.Fatalf("expected ~/.claude skills link, got %v", links)
	}
	if claudeLink.LinkSource != claudeStaged {
		t.Fatalf("claude link source = %s, want %s", claudeLink.LinkSource, claudeStaged)
	}
	if claudeLink.CompareShared != filepath.Join(src, "shared") ||
		claudeLink.CompareOverlay != filepath.Join(src, "runtimes/claude") {
		t.Fatalf("claude link compare = %q + %q", claudeLink.CompareShared, claudeLink.CompareOverlay)
	}

	for _, l := range links {
		if l.Target == filepath.Join(cfg.Home, ".cursor/skills/forked-review") {
			t.Fatal("forked teams must not link into ~/.cursor")
		}
	}

	wantAgents := map[string]string{
		"alpha-reviewer.md": filepath.Join(src, "shared/alpha-reviewer.md"),
		"beta-reviewer.md":  filepath.Join(src, "shared/beta-reviewer.md"),
		"team-lead.md":      filepath.Join(src, "runtimes/claude/team-lead.md"),
	}
	for md, compare := range wantAgents {
		l, ok := byTarget[filepath.Join(cfg.Home, ".claude/agents/forked-review-team", md)]
		if !ok {
			t.Fatalf("expected agent link for %s, got %v", md, links)
		}
		if l.LinkSource != filepath.Join(claudeStaged, md) {
			t.Fatalf("agent link %s source = %s", md, l.LinkSource)
		}
		if l.CompareSource != compare {
			t.Fatalf("agent link %s compare = %s, want %s", md, l.CompareSource, compare)
		}
		if l.CompareOverlay != "" || l.CompareShared != "" {
			t.Fatalf("agent link %s must compare file-to-file, got %q + %q", md, l.CompareShared, l.CompareOverlay)
		}
	}
	if _, ok := byTarget[filepath.Join(cfg.Home, ".claude/agents/forked-review-team/SKILL.md")]; ok {
		t.Fatal("SKILL.md must not be linked as an agent")
	}
	if len(links) != 5 {
		t.Fatalf("expected 5 links, got %d: %v", len(links), links)
	}

	// The claude tree link must precede its agent-file links: install syncs
	// assembled trees lazily per link, and the file links point inside the
	// claude tree.
	claudeIdx, firstAgentIdx := -1, -1
	for i, l := range links {
		if l.Target == claudeLink.Target {
			claudeIdx = i
		}
		if firstAgentIdx == -1 && filepath.Dir(l.Target) == filepath.Join(cfg.Home, ".claude/agents/forked-review-team") {
			firstAgentIdx = i
		}
	}
	if claudeIdx == -1 || firstAgentIdx == -1 || claudeIdx > firstAgentIdx {
		t.Fatalf("claude tree link (idx %d) must precede agent links (idx %d)", claudeIdx, firstAgentIdx)
	}
}

func TestInstallForkedTeamAssemblesRuntimeStagedTrees(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := makeForkedTeam(t, repo, "forked-review-team", true)
	skill := Skill{Kind: KindTeamHybrid, Name: "forked-review", Source: src, Forked: true}

	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}

	codexStaged := filepath.Join(cfg.StageDir, "runtimes/codex/agent-teams/forked-review-team")
	claudeStaged := filepath.Join(cfg.StageDir, "runtimes/claude/agent-teams/forked-review-team")

	// Both assemblies carry the shared reviewers; each carries only its own
	// overlay extras.
	for _, staged := range []string{codexStaged, claudeStaged} {
		for _, md := range []string{"alpha-reviewer.md", "beta-reviewer.md"} {
			if _, err := os.Stat(filepath.Join(staged, md)); err != nil {
				t.Fatalf("%s missing shared reviewer %s: %v", staged, md, err)
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(codexStaged, "SKILL.md")); err != nil || string(data) != "codex manifest\n" {
		t.Fatalf("codex staged SKILL.md = %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(codexStaged, "agents/openai.yaml")); err != nil {
		t.Fatalf("codex staged tree missing openai.yaml: %v", err)
	}
	assertNotExists(t, filepath.Join(codexStaged, "team-lead.md"),
		"claude-only agent definition must not reach the codex assembly")
	if data, err := os.ReadFile(filepath.Join(claudeStaged, "SKILL.md")); err != nil || string(data) != "claude manifest\n" {
		t.Fatalf("claude staged SKILL.md = %q, %v", data, err)
	}
	assertNotExists(t, filepath.Join(claudeStaged, "agents"),
		"codex metadata must not reach the claude assembly")

	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".agents/skills/forked-review"), codexStaged)
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/skills/forked-review"), claudeStaged)
	for _, md := range []string{"alpha-reviewer.md", "beta-reviewer.md", "team-lead.md"} {
		assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/agents/forked-review-team", md),
			filepath.Join(claudeStaged, md))
	}
	assertNotExists(t, filepath.Join(cfg.Home, ".claude/agents/forked-review-team/SKILL.md"),
		"SKILL.md must not be linked as an agent")
	assertNotExists(t, filepath.Join(cfg.Home, ".cursor/skills/forked-review"),
		"forked teams must not install into ~/.cursor")

	if got := cfg.SkillState(skill); got != StateInstalled {
		t.Fatalf("state after install = %s, want %s", got, StateInstalled)
	}
	// Re-apply is a no-op.
	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}
	if got := cfg.SkillState(skill); got != StateInstalled {
		t.Fatalf("state after re-apply = %s, want %s", got, StateInstalled)
	}

	// Editing a repo file makes the skill upgradeable, and re-applying
	// refreshes the assemblies back to installed.
	writeFile(t, filepath.Join(src, "shared/alpha-reviewer.md"), "alpha checklist v2\n")
	if got := cfg.SkillState(skill); got != StateUpgrade {
		t.Fatalf("state after repo edit = %s, want %s", got, StateUpgrade)
	}
	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(claudeStaged, "alpha-reviewer.md")); err != nil || string(data) != "alpha checklist v2\n" {
		t.Fatalf("claude assembly not refreshed: %q, %v", data, err)
	}
	if got := cfg.SkillState(skill); got != StateInstalled {
		t.Fatalf("state after refresh = %s, want %s", got, StateInstalled)
	}
}

// A pre-fork install (links pointing at the whole-team staged copy, or at the
// flat repo files) migrates in place without force.
func TestInstallForkedTeamMigratesLegacyLinks(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := makeForkedTeam(t, repo, "forked-review-team", true)
	skill := Skill{Kind: KindTeamHybrid, Name: "forked-review", Source: src, Forked: true}

	legacyStaged := filepath.Join(cfg.StageDir, "agent-teams/forked-review-team")
	writeFile(t, filepath.Join(legacyStaged, "SKILL.md"), "legacy manifest\n")
	writeFile(t, filepath.Join(legacyStaged, "alpha-reviewer.md"), "alpha checklist\n")

	for target, dest := range map[string]string{
		filepath.Join(cfg.Home, ".agents/skills/forked-review"):                        legacyStaged,
		filepath.Join(cfg.Home, ".claude/skills/forked-review"):                        legacyStaged,
		filepath.Join(cfg.Home, ".claude/agents/forked-review-team/alpha-reviewer.md"): filepath.Join(legacyStaged, "alpha-reviewer.md"),
		filepath.Join(cfg.Home, ".claude/agents/forked-review-team/beta-reviewer.md"):  filepath.Join(src, "beta-reviewer.md"),
		filepath.Join(cfg.Home, ".claude/agents/forked-review-team/team-lead.md"):      filepath.Join(src, "team-lead.md"),
	} {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(dest, target); err != nil {
			t.Fatal(err)
		}
	}

	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}

	codexStaged := filepath.Join(cfg.StageDir, "runtimes/codex/agent-teams/forked-review-team")
	claudeStaged := filepath.Join(cfg.StageDir, "runtimes/claude/agent-teams/forked-review-team")
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".agents/skills/forked-review"), codexStaged)
	assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/skills/forked-review"), claudeStaged)
	for _, md := range []string{"alpha-reviewer.md", "beta-reviewer.md", "team-lead.md"} {
		assertSymlinkTarget(t, filepath.Join(cfg.Home, ".claude/agents/forked-review-team", md),
			filepath.Join(claudeStaged, md))
	}
}

func TestUninstallForkedTeamRemovesOwnedLinks(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := makeForkedTeam(t, repo, "forked-review-team", true)
	skill := Skill{Kind: KindTeamHybrid, Name: "forked-review", Source: src, Forked: true}

	if err := cfg.InstallSkill(skill, false, false); err != nil {
		t.Fatal(err)
	}
	if err := cfg.UninstallSkill(skill); err != nil {
		t.Fatal(err)
	}

	assertNotExists(t, filepath.Join(cfg.Home, ".agents/skills/forked-review"), "agents link should be removed")
	assertNotExists(t, filepath.Join(cfg.Home, ".claude/skills/forked-review"), "claude link should be removed")
	assertNotExists(t, filepath.Join(cfg.Home, ".claude/agents/forked-review-team"), "empty agent dir should be pruned")
	if got := cfg.SkillState(skill); got != StateNotInstalled {
		t.Fatalf("state after uninstall = %s, want %s", got, StateNotInstalled)
	}
}

// Target gating for forked teams matches existing hybrid teams: claude and
// agents roots only, each honored independently.
func TestForkedTeamSkillLinksHonorTargets(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := makeForkedTeam(t, repo, "forked-review-team", true)
	skill := Skill{Kind: KindTeamHybrid, Name: "forked-review", Source: src, Forked: true}

	cfg.Targets = []Target{TargetClaude}
	for _, l := range cfg.SkillLinks(skill) {
		if l.Target == filepath.Join(cfg.Home, ".agents/skills/forked-review") {
			t.Fatal("claude-only targets must not produce an ~/.agents link")
		}
	}

	cfg.Targets = []Target{TargetAgents}
	links := cfg.SkillLinks(skill)
	if len(links) != 1 || links[0].Target != filepath.Join(cfg.Home, ".agents/skills/forked-review") {
		t.Fatalf("agents-only targets should produce only the ~/.agents link, got %v", links)
	}

	cfg.Targets = []Target{TargetCursor}
	if !cfg.SkipsTeam(skill.Kind) {
		t.Fatal("cursor-only targets must skip teams entirely")
	}
}

// Port of test_feature_review_team_discovered_and_installed (I2, fixture
// version): feature-review must be discovered, installed, SKILL.md excluded.
func TestFeatureReviewTeamDiscoveredAndInstalled(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "agent-teams/feature-review-team")
	staged := filepath.Join(cfg.StageDir, "agent-teams/feature-review-team")

	out, err := Discover(repo, io.Discard)
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

// A user may already have installed the migrated runtime skill before this
// cleanup shipped. The current links then look installed, but the legacy
// registrations still need to schedule an upgrade so ApplySkill runs the
// migration prune.
func TestApplySkillPrunesLegacyTeamRegistrationsForAlreadyMigratedInstall(t *testing.T) {
	cfg := stageConfig(t)
	repo := t.TempDir()
	src := makeMigratedSkill(t, repo, "go-review")
	s := Skill{Kind: KindFirst, Name: "go-review", Source: src, Forked: true}

	if err := cfg.InstallSkill(s, false, false); err != nil {
		t.Fatal(err)
	}
	staged := seedLegacyTeamInstall(t, cfg, "go-review-team", "review-lead.md")
	agentsDir := filepath.Join(cfg.Home, ".claude/agents/go-review-team")

	if got := cfg.SkillState(s); got != StateUpgrade {
		t.Fatalf("already-migrated install with legacy registrations: got state %q, want %q", got, StateUpgrade)
	}
	res := cfg.ApplySkill(s, DesiredInstall, false)
	if res.Action != ActionUpgrade || res.Outcome != OutcomeUpgraded {
		t.Fatalf("apply result = %#v, want upgrade/upgraded", res)
	}
	assertNotExists(t, agentsDir, "legacy agent dir should be pruned")
	assertNotExists(t, staged, "legacy staged team copy should be pruned")
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
			for _, legacy := range cfg.legacyTeamStagedPaths(tc.skill) {
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

// If an owned agent registration cannot be removed, its staged target must
// survive too. Otherwise the failed cleanup turns a working registration into
// a dangling symlink and cannot be retried safely.
func TestPruneLegacyTeamPreservesStageWhenAgentPruneFails(t *testing.T) {
	cfg := stageConfig(t)
	repo := t.TempDir()
	src := makeMigratedSkill(t, repo, "go-review")
	staged := seedLegacyTeamInstall(t, cfg, "go-review-team", "review-lead.md")
	agentsDir := filepath.Join(cfg.Home, ".claude/agents/go-review-team")
	agentLink := filepath.Join(agentsDir, "review-lead.md")

	if err := os.Chmod(agentsDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(agentsDir, 0o755)
	})

	s := Skill{Kind: KindFirst, Name: "go-review", Source: src, Forked: true}
	if err := cfg.InstallSkill(s, false, false); err == nil {
		t.Fatal("expected the read-only agent directory to make pruning fail")
	}
	if _, err := os.Stat(staged); err != nil {
		t.Fatalf("legacy stage must survive a failed agent prune: %v", err)
	}
	if _, err := os.Lstat(agentLink); err != nil {
		t.Fatalf("legacy agent link must remain retryable: %v", err)
	}
}
