package skills

import (
	"bytes"
	"os"
	"path/filepath"
)

// PathsMatch reports whether actual is an up-to-date real copy of expected,
// mirroring bash paths_match: false when either side is missing or when
// actual is a symlink; files must be byte-identical with equal permission
// bits; directories must match recursively (content and permission bits,
// excluding .DS_Store entries).
func PathsMatch(actual, expected string) bool {
	if lst, err := os.Lstat(actual); err != nil || lst.Mode()&os.ModeSymlink != 0 {
		return false
	}
	ai, err := os.Stat(actual)
	if err != nil {
		return false
	}
	ei, err := os.Stat(expected)
	if err != nil {
		return false
	}

	switch {
	case ai.IsDir() && ei.IsDir():
		return dirsMatch(actual, expected)
	case ai.Mode().IsRegular() && ei.Mode().IsRegular():
		return filesMatch(actual, expected)
	default:
		return false
	}
}

// dirsMatch recursively compares two directories, mirroring bash
// `diff -rq --exclude=.DS_Store` (bidirectional content equality) plus
// tree_modes_match (permission bits on every entry of the expected tree,
// including the roots themselves).
func dirsMatch(actual, expected string) bool {
	if !modesMatch(actual, expected) {
		return false
	}

	names := map[string]bool{}
	for _, dir := range []string{actual, expected} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return false
		}
		for _, e := range entries {
			if e.Name() == ".DS_Store" {
				continue
			}
			names[e.Name()] = true
		}
	}

	for name := range names {
		a := filepath.Join(actual, name)
		e := filepath.Join(expected, name)
		ai, err := os.Stat(a)
		if err != nil {
			return false
		}
		ei, err := os.Stat(e)
		if err != nil {
			return false
		}
		switch {
		case ai.IsDir() && ei.IsDir():
			if !dirsMatch(a, e) {
				return false
			}
		case ai.Mode().IsRegular() && ei.Mode().IsRegular():
			if !filesMatch(a, e) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// filesMatch reports byte-identical content and equal permission bits,
// mirroring bash `cmp -s` plus a path_mode comparison.
func filesMatch(actual, expected string) bool {
	if !modesMatch(actual, expected) {
		return false
	}
	a, err := os.ReadFile(actual)
	if err != nil {
		return false
	}
	e, err := os.ReadFile(expected)
	if err != nil {
		return false
	}
	return bytes.Equal(a, e)
}

// modeBits are the bits bash path_mode reports: `stat -c %a` (and the macOS
// `stat -f %Lp` fallback's GNU-first ordering) prints the permission bits
// including setuid/setgid/sticky, e.g. 4755 or 1777.
const modeBits = os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky

// modesMatch compares the mode bits of two paths without following symlinks,
// like bash path_mode (stat without -L lstats the path itself), and includes
// the special bits.
func modesMatch(actual, expected string) bool {
	ai, err := os.Lstat(actual)
	if err != nil {
		return false
	}
	ei, err := os.Lstat(expected)
	if err != nil {
		return false
	}
	return ai.Mode()&modeBits == ei.Mode()&modeBits
}
