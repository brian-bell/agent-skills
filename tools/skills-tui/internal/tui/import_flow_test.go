package tui

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-skills/tools/skills-tui/internal/importer"
	"agent-skills/tools/skills-tui/internal/skills"
)

func TestRepositoryPickerRendersNewURLFirstAndSavedRepositoriesInMRUOrder(t *testing.T) {
	empty := newRepositoryPicker(nil)
	emptyFrame := renderRepositoryPicker(empty, 24)
	if !strings.Contains(emptyFrame, ">") || !strings.Contains(emptyFrame, "Paste a new repository URL") {
		t.Fatalf("empty picker should select the new URL row:\n%s", emptyFrame)
	}

	newer := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	populated := newRepositoryPicker([]importer.RepositoryRecord{
		{URL: "https://github.com/newer/repo", LastUsedAt: newer},
		{URL: "https://github.com/older/repo", LastUsedAt: newer.Add(-time.Hour)},
	})
	frame := renderRepositoryPicker(populated, 24)
	newRow := strings.Index(frame, "Paste a new repository URL")
	newerRow := strings.Index(frame, "https://github.com/newer/repo")
	olderRow := strings.Index(frame, "https://github.com/older/repo")
	if newRow < 0 || newerRow <= newRow || olderRow <= newerRow {
		t.Fatalf("picker rows are not in new-then-MRU order:\n%s", frame)
	}
	for _, hint := range []string{"↑↓ move", "enter select", "d delete", "esc back"} {
		if !strings.Contains(frame, hint) {
			t.Fatalf("picker missing key hint %q:\n%s", hint, frame)
		}
	}

	populated.moveUp()
	if populated.Cursor != 2 {
		t.Fatalf("move up should wrap to the final saved row, got %d", populated.Cursor)
	}
	populated.moveDown()
	if populated.Cursor != 0 {
		t.Fatalf("move down should wrap to the new URL row, got %d", populated.Cursor)
	}
}

func TestRepositoryURLInputHandlesPasteEditingSubmitAndCancel(t *testing.T) {
	input := repositoryURLInput{}
	for _, key := range strings.Split("https://github.com/example/repoX", "") {
		input.handle(key)
	}
	input.handle("\x7f")
	if input.Value != "https://github.com/example/repo" {
		t.Fatalf("backspace did not edit pasted URL, got %q", input.Value)
	}
	if action := input.handle(""); action != inputSubmit {
		t.Fatalf("Enter with a URL should submit, got %v", action)
	}

	input.handle("\x15")
	if input.Value != "" {
		t.Fatalf("Ctrl-U should clear input, got %q", input.Value)
	}
	if action := input.handle(""); action != inputStay || !strings.Contains(input.Message, "required") {
		t.Fatalf("empty submit should stay with validation message, action=%v message=%q", action, input.Message)
	}
	input.handle("h")
	frame := renderRepositoryURLInput(input)
	for _, want := range []string{"Repository URL", "h", "enter scan", "ctrl-u clear", "esc cancel"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("URL input frame missing %q:\n%s", want, frame)
		}
	}
	if action := input.handle(esc); action != inputCancel {
		t.Fatalf("Esc should cancel URL entry, got %v", action)
	}
}

func TestRepositoryPickerPastesNewURLAndStartsScan(t *testing.T) {
	wantSession := &importer.ScanSession{
		Root: "/tmp/checkout", RepositoryURL: "https://github.com/example/repo",
		Commit:     "0123456789abcdef0123456789abcdef01234567",
		Candidates: []importer.Candidate{{ID: "skill", Name: "skill", Valid: true}},
	}
	service := &fakeRepositoryScanService{scanSession: wantSession}
	keys := NewKeyReader(bytes.NewBufferString("\rhttps://github.com/example/repo\r"))
	defer keys.Close()
	var output bytes.Buffer

	session, err := runRepositoryPicker(service, keys, &output, 24)
	if err != nil {
		t.Fatal(err)
	}
	if session != wantSession {
		t.Fatalf("picker returned wrong scan session: %#v", session)
	}
	if service.scannedURL != "https://github.com/example/repo" {
		t.Fatalf("picker scanned %q", service.scannedURL)
	}
	if !strings.Contains(output.String(), "Repository URL") || !strings.Contains(output.String(), "Scanning https://github.com/example/repo") {
		t.Fatalf("picker did not render URL input and scanning states:\n%s", output.String())
	}
}

