package tui

import (
	"strings"
	"testing"

	"agent-skills/tools/skills-tui/internal/skills"
)

// #9: control bytes (ESC, C0, DEL) are stripped from rendered skill names so a
// maliciously named directory cannot inject escape sequences into the raw-mode
// terminal.
func TestSanitizeLabelStripsControlBytes(t *testing.T) {
	in := "ev\x1b[2Jil\x07\x7fname\t"
	got := sanitizeLabel(in)
	if strings.ContainsAny(got, "\x1b\x07\x7f\t") {
		t.Fatalf("control bytes survived sanitizing: %q", got)
	}
	if got != "ev[2Jilname" {
		t.Fatalf("got %q, want %q", got, "ev[2Jilname")
	}
	if sanitizeLabel("commit") != "commit" {
		t.Fatal("ordinary names must pass through unchanged")
	}
}

// #9: a control byte in a skill name never reaches the rendered frame.
func TestRenderStripsControlBytesInNames(t *testing.T) {
	m := Model{Rows: []Row{{
		Skill:   skills.Skill{Kind: skills.KindFirst, Name: "x\x1by"},
		State:   skills.StateNotInstalled,
		Desired: skills.DesiredRemove,
	}}}
	if strings.Contains(Render(m, 24), "x\x1by") {
		t.Fatal("raw control byte from a skill name leaked into the frame")
	}
}

// #14: viewportRange centers on the selected row and clamps to both ends.
func TestViewportRange(t *testing.T) {
	cases := []struct {
		total, selected, available, wantStart, wantEnd int
	}{
		{5, 0, 10, 0, 5},  // everything fits
		{10, 0, 4, 0, 4},  // clamp to top
		{10, 9, 4, 6, 10}, // clamp to bottom
		{10, 5, 4, 3, 7},  // centered
		{10, 3, 5, 1, 6},  // centered, odd window
	}
	for _, c := range cases {
		start, end := viewportRange(c.total, c.selected, c.available)
		if start != c.wantStart || end != c.wantEnd {
			t.Errorf("viewportRange(%d,%d,%d) = (%d,%d), want (%d,%d)",
				c.total, c.selected, c.available, start, end, c.wantStart, c.wantEnd)
		}
	}
}

// #11: Close stops the key-reader pump and is safe to call more than once.
func TestKeyReaderCloseIsIdempotent(t *testing.T) {
	kr := NewKeyReader(strings.NewReader("abc"))
	kr.Close()
	kr.Close() // must not panic on a double close
}
