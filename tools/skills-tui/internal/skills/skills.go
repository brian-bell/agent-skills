// Package skills implements the engine behind the agent-skills installer:
// discovery, staging, comparison, and (in later stages) linking and state.
// It is a behavior-identical port of scripts/skills-tui.sh.
package skills

import (
	"io"
	"time"
)

// Kind classifies a discovered skill, mirroring the bash discover_skills
// kinds: first | third | team | team-hybrid.
type Kind string

const (
	KindFirst      Kind = "first"
	KindThird      Kind = "third"
	KindTeam       Kind = "team"
	KindTeamHybrid Kind = "team-hybrid"
)

// Skill is one discovered skill: its kind, display name, and repo source dir.
type Skill struct {
	Kind   Kind
	Name   string
	Source string
}

// Config carries the injected environment for the engine. All paths and the
// clock are explicit; nothing in this package reads os.Getenv or time.Now.
type Config struct {
	RepoDir  string
	Home     string
	StageDir string           // ~/.skill-symlinks or $SKILL_SYMLINKS_DIR
	Targets  []string         // normalized runtime roots (agents, claude, cursor)
	WarnW    io.Writer        // destination for warning lines
	Now      func() time.Time // clock, used for backup timestamps
}
