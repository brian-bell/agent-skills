package importer

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ImportRequest identifies scanner candidates selected from one checked-out
// repository snapshot.
type ImportRequest struct {
	CheckoutRoot  string
	RepositoryURL string
	Commit        string
	Candidates    []Candidate
	SelectedIDs   []string
}

// RepositoryImporter transactionally publishes imported skills into the
// repository. RepoDir is the agent-skills checkout being modified.
type RepositoryImporter struct {
	RepoDir string
	// Publish is the no-overwrite directory publication boundary. Nil selects
	// the platform implementation; tests may inject a failing rename.
	Publish func(source, destination string) error
	// RemovePublished is the rollback filesystem boundary. Nil selects
	// os.RemoveAll; tests may inject a cleanup failure.
	RemovePublished func(path string) error
}

var repositoryImportProcessLocks sync.Map

// ValidateCandidates returns a copy with existing repository install-name
// collisions disabled for display and selection in the TUI.
func (r RepositoryImporter) ValidateCandidates(candidates []Candidate) ([]Candidate, error) {
	out := append([]Candidate(nil), candidates...)
	collisions := make(map[string]string)
	if err := collectInstallNames(filepath.Join(r.RepoDir, "skills"), "first-party", false, false, collisions); err != nil {
		return nil, err
	}
	if err := collectInstallNames(filepath.Join(r.RepoDir, "third-party"), "third-party", false, true, collisions); err != nil {
		return nil, err
	}
	if err := collectInstallNames(filepath.Join(r.RepoDir, "agent-teams"), "agent-team", true, false, collisions); err != nil {
		return nil, err
	}
	for i := range out {
		if !out[i].Valid {
			continue
		}
		if kind, exists := collisions[strings.ToLower(out[i].Name)]; exists {
			out[i].Valid = false
			out[i].Reason = fmt.Sprintf("install name conflicts with existing %s", kind)
		}
	}
	return out, nil
}

// Import preflights and stages the complete selected batch, publishes each
// skill, and publishes its attribution update last.
func (r RepositoryImporter) Import(ctx context.Context, request ImportRequest) ([]string, error) {
	unlock, err := lockRepositoryImport(ctx, r.RepoDir)
	if err != nil {
		return nil, err
	}
	defer unlock()
	return r.importLocked(ctx, request)
}

func (r RepositoryImporter) importLocked(ctx context.Context, request ImportRequest) ([]string, error) {
	repositoryURL, err := NormalizeGitHubURL(request.RepositoryURL)
	if err != nil || repositoryURL != request.RepositoryURL {
		return nil, fmt.Errorf("import repository URL is not canonical")
	}
	if !commitSHA.MatchString(request.Commit) {
		return nil, fmt.Errorf("import commit is not a valid SHA")
	}
	request.Candidates, err = r.ValidateCandidates(request.Candidates)
	if err != nil {
		return nil, err
	}
	selected, err := selectedCandidates(request)
	if err != nil {
		return nil, err
	}
	thirdParty := filepath.Join(r.RepoDir, "third-party")
	for _, candidate := range selected {
		if _, err := os.Lstat(filepath.Join(thirdParty, candidate.Name)); !os.IsNotExist(err) {
			if err == nil {
				return nil, fmt.Errorf("install name %q already exists in third-party", candidate.Name)
			}
			return nil, fmt.Errorf("check import destination %q: %w", candidate.Name, err)
		}
	}

	attributionPath := filepath.Join(thirdParty, "ATTRIBUTION.md")
	originalAttribution, err := os.ReadFile(attributionPath)
	if err != nil {
		return nil, fmt.Errorf("read third-party attribution: %w", err)
	}
	updatedAttribution, err := addAttributionRows(originalAttribution, repositoryURL, strings.ToLower(request.Commit), selected)
	if err != nil {
		return nil, err
	}

	stageRoot, err := os.MkdirTemp(thirdParty, ".import-stage-*")
	if err != nil {
		return nil, fmt.Errorf("create import staging directory: %w", err)
	}
	defer os.RemoveAll(stageRoot)
	for _, candidate := range selected {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("skill import cancelled: %w", err)
		}
		source, err := candidateSource(request.CheckoutRoot, candidate.SourcePath)
		if err != nil {
			return nil, err
		}
		if err := copySkillTree(ctx, source, filepath.Join(stageRoot, candidate.Name)); err != nil {
			return nil, fmt.Errorf("stage skill %q: %w", candidate.Name, err)
		}
	}
	attributionTemp, err := stageAttribution(thirdParty, updatedAttribution)
	if err != nil {
		return nil, err
	}
	defer os.Remove(attributionTemp)

	published := make([]string, 0, len(selected))
	publish := r.Publish
	if publish == nil {
		publish = renameNoReplace
	}
	removePublished := r.RemovePublished
	if removePublished == nil {
		removePublished = os.RemoveAll
	}
	rollback := func() error {
		var cleanupErrors []error
		for i := len(published) - 1; i >= 0; i-- {
			if err := removePublished(published[i]); err != nil {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("remove published skill %q: %w", filepath.Base(published[i]), err))
			}
		}
		return errors.Join(cleanupErrors...)
	}
	withRollback := func(operationErr error) error {
		if rollbackErr := rollback(); rollbackErr != nil {
			return errors.Join(operationErr, fmt.Errorf("rollback imported skills: %w", rollbackErr))
		}
		return operationErr
	}
	for _, candidate := range selected {
		destination := filepath.Join(thirdParty, candidate.Name)
		if err := publish(filepath.Join(stageRoot, candidate.Name), destination); err != nil {
			return nil, withRollback(fmt.Errorf("publish skill %q: %w", candidate.Name, err))
		}
		published = append(published, destination)
	}
	if err := os.Rename(attributionTemp, attributionPath); err != nil {
		return nil, withRollback(fmt.Errorf("publish third-party attribution: %w", err))
	}

	names := make([]string, len(selected))
	for i, candidate := range selected {
		names[i] = candidate.Name
	}
	return names, nil
}

