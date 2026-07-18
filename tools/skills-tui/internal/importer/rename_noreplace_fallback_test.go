//go:build !darwin && !linux

package importer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenameNoReplaceFallbackFailsClosed(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	destination := filepath.Join(dir, "destination")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := renameNoReplace(source, destination); err == nil {
		t.Fatal("fallback publication must fail rather than risk overwriting a raced destination")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("failed-closed publication should preserve source: %v", err)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("failed-closed publication should not create destination: %v", err)
	}
}
