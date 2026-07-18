package tui

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/term"

	"agent-skills/tools/skills-tui/internal/importer"
	"agent-skills/tools/skills-tui/internal/skills"
)

// ErrInterrupted reports that the user pressed Ctrl-C. bash never enters raw
// mode, so its Ctrl-C raises SIGINT and the script exits non-zero via the
// trap; the Go loop sees byte 0x03 in-band and surfaces it as this error so
// main can exit 130 like a SIGINT death.
var ErrInterrupted = errors.New("interrupted")

// Conventional 128+signal exit codes, matching the bash trap.
const (
	exitSIGINT  = 128 + int(syscall.SIGINT)
	exitSIGTERM = 128 + int(syscall.SIGTERM)
)

// Run launches the interactive TUI, mirroring bash run_tui: load and seed the
// rows, enter raw mode with the cursor hidden and the screen cleared once,
// loop over render/read_key, and restore the terminal on every exit path
// (normal quit, EOF, panic, SIGINT/SIGTERM).
func Run(cfg skills.Config, stdout io.Writer) error {
	m, err := LoadSkills(cfg)
	if err != nil {
		return err
	}
	if len(m.Rows) == 0 {
		return fmt.Errorf("No skills found in %s", cfg.RepoDir)
	}

	// Terminal height drives render's viewport windowing for oversized lists.
	termRows := terminalRows()

	restoreMode, err := enterRaw()
	if err != nil {
		return err
	}
	var once sync.Once
	restore := func() {
		once.Do(func() {
			restoreMode()
			// Bash trap: restore stty state, then show the cursor + newline.
			fmt.Fprint(stdout, esc+"[?25h\n")
		})
	}
	defer restore()

	// Restore the terminal on SIGINT/SIGTERM too. In raw mode Ctrl-C arrives
	// as byte 0x03 and is handled in the event loop; this covers external
	// signals. The done channel lets the goroutine exit on a normal return so
	// it does not park forever if Run is ever invoked more than once.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case sig, ok := <-sigCh:
			if !ok {
				return
			}
			restore()
			// Die with the conventional 128+signal code, like the bash trap.
			if sig == syscall.SIGTERM {
				os.Exit(exitSIGTERM)
			}
			os.Exit(exitSIGINT)
		case <-done:
			return
		}
	}()

	// Raw mode disables output post-processing, so translate \n to \r\n on
	// the way out; frames themselves are byte-identical to bash render().
	w := &crlfWriter{w: stdout}
	fmt.Fprint(w, esc+"[?25l"+esc+"[2J") // hide cursor, clear once on entry

	kr := NewKeyReader(os.Stdin)
	defer kr.Close()
	historyPath, err := importer.DefaultHistoryPath()
	if err != nil {
		return fmt.Errorf("resolve import repository history: %w", err)
	}
	service := &importer.Workflow{
		History:    importer.HistoryStore{Path: historyPath, Now: cfg.Now},
		Checkouts:  importer.GitHubCheckoutProvider{},
		Repository: importer.RepositoryImporter{RepoDir: cfg.RepoDir},
	}
	return runLoopWithRepositoryImport(cfg, m, kr, w, termRows, service)
}

// runLoop is the render/read/dispatch cycle, mirroring the bash while loop.
// It returns nil on a clean quit (q, lone ESC, stream end) and ErrInterrupted
// on Ctrl-C, which raw mode delivers in-band as byte 0x03.
func runLoop(cfg skills.Config, m *Model, kr *KeyReader, w io.Writer, termRows int) error {
	return runLoopWithRepositoryImport(cfg, m, kr, w, termRows, nil)
}

func runLoopWithRepositoryImport(cfg skills.Config, m *Model, kr *KeyReader, w io.Writer, termRows int, imports repositoryScanService) error {
	for {
		fmt.Fprint(w, Render(*m, termRows))
		m.Message = ""
		key, err := kr.ReadKey()
		if err != nil {
			return nil
		}
		switch key {
		case esc + "[A", "k":
			m.MoveUp()
		case esc + "[B", "j":
			m.MoveDown()
		case " ":
			m.Toggle()
		case "a":
			m.SelectAll()
		case "n":
			m.SelectNone()
		case "i":
			if imports == nil {
				m.Message = "GitHub repository import is unavailable."
				continue
			}
			session, err := runRepositoryPicker(imports, kr, w, termRows)
			if err != nil {
				return err
			}
			if session != nil {
				m.Message = fmt.Sprintf("Scanned %d candidate(s) from %s.", len(session.Candidates), session.RepositoryURL)
				if err := session.Close(); err != nil {
					m.Message = fmt.Sprintf("Scanned repository, but temporary checkout cleanup failed: %v", err)
				}
			}
		case "": // Enter
			fmt.Fprint(w, esc+"[2J"+esc+"[H\n")
			fmt.Fprintln(w, "  Applying…")
			fmt.Fprintln(w)
			m.ApplyChanges(cfg, w)
			fmt.Fprintln(w)
			fmt.Fprintln(w, "  Done. Press any key to continue, q to quit.")
			key, err = kr.ReadKey()
			if err != nil || key == "q" {
				return nil
			}
			if key == "\x03" {
				return ErrInterrupted
			}
		case "q", esc:
			return nil
		case "\x03": // Ctrl-C: bash's SIGINT path
			return ErrInterrupted
		}
	}
}

// terminalRows returns the terminal height, preferring x/term, then a stty
// fallback, then the bash default of 24.
func terminalRows() int {
	if _, h, err := term.GetSize(int(os.Stdin.Fd())); err == nil && h > 0 {
		return h
	}
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	if out, err := cmd.Output(); err == nil {
		fields := strings.Fields(string(out))
		if len(fields) > 0 {
			if rows, err := strconv.Atoi(fields[0]); err == nil && rows > 0 {
				return rows
			}
		}
	}
	return 24
}

// enterRaw puts stdin into raw mode without echo and returns a restore
// function. x/term is preferred; if it fails, fall back to shelling out to
// stty like the bash script does.
func enterRaw() (func(), error) {
	fd := int(os.Stdin.Fd())
	if state, err := term.MakeRaw(fd); err == nil {
		return func() {
			if err := term.Restore(fd, state); err != nil {
				fmt.Fprintln(os.Stderr, "warning: failed to restore terminal:", err)
			}
		}, nil
	}

	saved, err := stty("-g")
	if err != nil {
		return nil, fmt.Errorf("cannot set raw mode: %w", err)
	}
	if _, err := stty("raw", "-echo"); err != nil {
		return nil, fmt.Errorf("cannot set raw mode: %w", err)
	}
	savedState := strings.TrimSpace(saved)
	return func() {
		if _, err := stty(savedState); err != nil {
			fmt.Fprintln(os.Stderr, "warning: failed to restore terminal:", err)
		}
	}, nil
}

// stty runs stty against the process's stdin and returns its output.
func stty(args ...string) (string, error) {
	cmd := exec.Command("stty", args...)
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	return string(out), err
}

// crlfWriter rewrites \n to \r\n so cooked-mode output renders correctly
// while the terminal is raw (bash never enters raw mode, so its plain \n
// output relies on ONLCR; raw mode turns that off).
type crlfWriter struct {
	w io.Writer
}

func (c *crlfWriter) Write(p []byte) (int, error) {
	if _, err := c.w.Write(bytes.ReplaceAll(p, []byte("\n"), []byte("\r\n"))); err != nil {
		return 0, err
	}
	return len(p), nil
}
