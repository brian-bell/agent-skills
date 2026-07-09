package skills

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Discover lists the repo's skills, mirroring bash discover_skills:
// skills/* dirs are first-party and third-party/* dirs are third-party
// (plain files skipped), plus hooks/* dirs carrying an install.sh and a
// hook.json manifest. A hooks dir missing either is skipped with one line to
// warn — deliberate, so a half-added hook is loud rather than invisible.
func Discover(repoDir string, warn io.Writer) ([]Skill, error) {
	var out []Skill

	first, err := listDirs(filepath.Join(repoDir, "skills"))
	if err != nil {
		return nil, err
	}
	for _, dir := range first {
		out = append(out, Skill{
			Kind:   KindFirst,
			Name:   filepath.Base(dir),
			Source: dir,
			Forked: isForkedSkill(dir),
		})
	}

	third, err := listDirs(filepath.Join(repoDir, "third-party"))
	if err != nil {
		return nil, err
	}
	for _, dir := range third {
		out = append(out, Skill{Kind: KindThird, Name: filepath.Base(dir), Source: dir})
	}

	// Emit agent-teams grouped by kind so the renderer's consecutive-kind
	// header logic sees one contiguous block per kind. Directory order alone
	// can interleave team and team-hybrid (bash: `sort -k1,1 -s`).
	teamDirs, err := listDirs(filepath.Join(repoDir, "agent-teams"))
	if err != nil {
		return nil, err
	}
	var teams []Skill
	for _, dir := range teamDirs {
		base := filepath.Base(dir)
		if !strings.HasSuffix(base, "-team") {
			continue
		}
		kind := KindTeam
		if info, err := os.Stat(filepath.Join(dir, "agents/openai.yaml")); err == nil && info.Mode().IsRegular() {
			kind = KindTeamHybrid
		}
		teams = append(teams, Skill{
			Kind:   kind,
			Name:   strings.TrimSuffix(base, "-team"),
			Source: dir,
		})
	}
	// Sort by an explicit rank (team before hybrid) rather than relying on the
	// lexicographic accident that "team" < "team-hybrid".
	sort.SliceStable(teams, func(i, j int) bool { return kindRank(teams[i].Kind) < kindRank(teams[j].Kind) })
	out = append(out, teams...)

	hookDirs, err := listDirs(filepath.Join(repoDir, "hooks"))
	if err != nil {
		return nil, err
	}
	for _, dir := range hookDirs {
		if info, err := os.Lstat(filepath.Join(dir, "install.sh")); err != nil || !info.Mode().IsRegular() {
			if warn != nil {
				fmt.Fprintf(warn, "Skipping hook %s: missing install.sh\n", dir)
			}
			continue
		}
		m, err := parseHookManifest(dir)
		if err != nil {
			if warn != nil {
				fmt.Fprintf(warn, "Skipping hook %s: %v\n", dir, err)
			}
			continue
		}
		out = append(out, Skill{Kind: KindHook, Name: filepath.Base(dir), Source: dir, Hook: m})
	}

	return out, nil
}

// isForkedSkill reports whether dir is a runtime-forked portable skill.
// Claude and Codex overlays are required; Cursor is optional (cursor-less
// skills are still Forked — Cursor consumes the Claude skill via its
// ~/.claude/skills compat scan).
func isForkedSkill(dir string) bool {
	return hasRuntimeOverlay(dir, RuntimeClaude) && hasRuntimeOverlay(dir, RuntimeCodex)
}

func hasRuntimeOverlay(dir string, runtime Runtime) bool {
	info, err := os.Stat(filepath.Join(dir, "runtimes", string(runtime), "SKILL.md"))
	return err == nil && info.Mode().IsRegular()
}

// kindRank orders team kinds for grouping: plain teams before hybrid teams.
func kindRank(k Kind) int {
	if k == KindTeamHybrid {
		return 1
	}
	return 0
}

// listDirs returns the real (non-symlink) directory entries of parent, in
// lexicographic order — the same order as a bash glob. Leading-dot entries are
// skipped: bash globs never match hidden names. Unlike the bash `[ -d ]` test,
// symlinked entries are rejected so a planted symlink (e.g. third-party/foo ->
// ~/.ssh) can never be staged or exposed as a skill. A missing parent yields
// no entries (matching an unmatched glob); any other read error is propagated.
func listDirs(parent string) ([]string, error) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := filepath.Join(parent, e.Name())
		// os.Lstat (no follow): a symlink-to-dir reports ModeSymlink, so
		// IsDir() is false and the entry is skipped.
		if info, err := os.Lstat(path); err == nil && info.IsDir() {
			dirs = append(dirs, path)
		}
	}
	return dirs, nil
}
