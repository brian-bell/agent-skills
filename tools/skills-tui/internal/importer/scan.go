// Package importer implements safe GitHub skill discovery and repository
// imports for the skills TUI.
package importer

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const maxSkillFrontmatterBytes = 1 << 20

var safeInstallName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// Candidate is one directory containing a SKILL.md discovered in a checkout.
// ID is stable within the checkout and intentionally equals SourcePath.
type Candidate struct {
	ID          string
	Name        string
	Description string
	SourcePath  string
	Valid       bool
	Reason      string
}

// Scan discovers portable skills without executing checkout content.
func Scan(checkoutRoot string) ([]Candidate, error) {
	root, err := filepath.Abs(checkoutRoot)
	if err != nil {
		return nil, err
	}

	var candidates []Candidate
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		skillPath := filepath.Join(path, "SKILL.md")
		info, err := os.Lstat(skillPath)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		candidate := Candidate{ID: rel, SourcePath: rel}
		if !info.Mode().IsRegular() {
			candidate.Reason = "SKILL.md must be a real regular file"
			candidates = append(candidates, candidate)
			return nil
		}

		candidate.Name, candidate.Description, err = readFrontmatter(skillPath)
		if err != nil {
			candidate.Reason = err.Error()
			candidates = append(candidates, candidate)
			return nil
		}
		if !isSafeInstallName(candidate.Name) {
			candidate.Reason = "frontmatter name is not a safe install name"
			candidates = append(candidates, candidate)
			return nil
		}
		candidate.Valid = true
		candidates = append(candidates, candidate)
		return filepath.SkipDir
	})
	if err != nil {
		return nil, fmt.Errorf("scan checkout: %w", err)
	}
	nameCounts := make(map[string]int)
	for _, candidate := range candidates {
		if candidate.Valid {
			nameCounts[strings.ToLower(candidate.Name)]++
		}
	}
	for i := range candidates {
		if candidates[i].Valid && nameCounts[strings.ToLower(candidates[i].Name)] > 1 {
			candidates[i].Valid = false
			candidates[i].Reason = fmt.Sprintf("duplicate candidate name %q", candidates[i].Name)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].SourcePath < candidates[j].SourcePath })
	return candidates, nil
}

func readFrontmatter(path string) (string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	reader := bufio.NewReader(io.LimitReader(f, maxSkillFrontmatterBytes+1))
	first, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", "", err
	}
	readBytes := len(first)
	if trimLineEnding(first) != "---" {
		return "", "", fmt.Errorf("missing YAML frontmatter")
	}
	var document bytes.Buffer
	for {
		line, readErr := reader.ReadString('\n')
		readBytes += len(line)
		if readBytes > maxSkillFrontmatterBytes {
			return "", "", fmt.Errorf("SKILL.md frontmatter exceeds %d bytes", maxSkillFrontmatterBytes)
		}
		if trimLineEnding(line) == "---" {
			break
		}
		document.WriteString(line)
		if readErr != nil {
			if readErr == io.EOF {
				return "", "", fmt.Errorf("unterminated YAML frontmatter")
			}
			return "", "", readErr
		}
	}
	var frontmatter struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal(document.Bytes(), &frontmatter); err != nil {
		return "", "", fmt.Errorf("invalid YAML frontmatter: %w", err)
	}
	frontmatter.Name = strings.TrimSpace(frontmatter.Name)
	frontmatter.Description = strings.TrimSpace(frontmatter.Description)
	if frontmatter.Name == "" || frontmatter.Description == "" {
		return frontmatter.Name, frontmatter.Description, fmt.Errorf("frontmatter requires name and description")
	}
	return frontmatter.Name, frontmatter.Description, nil
}

func trimLineEnding(line string) string {
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
}

func isSafeInstallName(name string) bool {
	return name != "." && name != ".." && safeInstallName.MatchString(name)
}
