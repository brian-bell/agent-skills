// Package skills implements the engine behind the agent-skills installer:
// discovery, staging, comparison, linking, and state.
package skills

import (
	"io"
	"time"
)

// Kind classifies a discovered skill: the portable first-/third-party kinds
// plus the Go-only hook kind. The agent-team kinds were removed with as-77n,
// when the two review teams became inline-orchestrator skills.
type Kind string

const (
	KindFirst Kind = "first"
	KindThird Kind = "third"
	KindHook  Kind = "hook"
)

// Skill is one discovered skill: its kind, display name, and repo source dir.
// Hook is set only for KindHook (the manifest parsed at discovery).
type Skill struct {
	Kind   Kind
	Name   string
	Source string
	Forked bool
	Hook   *HookManifest
}

// Config carries the injected environment for the engine. All paths and the
// clock are explicit; nothing in this package reads os.Getenv or time.Now.
type Config struct {
	RepoDir  string
	Home     string
	StageDir string           // ~/.skill-symlinks or $SKILL_SYMLINKS_DIR
	Targets  []Target         // normalized runtime roots (agents, claude)
	WarnW    io.Writer        // destination for warning lines
	Now      func() time.Time // clock, used for backup timestamps
	Path     string           // caller's PATH, forwarded to hook install scripts
	// RunHook executes a hook's install.sh with the given env and args; nil
	// selects the default os/exec runner.
	RunHook func(dir string, env []string, args ...string) error
}
