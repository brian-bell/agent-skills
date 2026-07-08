package skills

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// claudeHookManifest is the save-claude-session manifest fixture; command is
// stored literal (the scripts write unexpanded $HOME strings into settings).
const claudeHookManifest = `{
  "script_source": "save-session.sh",
  "script_target": "~/.claude/hooks/save-session.sh",
  "settings_file": "~/.claude/settings.json",
  "event": "SessionEnd",
  "command": "$HOME/.claude/hooks/save-session.sh"
}
`

// makeHook adds a well-formed hook dir to a repo fixture and returns its path.
func makeHook(t *testing.T, repo, name, manifest string) string {
	t.Helper()
	dir := filepath.Join(repo, "hooks", name)
	writeFile(t, filepath.Join(dir, "install.sh"), "#!/bin/sh\nexit 0\n")
	writeFile(t, filepath.Join(dir, "hook.json"), manifest)
	writeFile(t, filepath.Join(dir, "save-session.sh"), "#!/bin/sh\nexit 0\n")
	return dir
}

func TestDiscoverHooks(t *testing.T) {
	repo := makeRepo(t)
	src := makeHook(t, repo, "save-claude-session", claudeHookManifest)
	// A hooks dir without a manifest is skipped with a warning.
	writeFile(t, filepath.Join(repo, "hooks/broken/install.sh"), "#!/bin/sh\nexit 0\n")

	var warn bytes.Buffer
	out, err := Discover(repo, &warn)
	if err != nil {
		t.Fatal(err)
	}

	s, ok := findSkill(out, KindHook, "save-claude-session")
	if !ok {
		t.Fatalf("expected hook save-claude-session, got: %v", out)
	}
	if s.Source != src {
		t.Fatalf("expected source %s, got %s", src, s.Source)
	}
	if s.Hook == nil {
		t.Fatal("discovered hook must carry its parsed manifest")
	}
	if s.Hook.Command != "$HOME/.claude/hooks/save-session.sh" {
		t.Fatalf("manifest command must be carried verbatim, got %q", s.Hook.Command)
	}

	// Hooks come last: nothing after the first hook row may be a non-hook.
	seenHook := false
	for _, s := range out {
		if s.Kind == KindHook {
			seenHook = true
		} else if seenHook {
			t.Fatalf("hooks must be ordered after all other kinds, got: %v", out)
		}
	}

	for _, s := range out {
		if s.Name == "broken" {
			t.Fatalf("hooks dir without hook.json must be skipped, got: %v", out)
		}
	}
	if !strings.Contains(warn.String(), "broken") {
		t.Fatalf("skipping a malformed hooks dir must warn, got: %q", warn.String())
	}
}

func TestParseHookManifest(t *testing.T) {
	repo := t.TempDir()
	dir := makeHook(t, repo, "save-claude-session", claudeHookManifest)

	m, err := parseHookManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Every field comes back raw, byte-for-byte as written — no expansion.
	want := HookManifest{
		ScriptSource: "save-session.sh",
		ScriptTarget: "~/.claude/hooks/save-session.sh",
		SettingsFile: "~/.claude/settings.json",
		Event:        "SessionEnd",
		Command:      "$HOME/.claude/hooks/save-session.sh",
	}
	if *m != want {
		t.Fatalf("parsed manifest = %+v, want %+v", *m, want)
	}

	// Missing or empty fields and bad JSON are errors.
	empty := makeHook(t, repo, "empty-field", `{"script_source":"s","script_target":"t","settings_file":"f","event":"","command":"c"}`)
	if _, err := parseHookManifest(empty); err == nil {
		t.Fatal("empty manifest field must be an error")
	}
	bad := makeHook(t, repo, "bad-json", "{not json")
	if _, err := parseHookManifest(bad); err == nil {
		t.Fatal("malformed hook.json must be an error")
	}
}

// hookFixture is one throwaway hook environment: a repo with one hook, a
// fake HOME, and a Config wired to both.
type hookFixture struct {
	cfg   Config
	skill Skill
	warn  *bytes.Buffer
}

