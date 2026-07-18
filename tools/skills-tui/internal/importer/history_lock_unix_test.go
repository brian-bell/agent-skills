//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package importer

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestLockFileExclusiveHonorsCancellationWhileContended(t *testing.T) {
	path := t.TempDir() + "/import.lock"
	owner, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	waiter, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer waiter.Close()

	unlock, err := lockFileExclusive(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := lockFileExclusive(ctx, waiter)
		result <- err
	}()
	time.Sleep(25 * time.Millisecond)
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("contended file lock should return context cancellation, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("contended file lock did not return after cancellation")
	}
}
