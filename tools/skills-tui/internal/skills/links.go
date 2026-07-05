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
// source).
type Link struct {
	Target        string
	LinkSource    string
	CompareSource string
}

// SkillLinks lists the symlink pairs for a skill, mirroring bash skill_links.
// Targets limits which runtime roots are managed. Portable skills link into
// each targeted skills root; teams link into ~/.claude (skill dir plus one
// agent link per top-level *.md except SKILL.md and README.md), and hybrid
// teams additionally into ~/.agents.
func (c Config) SkillLinks(s Skill) []Link {
	staged := c.StagedSource(s.Kind, s.Name, s.Source)
	var links []Link

	switch s.Kind {
	case KindFirst, KindThird:
		for _, root := range []struct{ target, dir string }{
			{"agents", ".agents"},
			{"claude", ".claude"},
			{"cursor", ".cursor"},
		} {
			if c.HasTarget(root.target) {
				links = append(links, Link{
					Target:        filepath.Join(c.Home, root.dir, "skills", s.Name),
					LinkSource:    staged,
					CompareSource: s.Source,
				})
			}
		}
	case KindTeam, KindTeamHybrid:
		teamdir := filepath.Base(s.Source)
		if s.Kind == KindTeamHybrid && c.HasTarget("agents") {
			links = append(links, Link{
				Target:        filepath.Join(c.Home, ".agents/skills", s.Name),
				LinkSource:    staged,
				CompareSource: s.Source,
			})
		}
		if c.HasTarget("claude") {
			links = append(links, Link{
				Target:        filepath.Join(c.Home, ".claude/skills", s.Name),
				LinkSource:    staged,
				CompareSource: s.Source,
			})
			for _, md := range teamAgentFiles(s.Source) {
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
// in glob order, mirroring the bash `"$source"/*.md` loop: files only
// (following symlinks like [ -f ]), excluding the SKILL.md manifest and
// README.md docs. Leading-dot names are skipped: the bash glob never matches
// hidden files (nor a literal ".md", since '*' does not match a leading dot).
func teamAgentFiles(source string) []string {
	entries, err := os.ReadDir(source)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".md") || name == "SKILL.md" || name == "README.md" {
			continue
		}
		if info, err := os.Stat(filepath.Join(source, name)); err != nil || !info.Mode().IsRegular() {
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

// LinkPath creates one symlink, creating parent dirs, mirroring bash
// link_path. A target symlink already pointing at source (exact readlink
// match) is a no-op. Replacing any existing symlink (foreign/dangling/stale)
// requires force; replacing a real file/directory requires destroy — the only
// path that can lose user data.
func LinkPath(source, target string, force, destroy bool) error {
	info, err := os.Lstat(target)
	switch {
	case err == nil && info.Mode()&os.ModeSymlink != 0:
		if dest, rerr := os.Readlink(target); rerr == nil && dest == source {
			return nil
		}
		if !force {
			return &RefusedSymlinkError{Target: target}
		}
		if err := os.Remove(target); err != nil {
			return err
		}
	case err == nil:
		if !destroy {
			return &RefusedRealPathError{Target: target}
		}
		if err := os.RemoveAll(target); err != nil {
			return err
		}
	}

	// bash mkdir -p: created parents get 0777 & ~umask.
	if err := os.MkdirAll(filepath.Dir(target), 0o777); err != nil {
		return err
	}
	return os.Symlink(source, target)
}

// InstallSkill stages the skill source and links every managed target,
// mirroring bash install_skill. Teams whose runtime roots are not targeted
// are skipped. Link failures are collected but do not stop the remaining
// links.
func (c Config) InstallSkill(s Skill, force, destroy bool) error {
	if (s.Kind == KindTeam || s.Kind == KindTeamHybrid) && !c.TeamManaged(s.Kind) {
		return nil
	}

	staged := c.StagedSource(s.Kind, s.Name, s.Source)
	if err := c.SyncStagedSource(s.Source, staged); err != nil {
		return err
	}

	var errs []error
	for _, l := range c.SkillLinks(s) {
		if err := LinkPath(l.LinkSource, l.Target, force, destroy); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// UnlinkOwned removes target only if it is a symlink whose readlink equals
// linksrc, mirroring bash unlink_owned. Real dirs and foreign symlinks are
// left untouched.
func UnlinkOwned(target, linksrc string) bool {
	info, err := os.Lstat(target)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return false
	}
	dest, err := os.Readlink(target)
	if err != nil || dest != linksrc {
		return false
	}
	return os.Remove(target) == nil
}

// UninstallSkill removes every owned link for a skill, mirroring bash
// uninstall_skill. Each target is matched against the staged LinkSource
// first, then the repo CompareSource (legacy repo-pointing symlink
// migration). For teams, the now-empty per-team agent dir is pruned (rmdir
// semantics: only an empty directory, errors ignored) — never any shared
// skills root.
func (c Config) UninstallSkill(s Skill) {
	if (s.Kind == KindTeam || s.Kind == KindTeamHybrid) && !c.TeamManaged(s.Kind) {
		return
	}

	for _, l := range c.SkillLinks(s) {
		if !UnlinkOwned(l.Target, l.LinkSource) {
			UnlinkOwned(l.Target, l.CompareSource)
		}
	}

	if s.Kind == KindTeam || s.Kind == KindTeamHybrid {
		dir := filepath.Join(c.Home, ".claude/agents", filepath.Base(s.Source))
		// rmdir: only remove an actual (empty) directory, never a file or
		// symlink at that path; failures (e.g. non-empty) are ignored.
		if info, err := os.Lstat(dir); err == nil && info.IsDir() {
			os.Remove(dir)
		}
	}
}
