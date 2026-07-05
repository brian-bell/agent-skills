package skills

import "strings"

// SanitizeLabel strips C0 control bytes (including ESC) and DEL from a string
// so a skill directory named with embedded escape sequences cannot spoof the
// TUI or trip terminal-emulator escape bugs when the name is printed — whether
// in the row list or in an apply status line, both of which reach the terminal
// while it is in raw mode. It is display-only: the raw Skill.Name is still used
// for path operations, so the installed symlink keeps the real directory name.
func SanitizeLabel(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}
