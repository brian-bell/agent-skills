package skills

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// withUmask runs fn under the given umask, restoring the previous one.
// Parity spec: bash creates parent directories with `mkdir -p`, whose mode is
// 0777 & ~umask. The Go port must honor the process umask the same way
// instead of hardcoding a 0755 creation mode.
func withUmask(t *testing.T, mask int, fn func()) {
	t.Helper()
	old := syscall.Umask(mask)
	defer syscall.Umask(old)
	fn()
}

func dirMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}

func TestLinkPathParentDirsHonorUmask(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(tmp, "home", ".claude", "skills", "commit")

	withUmask(t, 0o002, func() {
		if err := LinkPath(source, target, false, false); err != nil {
			t.Fatalf("LinkPath: %v", err)
		}
	})

	for _, dir := range []string{
		filepath.Join(tmp, "home"),
		filepath.Join(tmp, "home", ".claude"),
		filepath.Join(tmp, "home", ".claude", "skills"),
	} {
		if got := dirMode(t, dir); got != 0o775 {
			t.Errorf("parent %s mode = %o, want 775 (bash mkdir -p under umask 002)", dir, got)
		}
	}
}

func TestSyncStagedSourceParentDirsHonorUmask(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "repo", "skills", "commit")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{StageDir: filepath.Join(tmp, "home", ".skill-symlinks"), Now: time.Now}
	staged := filepath.Join(cfg.StageDir, "skills", "commit")

	withUmask(t, 0o002, func() {
		if err := cfg.SyncStagedSource(source, staged); err != nil {
			t.Fatalf("SyncStagedSource: %v", err)
		}
	})

	for _, dir := range []string{
		cfg.StageDir,
		filepath.Join(cfg.StageDir, "skills"),
	} {
		if got := dirMode(t, dir); got != 0o775 {
			t.Errorf("parent %s mode = %o, want 775 (bash mkdir -p under umask 002)", dir, got)
		}
	}
}

func TestBackupStagedSourceParentDirsHonorUmask(t *testing.T) {
	tmp := t.TempDir()
	cfg := Config{StageDir: filepath.Join(tmp, ".skill-symlinks"), Now: time.Now}
	staged := filepath.Join(cfg.StageDir, "skills", "commit")
	if err := os.MkdirAll(staged, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staged, "SKILL.md"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	withUmask(t, 0o002, func() {
		if err := cfg.BackupStagedSource(staged); err != nil {
			t.Fatalf("BackupStagedSource: %v", err)
		}
	})

	for _, dir := range []string{
		filepath.Join(cfg.StageDir, "backups"),
		filepath.Join(cfg.StageDir, "backups", "skills"),
		filepath.Join(cfg.StageDir, "backups", "skills", "commit"),
	} {
		if got := dirMode(t, dir); got != 0o775 {
			t.Errorf("parent %s mode = %o, want 775 (bash mkdir -p under umask 002)", dir, got)
		}
	}
}
