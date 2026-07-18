package tui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-skills/tools/skills-tui/internal/importer"
	"agent-skills/tools/skills-tui/internal/skills"
)

func TestGitHubImportWorkflowFromPasteThroughSeparateApply(t *testing.T) {
	cfg := testConfig(t)
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "third-party"), 0o755); err != nil {
		t.Fatal(err)
	}
	const attribution = "# Attribution\n\n| Skill | Source | License |\n|---|---|---|\n"
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "third-party", "ATTRIBUTION.md"), []byte(attribution), 0o644); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	writeE2ESkill(t, source, "alpha", "Alpha skill")
	writeE2ESkill(t, source, "beta", "Beta skill")
	writeE2EFile(t, source, "alpha/scripts/run.sh", "#!/bin/sh\necho alpha\n", 0o755)

	tempCheckouts := t.TempDir()
	historyPath := filepath.Join(t.TempDir(), "import-repositories.json")
	const commit = "0123456789abcdef0123456789abcdef01234567"
	workflow := &importer.Workflow{
		History: importer.HistoryStore{
			Path: historyPath,
			Now:  func() time.Time { return time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC) },
		},
		Checkouts: importer.GitHubCheckoutProvider{
			TempRoot: tempCheckouts,
			Runner:   fixtureGitRunner{source: source, commit: commit},
		},
		Repository: importer.RepositoryImporter{RepoDir: cfg.RepoDir},
	}
	model, err := LoadSkills(cfg)
	if err != nil {
		t.Fatal(err)
	}

	reader, writer := io.Pipe()
	keys := NewKeyReader(reader)
	defer keys.Close()
	output := newWorkflowOutput()
	done := make(chan error, 1)
	go func() {
		done <- runLoopWithRepositoryImport(cfg, model, keys, output, 24, workflow)
	}()
	if _, err := io.WriteString(writer, "i\rhttps://github.com/example/source.git/\r"); err != nil {
		t.Fatal(err)
	}
	output.waitFor(t, "select skills to import")
	if _, err := io.WriteString(writer, "n \r"); err != nil {
		t.Fatal(err)
	}
	output.waitFor(t, "Imported 1 skill(s). Press Enter to apply installation.")

	if _, err := os.Stat(filepath.Join(cfg.RepoDir, "third-party", "alpha", "scripts", "run.sh")); err != nil {
		t.Fatalf("selected skill was not imported: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.RepoDir, "third-party", "beta")); !os.IsNotExist(err) {
		t.Fatalf("unselected skill was imported: %v", err)
	}
	assertE2ENotInstalled(t, cfg, "alpha")

	if _, err := io.WriteString(writer, "\r"); err != nil {
		t.Fatal(err)
	}
	output.waitFor(t, "Done. Press any key to continue, q to quit.")
	if _, err := io.WriteString(writer, "q"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(cfg.StageDir, "skills", "alpha")); err != nil {
		t.Fatalf("Apply did not stage imported skill: %v", err)
	}
	for _, root := range []string{".agents", ".claude", ".cursor"} {
		if _, err := os.Readlink(filepath.Join(cfg.Home, root, "skills", "alpha")); err != nil {
			t.Fatalf("Apply did not link alpha in %s: %v", root, err)
		}
	}
	attributionData, err := os.ReadFile(filepath.Join(cfg.RepoDir, "third-party", "ATTRIBUTION.md"))
	if err != nil {
		t.Fatal(err)
	}
	wantSource := "https://github.com/example/source/tree/" + commit + "/alpha"
	if !strings.Contains(string(attributionData), "| `alpha` | "+wantSource+" | Unknown (unverified) |") {
		t.Fatalf("pinned unverified attribution missing:\n%s", attributionData)
	}
	records, err := workflow.SavedRepositories()
	if err != nil || len(records) != 1 || records[0].URL != "https://github.com/example/source" {
		t.Fatalf("successful URL was not saved: records=%#v err=%v", records, err)
	}
	assertE2EDirectoryEmpty(t, tempCheckouts)
	if leftovers, err := filepath.Glob(filepath.Join(cfg.RepoDir, "third-party", ".import-stage-*")); err != nil || len(leftovers) != 0 {
		t.Fatalf("transaction staging leaked: %v err=%v", leftovers, err)
	}
}

func assertE2ENotInstalled(t *testing.T, cfg skills.Config, name string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(cfg.StageDir, "skills", name)); !os.IsNotExist(err) {
		t.Fatalf("%s was staged before Apply: %v", name, err)
	}
	for _, root := range []string{".agents", ".claude", ".cursor"} {
		if _, err := os.Lstat(filepath.Join(cfg.Home, root, "skills", name)); !os.IsNotExist(err) {
			t.Fatalf("%s was linked in %s before Apply: %v", name, root, err)
		}
	}
}

func writeE2ESkill(t *testing.T, root, name, description string) {
	t.Helper()
	writeE2EFile(t, root, filepath.Join(name, "SKILL.md"), "---\nname: "+name+"\ndescription: "+description+"\n---\n", 0o644)
}

func writeE2EFile(t *testing.T, root, rel, content string, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

type fixtureGitRunner struct {
	source string
	commit string
}

func (r fixtureGitRunner) Run(ctx context.Context, command importer.Command) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(command.Args) > 0 && command.Args[0] == "clone" {
		if err := copyE2ETree(r.source, command.Args[len(command.Args)-1]); err != nil {
			return nil, err
		}
		return nil, nil
	}
	return []byte(r.commit + "\n"), nil
}

func copyE2ETree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, data, info.Mode().Perm()); err != nil {
			return err
		}
		return os.Chmod(target, info.Mode().Perm())
	})
}

type workflowOutput struct {
	mu      sync.Mutex
	buffer  bytes.Buffer
	updates chan struct{}
}

func newWorkflowOutput() *workflowOutput {
	return &workflowOutput{updates: make(chan struct{})}
}

func (w *workflowOutput) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buffer.Write(data)
	close(w.updates)
	w.updates = make(chan struct{})
	return n, err
}

func (w *workflowOutput) waitFor(t *testing.T, text string) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		w.mu.Lock()
		if strings.Contains(w.buffer.String(), text) {
			w.mu.Unlock()
			return
		}
		updates := w.updates
		w.mu.Unlock()
		select {
		case <-updates:
		case <-deadline.C:
			w.mu.Lock()
			output := w.buffer.String()
			w.mu.Unlock()
			t.Fatalf("timed out waiting for %q in output:\n%s", text, output)
		}
	}
}

func (w *workflowOutput) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return fmt.Sprint(w.buffer.String())
}

func assertE2EDirectoryEmpty(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("directory %s is not empty: %v", path, entries)
	}
}