func TestRepositoryPickerShowsScanErrorWithoutLosingURLOrSavedList(t *testing.T) {
	t.Run("new URL input", func(t *testing.T) {
		service := &fakeRepositoryScanService{scanErr: errors.New("clone failed")}
		keys := NewKeyReader(bytes.NewBufferString("\rhttps://github.com/example/repo\r"))
		defer keys.Close()
		var output bytes.Buffer

		session, err := runRepositoryPicker(service, keys, &output, 24)
		if err != nil || session != nil {
			t.Fatalf("failed scan should return to picker: session=%#v err=%v", session, err)
		}
		for _, want := range []string{"clone failed", "https://github.com/example/repo", "Paste a new repository URL"} {
			if !strings.Contains(output.String(), want) {
				t.Fatalf("failed new-URL scan lost %q:\n%s", want, output.String())
			}
		}
	})

	t.Run("saved URL list", func(t *testing.T) {
		savedURL := "https://github.com/saved/repo"
		service := &fakeRepositoryScanService{
			saved:   []importer.RepositoryRecord{{URL: savedURL}},
			scanErr: errors.New("network unavailable"),
		}
		keys := NewKeyReader(bytes.NewBufferString("j\r"))
		defer keys.Close()
		var output bytes.Buffer

		session, err := runRepositoryPicker(service, keys, &output, 24)
		if err != nil || session != nil {
			t.Fatalf("failed saved scan should return to picker: session=%#v err=%v", session, err)
		}
		for _, want := range []string{savedURL, "network unavailable"} {
			if !strings.Contains(output.String(), want) {
				t.Fatalf("failed saved scan lost %q:\n%s", want, output.String())
			}
		}
	})
}

func TestRepositoryPickerCancelsScanAndPropagatesCtrlC(t *testing.T) {
	for name, key := range map[string]string{"escape": esc, "ctrl-c": "\x03"} {
		t.Run(name, func(t *testing.T) {
			cancelled := make(chan struct{})
			service := &fakeRepositoryScanService{
				saved: []importer.RepositoryRecord{{URL: "https://github.com/saved/repo"}},
				scan: func(ctx context.Context, _ string) (*importer.ScanSession, error) {
					<-ctx.Done()
					close(cancelled)
					return nil, ctx.Err()
				},
			}
			input := "j\r" + key
			keys := NewKeyReader(bytes.NewBufferString(input))
			defer keys.Close()
			var output bytes.Buffer

			_, err := runRepositoryPicker(service, keys, &output, 24)
			if key == "\x03" && !errors.Is(err, ErrInterrupted) {
				t.Fatalf("Ctrl-C should propagate ErrInterrupted, got %v", err)
			}
			if key == esc && err != nil {
				t.Fatalf("Esc cancellation should return cleanly, got %v", err)
			}
			select {
			case <-cancelled:
			default:
				t.Fatal("scan context was not cancelled")
			}
		})
	}
}

func TestRepositoryPickerConfirmsAndDeletesOnlySavedHistory(t *testing.T) {
	savedURL := "https://github.com/saved/repo"
	service := &fakeRepositoryScanService{saved: []importer.RepositoryRecord{{URL: savedURL}}}
	keys := NewKeyReader(bytes.NewBufferString("jdndy"))
	defer keys.Close()
	var output bytes.Buffer

	_, err := runRepositoryPicker(service, keys, &output, 24)
	if err != nil {
		t.Fatal(err)
	}
	if service.deletedURL != savedURL {
		t.Fatalf("confirmed deletion targeted %q, want %q", service.deletedURL, savedURL)
	}
	if count := strings.Count(output.String(), "Delete "+savedURL+" from saved history? y/N"); count != 2 {
		t.Fatalf("expected cancelled then confirmed deletion prompts, got %d:\n%s", count, output.String())
	}
}

