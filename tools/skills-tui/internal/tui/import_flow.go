package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"agent-skills/tools/skills-tui/internal/importer"
	"agent-skills/tools/skills-tui/internal/skills"
)

type repositoryScanService interface {
	SavedRepositories() ([]importer.RepositoryRecord, error)
	DeleteSavedRepository(string) error
	Scan(context.Context, string) (*importer.ScanSession, error)
}

type repositoryPicker struct {
	Saved   []importer.RepositoryRecord
	Cursor  int
	Message string
}

type inputAction int

const (
	inputStay inputAction = iota
	inputSubmit
	inputCancel
	inputInterrupt
)

type repositoryURLInput struct {
	Value   string
	Message string
}

func (i *repositoryURLInput) handle(key string) inputAction {
	i.Message = ""
	switch key {
	case esc:
		return inputCancel
	case "\x03":
		return inputInterrupt
	case "":
		if strings.TrimSpace(i.Value) == "" {
			i.Message = "A GitHub repository URL is required."
			return inputStay
		}
		return inputSubmit
	case "\x7f", "\x08":
		if len(i.Value) > 0 {
			i.Value = i.Value[:len(i.Value)-1]
		}
	case "\x15":
		i.Value = ""
	default:
		if len(key) == 1 && key[0] >= 0x20 && key[0] <= 0x7e {
			i.Value += key
		}
	}
	return inputStay
}

func newRepositoryPicker(saved []importer.RepositoryRecord) repositoryPicker {
	return repositoryPicker{Saved: append([]importer.RepositoryRecord(nil), saved...)}
}

func (p *repositoryPicker) moveUp() {
	total := len(p.Saved) + 1
	p.Cursor = (p.Cursor - 1 + total) % total
}

func (p *repositoryPicker) moveDown() {
	total := len(p.Saved) + 1
	p.Cursor = (p.Cursor + 1) % total
}

func renderRepositoryPicker(p repositoryPicker, termRows int) string {
	eol := esc + "[K"
	nl := eol + "\n"
	var rows []string
	rows = append(rows, pickerRow(p.Cursor == 0, "Paste a new repository URL")+eol)
	for i, record := range p.Saved {
		rows = append(rows, pickerRow(p.Cursor == i+1, skills.SanitizeLabel(record.URL))+eol)
	}
	footer := 0
	if p.Message != "" {
		footer = footerRows
	}
	available := termRows - headerRows - footer
	if available < 1 {
		available = 1
	}
	start, end := viewportRange(len(rows), p.Cursor, available)

	var b strings.Builder
	b.WriteString(cBold + "  agent-skills · import from GitHub" + cReset + nl)
	b.WriteString(cDim + "  ↑↓ move · enter select · d delete · esc back" + cReset + nl)
	for i := start; i < end; i++ {
		b.WriteString(rows[i] + "\n")
	}
	if p.Message != "" {
		b.WriteString(nl + "  " + skills.SanitizeLabel(p.Message) + nl)
	}
	return esc + "[H" + strings.TrimSuffix(b.String(), "\n") + esc + "[J"
}

func pickerRow(selected bool, label string) string {
	marker := " "
	if selected {
		marker = cBold + ">" + cReset
	}
	return fmt.Sprintf("  %s %s", marker, label)
}

func renderRepositoryURLInput(input repositoryURLInput) string {
	eol := esc + "[K"
	nl := eol + "\n"
	var b strings.Builder
	b.WriteString(cBold + "  agent-skills · import from GitHub" + cReset + nl)
	b.WriteString(cDim + "  enter scan · backspace edit · ctrl-u clear · esc cancel" + cReset + nl)
	b.WriteString(nl)
	b.WriteString("  Repository URL" + nl)
	b.WriteString("  " + cCyan + "> " + cReset + skills.SanitizeLabel(input.Value) + "_" + nl)
	if input.Message != "" {
		b.WriteString(nl + "  " + skills.SanitizeLabel(input.Message) + nl)
	}
	return esc + "[H" + strings.TrimSuffix(b.String(), "\n") + esc + "[J"
}

