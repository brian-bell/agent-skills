// Command skills-tui is a behavior-identical Go port of
// scripts/skills-tui.sh: an interactive installer/uninstaller for the
// agent-skills repo, with --all/--none/--force non-interactive modes.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"agent-skills/tools/skills-tui/internal/skills"
	"agent-skills/tools/skills-tui/internal/tui"
)

// usageText mirrors the bash usage() heredoc, adapted for the Go binary's
// extra --repo flag (the bash script infers the repo from its own location).
const usageText = `Usage: skills-tui [options]

Interactive skill installer/uninstaller. With no options, launches the TUI.

Options:
  --all        Install every skill (non-interactive)
  --none       Uninstall every skill (non-interactive)
  --force      Force-install everything, overwriting foreign symlinks AND real
               directories at the targets (destructive; the only path that can
               delete non-repo data)
  --repo <dir> Operate on the given skills repo. Default: walk up from the
               current directory to a directory containing skills/ and
               AGENTS.md
  -h, --help   Show this help

Environment:
  SKILL_INSTALL_TARGETS   Comma-separated runtimes to manage (default:
                          agents,claude,cursor). Portable skills link into the
                          selected roots; agent-teams install only when claude
                          is included. Install, uninstall, and state checks
                          all honor this list.
`

// runTUI is the interactive-mode hook, overridable in tests.
var runTUI = func(cfg skills.Config, stdout io.Writer) error {
	return tui.Run(cfg, stdout)
}

// run is the in-process-testable CLI entry that main() wraps. It parses
// flags like bash main(), assembles the engine Config from the injected
// environment, and dispatches to the non-interactive apply or the TUI.
func run(args []string, stdout, stderr io.Writer, getenv func(string) string, isTTY func() bool) int {
	mode := "tui"
	force := false
	repoFlag := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--all":
			mode = "all"
		case "--none":
			mode = "none"
		case "--force":
			force = true
		case "--repo":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "Missing value for --repo")
				fmt.Fprint(stderr, usageText)
				return 1
			}
			repoFlag = args[i]
		case "-h", "--help":
			fmt.Fprint(stdout, usageText)
			return 0
		default:
			fmt.Fprintf(stderr, "Unknown option: %s\n", args[i])
			fmt.Fprint(stderr, usageText)
			return 1
		}
	}

	repo, err := resolveRepo(repoFlag)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	// bash runs under `set -u` and dies at its first $HOME expansion when HOME
	// is unset. An empty HOME would turn every managed root (~/.agents,
	// ~/.claude, ~/.cursor, ~/.skill-symlinks) into a cwd-relative path, so
	// refuse before touching the filesystem.
	if getenv("HOME") == "" {
		fmt.Fprintln(stderr, "HOME is not set; cannot resolve the managed install roots (~/.agents, ~/.claude, ~/.cursor)")
		return 1
	}

	cfg := skills.Config{
		RepoDir:  repo,
		Home:     getenv("HOME"),
		StageDir: getenv("SKILL_SYMLINKS_DIR"),
		Targets:  skills.NormalizeTargets(getenv("SKILL_INSTALL_TARGETS"), stderr),
		WarnW:    stderr,
		Now:      time.Now,
	}
	if cfg.StageDir == "" {
		cfg.StageDir = filepath.Join(cfg.Home, ".skill-symlinks")
	}

	// `--force` on its own means "force-install everything" (non-interactive).
	if mode == "tui" && force {
		mode = "all"
	}

	switch mode {
	case "all":
		return applyNoninteractive(cfg, skills.DesiredInstall, force, stdout, stderr)
	case "none":
		return applyNoninteractive(cfg, skills.DesiredRemove, false, stdout, stderr)
	default:
		if !isTTY() {
			fmt.Fprintln(stderr, "Not a terminal. Use --all or --none for non-interactive mode.")
			return 1
		}
		if err := runTUI(cfg, stdout); err != nil {
			if errors.Is(err, tui.ErrInterrupted) {
				// bash dies on SIGINT via the trap with the conventional
				// 128+SIGINT code and prints no extra error.
				return 130
			}
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
}

// applyNoninteractive mirrors bash apply_noninteractive: set every skill's
// desired selection and apply. With force (bash --force plus --all), planning
// is bypassed and everything is force-installed with destroy=true, printing
// bare "+ <name>" / "! <name>" lines; otherwise the plan runs and each action
// prints its two-space-indented status line, with "  nothing to do" when no
// action was taken.
func applyNoninteractive(cfg skills.Config, want skills.Desired, force bool, stdout, stderr io.Writer) int {
	list, err := skills.Discover(cfg.RepoDir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if force && want == skills.DesiredInstall {
		// Force-relink everything, overwriting foreign symlinks AND real dirs.
		for _, s := range list {
			if (s.Kind == skills.KindTeam || s.Kind == skills.KindTeamHybrid) && !cfg.TeamManaged(s.Kind) {
				continue
			}
			if err := cfg.InstallSkill(s, true, true); err == nil {
				fmt.Fprintf(stdout, "+ %s\n", s.Name)
			} else {
				fmt.Fprintln(stderr, err)
				fmt.Fprintf(stdout, "! %s\n", s.Name)
			}
		}
		return 0
	}

	// Mirrors bash apply_changes: every skill is gated on a state snapshot
	// taken before any changes (load_skills/refresh_states run first), so an
	// earlier install in the same run cannot re-trigger a skill that already
	// read as installed — apply_skill itself still recomputes fresh state.
	states := make([]skills.State, len(list))
	for i, s := range list {
		states[i] = cfg.SkillState(s)
	}
	changed := false
	for i, s := range list {
		if skills.PlanAction(states[i], want) == skills.ActionNone {
			continue
		}
		res := cfg.ApplySkill(s, want, false)
		if line := res.StatusLine(); line != "" {
			fmt.Fprintf(stdout, "  %s\n", line)
		}
		changed = true
	}
	if !changed {
		fmt.Fprintln(stdout, "  nothing to do")
	}
	return 0
}

// resolveRepo picks the skills repo: an explicit --repo value wins;
// otherwise walk up from the working directory looking for a directory
// containing both skills/ and AGENTS.md.
func resolveRepo(flag string) (string, error) {
	if flag != "" {
		return filepath.Abs(flag)
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; ; {
		if isSkillsRepo(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no skills repo found at or above %s (expected a directory containing skills/ and AGENTS.md); use --repo <dir>", wd)
		}
		dir = parent
	}
}

// isSkillsRepo reports whether dir looks like the agent-skills repo root:
// a skills/ directory next to an AGENTS.md file.
func isSkillsRepo(dir string) bool {
	if info, err := os.Stat(filepath.Join(dir, "skills")); err != nil || !info.IsDir() {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, "AGENTS.md"))
	return err == nil && info.Mode().IsRegular()
}

func main() {
	// Bash: [ -t 0 ] && [ -t 1 ] — both stdin and stdout must be terminals.
	isTTY := func() bool {
		return isCharDevice(os.Stdin) && isCharDevice(os.Stdout)
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv, isTTY))
}

func isCharDevice(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