func TestRepositoryPickerKeepsSavedRowWhenDeletionPersistenceFails(t *testing.T) {
	savedURL := "https://github.com/saved/repo"
	service := &fakeRepositoryScanService{
		saved:     []importer.RepositoryRecord{{URL: savedURL}},
		deleteErr: errors.New("history is read-only"),
	}
	keys := NewKeyReader(bytes.NewBufferString("jdy"))
	defer keys.Close()
	var output bytes.Buffer

	_, err := runRepositoryPicker(service, keys, &output, 24)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "history is read-only") || strings.Count(output.String(), savedURL) < 3 {
		t.Fatalf("failed delete should show error and keep saved row:\n%s", output.String())
	}
}

func TestMainRunLoopEntersRepositoryPickerWithIAndReturnsUnchanged(t *testing.T) {
	cfg := testConfig(t)
	model, err := LoadSkills(cfg)
	if err != nil {
		t.Fatal(err)
	}
	before := append([]Row(nil), model.Rows...)
	service := &fakeRepositoryScanService{}
	keys := NewKeyReader(bytes.NewBufferString("i\x1b"))
	defer keys.Close()
	var output bytes.Buffer

	if err := runLoopWithRepositoryImport(cfg, model, keys, &output, 24, service); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(model.Rows, before) {
		t.Fatalf("entering and escaping picker changed main desired state:\n got %#v\nwant %#v", model.Rows, before)
	}
	if !strings.Contains(output.String(), "import from GitHub") || !strings.Contains(output.String(), "manage skills") {
		t.Fatalf("run loop did not transition to picker and back:\n%s", output.String())
	}
}

func TestCandidatePickerRendersAndSelectsOnlyValidCandidates(t *testing.T) {
	picker := newCandidatePicker([]importer.Candidate{
		{ID: "alpha", Name: "alpha", SourcePath: ".claude/skills/alpha", Description: "Alpha", Valid: true},
		{ID: "blocked", Name: "blocked", SourcePath: "skills/blocked", Description: "Blocked", Valid: false, Reason: "install name conflicts with existing first-party"},
		{ID: "gamma", Name: "gamma", SourcePath: "nested/gamma", Description: "Gamma", Valid: true},
	})
	frame := renderCandidatePicker(picker, 24)
	for _, want := range []string{"alpha", ".claude/skills/alpha", "blocked", "skills/blocked", "existing first-party", "gamma", "nested/gamma"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("candidate frame missing %q:\n%s", want, frame)
		}
	}
	if !picker.Rows[0].Selected || picker.Rows[1].Selected || !picker.Rows[2].Selected {
		t.Fatalf("only valid candidates should default selected: %#v", picker.Rows)
	}

	picker.moveDown()
	picker.toggle()
	if picker.Rows[1].Selected {
		t.Fatal("disabled candidate toggled on")
	}
	picker.selectNone()
	if len(picker.selectedIDs()) != 0 {
		t.Fatalf("select none left selections: %v", picker.selectedIDs())
	}
	picker.selectAll()
	if got := picker.selectedIDs(); !reflect.DeepEqual(got, []string{"alpha", "gamma"}) {
		t.Fatalf("select all should include only valid IDs, got %v", got)
	}
	picker.moveUp()
	if picker.Cursor != 0 {
		t.Fatalf("movement should wrap from blocked row to first row, got %d", picker.Cursor)
	}
}

func TestCandidatePickerImportsOnlySelectedValidCandidateIDs(t *testing.T) {
	session := &importer.ScanSession{
		Candidates: []importer.Candidate{
			{ID: "alpha-id", Name: "alpha", SourcePath: "alpha", Valid: true},
			{ID: "blocked-id", Name: "blocked", SourcePath: "blocked", Valid: false, Reason: "collision"},
			{ID: "gamma-id", Name: "gamma", SourcePath: "gamma", Valid: true},
		},
	}
	service := &fakeRepositoryScanService{importedNames: []string{"alpha", "gamma"}}
	keys := NewKeyReader(bytes.NewBufferString("n j j \r"))
	defer keys.Close()
	var output bytes.Buffer

	imported, cancelled, err := runCandidatePicker(service, session, keys, &output, 24)
	if err != nil || cancelled {
		t.Fatalf("candidate import failed: imported=%v cancelled=%v err=%v", imported, cancelled, err)
	}
	if !reflect.DeepEqual(service.importedIDs, []string{"alpha-id", "gamma-id"}) {
		t.Fatalf("import received wrong candidate IDs: %v", service.importedIDs)
	}
	if !reflect.DeepEqual(imported, []string{"alpha", "gamma"}) {
		t.Fatalf("unexpected imported names: %v", imported)
	}
}

