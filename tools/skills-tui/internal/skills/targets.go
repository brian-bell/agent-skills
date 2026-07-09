package skills

import (
	"fmt"
	"io"
	"strings"
)

// Target is a managed runtime root, mirroring the bash install-target tokens.
type Target string

const (
	TargetAgents Target = "agents"
	TargetClaude Target = "claude"
	TargetCursor Target = "cursor"
)

// Runtime is the instruction overlay selected for one managed target root.
type Runtime string

const (
	RuntimeCodex  Runtime = "codex"
	RuntimeClaude Runtime = "claude"
	RuntimeCursor Runtime = "cursor"
)

// DefaultTargets is the full runtime-root list managed when
// SKILL_INSTALL_TARGETS is unset or empty.
const DefaultTargets = "agents,claude,cursor"

// HasTarget reports whether the given runtime root is managed.
func (c Config) HasTarget(name Target) bool {
	for _, t := range c.Targets {
		if t == name {
			return true
		}
	}
	return false
}

// TeamManaged reports whether a team skill of the given kind is managed,
// mirroring bash team_skill_managed: a team is managed when at least one of
// its runtime roots is targeted — claude for any team, plus agents for
// hybrid teams (which also link into ~/.agents).
func (c Config) TeamManaged(kind Kind) bool {
	if c.HasTarget(TargetClaude) {
		return true
	}
	if kind == KindTeamHybrid && c.HasTarget(TargetAgents) {
		return true
	}
	return false
}

// SkipsTeam reports whether a team skill of the given kind should be skipped
// because none of its runtime roots are targeted. Non-team kinds are never
// skipped. Extracted from the guard that was copy-pasted across install,
// uninstall, and state computation.
func (c Config) SkipsTeam(kind Kind) bool {
	return (kind == KindTeam || kind == KindTeamHybrid) && !c.TeamManaged(kind)
}

// SkipsForkedSkill reports whether a forked portable skill has no overlays
// for the currently selected install targets (e.g. product-manager under
// SKILL_INSTALL_TARGETS=cursor). Such skills cannot be linked into any
// managed root and must be skipped — not reported as not-installed — so
// --all does not attempt an impossible install.
func (c Config) SkipsForkedSkill(s Skill) bool {
	if !s.Forked || (s.Kind != KindFirst && s.Kind != KindThird) {
		return false
	}
	for _, root := range portableRoots {
		if !c.HasTarget(root.target) {
			continue
		}
		runtime, ok := targetRuntime(root.target)
		if ok && hasRuntimeOverlay(s.Source, runtime) {
			return false
		}
	}
	return true
}

func targetRuntime(target Target) (Runtime, bool) {
	switch target {
	case TargetAgents:
		return RuntimeCodex, true
	case TargetClaude:
		return RuntimeClaude, true
	case TargetCursor:
		return RuntimeCursor, true
	default:
		return "", false
	}
}

// NormalizeTargets parses a SKILL_INSTALL_TARGETS value, mirroring bash
// normalize_install_targets: comma-separated, whitespace-trimmed,
// case-insensitive match of agents/claude/cursor, deduplicated preserving
// first-seen order. The first unknown token emits one warning line to warnW.
// An empty raw value falls back to DefaultTargets.
func NormalizeTargets(raw string, warnW io.Writer) []Target {
	if raw == "" {
		raw = DefaultTargets
	}

	var list []Target
	warned := false
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		canon := Target(strings.ToLower(part))
		switch canon {
		case TargetAgents, TargetClaude, TargetCursor:
		default:
			if !warned && warnW != nil {
				fmt.Fprintf(warnW, "Unknown install target '%s' in SKILL_INSTALL_TARGETS (expected agents, claude, cursor)\n", part)
				warned = true
			}
			continue
		}
		seen := false
		for _, have := range list {
			if have == canon {
				seen = true
				break
			}
		}
		if !seen {
			list = append(list, canon)
		}
	}
	return list
}