func runRepositoryPicker(service repositoryScanService, keys *KeyReader, output io.Writer, termRows int) (*importer.ScanSession, error) {
	saved, err := service.SavedRepositories()
	picker := newRepositoryPicker(saved)
	if err != nil {
		picker.Message = fmt.Sprintf("Could not read saved repositories: %v", err)
	}
	for {
		fmt.Fprint(output, renderRepositoryPicker(picker, termRows))
		picker.Message = ""
		key, err := keys.ReadKey()
		if err != nil {
			return nil, nil
		}
		switch key {
		case esc + "[A", "k":
			picker.moveUp()
		case esc + "[B", "j":
			picker.moveDown()
		case esc:
			return nil, nil
		case "\x03":
			return nil, ErrInterrupted
		case "d":
			if picker.Cursor == 0 {
				continue
			}
			index := picker.Cursor - 1
			repositoryURL := picker.Saved[index].URL
			confirmed, err := confirmRepositoryDeletion(repositoryURL, keys, output)
			if err != nil {
				return nil, err
			}
			if !confirmed {
				continue
			}
			if err := service.DeleteSavedRepository(repositoryURL); err != nil {
				picker.Message = fmt.Sprintf("Could not delete saved repository: %v", err)
				continue
			}
			picker.Saved = append(picker.Saved[:index], picker.Saved[index+1:]...)
			if picker.Cursor > len(picker.Saved) {
				picker.Cursor = len(picker.Saved)
			}
			picker.Message = "Deleted saved repository history."
		case "":
			if picker.Cursor == 0 {
				session, err := runRepositoryURLInput(service, keys, output)
				if err != nil {
					return nil, err
				}
				if session != nil {
					return session, nil
				}
				continue
			}
			repositoryURL := picker.Saved[picker.Cursor-1].URL
			session, cancelled, err := scanRepository(service, repositoryURL, keys, output)
			if err != nil {
				if errors.Is(err, ErrInterrupted) {
					return nil, err
				}
				picker.Message = err.Error()
				continue
			}
			if !cancelled && session != nil {
				return session, nil
			}
		}
	}
}

func confirmRepositoryDeletion(repositoryURL string, keys *KeyReader, output io.Writer) (bool, error) {
	for {
		fmt.Fprint(output, renderRepositoryDeleteConfirmation(repositoryURL))
		key, err := keys.ReadKey()
		if err != nil {
			return false, nil
		}
		switch key {
		case "y", "Y":
			return true, nil
		case "n", "N", "", esc:
			return false, nil
		case "\x03":
			return false, ErrInterrupted
		}
	}
}

func runRepositoryURLInput(service repositoryScanService, keys *KeyReader, output io.Writer) (*importer.ScanSession, error) {
	input := repositoryURLInput{}
	for {
		fmt.Fprint(output, renderRepositoryURLInput(input))
		input.Message = ""
		key, err := keys.ReadKey()
		if err != nil {
			return nil, nil
		}
		switch input.handle(key) {
		case inputCancel:
			return nil, nil
		case inputInterrupt:
			return nil, ErrInterrupted
		case inputSubmit:
			session, cancelled, err := scanRepository(service, input.Value, keys, output)
			if err != nil {
				if errors.Is(err, ErrInterrupted) {
					return nil, err
				}
				input.Message = err.Error()
				continue
			}
			if cancelled {
				continue
			}
			return session, nil
		}
	}
}

type scanOutcome struct {
	session *importer.ScanSession
	err     error
}

func scanRepository(service repositoryScanService, repositoryURL string, keys *KeyReader, output io.Writer) (*importer.ScanSession, bool, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results := make(chan scanOutcome, 1)
	done := make(chan struct{})
	go func() {
		session, err := service.Scan(ctx, repositoryURL)
		results <- scanOutcome{session: session, err: err}
		close(done)
	}()

	fmt.Fprint(output, renderRepositoryScan(repositoryURL))
	for {
		select {
		case result := <-results:
			return result.session, false, result.err
		default:
		}

		readCtx, stopRead := context.WithCancel(context.Background())
		go func() {
			select {
			case <-done:
				stopRead()
			case <-readCtx.Done():
			}
		}()
		key, err := keys.ReadKeyContext(readCtx)
		stopRead()
		if errors.Is(err, context.Canceled) {
			result := <-results
			return result.session, false, result.err
		}
		if errors.Is(err, io.EOF) {
			result := <-results
			return result.session, false, result.err
		}
		if err != nil {
			cancel()
			result := <-results
			if result.session != nil {
				_ = result.session.Close()
			}
			return nil, true, nil
		}
		switch key {
		case esc:
			cancel()
			result := <-results
			if result.session != nil {
				_ = result.session.Close()
			}
			return nil, true, nil
		case "\x03":
			cancel()
			result := <-results
			if result.session != nil {
				_ = result.session.Close()
			}
			return nil, false, ErrInterrupted
		}
	}
}

func renderRepositoryScan(repositoryURL string) string {
	eol := esc + "[K"
	nl := eol + "\n"
	return esc + "[H" + cBold + "  agent-skills · import from GitHub" + cReset + nl +
		cDim + "  esc cancel · ctrl-c quit" + cReset + nl + nl +
		"  Scanning " + skills.SanitizeLabel(repositoryURL) + "…" + eol + esc + "[J"
}

func renderRepositoryDeleteConfirmation(repositoryURL string) string {
	eol := esc + "[K"
	nl := eol + "\n"
	return esc + "[H" + cBold + "  agent-skills · import from GitHub" + cReset + nl +
		cDim + "  y confirm · N/enter/esc cancel" + cReset + nl + nl +
		"  Delete " + skills.SanitizeLabel(repositoryURL) + " from saved history? y/N" + eol + esc + "[J"
}
