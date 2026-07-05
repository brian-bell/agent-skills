package tui

import (
	"bytes"
	"io"
	"testing"
	"time"
)

// readOne decodes a single key from the given raw byte stream.
func readOne(t *testing.T, input string) string {
	t.Helper()
	k, err := NewKeyReader(bytes.NewReader([]byte(input))).ReadKey()
	if err != nil {
		t.Fatalf("ReadKey(%q) error: %v", input, err)
	}
	return k
}

// Port of bash test_read_key_parses_arrow_sequences.
func TestReadKeyParsesArrowSequences(t *testing.T) {
	if got := readOne(t, "\x1b[A"); got != "\x1b[A" {
		t.Fatalf("up arrow not parsed, got %q", got)
	}
	if got := readOne(t, "\x1b[B"); got != "\x1b[B" {
		t.Fatalf("down arrow not parsed, got %q", got)
	}
	if got := readOne(t, "q"); got != "q" {
		t.Fatalf("plain key not parsed, got %q", got)
	}
}

func TestReadKeyLoneEscape(t *testing.T) {
	if got := readOne(t, "\x1b"); got != "\x1b" {
		t.Fatalf("lone ESC not parsed, got %q", got)
	}
}

// bash read_key waits up to a full second (`read -rsn2 -t 1`) for the
// trailing bytes of an escape sequence, so an arrow keypress whose bytes
// arrive in separate packets is still decoded as an arrow, not a lone ESC.
func TestReadKeyWaitsForSlowEscapeSequence(t *testing.T) {
	pr, pw := io.Pipe()
	kr := NewKeyReader(pr)
	go func() {
		pw.Write([]byte{0x1b})
		time.Sleep(120 * time.Millisecond)
		pw.Write([]byte("[A"))
		pw.Close()
	}()

	got, err := kr.ReadKey()
	if err != nil {
		t.Fatalf("ReadKey error: %v", err)
	}
	if got != "\x1b[A" {
		t.Fatalf("slow arrow sequence decoded as %q, want ESC [ A", got)
	}
}

// Enter reads as the empty key, consistent with bash `read -rsn1` returning
// "" on the \n delimiter (and \r in raw mode).
func TestReadKeyEnterIsEmpty(t *testing.T) {
	if got := readOne(t, "\r"); got != "" {
		t.Fatalf("\\r should decode as Enter (empty), got %q", got)
	}
	if got := readOne(t, "\n"); got != "" {
		t.Fatalf("\\n should decode as Enter (empty), got %q", got)
	}
}

func TestReadKeySpaceAndSequence(t *testing.T) {
	kr := NewKeyReader(bytes.NewReader([]byte(" jk")))
	for _, want := range []string{" ", "j", "k"} {
		got, err := kr.ReadKey()
		if err != nil {
			t.Fatalf("ReadKey error: %v", err)
		}
		if got != want {
			t.Fatalf("ReadKey = %q, want %q", got, want)
		}
	}
	if _, err := kr.ReadKey(); err != io.EOF {
		t.Fatalf("exhausted reader should return io.EOF, got %v", err)
	}
}
