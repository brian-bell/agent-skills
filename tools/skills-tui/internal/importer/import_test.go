package importer_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"agent-skills/tools/skills-tui/internal/importer"
)

const attributionFixture = `# Attribution

The skills in this directory are sourced from third-party projects.

| Skill | Source | License |
|---|---|---|
| ` + "`existing`" + ` | https://github.com/example/existing | MIT |

Existing verification note.
`

func TestRepositoryImporterCopiesSelectedSkillAndAddsPinnedProvenance(t *testing.T) {
	repo := newImportRepo(t)
	checkout := t.TempDir()
	writeSkill(t, checkout, "alpha", "alpha", "Alpha skill")
	writeSkill(t, checkout, "beta", "beta", "Beta skill")
	writeImportFile(t, checkout, "alpha/scripts/run.sh", "#!/bin/sh\necho alpha\n", 0o755)
	writeImportFile(t, checkout, "alpha/templates/prompt.md", "prompt\n", 0o644)
	writeImportFile(t, checkout, "alpha/references/guide.md", "guide\n", 0o644)
	writeImportFile(t, checkout, "alpha/.git/config", "must not be copied\n", 0o644)
	candidates, err := importer.Scan(checkout)
	if err != nil {
		t.Fatal(err)
	}

	const commit = "0123456789abcdef0123456789abcdef01234567"
	service := importer.RepositoryImporter{RepoDir: repo}
	imported, err := service.Import(context.Background(), importer.ImportRequest{
		CheckoutRoot:  checkout,
		RepositoryURL: "https://github.com/example/source",
		Commit:        commit,
		Candidates:    candidates,
		SelectedIDs:   []string{"alpha"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(imported, []string{"alpha"}) {
		t.Fatalf("unexpected imported names: %v", imported)
	}
	for _, rel := range []string{"SKILL.md", "scripts/run.sh", "templates/prompt.md", "references/guide.md"} {
		if _, err := os.Stat(filepath.Join(repo, "third-party", "alpha", rel)); err != nil {
			t.Fatalf("supporting file %s was not preserved: %v", rel, err)
		}
	}
	if info, err := os.Stat(filepath.Join(repo, "third-party", "alpha", "scripts/run.sh")); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("executable mode was not preserved: info=%v err=%v", info, err)
	}
	if _, err := os.Stat(filepath.Join(repo, "third-party", "beta")); !os.IsNotExist(err) {
		t.Fatalf("unselected beta should not be imported, stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "third-party", "alpha", ".git")); !os.IsNotExist(err) {
		t.Fatalf(".git metadata should not be copied, stat error: %v", err)
	}
	attribution, err := os.ReadFile(filepath.Join(repo, "third-party", "ATTRIBUTION.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"| `alpha` | https://github.com/example/source/tree/" + commit + "/alpha | Unknown (unverified) |",
		"Existing verification note.",
	} {
		if !strings.Contains(string(attribution), want) {
			t.Fatalf("attribution missing %q:\n%s", want, attribution)
		}
	}
}

func TestValidateCandidatesDisablesExistingInstallNameCollisions(t *testing.T) {
	repo := newImportRepo(t)
	for _, rel := range []string{
		"skills/first-party/SKILL.md",
		"third-party/third-party/SKILL.md",
		"agent-teams/reviewer-team/SKILL.md",
	} {
		writeImportFile(t, repo, rel, "stub\n", 0o644)
	}
	candidates := []importer.Candidate{
		{ID: "one", SourcePath: "one", Name: "First-Party", Valid: true},
		{ID: "two", SourcePath: "two", Name: "third-party", Valid: true},
		{ID: "three", SourcePath: "three", Name: "reviewer", Valid: true},
		{ID: "four", SourcePath: "four", Name: "ATTRIBUTION.md", Valid: true},
		{ID: "five", SourcePath: "five", Name: "fresh", Valid: true},
	}

	validated, err := (importer.RepositoryImporter{RepoDir: repo}).ValidateCandidates(candidates)
	if err != nil {
		t.Fatal(err)
	}
	for i, kind := range []string{"first-party", "third-party", "agent-team"} {
		if validated[i].Valid || !strings.Contains(validated[i].Reason, kind) {
			t.Fatalf("candidate %q should be disabled by %s collision, got %#v", validated[i].Name, kind, validated[i])
		}
	}
	if validated[3].Valid || !strings.Contains(validated[3].Reason, "third-party") {
		t.Fatalf("existing regular destination should be disabled, got %#v", validated[3])
	}
	if !validated[4].Valid || validated[4].Reason != "" {
		t.Fatalf("fresh candidate should stay enabled, got %#v", validated[4])
	}
}

func TestRepositoryImporterRejectsDuplicateSelectedDestinationsBeforeStaging(t *testing.T) {
	repo := newImportRepo(t)
	checkout := t.TempDir()
	writeSkill(t, checkout, "one", "same-name", "First source")
	writeSkill(t, checkout, "two", "same-name", "Second source")
	before, err := os.ReadFile(filepath.Join(repo, "third-party", "ATTRIBUTION.md"))
	if err != nil {
		t.Fatal(err)
	}
	request := importer.ImportRequest{
		CheckoutRoot:  checkout,
		RepositoryURL: "https://github.com/example/source",
		Commit:        "0123456789abcdef0123456789abcdef01234567",
		Candidates: []importer.Candidate{
			{ID: "one", SourcePath: "one", Name: "same-name", Description: "First source", Valid: true},
			{ID: "two", SourcePath: "two", Name: "Same-Name", Description: "Second source", Valid: true},
		},
		SelectedIDs: []string{"one", "two"},
	}

	_, err = (importer.RepositoryImporter{RepoDir: repo}).Import(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "duplicate selected install name") {
		t.Fatalf("expected duplicate destination preflight error, got %v", err)
	}
	assertThirdPartyUnchanged(t, repo, before)
}

func TestRepositoryImporterRevalidatesUnsafeSelectedNamesBeforeFilesystemUse(t *testing.T) {
	repo := newImportRepo(t)
	checkout := t.TempDir()
	writeSkill(t, checkout, "source", "source", "Safe source metadata")
	before, err := os.ReadFile(filepath.Join(repo, "third-party", "ATTRIBUTION.md"))
	if err != nil {
		t.Fatal(err)
	}
	request := importer.ImportRequest{
		CheckoutRoot:  checkout,
		RepositoryURL: "https://github.com/example/source",
		Commit:        "0123456789abcdef0123456789abcdef01234567",
		Candidates: []importer.Candidate{
			{ID: "source", SourcePath: "source", Name: "../escape", Description: "Unsafe destination", Valid: true},
		},
		SelectedIDs: []string{"source"},
	}

	_, err = (importer.RepositoryImporter{RepoDir: repo}).Import(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "unsafe install name") {
		t.Fatalf("expected unsafe-name preflight error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "escape")); !os.IsNotExist(err) {
		t.Fatalf("unsafe candidate escaped third-party, stat error: %v", err)
	}
	assertThirdPartyUnchanged(t, repo, before)
}

func TestRepositoryImporterRejectsSymlinksAndSpecialFilesWithoutPartialImport(t *testing.T) {
	for _, testCase := range []struct {
		name string
		add  func(*testing.T, string)
	}{
		{
			name: "symlink",
			add: func(t *testing.T, skillDir string) {
				if err := os.Symlink("SKILL.md", filepath.Join(skillDir, "linked.md")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "FIFO",
			add: func(t *testing.T, skillDir string) {
				if err := syscall.Mkfifo(filepath.Join(skillDir, "pipe"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := newImportRepo(t)
			checkout := t.TempDir()
			writeSkill(t, checkout, "unsafe-tree", "unsafe-tree", "Unsafe tree")
			testCase.add(t, filepath.Join(checkout, "unsafe-tree"))
			candidates, err := importer.Scan(checkout)
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(filepath.Join(repo, "third-party", "ATTRIBUTION.md"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = (importer.RepositoryImporter{RepoDir: repo}).Import(context.Background(), importer.ImportRequest{
				CheckoutRoot: checkout, RepositoryURL: "https://github.com/example/source",
				Commit: "0123456789abcdef0123456789abcdef01234567", Candidates: candidates,
				SelectedIDs: []string{"unsafe-tree"},
			})
			if err == nil {
				t.Fatalf("import should reject %s", testCase.name)
			}
			assertThirdPartyUnchanged(t, repo, before)
		})
	}
}

func TestRepositoryImporterRollsBackPublishedBatchWhenPublicationFails(t *testing.T) {
	repo := newImportRepo(t)
	checkout := t.TempDir()
	writeSkill(t, checkout, "one", "one", "First skill")
	writeSkill(t, checkout, "two", "two", "Second skill")
	candidates, err := importer.Scan(checkout)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(repo, "third-party", "ATTRIBUTION.md"))
	if err != nil {
		t.Fatal(err)
	}
	published := 0
	service := importer.RepositoryImporter{
		RepoDir: repo,
		Publish: func(source, destination string) error {
			published++
			if published == 2 {
				return errors.New("injected publication failure")
			}
			return os.Rename(source, destination)
		},
	}
	_, err = service.Import(context.Background(), importer.ImportRequest{
		CheckoutRoot: checkout, RepositoryURL: "https://github.com/example/source",
		Commit: "0123456789abcdef0123456789abcdef01234567", Candidates: candidates,
		SelectedIDs: []string{"one", "two"},
	})
	if err == nil || !strings.Contains(err.Error(), "injected publication failure") {
		t.Fatalf("expected injected batch failure, got %v", err)
	}
	assertThirdPartyUnchanged(t, repo, before)
}

func TestRepositoryImporterReportsRollbackCleanupFailure(t *testing.T) {
	repo := newImportRepo(t)
	checkout := t.TempDir()
	writeSkill(t, checkout, "one", "one", "First skill")
	writeSkill(t, checkout, "two", "two", "Second skill")
	candidates, err := importer.Scan(checkout)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(repo, "third-party", "ATTRIBUTION.md"))
	if err != nil {
		t.Fatal(err)
	}
	publicationErr := errors.New("injected publication failure")
	rollbackErr := errors.New("injected rollback cleanup failure")
	published := 0
	service := importer.RepositoryImporter{
		RepoDir: repo,
		Publish: func(source, destination string) error {
			published++
			if published == 2 {
				return publicationErr
			}
			return os.Rename(source, destination)
		},
		RemovePublished: func(string) error { return rollbackErr },
	}

	_, err = service.Import(context.Background(), importer.ImportRequest{
		CheckoutRoot: checkout, RepositoryURL: "https://github.com/example/source",
		Commit: "0123456789abcdef0123456789abcdef01234567", Candidates: candidates,
		SelectedIDs: []string{"one", "two"},
	})
	if !errors.Is(err, publicationErr) || !errors.Is(err, rollbackErr) {
		t.Fatalf("failed import should preserve publication and rollback errors, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "third-party", "one")); err != nil {
		t.Fatalf("injected cleanup failure should leave the published destination for inspection: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(repo, "third-party", "ATTRIBUTION.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("attribution changed despite publication failure:\n before=%q\n after=%q", before, after)
	}
}

func TestRepositoryImporterNeverOverwritesExistingDestinationOrMalformedAttribution(t *testing.T) {
	t.Run("existing destination", func(t *testing.T) {
		repo := newImportRepo(t)
		checkout := t.TempDir()
		writeSkill(t, checkout, "existing", "existing", "Incoming skill")
		writeImportFile(t, repo, "third-party/existing/sentinel.txt", "keep me\n", 0o644)
		candidates, err := importer.Scan(checkout)
		if err != nil {
			t.Fatal(err)
		}
		_, err = (importer.RepositoryImporter{RepoDir: repo}).Import(context.Background(), importer.ImportRequest{
			CheckoutRoot: checkout, RepositoryURL: "https://github.com/example/source",
			Commit: "0123456789abcdef0123456789abcdef01234567", Candidates: candidates,
			SelectedIDs: []string{"existing"},
		})
		if err == nil {
			t.Fatal("existing destination should refuse import")
		}
		data, err := os.ReadFile(filepath.Join(repo, "third-party", "existing", "sentinel.txt"))
		if err != nil || string(data) != "keep me\n" {
			t.Fatalf("existing destination was changed: data=%q err=%v", data, err)
		}
	})

	t.Run("malformed attribution", func(t *testing.T) {
		repo := newImportRepo(t)
		checkout := t.TempDir()
		writeSkill(t, checkout, "fresh", "fresh", "Fresh skill")
		candidates, err := importer.Scan(checkout)
		if err != nil {
			t.Fatal(err)
		}
		malformed := []byte("# no attribution table\n")
		if err := os.WriteFile(filepath.Join(repo, "third-party", "ATTRIBUTION.md"), malformed, 0o644); err != nil {
			t.Fatal(err)
		}
		_, err = (importer.RepositoryImporter{RepoDir: repo}).Import(context.Background(), importer.ImportRequest{
			CheckoutRoot: checkout, RepositoryURL: "https://github.com/example/source",
			Commit: "0123456789abcdef0123456789abcdef01234567", Candidates: candidates,
			SelectedIDs: []string{"fresh"},
		})
		if err == nil || !strings.Contains(err.Error(), "expected skill table") {
			t.Fatalf("expected attribution validation error, got %v", err)
		}
		assertThirdPartyUnchanged(t, repo, malformed)
	})
}

func TestRepositoryImporterCancelsWhileWaitingForAnotherImport(t *testing.T) {
	repo := newImportRepo(t)
	prepare := func(name string) importer.ImportRequest {
		checkout := t.TempDir()
		writeSkill(t, checkout, name, name, "Concurrent skill")
		candidates, err := importer.Scan(checkout)
		if err != nil {
			t.Fatal(err)
		}
		return importer.ImportRequest{
			CheckoutRoot: checkout, RepositoryURL: "https://github.com/example/source",
			Commit: "0123456789abcdef0123456789abcdef01234567", Candidates: candidates,
			SelectedIDs: []string{name},
		}
	}
	firstRequest := prepare("first")
	secondRequest := prepare("second")

	firstAtPublish := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstResult := make(chan error, 1)
	go func() {
		_, err := (importer.RepositoryImporter{
			RepoDir: repo,
			Publish: func(source, destination string) error {
				close(firstAtPublish)
				<-releaseFirst
				return os.Rename(source, destination)
			},
		}).Import(context.Background(), firstRequest)
		firstResult <- err
	}()
	<-firstAtPublish
	released := false
	release := func() {
		if !released {
			close(releaseFirst)
			released = true
		}
	}
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	secondStarted := make(chan struct{})
	secondResult := make(chan error, 1)
	go func() {
		close(secondStarted)
		_, err := (importer.RepositoryImporter{RepoDir: repo}).Import(ctx, secondRequest)
		secondResult <- err
	}()
	<-secondStarted
	time.Sleep(25 * time.Millisecond)
	cancel()

	select {
	case err := <-secondResult:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("waiting import should return context cancellation, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		release()
		<-firstResult
		t.Fatal("waiting import did not return after cancellation")
	}
	if _, err := os.Stat(filepath.Join(repo, "third-party", "second")); !os.IsNotExist(err) {
		t.Fatalf("cancelled waiting import published content: %v", err)
	}

	release()
	if err := <-firstResult; err != nil {
		t.Fatalf("lock-owning import failed: %v", err)
	}
}

func TestRepositoryImporterSerializesConcurrentAttributionTransactions(t *testing.T) {
	repo := newImportRepo(t)
	const imports = 12
	type preparedImport struct {
		request importer.ImportRequest
		name    string
		repoDir string
	}
	alias := filepath.Join(t.TempDir(), "repo-alias")
	if err := os.Symlink(repo, alias); err != nil {
		t.Fatal(err)
	}
	prepared := make([]preparedImport, imports)
	for i := 0; i < imports; i++ {
		name := fmt.Sprintf("skill-%02d", i)
		checkout := t.TempDir()
		writeSkill(t, checkout, name, name, "Concurrent skill")
		candidates, err := importer.Scan(checkout)
		if err != nil {
			t.Fatal(err)
		}
		prepared[i] = preparedImport{
			name:    name,
			repoDir: []string{repo, alias}[i%2],
			request: importer.ImportRequest{
				CheckoutRoot: checkout, RepositoryURL: "https://github.com/example/source",
				Commit: "0123456789abcdef0123456789abcdef01234567", Candidates: candidates,
				SelectedIDs: []string{name},
			},
		}
	}
	start := make(chan struct{})
	errors := make(chan error, imports)
	var group sync.WaitGroup
	for _, item := range prepared {
		item := item
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			_, err := (importer.RepositoryImporter{RepoDir: item.repoDir}).Import(context.Background(), item.request)
			errors <- err
		}()
	}
	close(start)
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	attribution, err := os.ReadFile(filepath.Join(repo, "third-party", "ATTRIBUTION.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range prepared {
		if !strings.Contains(string(attribution), "| `"+item.name+"` |") {
			t.Fatalf("concurrent import lost attribution for %s:\n%s", item.name, attribution)
		}
		if _, err := os.Stat(filepath.Join(repo, "third-party", item.name)); err != nil {
			t.Fatalf("concurrent import lost directory for %s: %v", item.name, err)
		}
	}
}

func TestRepositoryImporterPreservesFileModesUnderRestrictiveUmask(t *testing.T) {
	repo := newImportRepo(t)
	checkout := t.TempDir()
	writeSkill(t, checkout, "mode-test", "mode-test", "Mode preservation")
	writeImportFile(t, checkout, "mode-test/references/shared.md", "shared\n", 0o664)
	if err := os.Chmod(filepath.Join(checkout, "mode-test", "references", "shared.md"), 0o664); err != nil {
		t.Fatal(err)
	}
	candidates, err := importer.Scan(checkout)
	if err != nil {
		t.Fatal(err)
	}
	previousUmask := syscall.Umask(0o077)
	defer syscall.Umask(previousUmask)
	_, err = (importer.RepositoryImporter{RepoDir: repo}).Import(context.Background(), importer.ImportRequest{
		CheckoutRoot: checkout, RepositoryURL: "https://github.com/example/source",
		Commit: "0123456789abcdef0123456789abcdef01234567", Candidates: candidates,
		SelectedIDs: []string{"mode-test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(repo, "third-party", "mode-test", "references", "shared.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o664 {
		t.Fatalf("imported file mode should survive umask, got %04o, want 0664", got)
	}
}

func assertThirdPartyUnchanged(t *testing.T, repo string, attribution []byte) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repo, "third-party"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "ATTRIBUTION.md" {
		t.Fatalf("third-party changed after failed import: %v", entries)
	}
	got, err := os.ReadFile(filepath.Join(repo, "third-party", "ATTRIBUTION.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, attribution) {
		t.Fatalf("attribution changed after failed import:\n got %q\nwant %q", got, attribution)
	}
}

func newImportRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, rel := range []string{"skills", "third-party", "agent-teams"} {
		if err := os.MkdirAll(filepath.Join(repo, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeImportFile(t, repo, "third-party/ATTRIBUTION.md", attributionFixture, 0o644)
	return repo
}

func writeImportFile(t *testing.T, root, rel, content string, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}