func TestCandidatePickerPreventsEmptySubmissionAndShowsBatchFailure(t *testing.T) {
	session := &importer.ScanSession{Candidates: []importer.Candidate{{ID: "alpha", Name: "alpha", SourcePath: "alpha", Valid: true}}}
	t.Run("empty", func(t *testing.T) {
		service := &fakeRepositoryScanService{}
		keys := NewKeyReader(bytes.NewBufferString("n\r"))
		defer keys.Close()
		var output bytes.Buffer
		_, cancelled, err := runCandidatePicker(service, session, keys, &output, 24)
		if err != nil || !cancelled || len(service.importedIDs) != 0 {
			t.Fatalf("empty selection should stay then exit cleanly: cancelled=%v ids=%v err=%v", cancelled, service.importedIDs, err)
		}
		if !strings.Contains(output.String(), "Select at least one valid skill") {
			t.Fatalf("empty submission message missing:\n%s", output.String())
		}
	})

	t.Run("transaction failure", func(t *testing.T) {
		service := &fakeRepositoryScanService{importErr: errors.New("batch rolled back")}
		keys := NewKeyReader(bytes.NewBufferString("\r"))
		defer keys.Close()
		var output bytes.Buffer
		_, cancelled, err := runCandidatePicker(service, session, keys, &output, 24)
		if err != nil || !cancelled {
			t.Fatalf("failed batch should remain until clean EOF: cancelled=%v err=%v", cancelled, err)
		}
		if !strings.Contains(output.String(), "batch rolled back") {
			t.Fatalf("batch failure message missing:\n%s", output.String())
		}
	})
}

func TestCandidatePickerCancelsInProgressImportWithEscapeOrCtrlC(t *testing.T) {
	for name, key := range map[string]string{"escape": esc, "ctrl-c": "\x03"} {
		t.Run(name, func(t *testing.T) {
			cancelled := make(chan struct{})
			service := &fakeRepositoryScanService{
				importFunc: func(ctx context.Context, _ *importer.ScanSession, _ []string) ([]string, error) {
					if ctx.Done() == nil {
						return nil, errors.New("import received an uncancellable context")
					}
					<-ctx.Done()
					close(cancelled)
					return nil, ctx.Err()
				},
			}
			session := &importer.ScanSession{Candidates: []importer.Candidate{{ID: "alpha", Name: "alpha", SourcePath: "alpha", Valid: true}}}
			keys := NewKeyReader(bytes.NewBufferString("\r" + key))
			defer keys.Close()
			var output bytes.Buffer

			_, _, err := runCandidatePicker(service, session, keys, &output, 24)
			if key == "\x03" && !errors.Is(err, ErrInterrupted) {
				t.Fatalf("Ctrl-C should propagate ErrInterrupted, got %v", err)
			}
			if key == esc && err != nil {
				t.Fatalf("Esc should cancel import cleanly, got %v", err)
			}
			select {
			case <-cancelled:
			default:
				t.Fatal("in-progress import context was not cancelled")
			}
		})
	}
}

