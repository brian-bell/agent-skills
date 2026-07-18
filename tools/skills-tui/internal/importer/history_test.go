package importer_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"agent-skills/tools/skills-tui/internal/importer"
)

func TestHistoryStoreTreatsAMissingFileAsEmpty(t *testing.T) {
	store := importer.HistoryStore{
		Path: filepath.Join(t.TempDir(), "nested", "import-repositories.json"),
		Now:  func() time.Time { return time.Unix(0, 0).UTC() },
	}

	records, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("missing history should be empty, got %#v", records)
	}

	defaultPath, err := importer.DefaultHistoryPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(defaultPath) != "import-repositories.json" || filepath.Base(filepath.Dir(defaultPath)) != "agent-skills" {
		t.Fatalf("default history path should be beneath the agent-skills user config directory, got %q", defaultPath)
	}
}

func TestHistoryStoreNormalizesDeduplicatesAndSortsByRecentUse(t *testing.T) {
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	store := importer.HistoryStore{
		Path: filepath.Join(t.TempDir(), "import-repositories.json"),
		Now:  func() time.Time { return now },
	}

	if err := store.Record("https://github.com/Owner/Repo.git/"); err != nil {
		t.Fatal(err)
	}
	addedRepo := now
	now = now.Add(time.Hour)
	if err := store.Record("https://github.com/another/project"); err != nil {
		t.Fatal(err)
	}
	addedProject := now
	now = now.Add(time.Hour)
	if err := store.Record("https://github.com/owner/repo/"); err != nil {
		t.Fatal(err)
	}

	records, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	want := []importer.RepositoryRecord{
		{URL: "https://github.com/owner/repo", AddedAt: addedRepo, LastUsedAt: now},
		{URL: "https://github.com/another/project", AddedAt: addedProject, LastUsedAt: addedProject},
	}
	if !reflect.DeepEqual(records, want) {
		t.Fatalf("unexpected MRU history:\n got: %#v\nwant: %#v", records, want)
	}
}

func TestHistoryStoreDeletesExactlyOneNormalizedURL(t *testing.T) {
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	store := importer.HistoryStore{
		Path: filepath.Join(t.TempDir(), "import-repositories.json"),
		Now: func() time.Time {
			now = now.Add(time.Minute)
			return now
		},
	}
	for _, repositoryURL := range []string{
		"https://github.com/one/first",
		"https://github.com/two/second",
		"https://github.com/three/third",
	} {
		if err := store.Record(repositoryURL); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.Delete("https://github.com/TWO/SECOND.git/"); err != nil {
		t.Fatal(err)
	}
	records, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(records))
	for i, record := range records {
		got[i] = record.URL
	}
	want := []string{"https://github.com/three/third", "https://github.com/one/first"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("delete changed the wrong records: got %v, want %v", got, want)
	}
}

func TestHistoryStoreWritesVersionedJSONAtomicallyWithPrivatePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "import-repositories.json")
	if err := os.WriteFile(path, []byte("{\"version\":1,\"repositories\":[]}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := importer.HistoryStore{
		Path: path,
		Now:  func() time.Time { return time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC) },
	}
	if err := store.Record("https://github.com/example/skill-set"); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("history permissions should be user-only, got %04o", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("history is not valid JSON: %v", err)
	}
	if document.Version != 1 {
		t.Fatalf("expected version 1 document, got %d", document.Version)
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, ".import-repositories-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("atomic history write left temporary files: %v", leftovers)
	}
}

func TestHistoryStoreRejectsMalformedAndUnsupportedDocumentsWithoutOverwrite(t *testing.T) {
	for name, original := range map[string][]byte{
		"malformed":           []byte("{\"version\":1,\"repositories\":null}\n"),
		"unsupported version": []byte("{\"version\":99,\"repositories\":[]}\n"),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "import-repositories.json")
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			store := importer.HistoryStore{Path: path, Now: time.Now}

			if _, err := store.List(); err == nil {
				t.Fatal("List should reject invalid persisted history")
			}
			if err := store.Record("https://github.com/example/repo"); err == nil {
				t.Fatal("Record should not overwrite invalid persisted history")
			}
			if err := store.Delete("https://github.com/example/repo"); err == nil {
				t.Fatal("Delete should not overwrite invalid persisted history")
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, original) {
				t.Fatalf("invalid history was changed:\n got %q\nwant %q", got, original)
			}
		})
	}
}

func TestHistoryStoreSerializesConcurrentReadModifyWriteUpdates(t *testing.T) {
	store := importer.HistoryStore{
		Path: filepath.Join(t.TempDir(), "import-repositories.json"),
		Now:  func() time.Time { return time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC) },
	}
	const writers = 32
	start := make(chan struct{})
	errors := make(chan error, writers)
	var group sync.WaitGroup
	for i := 0; i < writers; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start
			errors <- store.Record(fmt.Sprintf("https://github.com/example/repo-%02d", index))
		}(i)
	}
	close(start)
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}

	records, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != writers {
		t.Fatalf("concurrent updates were lost: got %d records, want %d", len(records), writers)
	}
}

func TestHistoryStoreRejectsInvalidPersistedRepositoryRecords(t *testing.T) {
	const timestamp = `"2026-07-17T12:00:00Z"`
	valid := `{"url":"https://github.com/example/repo","added_at":` + timestamp + `,"last_used_at":` + timestamp + `}`
	cases := map[string]string{
		"missing URL":      `{"added_at":` + timestamp + `,"last_used_at":` + timestamp + `}`,
		"noncanonical URL": `{"url":"https://github.com/Example/Repo.git","added_at":` + timestamp + `,"last_used_at":` + timestamp + `}`,
		"missing added at": `{"url":"https://github.com/example/repo","last_used_at":` + timestamp + `}`,
		"missing used at":  `{"url":"https://github.com/example/repo","added_at":` + timestamp + `}`,
		"duplicate URL":    valid + `,` + valid,
	}
	for name, records := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "import-repositories.json")
			document := []byte(`{"version":1,"repositories":[` + records + `]}`)
			if err := os.WriteFile(path, document, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := (importer.HistoryStore{Path: path}).List(); err == nil {
				t.Fatalf("List accepted invalid persisted record: %s", document)
			}
		})
	}
}

func TestHistoryStoreKeepsRefreshTimestampsValidWhenTheClockMovesBackward(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	store := importer.HistoryStore{
		Path: filepath.Join(t.TempDir(), "import-repositories.json"),
		Now:  func() time.Time { return now },
	}
	if err := store.Record("https://github.com/example/repo"); err != nil {
		t.Fatal(err)
	}
	original := now
	now = now.Add(-time.Hour)
	if err := store.Record("https://github.com/example/repo"); err != nil {
		t.Fatal(err)
	}
	records, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || !records[0].AddedAt.Equal(original) || !records[0].LastUsedAt.Equal(original) {
		t.Fatalf("clock rollback should preserve monotonic timestamps, got %#v", records)
	}
}
