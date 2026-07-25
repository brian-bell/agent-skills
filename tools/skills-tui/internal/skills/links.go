package skills

import (
	"errors"
	"fmt"
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
}

// SkillLinks lists the symlink pairs for a skill, mirroring bash skill_links.
// Targets limits which runtime roots are managed: portable skills link into
// each targeted skills root.
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
	}
	return links
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

	// Captured before the loop: once links are repointed, the evidence that
	// they rested on a legacy stage tree is gone.
	claimedLegacy := c.claimedLegacyTeamPaths(s)

	cleanupPending := c.legacyTeamCleanupPending(s)
	var errs []error
	for _, l := range c.SkillLinks(s) {
		st := c.targetState(l)
		// A matching real directory is already current. When this apply was
		// scheduled solely because migration cleanup remains, preserve the
		// copy instead of requiring destructive --force just to reach the
		// cleanup below.
		if cleanupPending && st == TargetCopy {
			continue
		}
		// Only tree links carry a Compare overlay; a forked team's agent-file
		// links point inside the claude tree, which the preceding tree link's
		// sync has already assembled.
		if s.Forked && l.CompareOverlay != "" && needsSync(st) {
			if err := c.SyncAssembledStagedSource(l.CompareShared, l.CompareOverlay, l.LinkSource); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", s.Name, err))
				continue
			}
		}
		if err := LinkPath(l.LinkSource, l.Target, force, destroy, c.ownedSources(s, l)...); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.Name, err))
		}
	}
	// Only once every managed link is actually pointing at the new assembly.
	// Pruning after a failed sync or relink would delete a stage tree that a
	// still-legacy link depends on, turning a visible, recoverable error into
	// a dangling install.
	if len(errs) == 0 {
		if err := c.pruneLegacyTeamInstall(s, claimedLegacy); err != nil {
			errs = append(errs, err)
		}
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
		// A pre-as-77n install pointed this same skill path at a team tree —
		// staged, or the checkout itself on installs predating staging.
		// Those are ours to repoint and to unlink, so the upgrade relinks in
		// place instead of reporting a foreign target and demanding --force,
		// and `--none` does not leave a dangling link behind while claiming
		// it removed one.
		owned = append(owned, c.legacyTeamOwnedPaths(s.Name)...)
	}
	return owned
}

// legacyTeamDirs maps each review skill migrated off agent-teams/ (as-77n) to
// the team directory it used to install as. Both entries are migration
// cleanup, not team support: the skills are ordinary forked first-party skills
// now, so nothing else in the engine knows these names.
//
// Retire this table (and pruneLegacyTeamInstall) once the migration has been
// applied everywhere.
var legacyTeamDirs = map[string]string{
	"go-review":      "go-review-team",
	"feature-review": "feature-review-team",
}

// legacyTeamOwnedPaths lists every tree a pre-as-77n install of this skill
// could have linked at — skill links and agent registrations alike:
//
//   - the flat whole-team staged copy (go-review was a flat hybrid team),
//   - both staged runtime assemblies (feature-review was forked),
//   - the checkout's former agent-teams/<team-dir>, from installs old enough
//     to predate staging entirely.
//
// Which shape a given machine has depends on when it last installed, so all
// of them are ours for relinking, unlinking, and prune ownership. Only the
// staged ones are ever deleted; the repo path is recognition only.
func (c Config) legacyTeamOwnedPaths(name string) []string {
	teamdir, ok := legacyTeamDirs[name]
	if !ok {
		return nil
	}
	paths := []string{
		filepath.Join(c.StageDir, "agent-teams", teamdir),
		c.RuntimeTeamStagedSource(teamdir, RuntimeCodex),
		c.RuntimeTeamStagedSource(teamdir, RuntimeClaude),
	}
	if c.RepoDir != "" {
		paths = append(paths, filepath.Join(c.RepoDir, "agent-teams", teamdir))
	}
	return paths
}

