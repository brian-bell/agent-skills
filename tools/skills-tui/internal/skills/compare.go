package skills

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// modeBits are the bits bash path_mode reports: `stat -c %a` (and the macOS
// `stat -f %Lp` fallback's GNU-first ordering) prints the permission bits
// including setuid/setgid/sticky, e.g. 4755 or 1777. Staging and comparison
// preserve them to stay byte-identical to the bash spec (see
// TestSyncPreservesSpecialModeBits); a divergent mask here would either strip
// bits bash keeps or read a faithful copy as perpetually stale.
const modeBits = os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky

// PathsMatch reports whether actual is an up-to-date real copy of expected,
// mirroring bash paths_match: false when either side is missing or when
// actual is a symlink; files must be byte-identical with equal permission
// bits; directories must match recursively (content and permission bits,
// excluding .DS_Store entries). Comparison errors other than "not found" are
// discarded — use pathsMatch to surface them.
func PathsMatch(actual, expected string) bool {
	return pathsMatch(actual, expected, io.Discard)
}

// pathsMatch is PathsMatch with a destination for unexpected errors. A missing
// path is an expected mismatch and is silent; any other stat/read error (e.g.
// EPERM) still yields a mismatch but is reported to warn, so a permission
// problem is not silently misread as a stale/upgradeable copy.
func pathsMatch(actual, expected string, warn io.Writer) bool {
	if lst, err := os.Lstat(actual); err != nil || lst.Mode()&os.ModeSymlink != 0 {
		warnUnexpected(warn, actual, err)
		return false
	}
	ai, err := os.Stat(actual)
	if err != nil {
		warnUnexpected(warn, actual, err)
		return false
	}
	ei, err := os.Stat(expected)
	if err != nil {
		warnUnexpected(warn, expected, err)
		return false
	}

	switch {
	case ai.IsDir() && ei.IsDir():
		return dirsMatch(actual, expected, warn)
	case ai.Mode().IsRegular() && ei.Mode().IsRegular():
		return filesMatch(actual, expected, warn)
	default:
		return false
	}
}

// warnUnexpected emits one line to warn when err is a real error rather than a
// plain "not found" (the expected, silent mismatch case).
func warnUnexpected(warn io.Writer, path string, err error) {
	if err == nil || warn == nil || os.IsNotExist(err) {
		return
	}
	fmt.Fprintf(warn, "warning: comparison error on %s: %v\n", path, err)
}

// dirsMatch recursively compares two directories, mirroring bash
// `diff -rq --exclude=.DS_Store` (bidirectional content equality) plus
// tree_modes_match (permission bits on every entry of the expected tree,
// including the roots themselves).
func dirsMatch(actual, expected string, warn io.Writer) bool {
	if !modesMatch(actual, expected, warn) {
		return false
	}

	names := map[string]bool{}
	for _, dir := range []string{actual, expected} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			warnUnexpected(warn, dir, err)
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
			warnUnexpected(warn, a, err)
			return false
		}
		ei, err := os.Stat(e)
		if err != nil {
			warnUnexpected(warn, e, err)
			return false
		}
		switch {
		case ai.IsDir() && ei.IsDir():
			if !dirsMatch(a, e, warn) {
				return false
			}
		case ai.Mode().IsRegular() && ei.Mode().IsRegular():
			if !filesMatch(a, e, warn) {
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
func filesMatch(actual, expected string, warn io.Writer) bool {
	if !modesMatch(actual, expected, warn) {
		return false
	}
	a, err := os.ReadFile(actual)
	if err != nil {
		warnUnexpected(warn, actual, err)
		return false
	}
	e, err := os.ReadFile(expected)
	if err != nil {
		warnUnexpected(warn, expected, err)
		return false
	}
	return bytes.Equal(a, e)
}

// modesMatch compares the mode bits of two paths without following symlinks,
// like bash path_mode (stat without -L lstats the path itself).
func modesMatch(actual, expected string, warn io.Writer) bool {
	ai, err := os.Lstat(actual)
	if err != nil {
		warnUnexpected(warn, actual, err)
		return false
	}
	ei, err := os.Lstat(expected)
	if err != nil {
		warnUnexpected(warn, expected, err)
		return false
	}
	return ai.Mode()&modeBits == ei.Mode()&modeBits
}
