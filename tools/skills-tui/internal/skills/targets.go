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
)

// Runtime is the instruction overlay selected for one managed target root.
type Runtime string

const (
	RuntimeCodex  Runtime = "codex"
	RuntimeClaude Runtime = "claude"
)

// DefaultTargets is the full runtime-root list managed when
// SKILL_INSTALL_TARGETS is unset or empty.
const DefaultTargets = "agents,claude"

// HasTarget reports whether the given runtime root is managed.
func (c Config) HasTarget(name Target) bool {
	for _, t := range c.Targets {
		if t == name {
			return true
		}
	}
	return false
}

func targetRuntime(target Target) (Runtime, bool) {
	switch target {
	case TargetAgents:
		return RuntimeCodex, true
	case TargetClaude:
		return RuntimeClaude, true
	default:
		return "", false
	}
}

// NormalizeTargets parses a SKILL_INSTALL_TARGETS value, mirroring bash
// normalize_install_targets: comma-separated, whitespace-trimmed,
// case-insensitive match of agents/claude, deduplicated preserving
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
		case TargetAgents, TargetClaude:
		default:
			if !warned && warnW != nil {
				fmt.Fprintf(warnW, "Unknown install target '%s' in SKILL_INSTALL_TARGETS (expected agents, claude)\n", part)
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
