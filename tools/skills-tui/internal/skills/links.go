package skills

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Link is one managed symlink for a skill, mirroring one line of bash
// skill_links: the installed target links to LinkSource (the staged copy),
// while state checks compare that staged copy to CompareSource (the repo
// source). Forked portable skills compare against CompareShared plus
// CompareOverlay instead of the legacy whole source directory.
type Link struct {
	Target         string
	LinkSource     string
	CompareSource  string
	CompareShared  string
	CompareOverlay string
}

// portableRoots maps each managed install target to its home-relative skills
// root directory (e.g. TargetClaude -> ".claude"). Shared by SkillLinks and
// orphan pruning so the table cannot drift.
var portableRoots = []struct {
	target Target
	dir    string
}{
	{TargetAgents, ".agents"},
	{TargetClaude, ".claude"},
	{TargetCursor, ".cursor"},
}

// SkillLinks lists the symlink pairs for a skill, mirroring bash skill_links.
// Targets limits which runtime roots are managed. Portable skills link into
// each targeted skills root; teams link into ~/.claude (skill dir plus one
// agent link per top-level *.md except SKILL.md and README.md), and hybrid
// teams additionally into ~/.agents. Forked skills skip a targeted runtime
// root whose overlay is missing (cursor-less skills emit no ~/.cursor link).
func (c Config) SkillLinks(s Skill) []Link {
	var links []Link

	switch s.Kind {
	case KindFirst, KindThird:
		for _, root := range portableRoots {
			if c.HasTarget(root.target) {
				staged := c.StagedSource(s.Kind, s.Name, s.Source)
				var shared, overlay string
				if s.Forked {
					runtime, ok := targetRuntime(root.target)
					if !ok {
						continue
					}
					if !hasRuntimeOverlay(s.Source, runtime) {
						continue
					}
					staged = c.RuntimeStagedSource(s.Name, runtime)
					shared = filepath.Join(s.Source, "shared")
					overlay = filepath.Join(s.Source, "runtimes", string(runtime))
				}
				links = append(links, Link{
					Target:         filepath.Join(c.Home, root.dir, "skills", s.Name),
					LinkSource:     staged,
					CompareSource:  s.Source,
					CompareShared:  shared,
					CompareOverlay: overlay,
				})
			}
		}
	case KindTeam, KindTeamHybrid:
		staged := c.StagedSource(s.Kind, s.Name, s.Source)
		teamdir := filepath.Base(s.Source)
		if s.Kind == KindTeamHybrid && c.HasTarget(TargetAgents) {
			links = append(links, Link{
				Target:        filepath.Join(c.Home, ".agents/skills", s.Name),
				LinkSource:    staged,
				CompareSource: s.Source,
			})
		}
		if c.HasTarget(TargetClaude) {
			links = append(links, Link{
				Target:        filepath.Join(c.Home, ".claude/skills", s.Name),
				LinkSource:    staged,
				CompareSource: s.Source,
			})
			for _, md := range teamAgentFiles(s.Source, c.WarnW) {
				links = append(links, Link{
					Target:        filepath.Join(c.Home, ".claude/agents", teamdir, md),
					LinkSource:    filepath.Join(staged, md),
					CompareSource: filepath.Join(s.Source, md),
				})
			}
		}
	}
	return links
}

// teamAgentFiles lists the top-level *.md agent definitions of a team source
// in glob order, mirroring the bash `"$source"/*.md` loop: regular files only,
// excluding the SKILL.md manifest and README.md docs. Leading-dot names are
// skipped: the bash glob never matches hidden files (nor a literal ".md",
// since '*' does not match a leading dot). Unlike bash `[ -f ]`, symlinks are
// rejected (os.Lstat, not os.Stat) so an `evil.md -> /etc/passwd` symlink can
// never be exposed as an agent definition. Read errors other than "not found"
// are reported to warn.
func teamAgentFiles(source string, warn io.Writer) []string {
	entries, err := os.ReadDir(source)
	if err != nil {
		warnUnexpected(warn, source, err)
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".md") || name == "SKILL.md" || name == "README.md" {
			continue
		}
		// os.Lstat (no follow): a symlinked *.md reports ModeSymlink, so
		// IsRegular() is false and the entry is skipped.
		if info, err := os.Lstat(filepath.Join(source, name)); err != nil || !info.Mode().IsRegular() {
			continue
		}
		out = append(out, name)
	}
	return out
}

// RefusedSymlinkError reports a target symlink that would be replaced only
// under force. Replacing a symlink is non-destructive: the data it points at
// survives.
type RefusedSymlinkError struct{ Target string }