// settingsJSON is the exact nesting both real install scripts produce, with
// the command stored as a literal $HOME/... string.
const settingsJSON = `{"hooks":{"SessionEnd":[{"matcher":"","hooks":[{"type":"command","command":"$HOME/.claude/hooks/save-session.sh","timeout":30}]}]}}`

func newHookFixture(t *testing.T) *hookFixture {
	t.Helper()
	repo := t.TempDir()
	makeHook(t, repo, "save-claude-session", claudeHookManifest)
	home := t.TempDir()
	warn := &bytes.Buffer{}
	cfg := Config{
		RepoDir:  repo,
		Home:     home,
		StageDir: filepath.Join(home, ".skill-symlinks"),
		Targets:  []Target{TargetAgents, TargetClaude, TargetCursor},
		WarnW:    warn,
	}
	list, err := Discover(repo, warn)
	if err != nil {
		t.Fatal(err)
	}
	s, ok := findSkill(list, KindHook, "save-claude-session")
	if !ok {
		t.Fatalf("fixture hook not discovered: %v", list)
	}
	return &hookFixture{cfg: cfg, skill: s, warn: warn}
}

// stage copies the repo hook dir into the staged path and returns the staged
// script path.
func (f *hookFixture) stage(t *testing.T) string {
	t.Helper()
	staged := f.cfg.StagedSource(KindHook, f.skill.Name, f.skill.Source)
	if err := f.cfg.SyncStagedSource(f.skill.Source, staged); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(staged, "save-session.sh")
}

func (f *hookFixture) linkScript(t *testing.T, dest string) {
	t.Helper()
	target := filepath.Join(f.cfg.Home, ".claude/hooks/save-session.sh")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dest, target); err != nil {
		t.Fatal(err)
	}
}

func (f *hookFixture) writeSettings(t *testing.T, content string) {
	t.Helper()
	writeFile(t, filepath.Join(f.cfg.Home, ".claude/settings.json"), content)
}

func TestHookState(t *testing.T) {
	cases := []struct {
		name     string
		setup    func(t *testing.T, f *hookFixture)
		want     State
		wantWarn bool
	}{
		{name: "nothing installed", setup: func(t *testing.T, f *hookFixture) {}, want: StateNotInstalled},
		{name: "staged link fresh with settings", setup: func(t *testing.T, f *hookFixture) {
			f.linkScript(t, f.stage(t))
			f.writeSettings(t, settingsJSON)
		}, want: StateInstalled},
		{name: "legacy repo link with settings", setup: func(t *testing.T, f *hookFixture) {
			f.linkScript(t, filepath.Join(f.skill.Source, "save-session.sh"))
			f.writeSettings(t, settingsJSON)
		}, want: StateUpgrade},
		{name: "staged link stale staged copy", setup: func(t *testing.T, f *hookFixture) {
			f.linkScript(t, f.stage(t))
			f.writeSettings(t, settingsJSON)
			writeFile(t, filepath.Join(f.skill.Source, "save-session.sh"), "#!/bin/sh\necho changed\n")
		}, want: StateUpgrade},
		{name: "foreign symlink with settings", setup: func(t *testing.T, f *hookFixture) {
			f.linkScript(t, "/somewhere/else.sh")
			f.writeSettings(t, settingsJSON)
		}, want: StateUpgrade},
		{name: "real file with settings", setup: func(t *testing.T, f *hookFixture) {
			writeFile(t, filepath.Join(f.cfg.Home, ".claude/hooks/save-session.sh"), "user's own\n")
			f.writeSettings(t, settingsJSON)
		}, want: StateUpgrade},
		{name: "link only", setup: func(t *testing.T, f *hookFixture) {
			f.linkScript(t, f.stage(t))
		}, want: StatePartial},
		{name: "settings only", setup: func(t *testing.T, f *hookFixture) {
			f.writeSettings(t, settingsJSON)
		}, want: StatePartial},
		{name: "settings without our command", setup: func(t *testing.T, f *hookFixture) {
			f.linkScript(t, f.stage(t))
			f.writeSettings(t, `{"hooks":{"SessionEnd":[{"hooks":[{"type":"command","command":"other"}]}]}}`)
		}, want: StatePartial},
		{name: "malformed settings with fresh link", setup: func(t *testing.T, f *hookFixture) {
			f.linkScript(t, f.stage(t))
			f.writeSettings(t, "{not json")
		}, want: StatePartial, wantWarn: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newHookFixture(t)
			tc.setup(t, f)
			if got := f.cfg.SkillState(f.skill); got != tc.want {
				t.Fatalf("state = %s, want %s", got, tc.want)
			}
			if tc.wantWarn && f.warn.Len() == 0 {
				t.Fatal("expected a warning for malformed settings JSON")
			}
		})
	}
}

