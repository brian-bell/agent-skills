package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func stageConfig(t *testing.T) Config {
	t.Helper()
	home := t.TempDir()
	return Config{
		Home:     home,
		StageDir: filepath.Join(home, ".skill-symlinks"),
		Targets:  []string{"agents", "claude", "cursor"},
		Now:      func() time.Time { return time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC) },
	}
}

func TestSyncPreservesRootAndNestedPermissions(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")

	writeFile(t, filepath.Join(src, "nested/helper.sh"), "echo helper\n")
	if err := os.Chmod(filepath.Join(src, "nested/helper.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(src, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := cfg.SyncStagedSource(src, staged); err != nil {
		t.Fatal(err)
	}

	if got := mustMode(t, staged); got != 0o700 {
		t.Fatalf("staged root mode should match source root mode, got %o", got)
	}
	if got := mustMode(t, filepath.Join(staged, "nested/helper.sh")); got != 0o755 {
		t.Fatalf("staged helper mode should match source, got %o", got)
	}
	if !PathsMatch(staged, src) {
		t.Fatal("staged copy should match source after sync")
	}
}

// bash copy_dir_contents uses `rsync -a` (which applies directory perms after
// transfer) or `cp -R` (POSIX ORs S_IRWXU while copying), so a read-only
// source subdirectory stages fine. The Go copy must not chmod a directory
// read-only before populating it.
func TestSyncCopiesReadOnlySourceSubdir(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")

	writeFile(t, filepath.Join(src, "locked/data.txt"), "locked\n")
	if err := os.Chmod(filepath.Join(src, "locked"), 0o555); err != nil {
		t.Fatal(err)
	}
	// Restore write perms so t.TempDir cleanup can remove both trees.
	t.Cleanup(func() {
		os.Chmod(filepath.Join(src, "locked"), 0o755)
		os.Chmod(filepath.Join(staged, "locked"), 0o755)
	})

	if err := cfg.SyncStagedSource(src, staged); err != nil {
		t.Fatalf("sync of read-only source subdir failed: %v", err)
	}
	if got := mustMode(t, filepath.Join(staged, "locked")); got != 0o555 {
		t.Fatalf("staged read-only dir mode = %o, want 555", got)
	}
	data, err := os.ReadFile(filepath.Join(staged, "locked/data.txt"))
	if err != nil || string(data) != "locked\n" {
		t.Fatalf("staged file under read-only dir wrong: %q, %v", data, err)
	}
}

// bash path_mode (`stat -c %a`) reports setuid/setgid/sticky bits and
// `rsync -a` preserves them in the staged copy; the Go copy must too.
func TestSyncPreservesSpecialModeBits(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")

	writeFile(t, filepath.Join(src, "tool.sh"), "echo tool\n")
	if err := os.Chmod(filepath.Join(src, "tool.sh"), 0o755|os.ModeSetuid); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "keep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(src, "keep"), 0o755|os.ModeSticky); err != nil {
		t.Fatal(err)
	}

	if err := cfg.SyncStagedSource(src, staged); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(staged, "tool.sh"))
	if err != nil || info.Mode()&os.ModeSetuid == 0 {
		t.Fatalf("staged tool.sh lost setuid bit: mode %v, err %v", info.Mode(), err)
	}
	info, err = os.Stat(filepath.Join(staged, "keep"))
	if err != nil || info.Mode()&os.ModeSticky == 0 {
		t.Fatalf("staged keep/ lost sticky bit: mode %v, err %v", info.Mode(), err)
	}
}

func TestSyncErrorsWhenSourceMissing(t *testing.T) {
	cfg := stageConfig(t)

	err := cfg.SyncStagedSource(filepath.Join(cfg.Home, "no-such-src"), filepath.Join(cfg.StageDir, "skills/x"))
	if err == nil {
		t.Fatal("expected error for missing source")
	}
	if !strings.Contains(err.Error(), "Missing skill source:") {
		t.Fatalf("expected bash-equivalent message, got: %v", err)
	}
}

func mustMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func TestRefreshReplacesStagedSymlinkWithoutTouchingTarget(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")
	elsewhere := t.TempDir()

	writeFile(t, filepath.Join(elsewhere, "keep.txt"), "external\n")
	if err := os.MkdirAll(filepath.Dir(staged), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, staged); err != nil {
		t.Fatal(err)
	}

	if err := cfg.SyncStagedSource(src, staged); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(elsewhere, "keep.txt")); err != nil {
		t.Fatal("refresh followed staged symlink and mutated its target")
	}
	if info, err := os.Lstat(staged); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("refresh left staged path as a symlink")
	}
	if _, err := os.Stat(filepath.Join(staged, "SKILL.md")); err != nil {
		t.Fatal("refresh did not create a real staged skill copy")
	}
}

func TestSyncBacksUpDifferingStagedCopy(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")

	if err := cfg.SyncStagedSource(src, staged); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(src, "SKILL.md"), "updated skill\n")

	if err := cfg.SyncStagedSource(src, staged); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(staged, "SKILL.md"))
	if err != nil || string(data) != "updated skill\n" {
		t.Fatalf("upgrade did not refresh staged copy: %q, %v", data, err)
	}
	backup := filepath.Join(cfg.StageDir, "backups/skills/commit/20260704120000")
	data, err = os.ReadFile(filepath.Join(backup, "SKILL.md"))
	if err != nil {
		t.Fatalf("upgrade did not create a staged skill backup at %s: %v", backup, err)
	}
	if string(data) != "commit skill\n" {
		t.Fatalf("backup did not preserve the previous staged skill, got %q", data)
	}
}

func TestSyncOfIdenticalStagedCopyDoesNotBackup(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")

	if err := cfg.SyncStagedSource(src, staged); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SyncStagedSource(src, staged); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(cfg.StageDir, "backups")); !os.IsNotExist(err) {
		t.Fatal("identical staged copy must not be backed up")
	}
}

func TestRefreshOfStagedSymlinkDoesNotBackupExternalTarget(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")
	elsewhere := t.TempDir()

	writeFile(t, filepath.Join(elsewhere, "private/secret.txt"), "private\n")
	if err := os.MkdirAll(filepath.Dir(staged), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, staged); err != nil {
		t.Fatal(err)
	}

	if err := cfg.SyncStagedSource(src, staged); err != nil {
		t.Fatal(err)
	}

	backupRoot := filepath.Join(cfg.StageDir, "backups")
	var copied []string
	filepath.WalkDir(backupRoot, func(path string, d os.DirEntry, err error) error {
		if err == nil && d != nil && d.Name() == "secret.txt" {
			copied = append(copied, path)
		}
		return nil
	})
	if len(copied) != 0 {
		t.Fatalf("refresh copied staged symlink target into backup: %v", copied)
	}
}

// bash paths_match compares each entry's own (lstat) mode via path_mode, so a
// staged tree where a symlink stands in for a regular file is stale and gets
// backed up before the refresh replaces it.
func TestSyncBacksUpStagedCopyWithSymlinkSubstitution(t *testing.T) {
	cfg := stageConfig(t)
	repo := makeRepo(t)
	src := filepath.Join(repo, "skills/commit")
	staged := filepath.Join(cfg.StageDir, "skills/commit")

	if err := cfg.SyncStagedSource(src, staged); err != nil {
		t.Fatal(err)
	}
	// Replace the staged file with a symlink resolving to identical content
	// and followed mode.
	stagedFile := filepath.Join(staged, "SKILL.md")
	if err := os.Remove(stagedFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(src, "SKILL.md"), stagedFile); err != nil {
		t.Fatal(err)
	}

	if err := cfg.SyncStagedSource(src, staged); err != nil {
		t.Fatal(err)
	}

	backup := filepath.Join(cfg.StageDir, "backups/skills/commit/20260704120000")
	if _, err := os.Lstat(filepath.Join(backup, "SKILL.md")); err != nil {
		t.Fatalf("staged copy with symlink substitution was not backed up before refresh: %v", err)
	}
}

