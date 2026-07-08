package skills

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// repoRoot resolves the agent-skills repo root from the package directory
// (tools/skills-tui/internal/skills → four levels up). Tests using it skip
// when the checkout layout is absent.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRepoGoReviewIsHybridFeatureReviewStaysClaudeOnly(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "agent-teams/go-review-team")); err != nil {
		t.Skip("agent-skills repo agent-teams dirs not present")
	}

	out, err := Discover(root, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	s, ok := findSkill(out, KindTeamHybrid, "go-review")
	if !ok {
		t.Fatalf("expected repo go-review team to be Codex-compatible, got: %v", out)
	}
	if want := filepath.Join(root, "agent-teams/go-review-team"); s.Source != want {
		t.Fatalf("expected go-review source %s, got %s", want, s.Source)
	}

	s, ok = findSkill(out, KindTeam, "feature-review")
	if !ok {
		t.Fatalf("expected repo feature-review team to remain Claude-only, got: %v", out)
	}
	if want := filepath.Join(root, "agent-teams/feature-review-team"); s.Source != want {
		t.Fatalf("expected feature-review source %s, got %s", want, s.Source)
	}
}

func TestGoReviewSkillDeclaresCodexWorkflow(t *testing.T) {
	root := repoRoot(t)
	file := filepath.Join(root, "agent-teams/go-review-team/SKILL.md")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Skip("agent-skills repo go-review SKILL.md not present")
	}
	content := string(data)

	if !strings.Contains(content, "Platform — Claude Code") {
		t.Fatal("go-review SKILL.md should keep a Claude Code platform block")
	}
	if !strings.Contains(content, "Platform — Codex") {
		t.Fatal("go-review SKILL.md should include a Codex platform block")
	}

	// Port of the bash awk extraction: lines from "## Platform — Codex"
	// until the next "## Platform — " heading.
	var block []string
	inBlock := false
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "## Platform — Codex") {
			inBlock = true
		} else if inBlock && strings.HasPrefix(line, "## Platform — ") {
			inBlock = false
		}
		if inBlock {
			block = append(block, line)
		}
	}
	codexBlock := strings.Join(block, "\n")

	if codexBlock == "" {
		t.Fatal("Codex platform block should not be empty")
	}
	if !strings.Contains(codexBlock, "checklist source material only") {
		t.Fatal("Codex platform block should treat reviewer files as checklist-only source material")
	}
	for _, reviewer := range []string{"structure-reviewer", "error-reviewer", "style-reviewer", "security-reviewer"} {
		if !strings.Contains(codexBlock, reviewer+".md") {
			t.Fatalf("Codex platform block should reference %s.md", reviewer)
		}
	}
	if regexp.MustCompile(`TeamCreate|TaskCreate|SendMessage|subagent_type`).MatchString(codexBlock) {
		t.Fatal("Codex platform block should not depend on Claude-only team/subagent primitives")
	}
}
