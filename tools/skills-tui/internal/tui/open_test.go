package tui

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// stubOpenPath swaps the platform opener for a recording stub and restores it
// after the test.
func stubOpenPath(t *testing.T, err error) (opened *[]string) {
	t.Helper()
	calls := []string{}
	orig := openPath
	openPath = func(path string) error {
		calls = append(calls, path)
		return err
	}
	t.Cleanup(func() { openPath = orig })
	return &calls
}

// OpenStageDir on a fresh home creates the missing staging dir, hands it to
// the platform opener, and confirms via the one-shot footer message.
func TestOpenStageDirCreatesDirAndOpens(t *testing.T) {
	opened := stubOpenPath(t, nil)
	cfg := testConfig(t) // StageDir under a temp home, not yet created
	m := &Model{}

	m.OpenStageDir(cfg)

	if info, err := os.Stat(cfg.StageDir); err != nil || !info.IsDir() {
		t.Fatalf("staging dir should exist after open, stat err=%v", err)
	}
	if len(*opened) != 1 || (*opened)[0] != cfg.StageDir {
		t.Fatalf("opener should be called once with the stage dir, got %v", *opened)
	}
	if !strings.Contains(m.Message, "Opening") || !strings.Contains(m.Message, cfg.StageDir) {
		t.Fatalf("success message should name the opened dir, got %q", m.Message)
	}
}

// When the platform opener fails (no xdg-open handler, etc.) the TUI keeps
// running and reports the failure through the same one-shot footer instead of
// crashing or losing the staging dir.
func TestOpenStageDirOpenerFailureSetsMessage(t *testing.T) {
	stubOpenPath(t, errors.New("no such binary"))
	cfg := testConfig(t)
	m := &Model{}

	m.OpenStageDir(cfg)

	if !strings.Contains(m.Message, "open failed") {
		t.Fatalf("failure should surface an 'open failed' message, got %q", m.Message)
	}
	if info, err := os.Stat(cfg.StageDir); err != nil || !info.IsDir() {
		t.Fatalf("staging dir should survive an opener failure, stat err=%v", err)
	}
}

// A staging dir first created by the 'o' key must get the same mode the
// install paths produce: bash `mkdir -p` parity is 0777 & ~umask, not a
// hardcoded 0755 (engine invariant, see stage.go's MkdirAll).
func TestOpenStageDirHonorsUmask(t *testing.T) {
	stubOpenPath(t, nil)
	cfg := testConfig(t)
	m := &Model{}

	old := syscall.Umask(0o002)
	defer syscall.Umask(old)
	m.OpenStageDir(cfg)

	info, err := os.Stat(cfg.StageDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o775 {
		t.Fatalf("staging dir mode = %o, want 775 (mkdir -p under umask 002)", got)
	}
}

// The staging dir path is env-configurable and may legally contain control
// bytes on POSIX; the footer reaches the raw-mode terminal, so the success
// message must scrub the path the same way row labels are scrubbed (the raw
// path still goes to the opener).
func TestOpenStageDirSanitizesSuccessMessage(t *testing.T) {
	opened := stubOpenPath(t, nil)
	cfg := testConfig(t)
	cfg.StageDir += "\x1b[2m" // legal POSIX name bytes, hostile in a terminal
	m := &Model{}

	m.OpenStageDir(cfg)

	if len(*opened) != 1 || (*opened)[0] != cfg.StageDir {
		t.Fatalf("opener should get the raw path, got %q", *opened)
	}
	for _, r := range m.Message {
		if r < 0x20 || r == 0x7f {
			t.Fatalf("footer message must not contain control byte %q: %q", r, m.Message)
		}
	}
	if !strings.Contains(m.Message, "Opening ") {
		t.Fatalf("message should still confirm the open, got %q", m.Message)
	}
}

// shadowLauncher puts a fake platform opener (a shell script with the given
// body) first on PATH so tests exercise the real openPath without launching a
// real file manager. Unix-only; skipped on Windows.
func shadowLauncher(t *testing.T, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script launcher shadow is unix-only")
	}
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	dir := t.TempDir()
	script := filepath.Join(dir, name)
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// A launcher that exits nonzero right away (xdg-open with no desktop handler)
// must surface as an error, not an "Opening ..." confirmation.
func TestOpenPathReportsFastLauncherFailure(t *testing.T) {
	shadowLauncher(t, "exit 3")
	if err := openPath(t.TempDir()); err == nil {
		t.Fatal("fast-failing launcher should report an error")
	}
}

// A launcher that stays foreground (some xdg-open handlers) must not block
// the caller: openPath returns once the handoff window passes, while the
// reaper goroutine keeps the child from zombifying.
func TestOpenPathDoesNotBlockOnForegroundHandler(t *testing.T) {
	shadowLauncher(t, "sleep 2")
	start := time.Now()
	if err := openPath(t.TempDir()); err != nil {
		t.Fatalf("long-lived launcher is a successful handoff, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("openPath blocked for %v on a foreground handler", elapsed)
	}
}

// A launcher that exits zero quickly (open, gio open) reports success.
func TestOpenPathQuickSuccess(t *testing.T) {
	shadowLauncher(t, "exit 0")
	if err := openPath(t.TempDir()); err != nil {
		t.Fatalf("clean launcher should succeed, got %v", err)
	}
}

// The 'o' key opens the staging dir from the list view: the opener fires, the
// next frame carries the one-shot confirmation in the footer, and the loop
// keeps going until a normal quit key.
func TestRunLoopOpenKeyOpensStageDir(t *testing.T) {
	opened := stubOpenPath(t, nil)
	cfg := testConfig(t)
	m, err := LoadSkills(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	kr := NewKeyReader(bytes.NewReader([]byte("oq")))
	if err := runLoop(cfg, m, kr, &out, 24); err != nil {
		t.Fatalf("o then q should quit cleanly, got %v", err)
	}
	if len(*opened) != 1 || (*opened)[0] != cfg.StageDir {
		t.Fatalf("o should open the stage dir once, got %v", *opened)
	}
	if !strings.Contains(out.String(), "Opening "+cfg.StageDir) {
		t.Fatalf("frame after o should render the confirmation message, got %q", out.String())
	}
}
