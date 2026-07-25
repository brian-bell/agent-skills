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

func TestRepoFeatureReviewTeamIsHybridForked(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "agent-teams/feature-review-team")); err != nil {
		t.Skip("agent-skills repo agent-teams dirs not present")
	}

	out, err := Discover(root, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	s, ok := findSkill(out, KindTeamHybrid, "feature-review")
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

// TestRepoGoReviewIsForkedFirstPartySkill pins the as-77n.1 migration:
// go-review left agent-teams/ and became a runtime-forked first-party skill
// whose orchestrator runs inline in the main session. Discovering it as a team
// again would mean the agent-team package was reintroduced.
func TestRepoGoReviewIsForkedFirstPartySkill(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "skills/go-review")); err != nil {
		t.Skip("agent-skills repo skills/go-review not present")
	}

	if _, err := os.Stat(filepath.Join(root, "agent-teams/go-review-team")); err == nil {
		t.Fatal("agent-teams/go-review-team should be gone after the inline-orchestrator migration")
	}

	out, err := Discover(root, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	s, ok := findSkill(out, KindFirst, "go-review")
	if !ok {
		t.Fatalf("expected go-review to be a first-party skill, got: %v", out)
	}
	if want := filepath.Join(root, "skills/go-review"); s.Source != want {
		t.Fatalf("expected go-review source %s, got %s", want, s.Source)
	}
	if !s.Forked {
		t.Fatal("go-review skill should be runtime-forked (claude+codex)")
	}
	if _, ok := findSkill(out, KindTeam, "go-review"); ok {
		t.Fatal("go-review should no longer be discovered as an agent team")
	}
	if _, ok := findSkill(out, KindTeamHybrid, "go-review"); ok {
		t.Fatal("go-review should no longer be discovered as a hybrid agent team")
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

// goReviewRoles are the four focus areas / shared role briefs of the go-review
// skill.
var goReviewRoles = []string{
	"structure-reviewer",
	"error-reviewer",
	"style-reviewer",
	"security-reviewer",
}

// TestGoReviewRolesAreSelfContainedPrompts pins the option-A enforcement
// decision from as-77n: the role briefs are runtime-neutral prompt source, not
// registered agent definitions, so they carry no frontmatter and no
// Claude-only primitives — and each restates the read-only gate itself, since
// a prose gate is the only constraint a dispatched leaf worker gets.
func TestGoReviewRolesAreSelfContainedPrompts(t *testing.T) {
	root := repoRoot(t)
	rolesDir := filepath.Join(root, "skills/go-review/shared/roles")
	if _, err := os.Stat(rolesDir); err != nil {
		t.Skip("agent-skills repo skills/go-review not present")
	}

	claudeOnly := regexp.MustCompile(`Claude Code|Agent tool|subagent_type|TaskCreate|TaskUpdate|TaskList|TeamCreate|SendMessage|AskUserQuestion|Artifact|WebSearch|WebFetch`)

	for _, role := range goReviewRoles {
		data, err := os.ReadFile(filepath.Join(rolesDir, role+".md"))
		if err != nil {
			t.Fatalf("%s.md should live in shared/roles/: %v", role, err)
		}
		content := string(data)

		if strings.HasPrefix(content, "---") {
			t.Fatalf("%s.md is prompt source, not an agent definition — it must not carry frontmatter", role)
		}
		if !strings.Contains(content, "<HARD-GATE>") {
			t.Fatalf("%s.md must restate the read-only HARD-GATE", role)
		}
		if !strings.Contains(content, "Never mutate git state") {
			t.Fatalf("%s.md gate must cover git mutation, not just file edits", role)
		}
		if !strings.Contains(content, "leaf worker") {
			t.Fatalf("%s.md must forbid spawning further agents", role)
		}
		if m := claudeOnly.FindString(content); m != "" {
			t.Fatalf("%s.md is shared prompt source and must not use Claude-only tokens: %s", role, m)
		}
	}
}

// TestGoReviewClaudeOverlayOrchestratesInline pins the core of as-77n.1: the
// Claude overlay runs the orchestrator in the main session and dispatches leaf
// roles, rather than delegating the whole review to a registered lead agent.
func TestGoReviewClaudeOverlayOrchestratesInline(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "skills/go-review/runtimes/claude/SKILL.md"))
	if err != nil {
		t.Skip("agent-skills repo go-review claude overlay not present")
	}
	content := string(data)

	if strings.Contains(content, "Platform —") {
		t.Fatal("runtime overlays must not contain Platform blocks")
	}
	if strings.Contains(content, "review-lead") {
		t.Fatal("claude overlay must not delegate to a review-lead agent — the orchestrator runs inline")
	}
	if !strings.Contains(content, "inline as the orchestrator") {
		t.Fatal("claude overlay should state that the orchestrator runs inline")
	}
	if !strings.Contains(content, "<HARD-GATE>") {
		t.Fatal("claude overlay should carry the read-only HARD-GATE")
	}
	for _, role := range goReviewRoles {
		if !strings.Contains(content, "roles/"+role+".md") {
			t.Fatalf("claude overlay should reference roles/%s.md", role)
		}
	}
}

// TestGoReviewCodexOverlayIsSelfContained mirrors the feature-review codex
// contract: parallel fan-out via the native subagent tools, an inline
// fallback, and no Claude-only tokens.
func TestGoReviewCodexOverlayIsSelfContained(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "skills/go-review/runtimes/codex/SKILL.md"))
	if err != nil {
		t.Skip("agent-skills repo go-review codex overlay not present")
	}
	content := string(data)

	if strings.Contains(content, "Platform —") {
		t.Fatal("runtime overlays must not contain Platform blocks")
	}
	for _, role := range goReviewRoles {
		if !strings.Contains(content, "roles/"+role+".md") {
			t.Fatalf("codex overlay should reference roles/%s.md", role)
		}
	}
	// The fan-in tool is wait_agent (per Codex runtime review on PR #74) — a
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