// bash strips the stage root via the case pattern "$root"/*, which tolerates
// a trailing slash on SKILL_SYMLINKS_DIR; the backup must keep the
// skills/<name> namespace either way.
func TestBackupStagedSourceWithTrailingSlashStageDir(t *testing.T) {
	cfg := stageConfig(t)
	cfg.StageDir += string(filepath.Separator)
	staged := filepath.Join(cfg.StageDir, "skills/commit")
	writeFile(t, filepath.Join(staged, "SKILL.md"), "v1\n")

	if err := cfg.BackupStagedSource(staged); err != nil {
		t.Fatal(err)
	}

	backup := filepath.Join(cfg.StageDir, "backups/skills/commit/20260704120000")
	if _, err := os.Stat(filepath.Join(backup, "SKILL.md")); err != nil {
		t.Fatalf("backup should land under backups/skills/commit even with a trailing-slash stage dir: %v", err)
	}
}

// bash `[ -e "$backup" ]` reads false on EACCES and proceeds (the copy then
// fails cleanly); the Go collision probe must not spin forever when Lstat
// fails with anything other than ENOENT.
func TestBackupCollisionProbeToleratesUnreadableParent(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits are not enforced for root")
	}
	cfg := stageConfig(t)
	staged := filepath.Join(cfg.StageDir, "skills/commit")
	writeFile(t, filepath.Join(staged, "SKILL.md"), "v1\n")
	parent := filepath.Join(cfg.StageDir, "backups/skills/commit")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(parent, 0o755) })

	done := make(chan error, 1)
	go func() { done <- cfg.BackupStagedSource(staged) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error copying into an unreadable backups parent")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("BackupStagedSource hung probing collision candidates under an unreadable parent")
	}
}

func TestBackupTimestampCollisionAddsSuffix(t *testing.T) {
	cfg := stageConfig(t)
	staged := filepath.Join(cfg.StageDir, "skills/commit")
	writeFile(t, filepath.Join(staged, "SKILL.md"), "v1\n")

	for i := 0; i < 3; i++ {
		if err := cfg.BackupStagedSource(staged); err != nil {
			t.Fatal(err)
		}
	}

	parent := filepath.Join(cfg.StageDir, "backups/skills/commit")
	for _, name := range []string{"20260704120000", "20260704120000-2", "20260704120000-3"} {
		if info, err := os.Stat(filepath.Join(parent, name)); err != nil || !info.IsDir() {
			t.Fatalf("expected backup dir %s: %v", name, err)
		}
	}
}

func TestStagedSourcePaths(t *testing.T) {
	cfg := stageConfig(t)

	cases := []struct {
		kind   Kind
		name   string
		source string
		want   string
	}{
		{KindFirst, "commit", "/repo/skills/commit", filepath.Join(cfg.StageDir, "skills/commit")},
		{KindThird, "autoreview", "/repo/third-party/autoreview", filepath.Join(cfg.StageDir, "skills/autoreview")},
		{KindTeam, "go-review", "/repo/agent-teams/go-review-team", filepath.Join(cfg.StageDir, "agent-teams/go-review-team")},
		{KindTeamHybrid, "hybrid-review", "/repo/agent-teams/hybrid-review-team", filepath.Join(cfg.StageDir, "agent-teams/hybrid-review-team")},
	}
	for _, c := range cases {
		if got := cfg.StagedSource(c.kind, c.name, c.source); got != c.want {
			t.Errorf("StagedSource(%s, %s, %s) = %s, want %s", c.kind, c.name, c.source, got, c.want)
		}
	}
}
