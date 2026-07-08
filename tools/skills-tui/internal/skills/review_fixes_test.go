package skills

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// assertNoSwapResidue fails if any temp/backup sibling of base survives in dir.
func assertNoSwapResidue(t *testing.T, dir, base string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir %s: %v", dir, err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), base+".tmp.") || strings.HasPrefix(e.Name(), base+".bak.") {
			t.Errorf("leftover swap residue: %s", e.Name())
		}
	}
}

// #1/#18: replacing a real directory under destroy yields a symlink to source
// and leaves no .tmp/.bak residue behind.
func TestLinkPathReplaceRealDirLeavesSymlinkNoResidue(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(tmp, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(target, "old.txt"), "old\n")

	if err := LinkPath(source, target, true, true); err != nil {
		t.Fatalf("LinkPath destroy: %v", err)
	}
	info, err := os.Lstat(target)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("target is not a symlink: mode=%v err=%v", info.Mode(), err)
	}
	if dest, _ := os.Readlink(target); dest != source {
		t.Fatalf("symlink points to %q, want %q", dest, source)
	}
	assertNoSwapResidue(t, tmp, "target")
}

// #1/#18: replacing a foreign symlink under force is non-destructive — the data
// it pointed at survives — and leaves no residue.
func TestLinkPathReplaceForeignSymlinkPreservesData(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(tmp, "elsewhere")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(elsewhere, "keep.txt"), "keep\n")
	target := filepath.Join(tmp, "target")
	if err := os.Symlink(elsewhere, target); err != nil {
		t.Fatal(err)
	}

	if err := LinkPath(source, target, true, false); err != nil {
		t.Fatalf("LinkPath force: %v", err)
	}
	if dest, _ := os.Readlink(target); dest != source {
		t.Fatalf("symlink points to %q, want %q", dest, source)
	}
	if _, err := os.Stat(filepath.Join(elsewhere, "keep.txt")); err != nil {
		t.Fatalf("foreign data destroyed: %v", err)
	}
	assertNoSwapResidue(t, tmp, "target")
}

// #1: the refusal gating is unchanged — a real path without destroy and a
// symlink without force are still refused with the typed errors.
func TestLinkPathRefusalsUnchanged(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}

	realTarget := filepath.Join(tmp, "real")
	if err := os.MkdirAll(realTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	var rr *RefusedRealPathError
	if err := LinkPath(source, realTarget, true, false); !errors.As(err, &rr) {
		t.Fatalf("real path without destroy: got %v, want RefusedRealPathError", err)
	}

	symTarget := filepath.Join(tmp, "sym")
	if err := os.Symlink(filepath.Join(tmp, "nowhere"), symTarget); err != nil {
		t.Fatal(err)
	}
	var rs *RefusedSymlinkError
	if err := LinkPath(source, symTarget, false, false); !errors.As(err, &rs) {
		t.Fatalf("symlink without force: got %v, want RefusedSymlinkError", err)
	}
}

// #2: UnlinkOwned reports ownership as (removed, err): true for our own link,
// false for a foreign link or a missing target, and never an error on those.
func TestUnlinkOwnedReportsOwnership(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "staged")

	owned := filepath.Join(tmp, "owned")
	if err := os.Symlink(src, owned); err != nil {
		t.Fatal(err)
	}
	if removed, err := UnlinkOwned(owned, src); !removed || err != nil {
		t.Fatalf("owned link: got (%v, %v), want (true, nil)", removed, err)
	}
	if _, err := os.Lstat(owned); !os.IsNotExist(err) {
		t.Fatalf("owned link not removed: %v", err)
	}

	foreign := filepath.Join(tmp, "foreign")
	if err := os.Symlink(filepath.Join(tmp, "other"), foreign); err != nil {
		t.Fatal(err)
	}
	if removed, err := UnlinkOwned(foreign, src); removed || err != nil {
		t.Fatalf("foreign link: got (%v, %v), want (false, nil)", removed, err)
	}
	if _, err := os.Lstat(foreign); err != nil {
		t.Fatalf("foreign link should survive: %v", err)
	}

	if removed, err := UnlinkOwned(filepath.Join(tmp, "missing"), src); removed || err != nil {
		t.Fatalf("missing target: got (%v, %v), want (false, nil)", removed, err)
	}
}

// #3: a symlinked directory planted under skills/ is not discovered as a skill
// (bash [ -d ] would follow it; the port rejects it).
func TestDiscoverSkipsSymlinkedSkillDir(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "skills/real/SKILL.md"), "real\n")
	outside := filepath.Join(repo, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repo, "skills", "evil")); err != nil {
		t.Fatal(err)
	}

	list, err := Discover(repo, io.Discard)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	for _, s := range list {
		if s.Name == "evil" {
			t.Fatalf("symlinked skill dir was discovered: %+v", s)
		}
	}
	if len(list) != 1 || list[0].Name != "real" {
		t.Fatalf("expected only real skill, got %+v", list)
	}
}

// #8: an unreadable skills/ directory surfaces an error rather than silently
// reading as zero skills.
func TestDiscoverPropagatesReadError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	repo := t.TempDir()
	skillsDir := filepath.Join(repo, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(skillsDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(skillsDir, 0o755) })

	if _, err := Discover(repo, io.Discard); err == nil {
		t.Fatal("expected an error for an unreadable skills/ directory")
	}
}

