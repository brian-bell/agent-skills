package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPruneRetiredSkillInstallKeepsStageWhenOwnedLinkCannotBeRemoved(t *testing.T) {
	cfg := stageConfig(t)
	staged := cfg.RuntimeStagedSource(retiredProjectSkill, RuntimeCodex)
	target := filepath.Join(cfg.Home, ".agents", "skills", retiredProjectSkill)

	writeFile(t, filepath.Join(staged, "SKILL.md"), "legacy audit\n")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(staged, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(target), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(filepath.Dir(target), 0o755) })

	_, err := cfg.PruneRetiredSkillInstalls()
	if err == nil {
		t.Skip("filesystem allowed symlink removal from a read-only directory")
	}
	if _, statErr := os.Stat(filepath.Join(staged, "SKILL.md")); statErr != nil {
		t.Fatalf("failed unlink must preserve staged copy for retry: %v", statErr)
	}
}

func TestPruneRetiredSkillInstallRemovesUnreferencedLegacyStage(t *testing.T) {
	cfg := stageConfig(t)
	legacyStaged := cfg.LegacyStagedPath(retiredProjectSkill)
	writeFile(t, filepath.Join(legacyStaged, "SKILL.md"), "legacy audit\n")

	for _, root := range portableRoots {
		target := filepath.Join(
			cfg.Home,
			root.dir,
			"skills",
			retiredProjectSkill,
		)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(legacyStaged, target); err != nil {
			t.Fatal(err)
		}
	}

	changed, err := cfg.PruneRetiredSkillInstalls()
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("retiring legacy links and stage should report a change")
	}
	assertNotExists(t, legacyStaged, "unreferenced legacy stage should be removed")
}

func TestPruneRetiredSkillInstallKeepsLegacyStageForUnmanagedTarget(t *testing.T) {
	cfg := stageConfig(t)
	cfg.Targets = []Target{TargetAgents}
	legacyStaged := cfg.LegacyStagedPath(retiredProjectSkill)
	writeFile(t, filepath.Join(legacyStaged, "SKILL.md"), "legacy audit\n")

	for _, root := range portableRoots {
		target := filepath.Join(
			cfg.Home,
			root.dir,
			"skills",
			retiredProjectSkill,
		)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(legacyStaged, target); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := cfg.PruneRetiredSkillInstalls(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(legacyStaged, "SKILL.md")); err != nil {
		t.Fatalf("unmanaged Claude link still needs the legacy stage: %v", err)
	}
	assertSymlinkTarget(
		t,
		filepath.Join(cfg.Home, ".claude", "skills", retiredProjectSkill),
		legacyStaged,
	)
}
