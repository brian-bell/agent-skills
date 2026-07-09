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

func TestRepoTeamsAreHybridGoReviewFlatFeatureReviewForked(t *testing.T) {
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
	if s.Forked {
		t.Fatal("go-review team should remain flat (unforked) until migrated")
	}

	s, ok = findSkill(out, KindTeamHybrid, "feature-review")
	if !ok {
		t.Fatalf("expected repo feature-review team to be Codex-compatible, got: %v", out)
	}
	if want := filepath.Join(root, "agent-teams/feature-review-team"); s.Source != want {
		t.Fatalf("expected feature-review source %s, got %s", want, s.Source)
	}
	if !s.Forked {
		t.Fatal("feature-review team should be runtime-forked (claude+codex)")
	}
}

// featureReviewReviewers are the five focus areas / shared checklist files of
// the feature-review team.
var featureReviewReviewers = []string{
	"product-reviewer",
	"safety-reviewer",
	"quality-reviewer",
	"maintainability-reviewer",
	"documentation-reviewer",
}

func TestFeatureReviewCodexOverlayIsSelfContained(t *testing.T) {
	root := repoRoot(t)
	file := filepath.Join(root, "agent-teams/feature-review-team/runtimes/codex/SKILL.md")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Skip("agent-skills repo feature-review codex overlay not present")
	}
	content := string(data)

	if strings.Contains(content, "Platform —") {
		t.Fatal("runtime overlays must not contain Platform blocks")
	}
	if !strings.Contains(content, "checklist source material only") {
		t.Fatal("codex overlay should treat reviewer files as checklist-only source material")
	}
	for _, reviewer := range featureReviewReviewers {
		if !strings.Contains(content, reviewer+".md") {
			t.Fatalf("codex overlay should reference %s.md", reviewer)
		}
	}
	// The parallel fan-out and its inline fallback are load-bearing. The
	// fan-in tool is wait_agent (per Codex runtime review on PR #74) — a
	// bare `wait` names a nonexistent tool and derails the fan-out.
	if !strings.Contains(content, "spawn_agent") {
		t.Fatal("codex overlay should fan out reviewers via the native subagent tools")
	}
	if !strings.Contains(content, "wait_agent") {
		t.Fatal("codex overlay should fan in via wait_agent")
	}
	if !strings.Contains(content, "Fallback") {
		t.Fatal("codex overlay should document the inline fallback")
	}
	if re := regexp.MustCompile(`Claude Code|Agent tool|subagent_type|TaskCreate|TaskUpdate|TaskList|TeamCreate|SendMessage|AskUserQuestion|Artifact|WebSearch|WebFetch`); re.MatchString(content) {
		t.Fatalf("codex overlay must not use Claude-only tokens: %s", re.FindString(content))
	}
}

func TestFeatureReviewClaudeOverlayKeepsRegisteredTeam(t *testing.T) {
	root := repoRoot(t)
	team := filepath.Join(root, "agent-teams/feature-review-team")
	data, err := os.ReadFile(filepath.Join(team, "runtimes/claude/SKILL.md"))
	if err != nil {
		t.Skip("agent-skills repo feature-review claude overlay not present")
	}
	if !strings.Contains(string(data), "acceptance-lead") {
		t.Fatal("claude overlay should delegate to the acceptance-lead agent")
	}
	if _, err := os.Stat(filepath.Join(team, "runtimes/claude/acceptance-lead.md")); err != nil {
		t.Fatalf("acceptance-lead.md should live in the claude overlay: %v", err)
	}
	for _, reviewer := range featureReviewReviewers {
		if _, err := os.Stat(filepath.Join(team, "shared", reviewer+".md")); err != nil {
			t.Fatalf("%s.md should live in shared/: %v", reviewer, err)
		}
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
