package skills

import "os"

// TargetStatus classifies a single link target relative to its expected
// source, mirroring bash target_state.
type TargetStatus string

const (
	// TargetLinked: our symlink, and the staged copy matches the repo source.
	TargetLinked TargetStatus = "linked"
	// TargetMissing: nothing at the target.
	TargetMissing TargetStatus = "missing"
	// TargetForeign: a symlink pointing elsewhere (incl. dangling); replacing
	// it is non-destructive (the data it points at survives).
	TargetForeign TargetStatus = "foreign"
	// TargetCopy: a real path whose content matches the repo source.
	TargetCopy TargetStatus = "copy"
	// TargetStale: our symlink whose staged copy differs from the repo
	// source, or a real path whose content differs (replacing it destroys
	// data).
	TargetStale TargetStatus = "stale"
)

// TargetState classifies one link target, mirroring bash target_state.
func TargetState(l Link) TargetStatus {
	info, err := os.Lstat(l.Target)
	switch {
	case err == nil && info.Mode()&os.ModeSymlink != 0:
		if dest, rerr := os.Readlink(l.Target); rerr == nil && dest == l.LinkSource {
			if PathsMatch(l.LinkSource, l.CompareSource) {
				return TargetLinked
			}
			return TargetStale
		}
		return TargetForeign
	case err != nil:
		return TargetMissing
	case PathsMatch(l.Target, l.CompareSource):
		return TargetCopy
	default:
		return TargetStale
	}
}

// State aggregates the target states of one skill, mirroring bash
// skill_state.
type State string

const (
	StateNotInstalled State = "not-installed"
	StateInstalled    State = "installed"
	StateUpgrade      State = "upgrade"
	StatePartial      State = "partial"
	StateSkipped      State = "skipped"
)

// SkillState aggregates target states into one skill state, mirroring bash
// skill_state: any stale/foreign target makes the skill upgradeable; all
// missing means not installed; all linked or matching copies means installed;
// anything else is partial. Teams whose runtime roots are not targeted are
// skipped; zero links reads as not installed.
func (c Config) SkillState(s Skill) State {
	if (s.Kind == KindTeam || s.Kind == KindTeamHybrid) && !c.TeamManaged(s.Kind) {
		return StateSkipped
	}

	var n, linked, missing, differ, copies int
	for _, l := range c.SkillLinks(s) {
		n++
		switch TargetState(l) {
		case TargetLinked:
			linked++
		case TargetMissing:
			missing++
		case TargetStale, TargetForeign: // differs from repo → upgradeable
			differ++
		case TargetCopy:
			copies++
		}
	}

	switch {
	case n == 0:
		return StateNotInstalled
	case differ > 0:
		return StateUpgrade
	case missing == n:
		return StateNotInstalled
	case linked+copies == n:
		return StateInstalled
	default:
		return StatePartial
	}
}
