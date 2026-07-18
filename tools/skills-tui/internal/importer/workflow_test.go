package importer_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-skills/tools/skills-tui/internal/importer"
)

func TestWorkflowRecordsHistoryOnlyAfterSuccessfulValidScan(t *testing.T) {
	repo := newImportRepo(t)
	tempRoot := t.TempDir()
	history := importer.HistoryStore{
		Path: filepath.Join(t.TempDir(), "import-repositories.json"),
		Now:  func() time.Time { return time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC) },
	}
	const commit = "0123456789abcdef0123456789abcdef01234567"
	provider := importer.GitHubCheckoutProvider{
		TempRoot: tempRoot,
		Runner: commandRunnerFunc(func(_ context.Context, command importer.Command) ([]byte, error) {
			if command.Args[0] == "clone" {
				writeSkill(t, command.Args[len(command.Args)-1], "portable", "portable", "Portable skill")
				return nil, nil
			}
			return []byte(commit), nil
		}),
	}
	workflow := importer.Workflow{
		History:    history,
		Checkouts:  provider,
		Repository: importer.RepositoryImporter{RepoDir: repo},
	}

	session, err := workflow.Scan(context.Background(), "https://github.com/Example/Skills.git/")
	if err != nil {
		t.Fatal(err)
	}
	if session.RepositoryURL != "https://github.com/example/skills" || session.Commit != commit || len(session.Candidates) != 1 || !session.Candidates[0].Valid {
		t.Fatalf("unexpected scan session: %#v", session)
	}
	records, err := history.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].URL != "https://github.com/example/skills" {
		t.Fatalf("successful valid scan was not recorded: %#v", records)
	}
	if _, err := os.Stat(session.Root); err != nil {
		t.Fatalf("session checkout should remain available: %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	assertDirectoryEmpty(t, tempRoot)
}

func TestWorkflowFailedScansDoNotRecordHistoryAndCleanCheckout(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		skill      string
		historyDoc string
	}{
		{name: "zero valid skills", skill: "---\nname: incomplete\n---\n"},
		{name: "history persistence failure", skill: "---\nname: valid\ndescription: Valid skill\n---\n", historyDoc: "not json\n"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := newImportRepo(t)
			tempRoot := t.TempDir()
			historyPath := filepath.Join(t.TempDir(), "import-repositories.json")
			if testCase.historyDoc != "" {
				if err := os.WriteFile(historyPath, []byte(testCase.historyDoc), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			history := importer.HistoryStore{Path: historyPath, Now: time.Now}
			provider := importer.GitHubCheckoutProvider{
				TempRoot: tempRoot,
				Runner: commandRunnerFunc(func(_ context.Context, command importer.Command) ([]byte, error) {
					if command.Args[0] == "clone" {
						writeRawSkill(t, command.Args[len(command.Args)-1], "candidate", testCase.skill)
						return nil, nil
					}
					return []byte("0123456789abcdef0123456789abcdef01234567"), nil
				}),
			}
			workflow := importer.Workflow{History: history, Checkouts: provider, Repository: importer.RepositoryImporter{RepoDir: repo}}

			if session, err := workflow.Scan(context.Background(), "https://github.com/example/source"); err == nil || session != nil {
				t.Fatalf("failed scan should not return a session: session=%#v err=%v", session, err)
			}
			assertDirectoryEmpty(t, tempRoot)
			if testCase.historyDoc == "" {
				records, err := history.List()
				if err != nil || len(records) != 0 {
					t.Fatalf("failed scan changed history: records=%#v err=%v", records, err)
				}
			} else if data, err := os.ReadFile(historyPath); err != nil || string(data) != testCase.historyDoc {
				t.Fatalf("malformed history was overwritten: data=%q err=%v", data, err)
			}
		})
	}
}

func TestWorkflowReportsCheckoutCleanupFailureAfterFailedScan(t *testing.T) {
	repo := newImportRepo(t)
	tempRoot := t.TempDir()
	cleanupErr := errors.New("injected checkout cleanup failure")
	provider := importer.GitHubCheckoutProvider{
		TempRoot: tempRoot,
		Runner: commandRunnerFunc(func(_ context.Context, command importer.Command) ([]byte, error) {
			if command.Args[0] == "clone" {
				writeRawSkill(t, command.Args[len(command.Args)-1], "candidate", "---\nname: incomplete\n---\n")
				return nil, nil
			}
			return []byte("0123456789abcdef0123456789abcdef01234567"), nil
		}),
		RemoveAll: func(string) error { return cleanupErr },
	}
	workflow := importer.Workflow{
		History:    importer.HistoryStore{Path: filepath.Join(t.TempDir(), "history.json"), Now: time.Now},
		Checkouts:  provider,
		Repository: importer.RepositoryImporter{RepoDir: repo},
	}

	session, err := workflow.Scan(context.Background(), "https://github.com/example/source")
	if session != nil || err == nil || !strings.Contains(err.Error(), "no valid") || !errors.Is(err, cleanupErr) {
		t.Fatalf("failed scan should preserve scan and cleanup errors: session=%#v err=%v", session, err)
	}
}

func TestWorkflowCancellationAfterCheckoutDoesNotRecordHistory(t *testing.T) {
	repo := newImportRepo(t)
	checkoutRoot := t.TempDir()
	writeSkill(t, checkoutRoot, "portable", "portable", "Portable skill")
	history := importer.HistoryStore{Path: filepath.Join(t.TempDir(), "history.json"), Now: time.Now}
	ctx, cancel := context.WithCancel(context.Background())
	provider := checkoutProviderFunc(func(context.Context, string) (*importer.Checkout, error) {
		cancel()
		return &importer.Checkout{
			Root: checkoutRoot, RepositoryURL: "https://github.com/example/source",
			Commit: "0123456789abcdef0123456789abcdef01234567",
		}, nil
	})
	workflow := importer.Workflow{History: history, Checkouts: provider, Repository: importer.RepositoryImporter{RepoDir: repo}}

	if session, err := workflow.Scan(ctx, "https://github.com/example/source"); !errors.Is(err, context.Canceled) || session != nil {
		t.Fatalf("cancelled scan should return context cancellation only: session=%#v err=%v", session, err)
	}
	records, err := history.List()
	if err != nil || len(records) != 0 {
		t.Fatalf("cancelled scan changed history: records=%#v err=%v", records, err)
	}
}

func TestWorkflowHistorySurvivesRestartRefreshesMRUAndDeletesOnlyPickerState(t *testing.T) {
	repo := newImportRepo(t)
	checkoutRoot := t.TempDir()
	writeSkill(t, checkoutRoot, "portable", "portable", "Portable skill")
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	history := importer.HistoryStore{Path: filepath.Join(t.TempDir(), "history.json"), Now: func() time.Time { return now }}
	provider := checkoutProviderFunc(func(_ context.Context, rawURL string) (*importer.Checkout, error) {
		canonical, err := importer.NormalizeGitHubURL(rawURL)
		if err != nil {
			return nil, err
		}
		return &importer.Checkout{
			Root: checkoutRoot, RepositoryURL: canonical,
			Commit: "0123456789abcdef0123456789abcdef01234567",
		}, nil
	})
	newWorkflow := func() importer.Workflow {
		return importer.Workflow{History: history, Checkouts: provider, Repository: importer.RepositoryImporter{RepoDir: repo}}
	}
	firstURL := "https://github.com/example/first"
	secondURL := "https://github.com/example/second"
	for _, repositoryURL := range []string{firstURL, secondURL} {
		session, err := newWorkflow().Scan(context.Background(), repositoryURL)
		if err != nil {
			t.Fatal(err)
		}
		if err := session.Close(); err != nil {
			t.Fatal(err)
		}
		now = now.Add(time.Hour)
	}

	restarted := newWorkflow()
	records, err := restarted.SavedRepositories()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].URL != secondURL || records[1].URL != firstURL {
		t.Fatalf("restart did not preserve MRU history: %#v", records)
	}
	session, err := restarted.Scan(context.Background(), firstURL+".git/")
	if err != nil {
		t.Fatal(err)
	}
	_ = session.Close()
	records, err = restarted.SavedRepositories()
	if err != nil || len(records) != 2 || records[0].URL != firstURL || records[1].URL != secondURL {
		t.Fatalf("reuse did not refresh MRU without duplicates: records=%#v err=%v", records, err)
	}

	importedPath := filepath.Join(repo, "third-party", "imported", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(importedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(importedPath, []byte("imported\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	attributionPath := filepath.Join(repo, "third-party", "ATTRIBUTION.md")
	attributionBefore, err := os.ReadFile(attributionPath)
	if err != nil {
		t.Fatal(err)
	}
	stagePath := filepath.Join(t.TempDir(), "staged", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(stagePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stagePath, []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(t.TempDir(), "installed")
	if err := os.Symlink(filepath.Dir(stagePath), linkPath); err != nil {
		t.Fatal(err)
	}

	if err := restarted.DeleteSavedRepository(firstURL); err != nil {
		t.Fatal(err)
	}
	records, err = restarted.SavedRepositories()
	if err != nil || len(records) != 1 || records[0].URL != secondURL {
		t.Fatalf("delete changed wrong history: records=%#v err=%v", records, err)
	}
	for _, path := range []string{importedPath, stagePath, linkPath} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("history deletion touched %s: %v", path, err)
		}
	}
	attributionAfter, err := os.ReadFile(attributionPath)
	if err != nil || !bytes.Equal(attributionAfter, attributionBefore) {
		t.Fatalf("history deletion changed attribution: err=%v\n before=%q\n after=%q", err, attributionBefore, attributionAfter)
	}
}

type checkoutProviderFunc func(context.Context, string) (*importer.Checkout, error)

func (f checkoutProviderFunc) Checkout(ctx context.Context, repositoryURL string) (*importer.Checkout, error) {
	return f(ctx, repositoryURL)
}
