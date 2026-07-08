package tui

import (
	"fmt"
	"strings"

	"agent-skills/tools/skills-tui/internal/skills"
)

// ANSI fragments, mirroring the bash script's C_* variables.
const (
	esc     = "\x1b"
	cReset  = esc + "[0m"
	cBold   = esc + "[1m"
	cDim    = esc + "[2m"
	cGreen  = esc + "[32m"
	cYellow = esc + "[33m"
	cCyan   = esc + "[36m"
)

// kindHeader names a kind's section, mirroring bash kind_header.
func kindHeader(k skills.Kind) string {
	switch k {
	case skills.KindFirst:
		return "first-party"
	case skills.KindThird:
		return "third-party"
	case skills.KindTeam:
		return "agent-teams (claude only)"
	case skills.KindTeamHybrid:
		return "agent-teams (claude + codex)"
	case skills.KindHook:
		return "hooks"
	}
	return ""
}

// statusColor maps each engine Status to its ANSI colour. The engine decides
// which status applies; the tui owns the colour, keeping ANSI out of the
// engine while preserving byte-identical output.
var statusColor = map[skills.Status]string{
	skills.StatusInstalled:        cGreen,
	skills.StatusNotInstalled:     cDim,
	skills.StatusWillBeRemoved:    cYellow,
	skills.StatusWillBeUpdated:    cYellow,
	skills.StatusUpgradeAvailable: cYellow,
	skills.StatusPartial:          cCyan,
	skills.StatusSkipped:          cDim,
}

// stateLabel renders a row's colored status text, mirroring bash state_label.
// It asks the engine which status applies and applies the colour locally.
func stateLabel(state skills.State, desired skills.Desired) string {
	st := skills.Lifecycle{State: state, Desired: desired}.Status()
	if st == skills.StatusNone {
		return ""
	}
	return statusColor[st] + st.Label() + cReset
}

// Layout constants for the frame: the fixed two-line header, the message
// footer height, and the skill-name column width.
const (
	headerRows   = 2  // title + key hints
	footerRows   = 2  // blank line + message, when a message is present
	nameColWidth = 32 // "%-32s" name column
)

// sanitizeLabel strips control bytes from a rendered name. It reuses the
// engine's SanitizeLabel so the row list and apply status lines scrub names
// identically.
func sanitizeLabel(s string) string { return skills.SanitizeLabel(s) }

// viewportRange returns the [start, end) slice of rows visible when total rows
// must fit in available lines, centered on the selected row and clamped to the
// ends. When everything fits it returns the whole range.
func viewportRange(total, selected, available int) (start, end int) {
	if total <= available {
		return 0, total
	}
	half := available / 2
	start = selected - half
	if start < 0 {
		start = 0
	}
	if start > total-available {
		start = total - available
	}
	return start, start + available
}

// Render draws one frame as a single string, byte-identical to bash render():
// the frame starts with ESC[H, every line ends with ESC[K, the skill rows are
// windowed to the terminal height centered on the selected row, an optional
// message block precedes the tail, and the frame ends with ESC[J. It never
// emits a full-screen clear.
func Render(m Model, termRows int) string {
	eol := esc + "[K"
	nl := eol + "\n"

	var rows []string
	selectedRow := 0
	prevKind := skills.Kind("")
	for i, r := range m.Rows {
		if r.Skill.Kind != prevKind {
			rows = append(rows, "  "+cBold+kindHeader(r.Skill.Kind)+cReset+eol)
			prevKind = r.Skill.Kind
		}
		var box string
		switch r.Desired {
		case skills.DesiredInstall:
			box = "[x]"
		case skills.DesiredHold:
			box = "[-]"
		default:
			box = "[ ]"
		}
		mark := " "
		if i == m.Cursor {
			mark = cBold + ">" + cReset
		}
		rows = append(rows, fmt.Sprintf("  %s %s %-*s %s%s", mark, box, nameColWidth, sanitizeLabel(r.Skill.Name), stateLabel(r.State, r.Desired), eol))
		if i == m.Cursor {
			selectedRow = len(rows) - 1
		}
	}

	footer := 0
	if m.Message != "" {
		footer = footerRows
	}
	available := termRows - headerRows - footer
	if available < 1 {
		available = 1
	}

	total := len(rows)
	start, end := viewportRange(total, selectedRow, available)

	var b strings.Builder
	b.WriteString(cBold + "  agent-skills · manage skills" + cReset + nl)
	b.WriteString(cDim + "  ↑↓ move · space toggle · a all · n none · enter apply · q quit" + cReset + nl)
	for i := start; i < end; i++ {
		b.WriteString(rows[i] + "\n")
	}
	if m.Message != "" {
		b.WriteString(nl + "  " + m.Message + nl)
	}
	out := strings.TrimSuffix(b.String(), "\n")
	return esc + "[H" + out + esc + "[J"
}