func lockRepositoryImport(ctx context.Context, repoDir string) (func(), error) {
	absoluteRepo, err := filepath.Abs(repoDir)
	if err != nil {
		return nil, fmt.Errorf("resolve import repository lock: %w", err)
	}
	canonicalRepo, err := filepath.EvalSymlinks(absoluteRepo)
	if err != nil {
		return nil, fmt.Errorf("resolve import repository identity: %w", err)
	}
	absoluteRepo = canonicalRepo
	processLockValue, _ := repositoryImportProcessLocks.LoadOrStore(absoluteRepo, newContextMutex())
	processLock := processLockValue.(*contextMutex)
	if err := processLock.lock(ctx); err != nil {
		return nil, fmt.Errorf("lock repository import: %w", err)
	}

	lockDir := filepath.Join(os.TempDir(), "agent-skills-import-locks")
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		processLock.unlock()
		return nil, fmt.Errorf("create import lock directory: %w", err)
	}
	if err := os.Chmod(lockDir, 0o700); err != nil {
		processLock.unlock()
		return nil, fmt.Errorf("protect import lock directory: %w", err)
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(absoluteRepo)))
	lockFile, err := os.OpenFile(filepath.Join(lockDir, key+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		processLock.unlock()
		return nil, fmt.Errorf("open repository import lock: %w", err)
	}
	if err := lockFile.Chmod(0o600); err != nil {
		lockFile.Close()
		processLock.unlock()
		return nil, fmt.Errorf("protect repository import lock: %w", err)
	}
	fileUnlock, err := lockFileExclusive(ctx, lockFile)
	if err != nil {
		lockFile.Close()
		processLock.unlock()
		return nil, fmt.Errorf("lock repository import: %w", err)
	}
	return func() {
		fileUnlock()
		_ = lockFile.Close()
		processLock.unlock()
	}, nil
}

func collectInstallNames(parent, kind string, teams, includeFiles bool, collisions map[string]string) error {
	entries, err := os.ReadDir(parent)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s import collisions: %w", kind, err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		name := entry.Name()
		if teams {
			if !strings.HasSuffix(name, "-team") {
				continue
			}
			name = strings.TrimSuffix(name, "-team")
		} else if !includeFiles && !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			continue
		}
		key := strings.ToLower(name)
		if _, exists := collisions[key]; !exists {
			collisions[key] = kind
		}
	}
	return nil
}