// #4: a symlinked *.md under a team source is not exposed as an agent link
// (bash [ -f ] would follow it; the port rejects it).
func TestTeamAgentFilesRejectSymlinkedMd(t *testing.T) {
	cfg := stageConfig(t)
	repo := t.TempDir()
	teamSrc := filepath.Join(repo, "agent-teams", "demo-team")
	writeFile(t, filepath.Join(teamSrc, "SKILL.md"), "manifest\n")
	writeFile(t, filepath.Join(teamSrc, "real-lead.md"), "lead\n")
	secret := filepath.Join(repo, "secret.md")
	writeFile(t, secret, "secret\n")
	if err := os.Symlink(secret, filepath.Join(teamSrc, "evil.md")); err != nil {
		t.Fatal(err)
	}

	links := cfg.SkillLinks(Skill{Kind: KindTeam, Name: "demo", Source: teamSrc})
	for _, l := range links {
		if strings.HasSuffix(l.Target, "evil.md") {
			t.Fatalf("symlinked agent file was linked: %s", l.Target)
		}
	}
	// The real agent file must still be linked.
	found := false
	for _, l := range links {
		if strings.HasSuffix(l.Target, "real-lead.md") {
			found = true
		}
	}
	if !found {
		t.Fatal("real agent file was not linked")
	}
}

// #5: staging leaves no .tmp/.bak residue next to the staged copy.
func TestSyncStagedSourceLeavesNoResidue(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")

	if err := cfg.SyncStagedSource(src, staged); err != nil {
		t.Fatal(err)
	}
	// Re-stage after a change to exercise the swap-aside path.
	writeFile(t, filepath.Join(src, "SKILL.md"), "commit skill v2\n")
	if err := cfg.SyncStagedSource(src, staged); err != nil {
		t.Fatal(err)
	}
	assertNoSwapResidue(t, filepath.Dir(staged), "commit")
}

// #7: a comparison against a missing path is a silent mismatch, but an
// unreadable path (EPERM) reports a warning while still reporting a mismatch.
func TestPathsMatchWarnsOnUnexpectedError(t *testing.T) {
	tmp := t.TempDir()
	good := filepath.Join(tmp, "good.txt")
	writeFile(t, good, "data\n")

	var silent bytes.Buffer
	if pathsMatch(filepath.Join(tmp, "missing"), good, &silent) {
		t.Fatal("missing path should not match")
	}
	if silent.Len() != 0 {
		t.Fatalf("missing path should be silent, warned: %q", silent.String())
	}

	if os.Getuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	// A path under an unreadable directory fails to stat with EACCES (a real
	// error, not "not found"), which must be surfaced rather than silently
	// read as a mismatch.
	lockedDir := filepath.Join(tmp, "lockeddir")
	if err := os.MkdirAll(lockedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(lockedDir, "file.txt")
	writeFile(t, locked, "data\n")
	if err := os.Chmod(lockedDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(lockedDir, 0o755) })

	var warn bytes.Buffer
	if pathsMatch(locked, good, &warn) {
		t.Fatal("unreadable path should not match")
	}
	if warn.Len() == 0 {
		t.Fatal("unreadable path should warn")
	}
}

// #9: apply status lines scrub control bytes from the skill name, since they
// are printed while the TUI terminal is still in raw mode.
func TestStatusLineSanitizesName(t *testing.T) {
	for _, oc := range []Outcome{OutcomeInstalled, OutcomeUpgraded, OutcomeRemoved, OutcomePartial, OutcomeBlocked} {
		r := ApplyResult{Name: "ev\x1b[2Jil", Outcome: oc, State: StateInstalled}
		if strings.ContainsRune(r.StatusLine(), 0x1b) {
			t.Fatalf("outcome %q: ESC byte leaked into status line %q", oc, r.StatusLine())
		}
	}
	// An ordinary name is untouched (bash parity).
	if got := (ApplyResult{Name: "commit", Outcome: OutcomeInstalled}).StatusLine(); got != "+ installed commit" {
		t.Fatalf("ordinary name mangled: %q", got)
	}
}

// #12: IsTeam and SkipsTeam replace the copy-pasted team guard.
func TestIsTeamAndSkipsTeam(t *testing.T) {
	if (Skill{Kind: KindFirst}).IsTeam() || !(Skill{Kind: KindTeam}).IsTeam() || !(Skill{Kind: KindTeamHybrid}).IsTeam() {
		t.Fatal("IsTeam misclassified a kind")
	}
	claudeless := Config{Targets: []Target{TargetCursor}}
	if !claudeless.SkipsTeam(KindTeam) {
		t.Fatal("a claude-only team should be skipped when claude is not targeted")
	}
	if claudeless.SkipsTeam(KindFirst) {
		t.Fatal("a portable skill is never skipped as a team")
	}
	withClaude := Config{Targets: []Target{TargetClaude}}
	if withClaude.SkipsTeam(KindTeam) {
		t.Fatal("a team should not be skipped when claude is targeted")
	}
}

// #13: ApplyAll prints "nothing to do" when no action runs and reports change
// status.
func TestApplyAllNothingToDo(t *testing.T) {
	cfg := stageConfig(t)
	var buf bytes.Buffer
	plans := []ApplyPlan{
		{Skill: Skill{Kind: KindFirst, Name: "x"}, State: StateNotInstalled, Desired: DesiredRemove},
	}
	if changed := cfg.ApplyAll(plans, &buf); changed {
		t.Fatal("no action should report changed=false")
	}
	if got := buf.String(); got != "  nothing to do\n" {
		t.Fatalf("got %q, want %q", got, "  nothing to do\n")
	}
}

// #16: teams are grouped plain-before-hybrid by explicit rank.
func TestDiscoverGroupsTeamsByRank(t *testing.T) {
	repo := makeRepo(t)
	list, err := Discover(repo, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	lastRank := -1
	for _, s := range list {
		if !s.IsTeam() {
			continue
		}
		r := kindRank(s.Kind)
		if r < lastRank {
			t.Fatalf("teams not grouped by rank: %+v", list)
		}
		lastRank = r
	}
}
