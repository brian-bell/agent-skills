package skills

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const retiredProjectSkill = "skill-parity-audit"

// PruneRetiredSkillInstalls removes installer-owned global copies of skills
// that moved to project scope. Foreign symlinks and real target paths are
// preserved; only exact staged cache paths owned by this installer are removed.
func (c Config) PruneRetiredSkillInstalls() (bool, error) {
	reused, err := c.retiredNameReusedByThirdParty()
	if err != nil {
		return false, fmt.Errorf("%s: %w", retiredProjectSkill, err)
	}
	if reused {
		// Third-party lifecycle actions recognize the retired runtime-stage
		// links as migration-owned, so install/remove can replace/unlink them.
		return false, nil
	}

	changed := false
	var errs []error

	for _, root := range portableRoots {
		if !c.HasTarget(root.target) {
			continue
		}
		runtime, ok := targetRuntime(root.target)
		if !ok {
			continue
		}

		staged := c.RuntimeStagedSource(retiredProjectSkill, runtime)
		target := filepath.Join(
			c.Home,
			root.dir,
			"skills",
			retiredProjectSkill,
		)
		oldSource := filepath.Join(c.RepoDir, "skills", retiredProjectSkill)
		removed, err := UnlinkOwned(
			target,
			staged,
			oldSource,
			c.LegacyStagedPath(retiredProjectSkill),
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", retiredProjectSkill, err))
			continue
		} else if removed {
			changed = true
		}

		if _, err := os.Lstat(staged); err == nil {
			if err := os.RemoveAll(staged); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", retiredProjectSkill, err))
			} else {
				changed = true
			}
		} else if !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("%s: %w", retiredProjectSkill, err))
		}
	}

	legacyStaged := c.LegacyStagedPath(retiredProjectSkill)
	referenced, err := c.retiredStageReferenced(legacyStaged)
	if err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", retiredProjectSkill, err))
	} else if !referenced {
		if _, err := os.Lstat(legacyStaged); err == nil {
			if err := os.RemoveAll(legacyStaged); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", retiredProjectSkill, err))
			} else {
				changed = true
			}
		} else if !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("%s: %w", retiredProjectSkill, err))
		}
	}

	return changed, errors.Join(errs...)
}

func (c Config) retiredNameReusedByThirdParty() (bool, error) {
	if c.RepoDir == "" {
		return false, nil
	}
	skillMD := filepath.Join(
		c.RepoDir,
		"third-party",
		retiredProjectSkill,
		"SKILL.md",
	)
	info, err := os.Stat(skillMD)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.Mode().IsRegular(), nil
}

func (c Config) retiredStageReferenced(staged string) (bool, error) {
	for _, root := range portableRoots {
		target := filepath.Join(
			c.Home,
			root.dir,
			"skills",
			retiredProjectSkill,
		)
		info, err := os.Lstat(target)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		dest, err := os.Readlink(target)
		if err != nil {
			return false, err
		}
		if dest == staged {
			return true, nil
		}
	}
	return false, nil
}
