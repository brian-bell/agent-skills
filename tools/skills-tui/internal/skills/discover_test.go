package skills

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// makeRepo builds a throwaway repo fixture covering every skill kind.
func makeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "skills/commit/SKILL.md"), "commit skill\n")
	writeFile(t, filepath.Join(dir, "skills/tdd/SKILL.md"), "tdd skill\n")
	writeFile(t, filepath.Join(dir, "third-party/autoreview/SKILL.md"), "autoreview skill\n")
	writeFile(t, filepath.Join(dir, "third-party/ATTRIBUTION.md"), "stub\n")
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func makeForkedSkill(t *testing.T, repo, name string) string {
	t.Helper()
	src := filepath.Join(repo, "skills", name)
	writeFile(t, filepath.Join(src, "shared/scripts/helper.sh"), "echo shared\n")
	for _, runtime := range []string{"claude", "codex"} {
		writeFile(t, filepath.Join(src, "runtimes", runtime, "SKILL.md"), runtime+" skill\n")
	}
	writeFile(t, filepath.Join(src, "runtimes/codex/agents/openai.yaml"), "interface:\n")
	return src
}

// makeForkedTeam builds a runtime-forked agent team: shared reviewer files
// plus claude and codex overlays. Teams fork into exactly two runtimes —
// cursor is deliberately not part of the team contract.
func findSkill(list []Skill, kind Kind, name string) (Skill, bool) {
	for _, s := range list {
		if s.Kind == kind && s.Name == name {
			return s, true
		}
	}
	return Skill{}, false
}

func TestDiscoverListsThirdPartySkippingFiles(t *testing.T) {
	repo := makeRepo(t)

	out, err := Discover(repo, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	s, ok := findSkill(out, KindThird, "autoreview")
	if !ok {
		t.Fatalf("expected third-party autoreview, got: %v", out)
	}
	if want := filepath.Join(repo, "third-party/autoreview"); s.Source != want {
		t.Fatalf("expected source %s, got %s", want, s.Source)
	}
	for _, s := range out {
		if s.Name == "ATTRIBUTION.md" || s.Name == "ATTRIBUTION" {
			t.Fatalf("discovery should skip ATTRIBUTION.md, got: %v", out)
		}
	}
}

// bash globs (skills/*, third-party/*) never match leading-dot entries, so
// hidden directories are not skills.
func TestDiscoverSkipsHiddenDirectories(t *testing.T) {
	repo := makeRepo(t)
	writeFile(t, filepath.Join(repo, "skills/.archive/SKILL.md"), "hidden\n")
	writeFile(t, filepath.Join(repo, "third-party/.github/config.yml"), "hidden\n")

	out, err := Discover(repo, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range out {
		base := filepath.Base(s.Source)
		if base == ".archive" || base == ".github" {
			t.Fatalf("discovery must skip hidden directory %s (kind %s, name %s)", s.Source, s.Kind, s.Name)
		}
	}
}

func TestDiscoverListsFirstParty(t *testing.T) {
	repo := makeRepo(t)

	out, err := Discover(repo, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	s, ok := findSkill(out, KindFirst, "commit")
	if !ok {
		t.Fatalf("expected first-party commit in discovery, got: %v", out)
	}
	if want := filepath.Join(repo, "skills/commit"); s.Source != want {
		t.Fatalf("expected source %s, got %s", want, s.Source)
	}
}

func TestDiscoverMarksFullyForkedFirstPartySkill(t *testing.T) {
	repo := makeRepo(t)
	src := makeForkedSkill(t, repo, "runtime-demo")

	out, err := Discover(repo, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	s, ok := findSkill(out, KindFirst, "runtime-demo")
	if !ok {
		t.Fatalf("expected runtime-demo in discovery, got: %v", out)
	}
	if s.Source != src {
		t.Fatalf("expected source %s, got %s", src, s.Source)
	}
	if !s.Forked {
		t.Fatal("fully forked first-party skill should be marked Forked")
	}
}

func TestDiscoverMarksCursorLessForkedSkill(t *testing.T) {
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/cursor-less")
	writeFile(t, filepath.Join(src, "shared/note.md"), "shared\n")
	writeFile(t, filepath.Join(src, "runtimes/claude/SKILL.md"), "claude\n")
	writeFile(t, filepath.Join(src, "runtimes/codex/SKILL.md"), "codex\n")

	out, err := Discover(repo, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	s, ok := findSkill(out, KindFirst, "cursor-less")
	if !ok {
		t.Fatalf("expected cursor-less in discovery, got: %v", out)
	}
	if !s.Forked {
		t.Fatal("claude+codex overlays (no cursor) should still be marked Forked")
	}
}

func TestDiscoverRejectsMissingClaudeOrCodexOverlay(t *testing.T) {
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/incomplete")
	writeFile(t, filepath.Join(src, "shared/note.md"), "shared\n")
	writeFile(t, filepath.Join(src, "runtimes/claude/SKILL.md"), "claude\n")
	// codex overlay intentionally missing

	out, err := Discover(repo, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	s, ok := findSkill(out, KindFirst, "incomplete")
	if !ok {
		t.Fatalf("expected incomplete in discovery, got: %v", out)
	}
	if s.Forked {
		t.Fatal("missing codex overlay must not be marked Forked")
	}
}
