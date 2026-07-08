package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-skills/tools/skills-tui/internal/skills"
	"agent-skills/tools/skills-tui/internal/tui"
)

// makeRepo builds a throwaway repo fixture mirroring the bash test suite's
// make_repo, plus an AGENTS.md marker (the Go CLI resolves the repo by
// walking up to a dir containing skills/ and AGENTS.md). go-review-team is
// hybrid like the real repo so the roundtrip test can assert the
// ~/.agents/skills/go-review link the bash test checks.
func makeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("AGENTS.md", "agent context\n")
	write("skills/commit/SKILL.md", "commit skill\n")
	write("skills/tdd/SKILL.md", "tdd skill\n")
	write("third-party/autoreview/SKILL.md", "autoreview skill\n")
	write("third-party/ATTRIBUTION.md", "stub\n")
	write("agent-teams/go-review-team/review-lead.md", "lead\n")
	write("agent-teams/go-review-team/SKILL.md", "manifest\n")
	write("agent-teams/go-review-team/agents/openai.yaml", "interface:\n")
	write("agent-teams/feature-review-team/acceptance-lead.md", "acc\n")
	write("agent-teams/feature-review-team/SKILL.md", "manifest\n")

	// A stub hook: a full symmetric miniature of the real install scripts.
	// #!/bin/sh, no jq; install links the script target at its OWN dir's
	// save-session.sh (the staged copy, since the engine runs the staged
	// install.sh) via `ln -s` — an external command, so these tests stay red
	// if run() stops forwarding PATH — and writes the settings entry with the
	// manifest command VERBATIM in the exact nesting the state probe reads;
	// --uninstall reverses both.
	write("hooks/save-claude-session/hook.json", `{
  "script_source": "save-session.sh",
  "script_target": "~/.claude/hooks/save-session.sh",
  "settings_file": "~/.claude/settings.json",
  "event": "SessionEnd",
  "command": "$HOME/.claude/hooks/save-session.sh"
}
`)
	write("hooks/save-claude-session/save-session.sh", "#!/bin/sh\nexit 0\n")
	write("hooks/save-claude-session/install.sh", `#!/bin/sh
set -e
DIR=$(CDPATH= cd "$(dirname "$0")" && pwd)
TARGET="$HOME/.claude/hooks/save-session.sh"
SETTINGS="$HOME/.claude/settings.json"
if [ "${1:-}" = "--uninstall" ]; then
  rm -f "$TARGET" "$SETTINGS"
  exit 0
fi
mkdir -p "$HOME/.claude/hooks"
rm -f "$TARGET"
ln -s "$DIR/save-session.sh" "$TARGET"
printf '%s\n' '{"hooks":{"SessionEnd":[{"matcher":"","hooks":[{"type":"command","command":"$HOME/.claude/hooks/save-session.sh","timeout":30}]}]}}' > "$SETTINGS"
`)
	if err := os.Chmod(filepath.Join(dir, "hooks/save-claude-session/install.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// runCLI invokes run() in-process with a fake HOME injected via getenv and a
// non-TTY stdin/stdout.
func runCLI(t *testing.T, home string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	getenv := func(key string) string {
		switch key {
		case "HOME":
			return home
		case "PATH":
			// Hook install scripts need real tools (ln, mkdir); pass the
			// host PATH through so the fixture stub can run.
			return os.Getenv("PATH")
		}
		return ""
	}
	code := run(args, &stdout, &stderr, getenv, func() bool { return false })
	return code, stdout.String(), stderr.String()
}

func assertSymlinkTarget(t *testing.T, link, want string) {
	t.Helper()
	got, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("expected symlink at %s: %v", link, err)
	}
	if got != want {
		t.Fatalf("symlink %s points at %s, want %s", link, got, want)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
}

// Port of bash test_cli_all_then_none_roundtrip.
func TestCLIAllThenNoneRoundtrip(t *testing.T) {
	repo := makeRepo(t)
	home := t.TempDir()

	code, _, stderr := runCLI(t, home, "--all", "--repo", repo)
	if code != 0 {
		t.Fatalf("--all exited %d, stderr: %s", code, stderr)
	}

	stage := filepath.Join(home, ".skill-symlinks")
	assertSymlinkTarget(t, filepath.Join(home, ".claude/skills/commit"), filepath.Join(stage, "skills/commit"))
	assertSymlinkTarget(t, filepath.Join(home, ".agents/skills/commit"), filepath.Join(stage, "skills/commit"))
	assertSymlinkTarget(t, filepath.Join(home, ".cursor/skills/commit"), filepath.Join(stage, "skills/commit"))
	assertSymlinkTarget(t, filepath.Join(home, ".agents/skills/go-review"), filepath.Join(stage, "agent-teams/go-review-team"))
	assertSymlinkTarget(t, filepath.Join(home, ".claude/skills/go-review"), filepath.Join(stage, "agent-teams/go-review-team"))

	// Every discovered skill must read as installed.
	cfg := skills.Config{
		RepoDir:  repo,
		Home:     home,
		StageDir: stage,
		Targets:  []skills.Target{"agents", "claude", "cursor"},
		Now:      time.Now,
	}
	list, err := skills.Discover(repo, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatal("no skills discovered in fixture")
	}
	for _, s := range list {
		if st := cfg.SkillState(s); st != skills.StateInstalled {
			t.Fatalf("after --all, %s state = %s, want installed", s.Name, st)
		}
	}

	code, _, stderr = runCLI(t, home, "--none", "--repo", repo)
	if code != 0 {
		t.Fatalf("--none exited %d, stderr: %s", code, stderr)
	}
	for _, link := range []string{
		".claude/skills/commit",
		".agents/skills/commit",
		".cursor/skills/commit",
		".agents/skills/go-review",
		".claude/skills/go-review",
	} {
		if _, err := os.Lstat(filepath.Join(home, link)); !os.IsNotExist(err) {
			t.Fatalf("--none should remove %s", link)
		}
	}
	// Shared skills roots must survive.
	for _, root := range []string{".claude/skills", ".agents/skills", ".cursor/skills"} {
		info, err := os.Stat(filepath.Join(home, root))
		if err != nil || !info.IsDir() {
			t.Fatalf("--none removed shared root %s (err=%v)", root, err)
		}
	}
}

func TestCLIForceImpliesAll(t *testing.T) {
	repo := makeRepo(t)
	home := t.TempDir()

	// A real directory at a target: only --force (destroy) may replace it.
	realDir := filepath.Join(home, ".claude/skills/commit")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "SKILL.md"), []byte("different\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runCLI(t, home, "--force", "--repo", repo)
	if code != 0 {
		t.Fatalf("--force exited %d, stderr: %s", code, stderr)
	}
	// Force mode prints unindented "+ <name>" lines, exactly like bash.
	for _, want := range []string{"+ commit\n", "+ tdd\n", "+ autoreview\n", "+ feature-review\n", "+ go-review\n"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("--force output missing %q, got:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "+ installed") {
		t.Fatalf("--force must print bare \"+ <name>\" lines, got:\n%s", stdout)
	}

	stage := filepath.Join(home, ".skill-symlinks")
	assertSymlinkTarget(t, realDir, filepath.Join(stage, "skills/commit"))
	assertSymlinkTarget(t, filepath.Join(home, ".cursor/skills/tdd"), filepath.Join(stage, "skills/tdd"))
}

func TestCLIUnknownFlag(t *testing.T) {
	code, stdout, stderr := runCLI(t, t.TempDir(), "--bogus")
	if code != 1 {
		t.Fatalf("unknown flag exited %d, want 1", code)
	}
	if !strings.Contains(stderr, "Unknown option: --bogus") {
		t.Fatalf("stderr missing unknown-option message, got: %s", stderr)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Fatalf("stderr missing usage, got: %s", stderr)
	}
	if stdout != "" {
		t.Fatalf("unknown flag should print nothing to stdout, got: %s", stdout)
	}
}

func TestCLIHelp(t *testing.T) {
	code, stdout, stderr := runCLI(t, t.TempDir(), "--help")
	if code != 0 {
		t.Fatalf("--help exited %d, stderr: %s", code, stderr)
	}
	for _, want := range []string{"Usage:", "--all", "--none", "--force", "--repo", "SKILL_INSTALL_TARGETS"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("--help output missing %q, got:\n%s", want, stdout)
		}
	}
}

func TestCLIRepoMarkerWalk(t *testing.T) {
	repo := makeRepo(t)
	home := t.TempDir()

	// Run from a nested directory: run() must walk up to the repo root.
	nested := filepath.Join(repo, "skills/commit")
	chdir(t, nested)

	code, _, stderr := runCLI(t, home, "--all")
	if code != 0 {
		t.Fatalf("--all from nested dir exited %d, stderr: %s", code, stderr)
	}
	assertSymlinkTarget(t,
		filepath.Join(home, ".claude/skills/commit"),
		filepath.Join(home, ".skill-symlinks/skills/commit"))
}

func TestCLIRepoNotFound(t *testing.T) {
	// A bare temp dir has no skills/ + AGENTS.md anywhere above it.
	chdir(t, t.TempDir())

	code, _, stderr := runCLI(t, t.TempDir(), "--all")
	if code != 1 {
		t.Fatalf("expected exit 1 when no repo found, got %d", code)
	}
	if !strings.Contains(stderr, "skills/") || !strings.Contains(stderr, "AGENTS.md") {
		t.Fatalf("stderr should explain the repo markers, got: %s", stderr)
	}
}

func TestCLIExplicitRepoRejectsNonRepo(t *testing.T) {
	// A misspelled or non-repo --repo path must be rejected, not silently
	// treated as an empty repo (which would print "nothing to do" and exit 0,
	// letting scripts skip install/uninstall without noticing).
	bogus := t.TempDir() // no skills/ + AGENTS.md
	home := t.TempDir()

	code, stdout, stderr := runCLI(t, home, "--all", "--repo", bogus)
	if code != 1 {
		t.Fatalf("expected exit 1 for a non-repo --repo, got %d (stdout: %q, stderr: %q)", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "skills/") || !strings.Contains(stderr, "AGENTS.md") {
		t.Fatalf("stderr should explain the repo markers, got: %s", stderr)
	}
	if strings.Contains(stdout, "nothing to do") {
		t.Fatalf("a non-repo --repo must not reach the apply step, got stdout: %q", stdout)
	}
}

func TestCLINonTTYGuard(t *testing.T) {
	repo := makeRepo(t)

	var stdout, stderr bytes.Buffer
	home := t.TempDir()
	getenv := func(key string) string {
		if key == "HOME" {
			return home
		}
		return ""
	}
	code := run([]string{"--repo", repo}, &stdout, &stderr, getenv, func() bool { return false })
	if code != 1 {
		t.Fatalf("non-TTY TUI mode exited %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "Not a terminal. Use --all or --none for non-interactive mode.") {
		t.Fatalf("stderr missing non-TTY guard message, got: %s", stderr.String())
	}
}

// bash runs under `set -u`, so an unset HOME aborts before touching the
// filesystem. The Go port must refuse an empty HOME instead of resolving the
// managed roots relative to the working directory.
func TestCLIRefusesEmptyHome(t *testing.T) {
	repo := makeRepo(t)
	work := t.TempDir()
	chdir(t, work)

	code, _, stderr := runCLI(t, "", "--all", "--repo", repo)
	if code != 1 {
		t.Fatalf("empty HOME exited %d, want 1", code)
	}
	if !strings.Contains(stderr, "HOME") {
		t.Fatalf("stderr should mention HOME, got: %s", stderr)
	}
	for _, p := range []string{".skill-symlinks", ".agents", ".claude", ".cursor"} {
		if _, err := os.Lstat(filepath.Join(work, p)); !os.IsNotExist(err) {
			t.Fatalf("empty HOME run created %s in the working directory", p)
		}
	}
}

// bash apply_noninteractive plans every skill from a pre-apply snapshot
// (load_skills/refresh_states before apply_changes). When two skills collide
// on a staged path, a skill whose snapshot state was already 'installed' is
// skipped even though an earlier install in the same run just made it stale.
func TestCLIAllPlansFromPreApplySnapshot(t *testing.T) {
	repo := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("AGENTS.md", "agent context\n")
	write("skills/foo/SKILL.md", "first foo\n")
	write("third-party/foo/SKILL.md", "third foo\n")
	home := t.TempDir()

	// Seed: after the first --all, the shared staged copy holds the
	// third-party content and every link exists.
	if code, _, stderr := runCLI(t, home, "--all", "--repo", repo); code != 0 {
		t.Fatalf("seed --all exited %d, stderr: %s", code, stderr)
	}

	// Second --all: the snapshot reads skills/foo=upgrade,
	// third-party/foo=installed. Only the first applies; the third-party twin
	// stays gated on its snapshot state even though the upgrade just made it
	// stale again.
	code, stdout, stderr := runCLI(t, home, "--all", "--repo", repo)
	if code != 0 {
		t.Fatalf("second --all exited %d, stderr: %s", code, stderr)
	}
	if stdout != "  ^ upgraded foo\n" {
		t.Fatalf("second --all must upgrade foo exactly once (bash snapshot planning), got:\n%q", stdout)
	}
	data, err := os.ReadFile(filepath.Join(home, ".skill-symlinks/skills/foo/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "first foo\n" {
		t.Fatalf("staged copy should end at the first-party content like bash, got %q", data)
	}
}

// Ctrl-C in the TUI must exit like bash's SIGINT path (~130), not 0.
func TestCLIInterruptedTUIExitCode(t *testing.T) {
	repo := makeRepo(t)
	home := t.TempDir()

	old := runTUI
	runTUI = func(cfg skills.Config, stdout io.Writer) error { return tui.ErrInterrupted }
	defer func() { runTUI = old }()

	var stdout, stderr bytes.Buffer
	getenv := func(key string) string {
		if key == "HOME" {
			return home
		}
		return ""
	}
	code := run([]string{"--repo", repo}, &stdout, &stderr, getenv, func() bool { return true })
	if code != 130 {
		t.Fatalf("interrupted TUI exited %d, want 130", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("interrupt should not print an error, got: %s", stderr.String())
	}
}

func TestAllInstallsHooks(t *testing.T) {
	repo := makeRepo(t)
	home := t.TempDir()

	code, stdout, stderr := runCLI(t, home, "--all", "--repo", repo)
	if code != 0 {
		t.Fatalf("--all exited %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "  + installed save-claude-session\n") {
		t.Fatalf("--all must install the hook, got:\n%s%s", stdout, stderr)
	}
	assertSymlinkTarget(t,
		filepath.Join(home, ".claude/hooks/save-session.sh"),
		filepath.Join(home, ".skill-symlinks/hooks/save-claude-session/save-session.sh"))

	code, stdout, _ = runCLI(t, home, "--none", "--repo", repo)
	if code != 0 || !strings.Contains(stdout, "  - removed save-claude-session\n") {
		t.Fatalf("--none must remove the hook (exit %d), got:\n%s", code, stdout)
	}
	if _, err := os.Lstat(filepath.Join(home, ".claude/hooks/save-session.sh")); !os.IsNotExist(err) {
		t.Fatal("--none must remove the hook script symlink")
	}
}

func TestCLIApplyPrintsStatusLinesAndNothingToDo(t *testing.T) {
	repo := makeRepo(t)
	home := t.TempDir()

	_, stdout, _ := runCLI(t, home, "--all", "--repo", repo)
	if !strings.Contains(stdout, "  + installed commit\n") {
		t.Fatalf("--all output missing indented status line, got:\n%s", stdout)
	}

	// A second --all has nothing to change.
	_, stdout, _ = runCLI(t, home, "--all", "--repo", repo)
	if stdout != "  nothing to do\n" {
		t.Fatalf("idempotent --all should print \"  nothing to do\", got:\n%q", stdout)
	}

	// --none removes; a second --none has nothing to do.
	_, stdout, _ = runCLI(t, home, "--none", "--repo", repo)
	if !strings.Contains(stdout, "  - removed commit\n") {
		t.Fatalf("--none output missing removed line, got:\n%s", stdout)
	}
	_, stdout, _ = runCLI(t, home, "--none", "--repo", repo)
	if stdout != "  nothing to do\n" {
		t.Fatalf("idempotent --none should print \"  nothing to do\", got:\n%q", stdout)
	}
}