// fakeRun installs a recording RunHook on the fixture config.
type fakeRun struct {
	dir  string
	env  []string
	args []string
	runs int
}

func (f *hookFixture) record(fr *fakeRun) {
	f.cfg.RunHook = func(dir string, env []string, args ...string) error {
		fr.dir, fr.env, fr.args = dir, env, args
		fr.runs++
		return nil
	}
}

func TestInstallHookStagesAndRuns(t *testing.T) {
	cases := []struct {
		name      string
		force     bool // engine force (set on upgrades) — must NOT reach the script
		destroy   bool
		wantForce bool
	}{
		{name: "plain install", force: false, destroy: false, wantForce: false},
		{name: "upgrade force does not destroy", force: true, destroy: false, wantForce: false},
		{name: "destroy passes --force", force: true, destroy: true, wantForce: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newHookFixture(t)
			f.cfg.Path = "/fixture/bin"
			var fr fakeRun
			f.record(&fr)

			if err := f.cfg.InstallSkill(f.skill, tc.force, tc.destroy); err != nil {
				t.Fatal(err)
			}

			staged := f.cfg.StagedSource(KindHook, f.skill.Name, f.skill.Source)
			if !PathsMatch(staged, f.skill.Source) {
				t.Fatalf("install must sync the staged copy at %s from the repo", staged)
			}
			if fr.runs != 1 || fr.dir != staged {
				t.Fatalf("runner called %d times with dir %q, want once with staged dir %q", fr.runs, fr.dir, staged)
			}
			wantEnv := []string{"HOME=" + f.cfg.Home, "PATH=/fixture/bin"}
			if len(fr.env) != 2 || fr.env[0] != wantEnv[0] || fr.env[1] != wantEnv[1] {
				t.Fatalf("runner env = %v, want %v", fr.env, wantEnv)
			}
			gotForce := false
			for _, a := range fr.args {
				if a == "--force" {
					gotForce = true
				}
			}
			if gotForce != tc.wantForce {
				t.Fatalf("script --force = %v, want %v (args %v)", gotForce, tc.wantForce, fr.args)
			}
		})
	}
}

// installEffects makes a RunHook fake that performs the real script's effects
// in the fixture HOME: link the script target at the staged copy and/or write
// the settings entry. (Only the fake writes settings JSON — engine code never
// does.)
func (f *hookFixture) installEffects(t *testing.T, link, settings bool) func(string, []string, ...string) error {
	t.Helper()
	return func(dir string, env []string, args ...string) error {
		if link {
			target := filepath.Join(f.cfg.Home, ".claude/hooks/save-session.sh")
			os.MkdirAll(filepath.Dir(target), 0o755)
			os.Remove(target)
			if err := os.Symlink(filepath.Join(dir, "save-session.sh"), target); err != nil {
				return err
			}
		}
		if settings {
			f.writeSettings(t, settingsJSON)
		}
		return nil
	}
}

