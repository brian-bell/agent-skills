package skills

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// mkdirAll creates dir and its parents, replacing the "mkdir -p" idiom that
// was triplicated across staging and linking. The mode is 0o777 & ~umask, like
// bash `mkdir -p`: honoring the process umask is a documented parity
// requirement (see TestLinkPathParentDirsHonorUmask), not a hardcoded 0o755.
func mkdirAll(dir string) error { return os.MkdirAll(dir, 0o777) }

// mkdirParents ensures the parent directory of path exists, creating it and
// any missing ancestors with mkdirAll's umask-honoring 0o777 & ~umask mode.
func mkdirParents(path string) error { return mkdirAll(filepath.Dir(path)) }

// StagedSource returns the staged copy path for a skill source, mirroring
// bash staged_source: portable skills stage under skills/<name>, agent teams
// under agent-teams/<source basename>.
func (c Config) StagedSource(kind Kind, name, source string) string {
	switch kind {
	case KindFirst, KindThird:
		return filepath.Join(c.StageDir, "skills", name)
	case KindTeam, KindTeamHybrid:
		return filepath.Join(c.StageDir, "agent-teams", filepath.Base(source))
	case KindHook:
		return filepath.Join(c.StageDir, "hooks", name)
	}
	return ""
}

// LegacyStagedPath returns the pre-runtime-fork staged path for a portable
// skill. It is kept as an owned symlink source during migration.
func (c Config) LegacyStagedPath(name string) string {
	return filepath.Join(c.StageDir, "skills", name)
}

// RuntimeStagedSource returns the runtime-specific staged copy path for a
// forked portable skill.
func (c Config) RuntimeStagedSource(name string, runtime Runtime) string {
	return filepath.Join(c.StageDir, "runtimes", string(runtime), "skills", name)
}

// RuntimeTeamStagedSource returns the runtime-specific staged copy path for a
// forked agent team, keyed by the team directory basename (mirroring how flat
// teams stage under agent-teams/<source basename>).
func (c Config) RuntimeTeamStagedSource(teamDir string, runtime Runtime) string {
	return filepath.Join(c.StageDir, "runtimes", string(runtime), "agent-teams", teamDir)
}

// SyncStagedSource refreshes the staged copy that installed symlinks point
// at, mirroring bash sync_staged_source: error when the source dir is
// missing, back up an existing differing staged copy first, then copy the
// source into place.
func (c Config) SyncStagedSource(source, staged string) error {
	if info, err := os.Stat(source); err != nil || !info.IsDir() {
		return fmt.Errorf("Missing skill source: %s", source)
	}

	// bash: [ -e "$staged" ] follows symlinks, so a dangling staged symlink
	// skips the backup and is simply replaced by the copy below.
	if _, err := os.Stat(staged); err == nil && !pathsMatch(staged, source, c.WarnW) {
		if err := c.BackupStagedSource(staged); err != nil {
			return err
		}
	}

	if err := mkdirParents(staged); err != nil {
		return err
	}
	return copyDirContents(source, staged)
}

// SyncAssembledStagedSource refreshes one runtime-specific staged tree by
// assembling shared assets first and then overlaying the runtime directory.
func (c Config) SyncAssembledStagedSource(shared, overlay, staged string) error {
	if info, err := os.Stat(overlay); err != nil || !info.IsDir() {
		return fmt.Errorf("Missing skill source: %s", overlay)
	}
	if info, err := os.Stat(shared); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else if !info.IsDir() {
		return fmt.Errorf("Missing skill source: %s", shared)
	}

	if _, err := os.Stat(staged); err == nil && !c.pathsMatchAssembled(staged, shared, overlay) {
		if err := c.BackupStagedSource(staged); err != nil {
			return err
		}
	}
	return copyMergedDirContents(shared, overlay, staged)
}

// BackupStagedSource snapshots a staged copy under
// <stage>/backups/<rel>/<timestamp>, mirroring bash backup_staged_source.
// It is a no-op when staged is a symlink (never follow and copy an external
// target) or not a directory. Timestamp collisions get a -N suffix starting
// at -2.
func (c Config) BackupStagedSource(staged string) error {
	if info, err := os.Lstat(staged); err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil
	}

	// Clean the stage root so a trailing slash in SKILL_SYMLINKS_DIR still
	// strips like the bash case pattern "$root"/* (staged paths themselves
	// are filepath.Join-cleaned).
	rel := filepath.Base(staged)
	if prefix := filepath.Clean(c.StageDir) + string(filepath.Separator); strings.HasPrefix(staged, prefix) {
		rel = strings.TrimPrefix(staged, prefix)
	}

	parent := filepath.Join(c.StageDir, "backups", rel)
	stamp := c.Now().Format("20060102150405")
	backup := filepath.Join(parent, stamp)
	for i := 1; ; {
		// Any Lstat error — not just ENOENT — treats the candidate as free,
		// like bash `[ -e "$backup" ]` reading false on e.g. EACCES; the copy
		// below then fails cleanly instead of probing forever.
		if _, err := os.Lstat(backup); err != nil {
			break
		}
		i++
		backup = filepath.Join(parent, fmt.Sprintf("%s-%d", stamp, i))
	}

	if err := mkdirAll(parent); err != nil {
		return err
	}
	return copyDirContents(staged, backup)
}

