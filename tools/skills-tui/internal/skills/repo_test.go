package skills

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
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

// claudeOnlyTokens are the primitives that must not appear in runtime-neutral
// prompt source (shared/) or in a Codex overlay. Mirrors the same list in
// scripts/test-forked-skills-layout.sh.
var claudeOnlyTokens = regexp.MustCompile(`Claude Code|Agent tool|subagent_type|TaskCreate|TaskUpdate|TaskList|TeamCreate|SendMessage|AskUserQuestion|Artifact|WebSearch|WebFetch|Glob|Grep`)

// reviewSkills are the two skills migrated off agent-teams/ by as-77n: both
// are runtime-forked first-party skills whose orchestrator runs inline in the
// main session and dispatches leaf reviewer roles.
var reviewSkills = []struct {
	name string
	// roles are the shared/roles/ briefs, one per focus area.
	roles []string
	// legacyLead is the registered lead agent definition the migration
	// deleted; it must not come back.
	legacyLead string
	// inlineMarker is the phrase pinning the inline-orchestrator contract in
	// the Claude overlay.
	inlineMarker string
}{
	{
		name:         "go-review",
		roles:        []string{"structure-reviewer", "error-reviewer", "style-reviewer", "security-reviewer"},
		legacyLead:   "review-lead",
		inlineMarker: "inline as the orchestrator",
	},
	{
		name:         "feature-review",
		roles:        []string{"product-reviewer", "safety-reviewer", "quality-reviewer", "maintainability-reviewer", "documentation-reviewer"},
		legacyLead:   "acceptance-lead",
		inlineMarker: "inline as the acceptance lead",
	},
}

// TestRepoRetiredReviewTeamsAreGone pins the review-skill migrations and the
// removal of agent-team package support from the installer.
func TestRepoRetiredReviewTeamsAreGone(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "skills/go-review")); err != nil {
		t.Skip("agent-skills repo skills/ not present")
	}

	if _, err := os.Stat(filepath.Join(root, "agent-teams")); err == nil {
		t.Fatal("agent-teams/ should be gone after the inline-orchestrator migration")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}

	// The team kinds no longer exist, so "not discovered as a team" is now a
	// compile-time fact. What remains checkable is that every discovered
	// entry is one of the surviving kinds.
	out, err := Discover(root, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range out {
		switch s.Kind {
		case KindFirst, KindThird, KindHook:
		default:
			t.Fatalf("unexpected kind %s for %s", s.Kind, s.Name)
		}
	}
}

// TestRepoReviewSkillsAreForkedFirstParty pins that both review skills left
// agent-teams/ and are discovered as runtime-forked first-party skills.
func TestRepoReviewSkillsAreForkedFirstParty(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "skills/go-review")); err != nil {
		t.Skip("agent-skills repo skills/ not present")
	}

	out, err := Discover(root, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	for _, rs := range reviewSkills {
		t.Run(rs.name, func(t *testing.T) {
			s, ok := findSkill(out, KindFirst, rs.name)
			if !ok {
				t.Fatalf("expected %s to be a first-party skill, got: %v", rs.name, out)
			}
			if want := filepath.Join(root, "skills", rs.name); s.Source != want {
				t.Fatalf("expected %s source %s, got %s", rs.name, want, s.Source)
			}
			if !s.Forked {
				t.Fatalf("%s should be runtime-forked (claude+codex)", rs.name)
			}
		})
	}
}

// TestReviewRolesAreSelfContainedPrompts pins the option-A enforcement
// decision from as-77n: the role briefs are runtime-neutral prompt source, not
// registered agent definitions, so they carry no frontmatter and no
// Claude-only primitives — and each restates the read-only gate itself, since
// a prose gate is the only constraint a dispatched leaf worker gets.
func TestReviewRolesAreSelfContainedPrompts(t *testing.T) {
	root := repoRoot(t)

	for _, rs := range reviewSkills {
		t.Run(rs.name, func(t *testing.T) {
			rolesDir := filepath.Join(root, "skills", rs.name, "shared/roles")
			if _, err := os.Stat(rolesDir); err != nil {
				t.Skipf("agent-skills repo skills/%s not present", rs.name)
			}

			for _, role := range rs.roles {
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
				if m := claudeOnlyTokens.FindString(content); m != "" {
					t.Fatalf("%s.md is shared prompt source and must not use Claude-only tokens: %s", role, m)
				}
			}
		})
	}
}

