package skills

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPathsMatchDirsIgnoreDSStoreAndCheckModes(t *testing.T) {
	dir := t.TempDir()
	actual := filepath.Join(dir, "actual")
	expected := filepath.Join(dir, "expected")
	for _, root := range []string{actual, expected} {
		writeFile(t, filepath.Join(root, "SKILL.md"), "same\n")
		writeFile(t, filepath.Join(root, "nested/helper.sh"), "echo hi\n")
	}

	if !PathsMatch(actual, expected) {
		t.Fatal("identical trees should match")
	}

	// .DS_Store noise on either side is excluded from the comparison.
	writeFile(t, filepath.Join(actual, ".DS_Store"), "junk\n")
	writeFile(t, filepath.Join(expected, "nested/.DS_Store"), "other junk\n")
	if !PathsMatch(actual, expected) {
		t.Fatal(".DS_Store entries must be excluded from dir comparison")
	}

	// A nested permission drift is a mismatch (bash tree_modes_match).
	if err := os.Chmod(filepath.Join(actual, "nested/helper.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if PathsMatch(actual, expected) {
		t.Fatal("nested permission drift must not match")
	}
	if err := os.Chmod(filepath.Join(actual, "nested/helper.sh"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Root dir mode is part of the expected tree walk ("." from find).
	if err := os.Chmod(actual, 0o700); err != nil {
		t.Fatal(err)
	}
	if PathsMatch(actual, expected) {
		t.Fatal("root permission drift must not match")
	}
	if err := os.Chmod(actual, 0o755); err != nil {
		t.Fatal(err)
	}

	// Extra real entries on either side are a mismatch (diff -rq is
	// bidirectional).
	writeFile(t, filepath.Join(actual, "extra.txt"), "extra\n")
	if PathsMatch(actual, expected) {
		t.Fatal("extra file in actual must not match")
	}
	if err := os.Remove(filepath.Join(actual, "extra.txt")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(expected, "only-expected.txt"), "extra\n")
	if PathsMatch(actual, expected) {
		t.Fatal("extra file in expected must not match")
	}
}

// bash path_mode (`stat -c %a` / `stat -f %Lp`) reports special bits, so a
// setuid/sticky-only drift reads as stale.
func TestPathsMatchComparesSpecialModeBits(t *testing.T) {
	dir := t.TempDir()
	actual := filepath.Join(dir, "actual")
	expected := filepath.Join(dir, "expected")
	for _, root := range []string{actual, expected} {
		writeFile(t, filepath.Join(root, "tool.sh"), "echo hi\n")
	}
	if !PathsMatch(actual, expected) {
		t.Fatal("identical trees should match")
	}

	if err := os.Chmod(actual, 0o755|os.ModeSticky); err != nil {
		t.Fatal(err)
	}
	if PathsMatch(actual, expected) {
		t.Fatal("sticky-bit drift on the root dir must not match")
	}
	if err := os.Chmod(actual, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(filepath.Join(actual, "tool.sh"), 0o644|os.ModeSetuid); err != nil {
		t.Fatal(err)
	}
	if PathsMatch(actual, expected) {
		t.Fatal("setuid-bit drift on a file must not match")
	}
}

// bash path_mode stats without -L, so an entry that is a symlink is compared
// by its own mode: a symlink standing in for a regular file is stale even
// when the followed content and followed mode match.
func TestPathsMatchTreatsSymlinkEntryByItsOwnMode(t *testing.T) {
	dir := t.TempDir()
	actual := filepath.Join(dir, "actual")
	expected := filepath.Join(dir, "expected")
	for _, root := range []string{actual, expected} {
		writeFile(t, filepath.Join(root, "SKILL.md"), "same\n")
	}
	if err := os.Remove(filepath.Join(actual, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(expected, "SKILL.md"), filepath.Join(actual, "SKILL.md")); err != nil {
		t.Fatal(err)
	}

	if PathsMatch(actual, expected) {
		t.Fatal("a symlink entry replacing a regular file must not match")
	}
}

// TestPathsMatchHandlesCyclicSymlinkedDir pins that a directory symlink
// pointing back at an ancestor (e.g. sub -> ..) is compared by its target,
// not dereferenced and recursed into. copyTree recreates the symlink verbatim,
// so a faithful staged copy must read as an up-to-date match rather than
// recursing forever / walking until path-depth errors.
func TestPathsMatchHandlesCyclicSymlinkedDir(t *testing.T) {
	dir := t.TempDir()
	actual := filepath.Join(dir, "actual")
	expected := filepath.Join(dir, "expected")
	for _, root := range []string{actual, expected} {
		writeFile(t, filepath.Join(root, "SKILL.md"), "same\n")
		if err := os.Symlink("..", filepath.Join(root, "sub")); err != nil {
			t.Fatal(err)
		}
	}

	done := make(chan bool, 1)
	go func() { done <- PathsMatch(actual, expected) }()
	select {
	case got := <-done:
		if !got {
			t.Fatal("two faithful copies with an identical cyclic symlink should match")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("PathsMatch recursed through a cyclic symlink instead of comparing targets")
	}

	// A differing symlink target is a real mismatch.
	if err := os.Remove(filepath.Join(actual, "sub")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../elsewhere", filepath.Join(actual, "sub")); err != nil {
		t.Fatal(err)
	}
	if PathsMatch(actual, expected) {
		t.Fatal("symlink entries with different targets must not match")
	}
}

func TestPathsMatchFilesRequireContentAndMode(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	writeFile(t, a, "same\n")
	writeFile(t, b, "same\n")

	if !PathsMatch(a, b) {
		t.Fatal("identical files with equal modes should match")
	}

	if err := os.Chmod(b, 0o755); err != nil {
		t.Fatal(err)
	}
	if PathsMatch(a, b) {
		t.Fatal("files with differing permission bits must not match")
	}
	if err := os.Chmod(b, 0o644); err != nil {
		t.Fatal(err)
	}

	writeFile(t, b, "different\n")
	if PathsMatch(a, b) {
		t.Fatal("files with differing content must not match")
	}

	// A symlink on the actual side never matches, even when it resolves to
	// identical content.
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(a, link); err != nil {
		t.Fatal(err)
	}
	if PathsMatch(link, a) {
		t.Fatal("a symlink actual must not match")
	}

	if PathsMatch(filepath.Join(dir, "missing"), a) || PathsMatch(a, filepath.Join(dir, "missing")) {
		t.Fatal("missing paths must not match")
	}
}