func selectedCandidates(request ImportRequest) ([]Candidate, error) {
	if len(request.SelectedIDs) == 0 {
		return nil, fmt.Errorf("select at least one valid skill to import")
	}
	byID := make(map[string]Candidate, len(request.Candidates))
	for _, candidate := range request.Candidates {
		byID[candidate.ID] = candidate
	}
	selected := make([]Candidate, 0, len(request.SelectedIDs))
	seenIDs := make(map[string]struct{}, len(request.SelectedIDs))
	seenNames := make(map[string]struct{}, len(request.SelectedIDs))
	for _, id := range request.SelectedIDs {
		if _, duplicate := seenIDs[id]; duplicate {
			return nil, fmt.Errorf("candidate %q was selected more than once", id)
		}
		seenIDs[id] = struct{}{}
		candidate, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("selected candidate %q was not scanned", id)
		}
		if !candidate.Valid {
			return nil, fmt.Errorf("selected candidate %q is invalid: %s", id, candidate.Reason)
		}
		if !isSafeInstallName(candidate.Name) {
			return nil, fmt.Errorf("selected candidate %q has an unsafe install name", id)
		}
		nameKey := strings.ToLower(candidate.Name)
		if _, duplicate := seenNames[nameKey]; duplicate {
			return nil, fmt.Errorf("duplicate selected install name %q", candidate.Name)
		}
		seenNames[nameKey] = struct{}{}
		selected = append(selected, candidate)
	}
	return selected, nil
}

func candidateSource(checkoutRoot, sourcePath string) (string, error) {
	if sourcePath == "" || filepath.IsAbs(sourcePath) {
		return "", fmt.Errorf("candidate source path %q is unsafe", sourcePath)
	}
	clean := filepath.Clean(filepath.FromSlash(sourcePath))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("candidate source path %q escapes the checkout", sourcePath)
	}
	root, err := filepath.Abs(checkoutRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, clean), nil
}

func copySkillTree(ctx context.Context, source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel != "." && entry.Name() == ".git" && entry.IsDir() {
			return filepath.SkipDir
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink %s is not allowed", filepath.ToSlash(rel))
		}
		target := filepath.Join(destination, rel)
		if info.IsDir() {
			if err := os.MkdirAll(target, info.Mode().Perm()); err != nil {
				return err
			}
			return os.Chmod(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("special file %s is not allowed", filepath.ToSlash(rel))
		}
		return copyRegularFile(path, target, info.Mode().Perm())
	})
}

func copyRegularFile(source, destination string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return err
	}
	if err := output.Chmod(mode); err != nil {
		output.Close()
		return err
	}
	if err := output.Sync(); err != nil {
		output.Close()
		return err
	}
	return output.Close()
}

func addAttributionRows(original []byte, repositoryURL, commit string, selected []Candidate) ([]byte, error) {
	lines := strings.Split(string(original), "\n")
	header := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "| Skill | Source | License |" {
			header = i
			break
		}
	}
	if header < 0 || header+1 >= len(lines) {
		return nil, fmt.Errorf("third-party attribution does not contain the expected skill table")
	}
	insert := header + 2
	for insert < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[insert]), "|") {
		insert++
	}
	rows := make([]string, len(selected))
	for i, candidate := range selected {
		source := repositoryURL + "/tree/" + commit
		if candidate.SourcePath != "." {
			parts := strings.Split(candidate.SourcePath, "/")
			for j := range parts {
				parts[j] = url.PathEscape(parts[j])
			}
			source += "/" + strings.Join(parts, "/")
		}
		rows[i] = fmt.Sprintf("| `%s` | %s | Unknown (unverified) |", candidate.Name, source)
	}
	lines = append(lines[:insert], append(rows, lines[insert:]...)...)
	return []byte(strings.Join(lines, "\n")), nil
}

func stageAttribution(thirdParty string, content []byte) (string, error) {
	temp, err := os.CreateTemp(thirdParty, ".attribution-import-*.tmp")
	if err != nil {
		return "", fmt.Errorf("stage third-party attribution: %w", err)
	}
	path := temp.Name()
	if err := temp.Chmod(0o644); err != nil {
		temp.Close()
		os.Remove(path)
		return "", err
	}
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		os.Remove(path)
		return "", err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		os.Remove(path)
		return "", err
	}
	if err := temp.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}