func (e *RefusedSymlinkError) Error() string {
	return fmt.Sprintf("Refusing to replace existing symlink: %s (use --force)", e.Target)
}

// RefusedRealPathError reports a real file/directory at the target that would
// be destroyed; overwriting it requires destroy (bash --force).
type RefusedRealPathError struct{ Target string }

func (e *RefusedRealPathError) Error() string {
	return fmt.Sprintf("Refusing to overwrite real path: %s (use --force)", e.Target)
}

// isExpectedRefusal reports whether err is (or is entirely composed of)
// RefusedSymlinkError/RefusedRealPathError — the by-design "use --force"
// refusals that callers expect and must not log as unexpected failures.
func isExpectedRefusal(err error) bool {
	if err == nil {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		errs := joined.Unwrap()
		if len(errs) == 0 {
			return false
		}
		for _, e := range errs {
			if !isExpectedRefusal(e) {
				return false
			}
		}
		return true
	}
	var rs *RefusedSymlinkError
	var rr *RefusedRealPathError
	return errors.As(err, &rs) || errors.As(err, &rr)
}

// LinkPath creates one symlink, creating parent dirs, mirroring bash
// link_path. A target symlink already pointing at source is a no-op.
// Replacing a symlink pointing at another installer-owned source is allowed
// without force for legacy staged/repo migrations. Foreign symlinks still
// require force; replacing a real file/directory requires destroy — the only
// path that can lose user data. The replacement is staged as a temp symlink
// and swapped into place, so a failure mid-swap never destroys the existing
// target without a working symlink to show for it.
func LinkPath(source, target string, force, destroy bool, ownedSources ...string) error {
	info, err := os.Lstat(target)
	switch {
	case err == nil && info.Mode()&os.ModeSymlink != 0:
		if dest, rerr := os.Readlink(target); rerr == nil && dest == source {
			return nil
		} else if rerr == nil && isOwnedSymlink(dest, source, ownedSources) {
			return swapSymlink(source, target, false)
		}
		if !force {
			return &RefusedSymlinkError{Target: target}
		}
		// Replacing a symlink: rename over it atomically.
		return swapSymlink(source, target, false)
	case err == nil:
		if !destroy {
			return &RefusedRealPathError{Target: target}
		}
		// Replacing a real file/dir: move it aside, then swap in the symlink,
		// so the user's data is deleted only after the symlink is in place.
		return swapSymlink(source, target, true)
	}

	if err := mkdirParents(target); err != nil {
		return err
	}
	return os.Symlink(source, target)
}

func isOwnedSymlink(dest, source string, ownedSources []string) bool {
	if dest == source {
		return true
	}
	for _, owned := range ownedSources {
		if owned != "" && dest == owned {
			return true
		}
	}
	return false
}

// swapSymlink atomically replaces target with a symlink to source. It builds
// the new link at a temp sibling first. When target is a real path
// (moveAside=true) the existing target is renamed to a backup and only removed
// after the new link is in place; on any failure the original is restored.
func swapSymlink(source, target string, moveAside bool) error {
	pid := os.Getpid()
	tmp := fmt.Sprintf("%s.tmp.%d", target, pid)
	_ = os.Remove(tmp)
	if err := os.Symlink(source, tmp); err != nil {
		return err
	}

	if !moveAside {
		// target is a symlink: rename replaces it atomically.
		if err := os.Rename(tmp, target); err != nil {
			os.Remove(tmp)
			return err
		}
		return nil
	}

	bak := fmt.Sprintf("%s.bak.%d", target, pid)
	_ = os.RemoveAll(bak)
	if err := os.Rename(target, bak); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		os.Rename(bak, target) // restore the original
		os.Remove(tmp)
		return err
	}
	// The symlink is in place: the swap succeeded. Backup cleanup is
	// best-effort — a failed removal here must not be reported as a link error.
	os.RemoveAll(bak)
	return nil
}