// legacyTeamCleanupPending makes an otherwise-current migrated skill
// upgradeable while installer-owned pre-migration state remains. Without this,
// an already-migrated row is StateInstalled, so the apply plan never reaches
// InstallSkill and its cleanup.
func (c Config) legacyTeamCleanupPending(s Skill) bool {
	teamdir, ok := legacyTeamDirs[s.Name]
	if !ok || s.Kind != KindFirst {
		return false
	}

	legacyOwned := c.legacyTeamOwnedPaths(s.Name)
	if c.HasTarget(TargetClaude) {
		agentsDir := filepath.Join(c.Home, ".claude/agents", teamdir)
		info, err := os.Lstat(agentsDir)
		if err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			entries, readErr := os.ReadDir(agentsDir)
			if readErr != nil {
				// Schedule the cleanup so the apply path can surface the
				// permission error instead of silently treating the row as
				// fully migrated.
				return true
			}
			for _, entry := range entries {
				target := filepath.Join(agentsDir, entry.Name())
				einfo, lerr := os.Lstat(target)
				if lerr != nil || einfo.Mode()&os.ModeSymlink == 0 {
					continue
				}
				dest, rerr := os.Readlink(target)
				if _, ok := rootOf(dest, legacyOwned); rerr == nil && ok {
					return true
				}
			}
		}
	}

	// A directory merely occupying a legacy stage path is not evidence that
	// the installer owns it: StageDir is configurable and may contain user
	// data. Skill links at an exact legacy root are the other ownership proof.
	return len(c.claimedLegacyTeamPaths(s)) > 0
}

// legacyTeamPrunableStagedPaths returns only the legacy stage trees safe to
// remove for the configured target set.
func (c Config) legacyTeamPrunableStagedPaths(teamdir string) []string {
	claude, agents := c.HasTarget(TargetClaude), c.HasTarget(TargetAgents)
	var prunable []string
	if claude {
		prunable = append(prunable, c.RuntimeTeamStagedSource(teamdir, RuntimeClaude))
	}
	if agents {
		prunable = append(prunable, c.RuntimeTeamStagedSource(teamdir, RuntimeCodex))
	}
	// The flat tree was shared by the ~/.agents and ~/.claude links, so it is
	// only safe to remove once both roots have been migrated off it.
	if claude && agents {
		prunable = append(prunable, filepath.Join(c.StageDir, "agent-teams", teamdir))
	}
	return prunable
}

// pruneLegacyTeamInstall removes what a pre-as-77n install of this skill left
// behind: the per-team agent registrations under ~/.claude/agents/<team-dir>
// and every legacy staged team tree.
//
// Without this, upgrading is silently incomplete. The repo directory is gone,
// so the team is no longer discovered, so its uninstall path never runs — the
// deleted lead and reviewer agents stay registered and invocable alongside the
// new inline skill.
//
// This runs after the link loop so the skill's own links have already been
// repointed at the new staged assembly; pruning first would strand them.
//
// Ownership is narrow on purpose. An agent link is ours only if it points
// into one of this team's legacy staged trees — not merely somewhere under
// StageDir, which would also claim a user's symlink to an unrelated staged
// skill. Real files and foreign symlinks are left alone, and the directory is
// removed with rmdir semantics so anything surviving keeps it.
func (c Config) pruneLegacyTeamInstall(s Skill, claimed map[string]bool) error {
	teamdir, ok := legacyTeamDirs[s.Name]
	if !ok || s.Kind != KindFirst {
		return nil
	}
	// Ownership recognition spans every legacy shape, but removal must respect
	// the target contract: an agents-only run has no business deleting Claude
	// state, and deleting a stage tree that an unmanaged root still links at
	// would leave that root dangling.
	legacyOwned := c.legacyTeamOwnedPaths(s.Name)
	claude := c.HasTarget(TargetClaude)
	prunable := c.legacyTeamPrunableStagedPaths(teamdir)

	var errs []error
	if claude {
		removed, agentErrs := c.pruneLegacyAgentDir(s.Name, teamdir, legacyOwned)
		errs = append(errs, agentErrs...)
		for _, root := range removed {
			claimed[root] = true
		}
		if len(agentErrs) > 0 {
			// A surviving registration may still point into any one of the
			// owned legacy shapes. Preserve every staged tree so the failed
			// cleanup remains usable and retryable.
			return errors.Join(errs...)
		}
	}

	// A recursive delete needs evidence, not a matching name. StageDir is
	// configurable (SKILL_SYMLINKS_DIR), so a directory sitting at a legacy
	// path may be the user's, not ours. Remove one only when something we
	// actually migrated in this run pointed at it.
	for _, staged := range prunable {
		if !claimed[staged] {
			continue
		}
		info, serr := os.Lstat(staged)
		if serr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if rmErr := os.RemoveAll(staged); rmErr != nil {
			errs = append(errs, fmt.Errorf("%s: prune legacy staged team: %w", s.Name, rmErr))
		}
	}
	return errors.Join(errs...)
}