func TestApplyHookOutcomes(t *testing.T) {
	t.Run("full effect installs", func(t *testing.T) {
		f := newHookFixture(t)
		f.cfg.RunHook = f.installEffects(t, true, true)
		res := f.cfg.ApplySkill(f.skill, DesiredInstall, false)
		if res.Outcome != OutcomeInstalled {
			t.Fatalf("outcome = %s, want installed", res.Outcome)
		}
	})
	t.Run("no-op script blocks", func(t *testing.T) {
		f := newHookFixture(t)
		f.cfg.RunHook = func(dir string, env []string, args ...string) error { return nil }
		res := f.cfg.ApplySkill(f.skill, DesiredInstall, false)
		if res.Outcome != OutcomeBlocked {
			t.Fatalf("outcome = %s, want blocked", res.Outcome)
		}
	})
	t.Run("symlink-only script is partial", func(t *testing.T) {
		f := newHookFixture(t)
		f.cfg.RunHook = f.installEffects(t, true, false)
		res := f.cfg.ApplySkill(f.skill, DesiredInstall, false)
		if res.Outcome != OutcomePartial {
			t.Fatalf("outcome = %s, want partial", res.Outcome)
		}
	})
	// Safety pin: a real file at the script path plus a settings entry reads
	// as upgrade, but a plain apply must never pass --force to the script —
	// the script refuses, the file survives, the outcome is blocked.
	t.Run("plain apply never destroys a real file", func(t *testing.T) {
		f := newHookFixture(t)
		realFile := filepath.Join(f.cfg.Home, ".claude/hooks/save-session.sh")
		writeFile(t, realFile, "user's own script\n")
		f.writeSettings(t, settingsJSON)

		var sawForce bool
		f.cfg.RunHook = func(dir string, env []string, args ...string) error {
			for _, a := range args {
				if a == "--force" {
					sawForce = true
				}
			}
			// Mirror the real script: without --force it refuses and exits 1.
			if !sawForce {
				return &exitError{}
			}
			os.Remove(realFile)
			return f.installEffects(t, true, true)(dir, env, args...)
		}

		if st := f.cfg.SkillState(f.skill); st != StateUpgrade {
			t.Fatalf("precondition: state = %s, want upgrade", st)
		}
		res := f.cfg.ApplySkill(f.skill, DesiredInstall, false)
		if sawForce {
			t.Fatal("plain apply must not pass --force to the hook script")
		}
		if res.Outcome != OutcomeBlocked {
			t.Fatalf("outcome = %s, want blocked", res.Outcome)
		}
		data, err := os.ReadFile(realFile)
		if err != nil || string(data) != "user's own script\n" {
			t.Fatalf("user's real file must survive a plain apply, got %q, %v", data, err)
		}

		// destroy=true (CLI --force) is the only path that may replace it.
		res = f.cfg.ApplySkill(f.skill, DesiredInstall, true)
		if !sawForce {
			t.Fatal("destroy apply must pass --force to the hook script")
		}
		if res.Outcome != OutcomeUpgraded {
			t.Fatalf("outcome = %s, want upgraded", res.Outcome)
		}
	})
}

// exitError stands in for a script's non-zero exit.
type exitError struct{}

func (*exitError) Error() string { return "install.sh exited 1" }

