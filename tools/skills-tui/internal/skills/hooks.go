package skills

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// HookManifest is a hook's hook.json, parsed verbatim: no field is expanded
// at parse time. ScriptTarget and SettingsFile may start with "~/" and are
// expanded against Config.Home at use sites; Command is NEVER expanded — the
// hook install scripts store the literal $HOME/... string in the settings
// files and state detection compares it byte-for-byte.
type HookManifest struct {
	ScriptSource string `json:"script_source"`
	ScriptTarget string `json:"script_target"`
	SettingsFile string `json:"settings_file"`
	Event        string `json:"event"`
	Command      string `json:"command"`
}

// hookScriptTarget expands the manifest's script_target against Config.Home.
func (c Config) hookScriptTarget(m *HookManifest) string { return c.expandHome(m.ScriptTarget) }

// hookSettingsFile expands the manifest's settings_file against Config.Home.
func (c Config) hookSettingsFile(m *HookManifest) string { return c.expandHome(m.SettingsFile) }

// expandHome resolves a leading "~/" against Config.Home. There is
// deliberately no expansion for HookManifest.Command: the settings files
// store the literal $HOME/... string and it must compare verbatim.
func (c Config) expandHome(path string) string {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		return filepath.Join(c.Home, rest)
	}
	return path
}

// installHook stages the hook dir and executes the STAGED install.sh, so the
// script's own SCRIPT_DIR-derived SRC points the hook symlink at the staged
// copy (installs survive branch changes). The script's --force flag is passed
// iff the engine's destroy is set: unlike the skill engine's upgrade-time
// force (replace symlinks, non-destructive), the hook scripts' --force
// rm -rf's a real file at the script path — destroy semantics. The scripts
// relink any existing symlink without --force, so engine force maps to
// nothing.
func (c Config) installHook(s Skill, destroy bool) error {
	staged := c.StagedSource(KindHook, s.Name, s.Source)
	if err := c.SyncStagedSource(s.Source, staged); err != nil {
		return fmt.Errorf("%s: %w", s.Name, err)
	}
	var args []string
	if destroy {
		args = append(args, "--force")
	}
	if err := c.runHook(staged, args...); err != nil {
		return fmt.Errorf("%s: %w", s.Name, err)
	}
	return nil
}

// runHook executes <dir>/install.sh via the injected runner with the hook
// env: the engine's HOME plus the caller's PATH, nothing else.
func (c Config) runHook(dir string, args ...string) error {
	env := []string{"HOME=" + c.Home, "PATH=" + c.Path}
	if c.RunHook != nil {
		return c.RunHook(dir, env, args...)
	}
	return defaultRunHook(dir, env, args...)
}

// uninstallHook reverses installHook: the script's --uninstall removes the
// script-owned symlink and the settings entry. Prefer the staged copy (its
// SRC matches staged installs); fall back to the repo script when the stage
// dir is gone. The script only removes a symlink whose readlink equals its
// own SRC, so a legacy repo-pointing link from a pre-TUI install survives the
// staged run — sweep it with UnlinkOwned, which leaves foreign symlinks and
// real files untouched.
func (c Config) uninstallHook(s Skill) error {
	staged := c.StagedSource(KindHook, s.Name, s.Source)
	dir := staged
	if info, err := os.Stat(staged); err != nil || !info.IsDir() {
		dir = s.Source
	}
	var errs []error
	if err := c.runHook(dir, "--uninstall"); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", s.Name, err))
	}
	stagedScript := filepath.Join(staged, s.Hook.ScriptSource)
	repoScript := filepath.Join(s.Source, s.Hook.ScriptSource)
	if _, err := UnlinkOwned(c.hookScriptTarget(s.Hook), stagedScript, repoScript); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", s.Name, err))
	}
	return errors.Join(errs...)
}

// defaultRunHook is the os/exec-backed hook runner: it executes
// <dir>/install.sh with exactly the given env, captures combined output, and
// surfaces it only inside a non-zero-exit error. Success discards the output
// — the TUI is in raw mode mid-apply and ApplyResult.StatusLine is the only
// sanctioned stdout; failures reach the user via ApplySkill's WarnW path.
func defaultRunHook(dir string, env []string, args ...string) error {
	cmd := exec.Command(filepath.Join(dir, "install.sh"), args...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", filepath.Join(dir, "install.sh"),
			strings.Join(args, " "), err, tail(out, 800))
	}
	return nil
}

