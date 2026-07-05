package tui

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"agent-skills/tools/skills-tui/internal/skills"
)

// bash never enters raw mode, so Ctrl-C raises SIGINT and the script exits
// non-zero via the trap. The Go loop sees 0x03 in-band and must surface it as
// an interrupt, not a clean quit.
func TestRunLoopCtrlCReturnsInterrupted(t *testing.T) {
	err := runLoop(skills.Config{}, &Model{}, NewKeyReader(bytes.NewReader([]byte("\x03"))), io.Discard, 24)
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("Ctrl-C should return ErrInterrupted, got %v", err)
	}
}

// q, lone ESC, and stream exhaustion remain clean quits (bash break / read
// failure paths, exit 0).
func TestRunLoopCleanQuits(t *testing.T) {
	for _, input := range []string{"q", "\x1b", ""} {
		err := runLoop(skills.Config{}, &Model{}, NewKeyReader(bytes.NewReader([]byte(input))), io.Discard, 24)
		if err != nil {
			t.Fatalf("input %q should quit cleanly, got %v", input, err)
		}
	}
}