// pruneLegacyAgentDir removes this team's registrations under
// ~/.claude/agents/<team-dir>. A link is ours only when it points into one of
// legacyOwned; real files and symlinks elsewhere survive, and their presence
// keeps the directory.
func (c Config) pruneLegacyAgentDir(name, teamdir string, legacyOwned []string) (claimed []string, errs []error) {
	agentsDir := filepath.Join(c.Home, ".claude/agents", teamdir)
	// Lstat, not ReadDir: a symlinked agentsDir would otherwise be followed
	// and we would delete inside a directory we do not own.
	info, err := os.Lstat(agentsDir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, nil
	}

	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil, []error{fmt.Errorf("%s: prune legacy agent dir: %w", name, err)}
	}
	for _, e := range entries {
		target := filepath.Join(agentsDir, e.Name())
		einfo, elerr := os.Lstat(target)
		if elerr != nil || einfo.Mode()&os.ModeSymlink == 0 {
			continue
		}
		dest, rerr := os.Readlink(target)
		root, ok := rootOf(dest, legacyOwned)
		if rerr != nil || !ok {
			continue
		}
		if rmErr := os.Remove(target); rmErr != nil {
			errs = append(errs, fmt.Errorf("%s: prune legacy agent link: %w", name, rmErr))
			continue
		}
		claimed = append(claimed, root)
	}
	// rmdir semantics: only removes the dir once it is empty, so a foreign
	// entry left above keeps it.
	os.Remove(agentsDir)
	return claimed, errs
}

// exactRoot returns the entry of roots that dest IS. Skill links always
// pointed at a stage root, never inside one, and UnlinkOwned likewise only
// claims an exact match — so a link resting on a descendant is foreign, and
// must not be read as proof the whole tree is ours to delete.
func exactRoot(dest string, roots []string) (string, bool) {
	if !filepath.IsAbs(dest) {
		return "", false
	}
	clean := filepath.Clean(dest)
	for _, root := range roots {
		if root != "" && clean == root {
			return root, true
		}
	}
	return "", false
}

// rootOf returns the entry of roots that dest is, or lives inside. Descendant
// matching is for agent-file registrations, which pointed at individual *.md
// files inside a team tree.
func rootOf(dest string, roots []string) (string, bool) {
	if !filepath.IsAbs(dest) {
		return "", false
	}
	clean := filepath.Clean(dest)
	for _, root := range roots {
		if root == "" {
			continue
		}
		if clean == root || strings.HasPrefix(clean, root+string(filepath.Separator)) {
			return root, true
		}
	}
	return "", false
}

// claimedLegacyTeamPaths inspects a skill's link targets BEFORE they are
// relinked and reports which legacy stage trees they currently point at. That
// is the evidence the prune needs: a legacy path we can see an installed link
// resting on is ours, one merely sitting at a matching name is not.
func (c Config) claimedLegacyTeamPaths(s Skill) map[string]bool {
	claimed := map[string]bool{}
	legacy := c.legacyTeamOwnedPaths(s.Name)
	if len(legacy) == 0 {
		return claimed
	}
	for _, l := range c.SkillLinks(s) {
		info, err := os.Lstat(l.Target)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		dest, rerr := os.Readlink(l.Target)
		if rerr != nil {
			continue
		}
		if root, ok := exactRoot(dest, legacy); ok {
			claimed[root] = true
		}
	}
	return claimed
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
	if s.Kind == KindHook {
		return c.uninstallHook(s)
	}
	claimedLegacy := c.claimedLegacyTeamPaths(s)

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

	// Uninstalling a migrated skill must also clear what its pre-as-77n team
	// install left behind, or `--none` reports a clean uninstall while the old
	// agents stay registered. Same rule as install: only when every managed
	// unlink succeeded, so a failed removal never strands a surviving link.
	if len(errs) == 0 {
		if err := c.pruneLegacyTeamInstall(s, claimedLegacy); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