func TestMainImportWorkflowReloadsImportedRowsSelectedButNotInstalled(t *testing.T) {
	cfg := testConfig(t)
	model, err := LoadSkills(cfg)
	if err != nil {
		t.Fatal(err)
	}
	savedURL := "https://github.com/saved/repo"
	session := &importer.ScanSession{
		RepositoryURL: savedURL,
		Candidates: []importer.Candidate{
			{ID: "alpha", Name: "alpha", SourcePath: "alpha", Valid: true},
			{ID: "gamma", Name: "gamma", SourcePath: "gamma", Valid: true},
		},
	}
	service := &fakeRepositoryScanService{
		saved:       []importer.RepositoryRecord{{URL: savedURL}},
		scanSession: session,
		importFunc: func(_ context.Context, _ *importer.ScanSession, selectedIDs []string) ([]string, error) {
			for _, name := range selectedIDs {
				path := filepath.Join(cfg.RepoDir, "third-party", name, "SKILL.md")
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return nil, err
				}
				if err := os.WriteFile(path, []byte("---\nname: "+name+"\ndescription: Imported\n---\n"), 0o644); err != nil {
					return nil, err
				}
			}
			return append([]string(nil), selectedIDs...), nil
		},
	}
	reader, writer := io.Pipe()
	keys := NewKeyReader(reader)
	defer keys.Close()
	output := newNotifyingBuffer("select skills to import")
	done := make(chan error, 1)
	go func() {
		done <- runLoopWithRepositoryImport(cfg, model, keys, output, 24, service)
	}()
	if _, err := io.WriteString(writer, "ij\r"); err != nil {
		t.Fatal(err)
	}
	<-output.notified
	if _, err := io.WriteString(writer, "\r"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	rows := make(map[string]Row)
	for _, row := range model.Rows {
		rows[row.Skill.Name] = row
	}
	for _, name := range []string{"alpha", "gamma"} {
		if rows[name].Desired != skills.DesiredInstall || rows[name].State != skills.StateNotInstalled {
			t.Fatalf("%s should be selected but not installed: %#v", name, rows[name])
		}
		if _, err := os.Lstat(filepath.Join(cfg.Home, ".agents", "skills", name)); !os.IsNotExist(err) {
			t.Fatalf("%s link exists before Apply: %v", name, err)
		}
	}
	if !strings.Contains(output.String(), "Imported 2 skill(s). Press Enter to apply installation.") {
		t.Fatalf("main success message missing:\n%s", output.String())
	}
}

type notifyingBuffer struct {
	mu       sync.Mutex
	buffer   bytes.Buffer
	needle   string
	notified chan struct{}
	once     sync.Once
}

func newNotifyingBuffer(needle string) *notifyingBuffer {
	return &notifyingBuffer{needle: needle, notified: make(chan struct{})}
}

func (b *notifyingBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n, err := b.buffer.Write(data)
	if strings.Contains(b.buffer.String(), b.needle) {
		b.once.Do(func() { close(b.notified) })
	}
	return n, err
}

func (b *notifyingBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

type fakeRepositoryScanService struct {
	saved         []importer.RepositoryRecord
	savedErr      error
	scanSession   *importer.ScanSession
	scanErr       error
	scannedURL    string
	deleteErr     error
	deletedURL    string
	scan          func(context.Context, string) (*importer.ScanSession, error)
	importedIDs   []string
	importedNames []string
	importErr     error
	importFunc    func(context.Context, *importer.ScanSession, []string) ([]string, error)
}

func (f *fakeRepositoryScanService) SavedRepositories() ([]importer.RepositoryRecord, error) {
	return append([]importer.RepositoryRecord(nil), f.saved...), f.savedErr
}

func (f *fakeRepositoryScanService) Scan(ctx context.Context, repositoryURL string) (*importer.ScanSession, error) {
	f.scannedURL = repositoryURL
	if f.scan != nil {
		return f.scan(ctx, repositoryURL)
	}
	return f.scanSession, f.scanErr
}

func (f *fakeRepositoryScanService) DeleteSavedRepository(repositoryURL string) error {
	f.deletedURL = repositoryURL
	return f.deleteErr
}

func (f *fakeRepositoryScanService) Import(ctx context.Context, session *importer.ScanSession, selectedIDs []string) ([]string, error) {
	f.importedIDs = append([]string(nil), selectedIDs...)
	if f.importFunc != nil {
		return f.importFunc(ctx, session, selectedIDs)
	}
	return append([]string(nil), f.importedNames...), f.importErr
}

var _ repositoryScanService = (*fakeRepositoryScanService)(nil)
var _ repositoryImportService = (*fakeRepositoryScanService)(nil)