// tail returns at most the last n bytes of b as a trimmed string.
func tail(b []byte, n int) string {
	if len(b) > n {
		b = b[len(b)-n:]
	}
	return strings.TrimSpace(string(b))
}

// hookScriptState classifies the hook's installed script path, the analog of
// targetState for hook rows.
type hookScriptState int

const (
	hookScriptMissing hookScriptState = iota
	hookScriptFresh                   // symlink → staged script, staged copy matches repo
	hookScriptDiffer                  // legacy repo link, stale staged copy, foreign symlink, or real file
)

// hookState aggregates the two on-disk components of a hook install — the
// script symlink and the settings-file entry — into one skill State. Any
// differ-class script (legacy/stale/foreign/real file) reads as upgrade,
// mirroring SkillState's differ>0 priority. Note a foreign symlink with no
// settings entry therefore reads as upgrade forever (never not-installed):
// uninstall deliberately leaves foreign paths untouched, matching the skill
// engine's ownership rules.
func (c Config) hookState(s Skill) State {
	m := s.Hook
	if m == nil {
		if c.WarnW != nil {
			fmt.Fprintf(c.WarnW, "warning: hook %s has no manifest\n", s.Name)
		}
		return StateNotInstalled
	}

	script := c.hookScriptTargetState(s, m)
	settings := c.hookSettingsEntryPresent(m)

	switch {
	case script == hookScriptDiffer:
		return StateUpgrade
	case script == hookScriptMissing && !settings:
		return StateNotInstalled
	case script == hookScriptFresh && settings:
		return StateInstalled
	default:
		return StatePartial
	}
}

// hookScriptTargetState classifies what sits at the hook's script target.
func (c Config) hookScriptTargetState(s Skill, m *HookManifest) hookScriptState {
	target := c.hookScriptTarget(m)
	staged := c.StagedSource(KindHook, s.Name, s.Source)

	info, err := os.Lstat(target)
	switch {
	case err != nil:
		return hookScriptMissing
	case info.Mode()&os.ModeSymlink != 0:
		dest, rerr := os.Readlink(target)
		if rerr == nil && dest == filepath.Join(staged, m.ScriptSource) {
			if pathsMatch(staged, s.Source, c.WarnW) {
				return hookScriptFresh
			}
		}
		// Legacy repo-pointing link, stale staged copy, or foreign symlink.
		return hookScriptDiffer
	default:
		// A real file at the script path: only destroy (--force) may replace
		// it, and only the script itself does the replacing.
		return hookScriptDiffer
	}
}

// hookSettingsEntry mirrors the JSON shape both install scripts produce:
// {"hooks": {"<event>": [{"hooks": [{"command": ...}]}]}}. Only command is
// read; matcher/timeout/statusMessage are ignored (an old-timeout install
// still reads as installed — the entry is bumped on the next upgrade action).
type hookSettingsEntry struct {
	Hooks []struct {
		Command string `json:"command"`
	} `json:"hooks"`
}

// hookSettingsEntryPresent reports whether the hook's settings file carries
// an entry whose command equals the manifest command VERBATIM (the scripts
// store literal $HOME/... strings). A missing file or key is silently absent;
// a malformed file is absent with one warning, mirroring warnUnexpected's
// nil-writer tolerance.
func (c Config) hookSettingsEntryPresent(m *HookManifest) bool {
	path := c.hookSettingsFile(m)
	data, err := os.ReadFile(path)
	if err != nil {
		warnUnexpected(c.WarnW, path, err)
		return false
	}
	var doc struct {
		Hooks map[string][]hookSettingsEntry `json:"hooks"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		if c.WarnW != nil {
			fmt.Fprintf(c.WarnW, "warning: cannot parse %s: %v\n", path, err)
		}
		return false
	}
	for _, entry := range doc.Hooks[m.Event] {
		for _, h := range entry.Hooks {
			if h.Command == m.Command {
				return true
			}
		}
	}
	return false
}

// parseHookManifest reads and validates <dir>/hook.json. It needs no Config:
// all fields are returned raw, exactly as written.
func parseHookManifest(dir string) (*HookManifest, error) {
	path := filepath.Join(dir, "hook.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m HookManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for _, f := range []struct{ name, val string }{
		{"script_source", m.ScriptSource},
		{"script_target", m.ScriptTarget},
		{"settings_file", m.SettingsFile},
		{"event", m.Event},
		{"command", m.Command},
	} {
		if f.val == "" {
			return nil, fmt.Errorf("%s: missing or empty %s", path, f.name)
		}
	}
	return &m, nil
}
