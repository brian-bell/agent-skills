package tui

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"time"

	"agent-skills/tools/skills-tui/internal/skills"
)

// openHandoffWindow bounds how long openPath waits for the launcher to fail
// fast. Handler-resolution failures (xdg-open with no desktop session, no
// MIME handler, or an unusable display) exit nonzero almost immediately,
// while successful handoffs (open, gio open, kde-open) exit zero just as
// fast; a launcher still alive past the window is a foreground handler or a
// slow handoff, and waiting any longer would stall the raw-mode input loop.
const openHandoffWindow = 300 * time.Millisecond

// openPath reveals a directory in the OS file manager. It is a package-level
// var so tests can stub it, mirroring runTUI in main.go and Config.RunHook.
// Spawn failures and fast nonzero exits are reported; a launcher that stays
// foreground past openHandoffWindow counts as a successful handoff and is
// reaped in the background so it cannot zombie.
var openPath = func(path string) error {
	var name string
	switch runtime.GOOS {
	case "darwin":
		name = "open"
	case "windows":
		name = "explorer"
	default:
		name = "xdg-open"
	}
	cmd := exec.Command(name, path)
	// Keep the launcher's chatter out of the raw-mode frame.
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	case <-time.After(openHandoffWindow):
	}
	return nil
}

// OpenStageDir opens the staging dir (cfg.StageDir, the ~/.skill-symlinks
// install cache) in the OS file manager, creating it first if needed — the
// installer creates it on every apply anyway, so a fresh machine gets the
// same behavior. The outcome is reported via the one-shot footer Message.
func (m *Model) OpenStageDir(cfg skills.Config) {
	// Route through the engine's MkdirAll so a staging dir first created here
	// gets the same umask-honoring mode the install paths produce.
	if err := skills.MkdirAll(cfg.StageDir); err != nil {
		m.Message = "open failed: " + sanitizeLabel(err.Error())
		return
	}
	if err := openPath(cfg.StageDir); err != nil {
		m.Message = "open failed: " + sanitizeLabel(err.Error())
		return
	}
	m.Message = "Opening " + sanitizeLabel(cfg.StageDir)
}
