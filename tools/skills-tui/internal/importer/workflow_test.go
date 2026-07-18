package importer_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

type checkoutProviderFunc func(context.Context, string) (*importer.Checkout, error)

func (f checkoutProviderFunc) Checkout(ctx context.Context, repositoryURL string) (*importer.Checkout, error) {
	return f(ctx, repositoryURL)
}
