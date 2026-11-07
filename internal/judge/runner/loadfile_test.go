package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileReportsAMissingCase(t *testing.T) {
	// Serving an empty string for a missing test case turned an infrastructure
	// fault into a wrong-answer verdict against the contestant.
	if _, err := loadFile(filepath.Join(t.TempDir(), "missing.in")); err == nil {
		t.Fatal("expected an error for a missing file")
	}

	path := filepath.Join(t.TempDir(), "ok.in")
	if err := os.WriteFile(path, []byte("3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "3\n" {
		t.Fatalf("content = %q", got)
	}
}