// TestFeatureReviewGitHubAccessIsRuntimeSpecific pins the access split from
// AGENTS.md: shared role prompts consume a method supplied by the orchestrator,
// Codex prefers the connector, and Claude defaults to gh/CLI.
func TestFeatureReviewGitHubAccessIsRuntimeSpecific(t *testing.T) {
	root := repoRoot(t)
	rolesDir := filepath.Join(root, "skills/feature-review/shared/roles")
	if _, err := os.Stat(rolesDir); err != nil {
		t.Skip("agent-skills repo skills/feature-review not present")
	}

	for _, role := range reviewSkills[1].roles {
		data, err := os.ReadFile(filepath.Join(rolesDir, role+".md"))
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		if !strings.Contains(content, "- PR access:") {
			t.Fatalf("%s.md must accept the orchestrator's runtime-specific PR access method", role)
		}
		if strings.Contains(content, "GitHub connector") {
			t.Fatalf("%s.md is shared prompt source and must not prefer a runtime-specific connector", role)
		}
	}

	for runtime, want := range map[string]string{
		"codex":  "- PR access: <prefer an installed GitHub connector; use gh when connector coverage is insufficient>",
		"claude": "- PR access: <use gh/CLI unless the user provided another integration>",
	} {
		data, err := os.ReadFile(filepath.Join(root, "skills/feature-review/runtimes", runtime, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), want) {
			t.Fatalf("%s overlay must inject its PR access policy into review context: want %q", runtime, want)
		}
	}

	claude, err := os.ReadFile(filepath.Join(root, "skills/feature-review/runtimes/claude/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(claude), "use a user-provided integration when available; otherwise use `gh`/CLI") {
		t.Fatal("claude overlay must honor a user-provided integration while gathering PR context")
	}
}

func TestFeatureReviewOpenAIShortDescriptionLength(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "skills/feature-review/runtimes/codex/agents/openai.yaml"))
	if err != nil {
		t.Skip("agent-skills repo feature-review metadata not present")
	}

	match := regexp.MustCompile(`(?m)^\s*short_description:\s*"([^"]*)"\s*$`).FindSubmatch(data)
	if match == nil {
		t.Fatal("feature-review openai.yaml must define a quoted short_description")
	}
	if count := utf8.RuneCount(match[1]); count < 25 || count > 64 {
		t.Fatalf("feature-review short_description must be 25-64 characters, got %d", count)
	}
}

// TestReviewClaudeOverlaysOrchestrateInline pins the core of as-77n: the
// Claude overlay runs the orchestrator in the main session and dispatches leaf
// roles, rather than delegating the whole review to a registered lead agent.
func TestReviewClaudeOverlaysOrchestrateInline(t *testing.T) {
	root := repoRoot(t)

	for _, rs := range reviewSkills {
		t.Run(rs.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, "skills", rs.name, "runtimes/claude/SKILL.md"))
			if err != nil {
				t.Skipf("agent-skills repo %s claude overlay not present", rs.name)
			}
			content := string(data)

			if strings.Contains(content, "Platform —") {
				t.Fatal("runtime overlays must not contain Platform blocks")
			}
			if strings.Contains(content, rs.legacyLead) {
				t.Fatalf("claude overlay must not delegate to a %s agent — the orchestrator runs inline", rs.legacyLead)
			}
			if !strings.Contains(content, rs.inlineMarker) {
				t.Fatalf("claude overlay should state that the orchestrator runs inline (%q)", rs.inlineMarker)
			}
			if !strings.Contains(content, "<HARD-GATE>") {
				t.Fatal("claude overlay should carry the read-only HARD-GATE")
			}
			for _, role := range rs.roles {
				if !strings.Contains(content, "roles/"+role+".md") {
					t.Fatalf("claude overlay should reference roles/%s.md", role)
				}
			}
		})
	}
}

// TestReviewCodexOverlaysAreSelfContained pins the Codex contract: parallel
// fan-out via the native subagent tools, an inline fallback, explicit
// partial-fan-out handling, and no Claude-only tokens.
func TestReviewCodexOverlaysAreSelfContained(t *testing.T) {
	root := repoRoot(t)

	for _, rs := range reviewSkills {
		t.Run(rs.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, "skills", rs.name, "runtimes/codex/SKILL.md"))
			if err != nil {
				t.Skipf("agent-skills repo %s codex overlay not present", rs.name)
			}
			content := string(data)

			if strings.Contains(content, "Platform —") {
				t.Fatal("runtime overlays must not contain Platform blocks")
			}
			for _, role := range rs.roles {
				if !strings.Contains(content, "roles/"+role+".md") {
					t.Fatalf("codex overlay should reference roles/%s.md", role)
				}
			}
			// The fan-in tool is wait_agent (per Codex runtime review on
			// PR #74) — a bare `wait` names a nonexistent tool and derails
			// the fan-out.
			if !strings.Contains(content, "spawn_agent") {
				t.Fatal("codex overlay should fan out reviewers via the native subagent tools")
			}
			if !strings.Contains(content, "wait_agent") {
				t.Fatal("codex overlay should fan in via wait_agent")
			}
			if !strings.Contains(content, "Fallback") {
				t.Fatal("codex overlay should document the inline fallback")
			}
			// A partially launched fan-out must not silently drop a role;
			// an unmentioned missing pass reads as a clean result.
			if !strings.Contains(content, "Partial fan-out") {
				t.Fatal("codex overlay should handle a partially launched fan-out")
			}
			if m := claudeOnlyTokens.FindString(content); m != "" {
				t.Fatalf("codex overlay must not use Claude-only tokens: %s", m)
			}
		})
	}
}
