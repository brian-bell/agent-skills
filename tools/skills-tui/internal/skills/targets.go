package skills

import (
	"fmt"
	"io"
	"strings"
)

// DefaultTargets is the full runtime-root list managed when
// SKILL_INSTALL_TARGETS is unset or empty.
const DefaultTargets = "agents,claude,cursor"

// HasTarget reports whether the given runtime root is managed.
func (c Config) HasTarget(name string) bool {
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
	if c.HasTarget("claude") {
		return true
	}
	if kind == KindTeamHybrid && c.HasTarget("agents") {
		return true
	}
	return false
}

// NormalizeTargets parses a SKILL_INSTALL_TARGETS value, mirroring bash
// normalize_install_targets: comma-separated, whitespace-trimmed,
// case-insensitive match of agents/claude/cursor, deduplicated preserving
// first-seen order. The first unknown token emits one warning line to warnW.
// An empty raw value falls back to DefaultTargets.
func NormalizeTargets(raw string, warnW io.Writer) []string {
	if raw == "" {
		raw = DefaultTargets
	}

	var list []string
	warned := false
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		canon := strings.ToLower(part)
		switch canon {
		case "agents", "claude", "cursor":
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
