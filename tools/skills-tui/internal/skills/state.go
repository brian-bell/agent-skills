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

// targetState classifies one link target, mirroring bash target_state.
// Comparison errors other than "not found" are reported to c.WarnW so a
// permission problem is not silently misread as a stale/upgradeable target.
func (c Config) targetState(l Link) TargetStatus {
	info, err := os.Lstat(l.Target)
	switch {
	case err == nil && info.Mode()&os.ModeSymlink != 0:
		if dest, rerr := os.Readlink(l.Target); rerr == nil && dest == l.LinkSource {
			if c.linkContentMatches(l.LinkSource, l) {
				return TargetLinked
			}
			return TargetStale
		}
		return TargetForeign
	case err != nil:
		return TargetMissing
	case c.linkContentMatches(l.Target, l):
		return TargetCopy
	default:
		return TargetStale
	}
}

func (c Config) linkContentMatches(actual string, l Link) bool {
	if l.CompareOverlay != "" {
		return c.pathsMatchAssembled(actual, l.CompareShared, l.CompareOverlay)
	}
	return pathsMatch(actual, l.CompareSource, c.WarnW)
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
	if c.SkipsTeam(s.Kind) {
		return StateSkipped
	}
	if s.Kind == KindHook {
		return c.hookState(s)
	}

	var n, linked, missing, differ, copies int
	for _, l := range c.SkillLinks(s) {
		n++
		switch c.targetState(l) {
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
	// Owned links for removed overlays (e.g. a stale ~/.cursor symlink after
	// a skill went cursor-less) are not in SkillLinks, but they must still
	// make the skill upgradeable so install/upgrade/remove plans reach the
	// prune path instead of ActionNone.
	if len(c.forkedOrphanTargets(s)) > 0 {
		differ++
	}

	switch {
	case n == 0 && differ == 0:
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
