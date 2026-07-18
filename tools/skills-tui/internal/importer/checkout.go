package importer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const maxGitErrorDetailBytes = 4096

var commitSHA = regexp.MustCompile(`^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`)

// Command is a shell-free process invocation. Every argument remains a
// separate boundary so repository URLs can never be interpolated by a shell.
type Command struct {
	Dir  string
	Env  []string
	Name string
	Args []string
}

// CommandRunner is the external process boundary used by the checkout
// provider. ExecCommandRunner is the production implementation.
type CommandRunner interface {
	Run(context.Context, Command) ([]byte, error)
}

// ExecCommandRunner executes commands directly and inherits the current
// process environment, including any preconfigured non-interactive Git auth.
type ExecCommandRunner struct{}

func (ExecCommandRunner) Run(ctx context.Context, command Command) ([]byte, error) {
	process := exec.CommandContext(ctx, command.Name, command.Args...)
	process.Dir = command.Dir
	process.Env = append(os.Environ(), command.Env...)
	return process.CombinedOutput()
}

// Checkout is a temporary repository snapshot. Close releases the complete
// provider-owned session directory and is safe to call more than once.
type Checkout struct {
	Root          string
	RepositoryURL string
	Commit        string

	cleanup func() error
	once    sync.Once
	err     error
}

// Close removes the temporary checkout session.
func (c *Checkout) Close() error {
	if c == nil {
		return nil
	}
	c.once.Do(func() {
		if c.cleanup != nil {
			c.err = c.cleanup()
		}
	})
	return c.err
}

// CheckoutProvider supplies a cancellable temporary checkout for scanning.
type CheckoutProvider interface {
	Checkout(context.Context, string) (*Checkout, error)
}

// GitHubCheckoutProvider clones canonical GitHub repositories into temporary
// session directories. TempRoot is optional; an empty value uses os.TempDir.
type GitHubCheckoutProvider struct {
	Runner    CommandRunner
	TempRoot  string
	RemoveAll func(string) error
}

// Checkout validates rawURL, performs a shallow no-tags clone, captures the
// checked-out commit, and transfers temporary-directory ownership to the
// returned session.
func (p GitHubCheckoutProvider) Checkout(ctx context.Context, rawURL string) (_ *Checkout, err error) {
	repositoryURL, err := NormalizeGitHubURL(rawURL)
	if err != nil {
		return nil, err
	}
	removeAll := p.RemoveAll
	if removeAll == nil {
		removeAll = os.RemoveAll
	}
	sessionRoot, err := os.MkdirTemp(p.TempRoot, "agent-skills-import-*")
	if err != nil {
		return nil, fmt.Errorf("create temporary GitHub checkout: %w", err)
	}
	owned := true
	defer func() {
		if !owned {
			return
		}
		if cleanupErr := removeAll(sessionRoot); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("clean temporary GitHub checkout: %w", cleanupErr))
		}
	}()
	absoluteSessionRoot, err := filepath.Abs(sessionRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve temporary GitHub checkout: %w", err)
	}
	sessionRoot = absoluteSessionRoot

	runner := p.Runner
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	checkoutRoot := filepath.Join(sessionRoot, "checkout")
	env := []string{"GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=Never"}
	clone := Command{
		Env: env, Name: "git",
		Args: []string{"clone", "--depth", "1", "--no-tags", "--", repositoryURL, checkoutRoot},
	}
	output, err := runner.Run(ctx, clone)
	if err != nil {
		return nil, checkoutError(ctx, "clone GitHub repository", output, err)
	}
	resolve := Command{Env: env, Name: "git", Args: []string{"-C", checkoutRoot, "rev-parse", "HEAD"}}
	output, err = runner.Run(ctx, resolve)
	if err != nil {
		return nil, checkoutError(ctx, "resolve checked-out commit", output, err)
	}
	commit := strings.TrimSpace(string(output))
	if !commitSHA.MatchString(commit) {
		return nil, fmt.Errorf("resolve checked-out commit: git returned an invalid commit SHA")
	}

	owned = false
	return &Checkout{
		Root: checkoutRoot, RepositoryURL: repositoryURL, Commit: strings.ToLower(commit),
		cleanup: func() error { return removeAll(sessionRoot) },
	}, nil
}

func checkoutError(ctx context.Context, action string, output []byte, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("GitHub checkout cancelled: %w", ctxErr)
	}
	detail := strings.TrimSpace(string(output))
	if len(detail) > maxGitErrorDetailBytes {
		detail = detail[:maxGitErrorDetailBytes] + "…"
	}
	if detail == "" {
		return fmt.Errorf("%s: %w", action, err)
	}
	return fmt.Errorf("%s: %w: %s", action, err, detail)
}
