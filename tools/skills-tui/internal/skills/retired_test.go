package skills

import (
	"bytes"
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

func TestPruneRetiredSkillInstallKeepsStageWhenTargetInspectionFails(t *testing.T) {
	cfg := stageConfig(t)
	cfg.Targets = []Target{TargetAgents}
	staged := cfg.RuntimeStagedSource(retiredProjectSkill, RuntimeCodex)
	writeFile(t, filepath.Join(staged, "SKILL.md"), "legacy audit\n")

	agentsRoot := filepath.Join(cfg.Home, ".agents")
	if err := os.MkdirAll(agentsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(agentsRoot, "skills"),
		[]byte("not a directory\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	if _, err := cfg.PruneRetiredSkillInstalls(); err == nil {
		t.Fatal("target inspection failure should be reported")
	}
	if _, err := os.Stat(filepath.Join(staged, "SKILL.md")); err != nil {
		t.Fatalf("target inspection failure must preserve staged copy: %v", err)
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

func TestPruneRetiredSkillInstallAllowsThirdPartyNameReuse(t *testing.T) {
	cfg := stageConfig(t)
	cfg.RepoDir = t.TempDir()
	writeFile(
		t,
		filepath.Join(
			cfg.RepoDir,
			"third-party",
			retiredProjectSkill,
			"SKILL.md",
		),
		"---\nname: skill-parity-audit\ndescription: Third-party audit.\n---\n",
	)

	legacyStaged := cfg.LegacyStagedPath(retiredProjectSkill)
	writeFile(t, filepath.Join(legacyStaged, "SKILL.md"), "third-party audit\n")
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
	if changed {
		t.Fatal("active third-party skill with the retired name must not be pruned")
	}
	if _, err := os.Stat(filepath.Join(legacyStaged, "SKILL.md")); err != nil {
		t.Fatalf("third-party staged copy must be preserved: %v", err)
	}
	for _, root := range portableRoots {
		assertSymlinkTarget(
			t,
			filepath.Join(
				cfg.Home,
				root.dir,
				"skills",
				retiredProjectSkill,
			),
			legacyStaged,
		)
	}
}

func TestApplyAllUpgradesRetiredRuntimeLinksToThirdPartyReuse(t *testing.T) {
	cfg := stageConfig(t)
	cfg.RepoDir = t.TempDir()
	source := filepath.Join(
		cfg.RepoDir,
		"third-party",
		retiredProjectSkill,
	)
	writeFile(
		t,
		filepath.Join(source, "SKILL.md"),
		"---\nname: skill-parity-audit\ndescription: Third-party audit.\n---\n",
	)

	for _, root := range portableRoots {
		runtime, ok := targetRuntime(root.target)
		if !ok {
			t.Fatalf("missing runtime for target %s", root.target)
		}
		staged := cfg.RuntimeStagedSource(retiredProjectSkill, runtime)
		writeFile(t, filepath.Join(staged, "SKILL.md"), "retired audit\n")
		target := filepath.Join(
			cfg.Home,
			root.dir,
			"skills",
			retiredProjectSkill,
		)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(staged, target); err != nil {
			t.Fatal(err)
		}
	}

	thirdParty := Skill{
		Kind:   KindThird,
		Name:   retiredProjectSkill,
		Source: source,
	}
	if state := cfg.SkillState(thirdParty); state != StateUpgrade {
		t.Fatalf("retired runtime links should make the third-party row upgradeable, got %s", state)
	}
	var output bytes.Buffer
	changed := cfg.ApplyAll(
		[]ApplyPlan{{
			Skill:   thirdParty,
			State:   StateUpgrade,
			Desired: DesiredInstall,
		}},
		&output,
	)
	if !changed {
		t.Fatal("selected third-party upgrade should report a change")
	}

	legacyStaged := cfg.LegacyStagedPath(retiredProjectSkill)
	for _, root := range portableRoots {
		assertSymlinkTarget(
			t,
			filepath.Join(
				cfg.Home,
				root.dir,
				"skills",
				retiredProjectSkill,
			),
			legacyStaged,
		)
	}
}

func TestApplyAllRemovesRetiredRuntimeLinksForThirdPartyReuse(t *testing.T) {
	cfg := stageConfig(t)
	cfg.RepoDir = t.TempDir()
	source := filepath.Join(
		cfg.RepoDir,
		"third-party",
		retiredProjectSkill,
	)
	writeFile(
		t,
		filepath.Join(source, "SKILL.md"),
		"---\nname: skill-parity-audit\ndescription: Third-party audit.\n---\n",
	)

	for _, root := range portableRoots {
		runtime, ok := targetRuntime(root.target)
		if !ok {
			t.Fatalf("missing runtime for target %s", root.target)
		}
		staged := cfg.RuntimeStagedSource(retiredProjectSkill, runtime)
		writeFile(t, filepath.Join(staged, "SKILL.md"), "retired audit\n")
		target := filepath.Join(
			cfg.Home,
			root.dir,
			"skills",
			retiredProjectSkill,
		)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(staged, target); err != nil {
			t.Fatal(err)
		}
	}

	thirdParty := Skill{
		Kind:   KindThird,
		Name:   retiredProjectSkill,
		Source: source,
	}
	var output bytes.Buffer
	cfg.ApplyAll(
		[]ApplyPlan{{
			Skill:   thirdParty,
			State:   StateUpgrade,
			Desired: DesiredRemove,
		}},
		&output,
	)

	for _, root := range portableRoots {
		assertNotExists(
			t,
			filepath.Join(
				cfg.Home,
				root.dir,
				"skills",
				retiredProjectSkill,
			),
			"third-party removal should unlink retired runtime install",
		)
	}
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
