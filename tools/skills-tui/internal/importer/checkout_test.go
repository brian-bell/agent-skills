package importer_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"agent-skills/tools/skills-tui/internal/importer"
)

func TestNormalizeGitHubURLAcceptsOnlyCanonicalRepositoryScope(t *testing.T) {
	accepted := map[string]string{
		"https://github.com/Owner/Repository":      "https://github.com/owner/repository",
		"https://github.com/owner/repository.git":  "https://github.com/owner/repository",
		"https://github.com/owner/repository/":     "https://github.com/owner/repository",
		"https://github.com/owner/repository.git/": "https://github.com/owner/repository",
	}
	for raw, want := range accepted {
		t.Run("accept "+raw, func(t *testing.T) {
			got, err := importer.NormalizeGitHubURL(raw)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("NormalizeGitHubURL(%q) = %q, want %q", raw, got, want)
			}
		})
	}

	rejected := []string{
		"http://github.com/owner/repository",
		"https://user@github.com/owner/repository",
		"https://github.com:443/owner/repository",
		"https://github.com/owner/repository?ref=main",
		"https://github.com/owner/repository?",
		"https://github.com/owner/repository#readme",
		"https://github.com/owner/repository#",
		"https://github.com/owner/repository/tree/main",
		"https://github.example.com/owner/repository",
		"https://github.com/owner%2Frepository/skill",
		"https://github.com/../repository",
		"https://github.com/owner/repository//",
		"git@github.com:owner/repository.git",
	}
	for _, raw := range rejected {
		t.Run("reject "+raw, func(t *testing.T) {
			if got, err := importer.NormalizeGitHubURL(raw); err == nil {
				t.Fatalf("NormalizeGitHubURL(%q) unexpectedly accepted %q", raw, got)
			}
		})
	}
}

func TestGitHubCheckoutClonesWithoutShellPromptsAndCapturesCommit(t *testing.T) {
	tempRoot := t.TempDir()
	const commit = "0123456789abcdef0123456789abcdef01234567"
	var commands []importer.Command
	runner := commandRunnerFunc(func(_ context.Context, command importer.Command) ([]byte, error) {
		commands = append(commands, command)
		if len(command.Args) > 0 && command.Args[0] == "clone" {
			if err := os.MkdirAll(command.Args[len(command.Args)-1], 0o755); err != nil {
				return nil, err
			}
			return nil, nil
		}
		return []byte(commit + "\n"), nil
	})
	provider := importer.GitHubCheckoutProvider{Runner: runner, TempRoot: tempRoot}

	checkout, err := provider.Checkout(context.Background(), "https://github.com/Owner/Repository.git/")
	if err != nil {
		t.Fatal(err)
	}
	defer checkout.Close()
	if checkout.RepositoryURL != "https://github.com/owner/repository" || checkout.Commit != commit {
		t.Fatalf("unexpected checkout metadata: %#v", checkout)
	}
	if filepath.Dir(checkout.Root) == tempRoot || !filepath.IsAbs(checkout.Root) {
		// Root lives one level below a provider-owned session directory.
		t.Fatalf("checkout root should be absolute inside a session directory, got %q", checkout.Root)
	}
	if len(commands) != 2 {
		t.Fatalf("expected clone and rev-parse commands, got %#v", commands)
	}
	wantClone := []string{"clone", "--depth", "1", "--no-tags", "--", "https://github.com/owner/repository", checkout.Root}
	if commands[0].Name != "git" || !reflect.DeepEqual(commands[0].Args, wantClone) {
		t.Fatalf("unsafe or unexpected clone invocation: %#v", commands[0])
	}
	if !slices.Contains(commands[0].Env, "GIT_TERMINAL_PROMPT=0") || !slices.Contains(commands[0].Env, "GCM_INTERACTIVE=Never") {
		t.Fatalf("clone did not disable terminal credential prompts: %v", commands[0].Env)
	}
	wantRevParse := []string{"-C", checkout.Root, "rev-parse", "HEAD"}
	if commands[1].Name != "git" || !reflect.DeepEqual(commands[1].Args, wantRevParse) {
		t.Fatalf("unexpected commit resolution invocation: %#v", commands[1])
	}
}

