package importer_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"agent-skills/tools/skills-tui/internal/importer"
)

func TestScanFindsRootAndNestedSkills(t *testing.T) {
	t.Run("checkout root", func(t *testing.T) {
		checkout := t.TempDir()
		writeSkill(t, checkout, ".", "root-skill", "A root skill")

		candidates, err := importer.Scan(checkout)
		if err != nil {
			t.Fatal(err)
		}
		if len(candidates) != 1 {
			t.Fatalf("expected one candidate, got %#v", candidates)
		}
		assertCandidate(t, candidates[0], ".", "root-skill", "A root skill", true)
	})

	t.Run("nested directory", func(t *testing.T) {
		checkout := t.TempDir()
		writeSkill(t, checkout, "packages/nested", "nested-skill", "A nested skill")

		candidates, err := importer.Scan(checkout)
		if err != nil {
			t.Fatal(err)
		}
		if len(candidates) != 1 {
			t.Fatalf("expected one candidate, got %#v", candidates)
		}
		assertCandidate(t, candidates[0], "packages/nested", "nested-skill", "A nested skill", true)
	})
}

func TestScanIncludesHiddenCompatibilityRootsButExcludesGit(t *testing.T) {
	checkout := t.TempDir()
	writeSkill(t, checkout, ".claude/skills/portable", "portable", "A compatibility skill")
	writeSkill(t, checkout, ".git/skills/private", "private", "Git metadata content")

	candidates, err := importer.Scan(checkout)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected only the compatibility-root skill, got %#v", candidates)
	}
	assertCandidate(t, candidates[0], ".claude/skills/portable", "portable", "A compatibility skill", true)
}

func TestScanReturnsInvalidFrontmatterAndUnsafeNamesAsDisabledCandidates(t *testing.T) {
	checkout := t.TempDir()
	writeRawSkill(t, checkout, "missing-description", "---\nname: incomplete\n---\n")
	writeRawSkill(t, checkout, "malformed", "---\nname: [not valid YAML\ndescription: broken\n---\n")
	writeSkill(t, checkout, "unsafe", "../escape", "Unsafe install name")
	writeRawSkill(t, checkout, "quoted", "---\nname: quoted\ndescription: \"Supports: quoted YAML\"\n---\n")

	candidates, err := importer.Scan(checkout)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 4 {
		t.Fatalf("expected every SKILL.md to produce a candidate, got %#v", candidates)
	}

	byPath := make(map[string]importer.Candidate, len(candidates))
	for _, candidate := range candidates {
		byPath[candidate.SourcePath] = candidate
	}
	for _, path := range []string{"missing-description", "malformed", "unsafe"} {
		candidate := byPath[path]
		if candidate.Valid || strings.TrimSpace(candidate.Reason) == "" {
			t.Fatalf("%s should be disabled with an actionable reason, got %#v", path, candidate)
		}
	}
	assertCandidate(t, byPath["quoted"], "quoted", "quoted", "Supports: quoted YAML", true)
}

func TestScanStopsDescendingAfterAcceptingASkillRoot(t *testing.T) {
	checkout := t.TempDir()
	writeSkill(t, checkout, "bundle", "bundle", "The accepted skill root")
	writeSkill(t, checkout, "bundle/examples/nested", "nested", "Must not become a second skill")

	candidates, err := importer.Scan(checkout)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected accepted root to prune nested content, got %#v", candidates)
	}
	assertCandidate(t, candidates[0], "bundle", "bundle", "The accepted skill root", true)
}

func TestScanDisablesDuplicateNamesAndReturnsDeterministicIDs(t *testing.T) {
	checkout := t.TempDir()
	writeSkill(t, checkout, "zeta", "Same-Name", "First duplicate")
	writeSkill(t, checkout, "alpha", "same-name", "Second duplicate")
	writeSkill(t, checkout, "middle", "unique", "Unique skill")

	first, err := importer.Scan(checkout)
	if err != nil {
		t.Fatal(err)
	}
	second, err := importer.Scan(checkout)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("repeated scans differ:\nfirst:  %#v\nsecond: %#v", first, second)
	}
	if got := []string{first[0].ID, first[1].ID, first[2].ID}; !reflect.DeepEqual(got, []string{"alpha", "middle", "zeta"}) {
		t.Fatalf("candidate IDs should be stable source paths in sorted order, got %v", got)
	}
	for _, index := range []int{0, 2} {
		if first[index].Valid || !strings.Contains(first[index].Reason, "duplicate") {
			t.Fatalf("duplicate candidate should be disabled with reason, got %#v", first[index])
		}
	}
	if !first[1].Valid {
		t.Fatalf("unrelated unique candidate should remain valid, got %#v", first[1])
	}
}

func TestScanLimitsFrontmatterWithoutLimitingTheMarkdownBody(t *testing.T) {
	checkout := t.TempDir()
	writeRawSkill(t, checkout, "large-doc", "---\nname: large-doc\ndescription: A skill with extensive documentation\n---\n"+strings.Repeat("documentation\n", 100_000))

	candidates, err := importer.Scan(checkout)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected one candidate, got %#v", candidates)
	}
	assertCandidate(t, candidates[0], "large-doc", "large-doc", "A skill with extensive documentation", true)
}

func writeSkill(t *testing.T, root, rel, name, description string) {
	t.Helper()
	writeRawSkill(t, root, rel, "---\nname: "+name+"\ndescription: "+description+"\n---\n")
}

func writeRawSkill(t *testing.T, root, rel, content string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertCandidate(t *testing.T, got importer.Candidate, path, name, description string, valid bool) {
	t.Helper()
	if got.ID != path || got.SourcePath != path || got.Name != name || got.Description != description || got.Valid != valid {
		t.Fatalf("unexpected candidate: %#v", got)
	}
}
