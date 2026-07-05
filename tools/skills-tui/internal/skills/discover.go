package skills

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Discover lists the repo's skills, mirroring bash discover_skills:
// skills/* dirs are first-party and third-party/* dirs are third-party
// (plain files skipped).
func Discover(repoDir string) ([]Skill, error) {
	var out []Skill

	for _, dir := range listDirs(filepath.Join(repoDir, "skills")) {
		out = append(out, Skill{Kind: KindFirst, Name: filepath.Base(dir), Source: dir})
	}

	for _, dir := range listDirs(filepath.Join(repoDir, "third-party")) {
		out = append(out, Skill{Kind: KindThird, Name: filepath.Base(dir), Source: dir})
	}

	// Emit agent-teams grouped by kind so the renderer's consecutive-kind
	// header logic sees one contiguous block per kind. Directory order alone
	// can interleave team and team-hybrid (bash: `sort -k1,1 -s`).
	var teams []Skill
	for _, dir := range listDirs(filepath.Join(repoDir, "agent-teams")) {
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
	sort.SliceStable(teams, func(i, j int) bool { return teams[i].Kind < teams[j].Kind })
	out = append(out, teams...)

	return out, nil
}

// listDirs returns the directory entries of parent that are directories
// (following symlinks, like bash [ -d ]), in lexicographic order — the same
// order as a bash glob. Leading-dot entries are skipped: bash globs never
// match hidden names. A missing parent yields no entries, matching an
// unmatched glob.
func listDirs(parent string) []string {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := filepath.Join(parent, e.Name())
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			dirs = append(dirs, path)
		}
	}
	return dirs
}