func TestGitHubCheckoutCleansTemporarySessionsOnFailureAndCancellation(t *testing.T) {
	t.Run("clone failure", func(t *testing.T) {
		tempRoot := t.TempDir()
		provider := importer.GitHubCheckoutProvider{
			TempRoot: tempRoot,
			Runner: commandRunnerFunc(func(context.Context, importer.Command) ([]byte, error) {
				return []byte("repository unavailable"), errors.New("exit status 128")
			}),
		}
		_, err := provider.Checkout(context.Background(), "https://github.com/example/repo")
		if err == nil || !strings.Contains(err.Error(), "clone GitHub repository") || !strings.Contains(err.Error(), "repository unavailable") {
			t.Fatalf("expected a clear clone error, got %v", err)
		}
		assertDirectoryEmpty(t, tempRoot)
	})

	t.Run("context cancellation", func(t *testing.T) {
		tempRoot := t.TempDir()
		started := make(chan struct{})
		provider := importer.GitHubCheckoutProvider{
			TempRoot: tempRoot,
			Runner: commandRunnerFunc(func(ctx context.Context, _ importer.Command) ([]byte, error) {
				close(started)
				<-ctx.Done()
				return nil, ctx.Err()
			}),
		}
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan error, 1)
		go func() {
			_, err := provider.Checkout(ctx, "https://github.com/example/repo")
			result <- err
		}()
		<-started
		cancel()
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation error, got %v", err)
		}
		assertDirectoryEmpty(t, tempRoot)
	})
}

func TestCheckoutCloseReleasesTheCompleteSessionIdempotently(t *testing.T) {
	tempRoot := t.TempDir()
	const commit = "0123456789abcdef0123456789abcdef01234567"
	provider := importer.GitHubCheckoutProvider{
		TempRoot: tempRoot,
		Runner: commandRunnerFunc(func(_ context.Context, command importer.Command) ([]byte, error) {
			if command.Args[0] == "clone" {
				if err := os.MkdirAll(command.Args[len(command.Args)-1], 0o755); err != nil {
					return nil, err
				}
				return nil, nil
			}
			return []byte(commit), nil
		}),
	}

	checkout, err := provider.Checkout(context.Background(), "https://github.com/example/repo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(checkout.Root); err != nil {
		t.Fatalf("successful checkout should remain available: %v", err)
	}
	if err := checkout.Close(); err != nil {
		t.Fatal(err)
	}
	assertDirectoryEmpty(t, tempRoot)
	if err := checkout.Close(); err != nil {
		t.Fatalf("second Close should be harmless, got %v", err)
	}
}

func TestGitHubCheckoutRejectsInvalidCommitOutputAndCleansSession(t *testing.T) {
	tempRoot := t.TempDir()
	provider := importer.GitHubCheckoutProvider{
		TempRoot: tempRoot,
		Runner: commandRunnerFunc(func(_ context.Context, command importer.Command) ([]byte, error) {
			if command.Args[0] == "clone" {
				if err := os.MkdirAll(command.Args[len(command.Args)-1], 0o755); err != nil {
					return nil, err
				}
				return nil, nil
			}
			return []byte("not-a-commit\n"), nil
		}),
	}

	_, err := provider.Checkout(context.Background(), "https://github.com/example/repo")
	if err == nil || !strings.Contains(err.Error(), "invalid commit SHA") {
		t.Fatalf("expected invalid commit error, got %v", err)
	}
	assertDirectoryEmpty(t, tempRoot)
}

func TestGitHubCheckoutResolvesRelativeTempRootBeforeReturning(t *testing.T) {
	workingDir := t.TempDir()
	t.Chdir(workingDir)
	if err := os.Mkdir("scratch", 0o755); err != nil {
		t.Fatal(err)
	}
	const commit = "0123456789abcdef0123456789abcdef01234567"
	provider := importer.GitHubCheckoutProvider{
		TempRoot: "scratch",
		Runner: commandRunnerFunc(func(_ context.Context, command importer.Command) ([]byte, error) {
			if command.Args[0] == "clone" {
				if err := os.MkdirAll(command.Args[len(command.Args)-1], 0o755); err != nil {
					return nil, err
				}
				return nil, nil
			}
			return []byte(commit), nil
		}),
	}
	checkout, err := provider.Checkout(context.Background(), "https://github.com/example/repo")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(checkout.Root) {
		t.Fatalf("checkout root must not depend on later working-directory changes, got %q", checkout.Root)
	}
	sessionRoot := filepath.Dir(checkout.Root)
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := checkout.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sessionRoot); !os.IsNotExist(err) {
		t.Fatalf("Close should remove the original absolute session, stat error: %v", err)
	}
}

func assertDirectoryEmpty(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary sessions were not cleaned: %v", entries)
	}
}

type commandRunnerFunc func(context.Context, importer.Command) ([]byte, error)

func (f commandRunnerFunc) Run(ctx context.Context, command importer.Command) ([]byte, error) {
	return f(ctx, command)
}