func TestUninstallHook(t *testing.T) {
	t.Run("runs staged uninstall when staged exists", func(t *testing.T) {
		f := newHookFixture(t)
		staged := filepath.Dir(f.stage(t))
		var fr fakeRun
		f.record(&fr)
		if err := f.cfg.UninstallSkill(f.skill); err != nil {
			t.Fatal(err)
		}
		if fr.runs != 1 || fr.dir != staged {
			t.Fatalf("runner called %d times with dir %q, want once with staged %q", fr.runs, fr.dir, staged)
		}
		if len(fr.args) != 1 || fr.args[0] != "--uninstall" {
			t.Fatalf("args = %v, want [--uninstall]", fr.args)
		}
	})
	t.Run("falls back to repo script when stage missing", func(t *testing.T) {
		f := newHookFixture(t)
		var fr fakeRun
		f.record(&fr)
		if err := f.cfg.UninstallSkill(f.skill); err != nil {
			t.Fatal(err)
		}
		if fr.runs != 1 || fr.dir != f.skill.Source {
			t.Fatalf("runner dir = %q, want repo source %q", fr.dir, f.skill.Source)
		}
	})
	t.Run("legacy repo-owned symlink is swept", func(t *testing.T) {
		f := newHookFixture(t)
		f.stage(t)
		// A legacy install links to the REPO script; the staged script's SRC
		// ownership check won't remove it, so the Go sweep must.
		f.linkScript(t, filepath.Join(f.skill.Source, "save-session.sh"))
		f.cfg.RunHook = func(dir string, env []string, args ...string) error { return nil }
		if err := f.cfg.UninstallSkill(f.skill); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(f.cfg.Home, ".claude/hooks/save-session.sh")
		if _, err := os.Lstat(target); !os.IsNotExist(err) {
			t.Fatalf("legacy repo-owned symlink must be removed, err=%v", err)
		}
	})
	t.Run("foreign symlink and real file survive", func(t *testing.T) {
		f := newHookFixture(t)
		f.linkScript(t, "/somewhere/else.sh")
		f.cfg.RunHook = func(dir string, env []string, args ...string) error { return nil }
		if err := f.cfg.UninstallSkill(f.skill); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(f.cfg.Home, ".claude/hooks/save-session.sh")
		if dest, err := os.Readlink(target); err != nil || dest != "/somewhere/else.sh" {
			t.Fatalf("foreign symlink must survive uninstall, got %q, %v", dest, err)
		}

		g := newHookFixture(t)
		realFile := filepath.Join(g.cfg.Home, ".claude/hooks/save-session.sh")
		writeFile(t, realFile, "user's own\n")
		g.cfg.RunHook = func(dir string, env []string, args ...string) error { return nil }
		if err := g.cfg.UninstallSkill(g.skill); err != nil {
			t.Fatal(err)
		}
		if data, err := os.ReadFile(realFile); err != nil || string(data) != "user's own\n" {
			t.Fatalf("real file must survive uninstall, got %q, %v", data, err)
		}
	})
}

func TestDefaultHookRunner(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "ran.txt")
	script := "#!/bin/sh\necho \"HOME=$HOME PATH=$PATH ARGS=$*\" > \"" + out + "\"\necho some output\n"
	writeFile(t, filepath.Join(dir, "install.sh"), script)
	if err := os.Chmod(filepath.Join(dir, "install.sh"), 0o755); err != nil {
		t.Fatal(err)
	}

	c := Config{Home: "/fake/home", Path: os.Getenv("PATH")}
	if err := c.runHook(dir, "--uninstall"); err != nil {
		t.Fatalf("successful script must not error (output stays out of the error), got: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want := "HOME=/fake/home PATH=" + os.Getenv("PATH") + " ARGS=--uninstall\n"
	if string(data) != want {
		t.Fatalf("script saw %q, want %q", data, want)
	}

	// Non-zero exit: the error carries the script's output.
	writeFile(t, filepath.Join(dir, "install.sh"), "#!/bin/sh\necho jq is missing >&2\nexit 1\n")
	if err := os.Chmod(filepath.Join(dir, "install.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	err = c.runHook(dir)
	if err == nil || !strings.Contains(err.Error(), "jq is missing") {
		t.Fatalf("failing script's error must carry its output, got: %v", err)
	}
}

func TestHookManifestExpansion(t *testing.T) {
	c := Config{Home: "/fake/home"}
	m := &HookManifest{
		ScriptTarget: "~/.claude/hooks/save-session.sh",
		SettingsFile: "/absolute/settings.json",
	}
	if got, want := c.hookScriptTarget(m), "/fake/home/.claude/hooks/save-session.sh"; got != want {
		t.Fatalf("hookScriptTarget = %q, want %q", got, want)
	}
	// Absolute paths pass through untouched.
	if got, want := c.hookSettingsFile(m), "/absolute/settings.json"; got != want {
		t.Fatalf("hookSettingsFile = %q, want %q", got, want)
	}
}