// InstallSkill stages the skill source and links every managed target,
// mirroring bash install_skill. Teams whose runtime roots are not targeted
// are skipped. Link failures are collected (each wrapped with the skill name
// for batch attribution) but do not stop the remaining links.
func (c Config) InstallSkill(s Skill, force, destroy bool) error {
	if c.SkipsTeam(s.Kind) {
		return nil
	}
	if s.Kind == KindHook {
		// Hooks have no symlink loop: the staged install.sh does the linking
		// and the settings-file merge. Engine force is deliberately dropped —
		// see installHook.
		return c.installHook(s, destroy)
	}

	if !s.Forked {
		staged := c.StagedSource(s.Kind, s.Name, s.Source)
		if err := c.SyncStagedSource(s.Source, staged); err != nil {
			return fmt.Errorf("%s: %w", s.Name, err)
		}
	}

	var errs []error
	for _, l := range c.SkillLinks(s) {
		if s.Forked && needsSync(c.targetState(l)) {
			if err := c.SyncAssembledStagedSource(l.CompareShared, l.CompareOverlay, l.LinkSource); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", s.Name, err))
				continue
			}
		}
		if err := LinkPath(l.LinkSource, l.Target, force, destroy, c.ownedSources(s, l)...); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.Name, err))
		}
	}
	if err := c.pruneOrphanedForkedLinks(s); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func needsSync(st TargetStatus) bool {
	return st == TargetMissing || st == TargetStale || st == TargetForeign || st == TargetCopy
}

func (c Config) ownedSources(s Skill, l Link) []string {
	owned := []string{l.LinkSource, l.CompareSource}
	if s.Kind == KindFirst || s.Kind == KindThird {
		owned = append(owned, c.LegacyStagedPath(s.Name))
	}
	return owned
}

// pruneOrphanedForkedLinks removes installer-owned symlinks for forked-skill
// runtime roots whose overlay no longer exists (e.g. a machine that installed
// product-manager before the cursor overlay was removed). Foreign symlinks and
// real paths are left untouched. Only runs for forked first-party skills.
func (c Config) pruneOrphanedForkedLinks(s Skill) error {
	if !s.Forked || (s.Kind != KindFirst && s.Kind != KindThird) {
		return nil
	}
	var errs []error
	for _, root := range portableRoots {
		if !c.HasTarget(root.target) {
			continue
		}
		runtime, ok := targetRuntime(root.target)
		if !ok || hasRuntimeOverlay(s.Source, runtime) {
			continue
		}
		target := filepath.Join(c.Home, root.dir, "skills", s.Name)
		staged := c.RuntimeStagedSource(s.Name, runtime)
		owned := []string{staged, s.Source, c.LegacyStagedPath(s.Name)}
		if _, err := UnlinkOwned(target, staged, owned...); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.Name, err))
		}
	}
	return errors.Join(errs...)
}

// UnlinkOwned removes target only if it is a symlink whose readlink equals
// linksrc, mirroring bash unlink_owned. Real dirs and foreign symlinks are
// left untouched. It reports whether the target was ours (removed==true only
// when it was and the removal succeeded) and surfaces a real removal error
// (e.g. EPERM) instead of silently reporting failure as "not ours".
func UnlinkOwned(target, linksrc string, ownedSources ...string) (removed bool, err error) {
	info, lerr := os.Lstat(target)
	if lerr != nil || info.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}
	dest, rerr := os.Readlink(target)
	if rerr != nil || !isOwnedSymlink(dest, linksrc, ownedSources) {
		return false, nil
	}
	if err := os.Remove(target); err != nil {
		return false, err
	}
	return true, nil
}

// UninstallSkill removes every owned link for a skill, mirroring bash
// uninstall_skill. Each target is matched against the staged LinkSource
// first, then the repo CompareSource (legacy repo-pointing symlink
// migration). For teams, the now-empty per-team agent dir is pruned (rmdir
// semantics: only an empty directory, errors ignored) — never any shared
// skills root. Real removal errors are collected and returned (wrapped with
// the skill name) so callers can detect a failed uninstall.
func (c Config) UninstallSkill(s Skill) error {
	if c.SkipsTeam(s.Kind) {
		return nil
	}
	if s.Kind == KindHook {
		return c.uninstallHook(s)
	}

	var errs []error
	for _, l := range c.SkillLinks(s) {
		removed, err := UnlinkOwned(l.Target, l.LinkSource, c.ownedSources(s, l)...)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.Name, err))
			continue
		}
		if !removed {
			if _, err := UnlinkOwned(l.Target, l.CompareSource); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", s.Name, err))
			}
		}
	}
	if err := c.pruneOrphanedForkedLinks(s); err != nil {
		errs = append(errs, err)
	}

	if s.IsTeam() {
		dir := filepath.Join(c.Home, ".claude/agents", filepath.Base(s.Source))
		// rmdir: only remove an actual (empty) directory, never a file or
		// symlink at that path; failures (e.g. non-empty) are ignored.
		if info, err := os.Lstat(dir); err == nil && info.IsDir() {
			os.Remove(dir)
		}
	}
	return errors.Join(errs...)
}