func (c Config) pathsMatchAssembled(actual, shared, overlay string) bool {
	tmpParent, err := os.MkdirTemp("", "agent-skills-assembled-*")
	if err != nil {
		warnUnexpected(c.WarnW, tmpParent, err)
		return false
	}
	defer os.RemoveAll(tmpParent)

	expected := filepath.Join(tmpParent, "expected")
	if err := assembleSkillTree(shared, overlay, expected); err != nil {
		if c.WarnW != nil {
			fmt.Fprintf(c.WarnW, "warning: comparison error on assembled skill tree: %v\n", err)
		}
		return false
	}
	return pathsMatch(actual, expected, c.WarnW)
}

func copyMergedDirContents(shared, overlay, dest string) error {
	pid := os.Getpid()
	tmp := fmt.Sprintf("%s.tmp.%d", dest, pid)
	bak := fmt.Sprintf("%s.bak.%d", dest, pid)
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}

	committed := false
	defer func() {
		os.RemoveAll(tmp)
		if committed {
			os.RemoveAll(bak)
		} else if _, err := os.Lstat(bak); err == nil {
			os.Rename(bak, dest)
		}
	}()

	if err := mkdirParents(dest); err != nil {
		return err
	}
	if err := assembleSkillTree(shared, overlay, tmp); err != nil {
		return err
	}

	if err := os.RemoveAll(bak); err != nil {
		return err
	}
	if _, err := os.Lstat(dest); err == nil {
		if err := os.Rename(dest, bak); err != nil {
			return err
		}
	}
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	committed = true
	return nil
}

func assembleSkillTree(shared, overlay, dest string) error {
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := mkdirAll(dest); err != nil {
		return err
	}
	if info, err := os.Stat(shared); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("Missing skill source: %s", shared)
		}
		if err := mergeTree(shared, dest); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if info, err := os.Stat(overlay); err != nil || !info.IsDir() {
		return fmt.Errorf("Missing skill source: %s", overlay)
	}
	return mergeTree(overlay, dest)
}

func mergeTree(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		info, err := os.Lstat(s)
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(s)
			if err != nil {
				return err
			}
			if err := os.RemoveAll(d); err != nil {
				return err
			}
			if err := os.Symlink(target, d); err != nil {
				return err
			}
		case info.IsDir():
			if dstInfo, err := os.Lstat(d); err == nil && !dstInfo.IsDir() {
				if err := os.RemoveAll(d); err != nil {
					return err
				}
			}
			if err := mergeTree(s, d); err != nil {
				return err
			}
		case info.Mode().IsRegular():
			if err := os.RemoveAll(d); err != nil {
				return err
			}
			if err := copyFile(s, d, info.Mode()&modeBits); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported file type for %s: %s", s, info.Mode().Type())
		}
	}
	return os.Chmod(dst, srcInfo.Mode()&modeBits)
}

// copyDirContents replaces dest with a copy of source's contents, mirroring
// bash copy_dir_contents: build the copy in a temp sibling, chmod the temp
// root to the source root's mode, then swap it into place. dest itself is
// never followed if it is a symlink — it is replaced. The swap is done by
// moving any existing dest aside first, so a failed rename never leaves dest
// missing with the new tree orphaned; on any error the temp/backup are cleaned
// up via defer.
func copyDirContents(source, dest string) error {
	pid := os.Getpid()
	tmp := fmt.Sprintf("%s.tmp.%d", dest, pid)
	bak := fmt.Sprintf("%s.bak.%d", dest, pid)
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}

	committed := false
	defer func() {
		os.RemoveAll(tmp)
		if committed {
			os.RemoveAll(bak)
		} else if _, err := os.Lstat(bak); err == nil {
			// Swap failed after moving dest aside: restore the original.
			os.Rename(bak, dest)
		}
	}()

	if err := mkdirParents(dest); err != nil {
		return err
	}
	if err := copyTree(source, tmp); err != nil {
		return err
	}

	srcInfo, err := os.Stat(source)
	if err != nil {
		return err
	}
	if err := os.Chmod(tmp, srcInfo.Mode()&modeBits); err != nil {
		return err
	}

	if err := os.RemoveAll(bak); err != nil {
		return err
	}
	if _, err := os.Lstat(dest); err == nil {
		if err := os.Rename(dest, bak); err != nil {
			return err
		}
	}
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	committed = true
	return nil
}

// copyTree recursively copies src (a directory) to dst, preserving mode bits
// (including setuid/setgid/sticky) and recreating symlinks as symlinks, like
// `rsync -a` / `cp -R`. The destination directory's final mode is applied
// only after its contents are copied, so read-only source directories stage
// fine (rsync delays directory perms until after transfer; POSIX cp -R ORs
// S_IRWXU while copying).
func copyTree(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		info, err := os.Lstat(s)
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(s)
			if err != nil {
				return err
			}
			if err := os.Symlink(target, d); err != nil {
				return err
			}
		case info.IsDir():
			if err := copyTree(s, d); err != nil {
				return err
			}
		case info.Mode().IsRegular():
			if err := copyFile(s, d, info.Mode()&modeBits); err != nil {
				return err
			}
		default:
			// Reject FIFOs, sockets, and device nodes: copyFile would os.Open
			// a FIFO and block until a writer appears, hanging the installer on
			// a hostile or malformed skill tree. bash's rsync/cp -R would copy
			// the special file, but staging one into a skill install is
			// meaningless, so fail loudly instead of silently hanging.
			return fmt.Errorf("unsupported file type for %s: %s", s, info.Mode().Type())
		}
	}
	return os.Chmod(dst, srcInfo.Mode()&modeBits)
}

// copyFile copies one regular file; mode may include setuid/setgid/sticky
// bits, which are applied by the final chmod (creation itself uses only the
// permission bits and is umask-filtered).
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	// OpenFile's perm is filtered by umask; enforce the exact bits.
	return os.Chmod(dst, mode&modeBits)
}
