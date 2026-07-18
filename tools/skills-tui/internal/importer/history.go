package importer

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const historyVersion = 1

// RepositoryRecord is one successfully scanned repository in picker history.
type RepositoryRecord struct {
	URL        string    `json:"url"`
	AddedAt    time.Time `json:"added_at"`
	LastUsedAt time.Time `json:"last_used_at"`
}

type historyDocument struct {
	Version      int                `json:"version"`
	Repositories []RepositoryRecord `json:"repositories"`
}

// HistoryStore persists user-local import repository history. Path and Now
// are explicit so callers can keep configuration and time deterministic.
type HistoryStore struct {
	Path string
	Now  func() time.Time
}

var historyProcessLocks sync.Map

// DefaultHistoryPath resolves the history document beneath the user config
// directory for the current platform.
func DefaultHistoryPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "agent-skills", "import-repositories.json"), nil
}

// List returns saved repositories in persisted order.
func (s HistoryStore) List() ([]RepositoryRecord, error) {
	return s.load()
}

func (s HistoryStore) load() ([]RepositoryRecord, error) {
	data, err := os.ReadFile(s.Path)
	if os.IsNotExist(err) {
		return []RepositoryRecord{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read import repository history: %w", err)
	}
	var document historyDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("read import repository history: %w", err)
	}
	if document.Version != historyVersion {
		return nil, fmt.Errorf("unsupported import repository history version %d", document.Version)
	}
	if document.Repositories == nil {
		return nil, fmt.Errorf("malformed import repository history: repositories must be an array")
	}
	seen := make(map[string]struct{}, len(document.Repositories))
	for _, record := range document.Repositories {
		canonical, err := NormalizeGitHubURL(record.URL)
		if err != nil || canonical != record.URL {
			return nil, fmt.Errorf("malformed import repository history: repository URL %q is not canonical", record.URL)
		}
		if record.AddedAt.IsZero() || record.LastUsedAt.IsZero() || record.LastUsedAt.Before(record.AddedAt) {
			return nil, fmt.Errorf("malformed import repository history: repository %q has invalid timestamps", record.URL)
		}
		if _, duplicate := seen[record.URL]; duplicate {
			return nil, fmt.Errorf("malformed import repository history: duplicate repository URL %q", record.URL)
		}
		seen[record.URL] = struct{}{}
	}
	sortRecords(document.Repositories)
	return document.Repositories, nil
}

// Record normalizes and upserts one successfully used repository URL.
func (s HistoryStore) Record(rawURL string) error {
	canonical, err := NormalizeGitHubURL(rawURL)
	if err != nil {
		return err
	}
	return s.withMutationLock(func() error {
		records, err := s.load()
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if s.Now != nil {
			now = s.Now().UTC()
		}
		found := false
		for i := range records {
			if records[i].URL == canonical {
				if now.After(records[i].LastUsedAt) {
					records[i].LastUsedAt = now
				}
				found = true
				break
			}
		}
		if !found {
			records = append(records, RepositoryRecord{URL: canonical, AddedAt: now, LastUsedAt: now})
		}
		sortRecords(records)
		return s.write(historyDocument{Version: historyVersion, Repositories: records})
	})
}

// Delete removes exactly the saved record equivalent to rawURL. A URL that is
// not present is a successful no-op.
func (s HistoryStore) Delete(rawURL string) error {
	canonical, err := NormalizeGitHubURL(rawURL)
	if err != nil {
		return err
	}
	return s.withMutationLock(func() error {
		records, err := s.load()
		if err != nil {
			return err
		}
		kept := make([]RepositoryRecord, 0, len(records))
		for _, record := range records {
			if record.URL != canonical {
				kept = append(kept, record)
			}
		}
		if len(kept) == len(records) {
			return nil
		}
		return s.write(historyDocument{Version: historyVersion, Repositories: kept})
	})
}

func (s HistoryStore) withMutationLock(mutate func() error) error {
	processLockValue, _ := historyProcessLocks.LoadOrStore(s.Path, &sync.Mutex{})
	processLock := processLockValue.(*sync.Mutex)
	processLock.Lock()
	defer processLock.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("create import repository history directory: %w", err)
	}
	lockPath := s.Path + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open import repository history lock: %w", err)
	}
	defer lockFile.Close()
	if err := lockFile.Chmod(0o600); err != nil {
		return fmt.Errorf("protect import repository history lock: %w", err)
	}
	unlock, err := lockHistoryFile(lockFile)
	if err != nil {
		return fmt.Errorf("lock import repository history: %w", err)
	}
	defer unlock()
	return mutate()
}

func (s HistoryStore) write(document historyDocument) error {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create import repository history directory: %w", err)
	}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode import repository history: %w", err)
	}
	data = append(data, '\n')
	temp, err := os.CreateTemp(dir, ".import-repositories-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary import repository history: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return fmt.Errorf("protect temporary import repository history: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary import repository history: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync temporary import repository history: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary import repository history: %w", err)
	}
	if err := os.Rename(tempPath, s.Path); err != nil {
		return fmt.Errorf("publish import repository history: %w", err)
	}
	return nil
}

func sortRecords(records []RepositoryRecord) {
	sort.Slice(records, func(i, j int) bool {
		if records[i].LastUsedAt.Equal(records[j].LastUsedAt) {
			return records[i].URL < records[j].URL
		}
		return records[i].LastUsedAt.After(records[j].LastUsedAt)
	})
}

// NormalizeGitHubURL returns the canonical URL accepted by the first import
// release: an HTTPS github.com owner/repository URL without suffix or slash.
func NormalizeGitHubURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid GitHub repository URL: %w", err)
	}
	if parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "github.com") || parsed.Port() != "" {
		return "", fmt.Errorf("repository URL must use https://github.com")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" || parsed.RawPath != "" {
		return "", fmt.Errorf("repository URL must not contain credentials, escapes, query parameters, or fragments")
	}
	path := strings.TrimSuffix(parsed.Path, "/")
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("repository URL must contain only owner/repository")
	}
	repository := parts[1]
	if strings.HasSuffix(strings.ToLower(repository), ".git") {
		repository = repository[:len(repository)-len(".git")]
	}
	if !isSafeRepositoryPart(parts[0]) || !isSafeRepositoryPart(repository) {
		return "", fmt.Errorf("repository URL contains an unsafe owner or repository name")
	}
	return "https://github.com/" + strings.ToLower(parts[0]) + "/" + strings.ToLower(repository), nil
}

func isSafeRepositoryPart(part string) bool {
	return part != "." && part != ".." && safeInstallName.MatchString(part)
}
