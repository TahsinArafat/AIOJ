package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// writeMigrations lays out a source directory with n versioned migrations.
func writeMigrations(t *testing.T, dir string, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		for _, suffix := range []string{"up", "down"} {
			name := fmt.Sprintf("%06d_seed.%s.sql", i, suffix)
			body := "SELECT 1;\n"
			if suffix == "down" {
				body = "SELECT 1;\n"
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
				t.Fatalf("write migration: %v", err)
			}
		}
	}
}

func TestLatestVersionReadsTheHighestOnDisk(t *testing.T) {
	dir := t.TempDir()
	writeMigrations(t, dir, 5)

	got, ok := latestVersion(dir)
	if !ok {
		t.Fatal("latestVersion reported failure for a readable directory")
	}
	if got != 5 {
		t.Fatalf("latestVersion = %d, want 5", got)
	}
}

// A migration authored later can sit at a version *below* one already shipped —
// that is exactly how 000087 was written after 000088 existed. The set on disk
// is what matters, not the order the files were written in.
func TestLatestVersionIgnoresFileCreationOrder(t *testing.T) {
	dir := t.TempDir()
	// Versions 1..3, written out of order, which is all "authored later" means.
	for _, v := range []int{3, 1, 2} {
		for _, suffix := range []string{"up", "down"} {
			name := fmt.Sprintf("%06d_seed.%s.sql", v, suffix)
			if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	got, ok := latestVersion(dir)
	if !ok || got != 3 {
		t.Fatalf("latestVersion = %d (ok=%v), want 3", got, ok)
	}
}

// A renumbering that leaves the old file behind is rejected by the source
// driver, and latestVersion must report that rather than guessing a version.
// Reporting a wrong number here would make the repair step clear a dirty flag
// against the wrong schema.
func TestLatestVersionRejectsDuplicateVersions(t *testing.T) {
	dir := t.TempDir()
	writeMigrations(t, dir, 3)
	// An orphaned copy of version 2, as a half-finished renumber leaves behind.
	if err := os.WriteFile(filepath.Join(dir, "000002_old.up.sql"), []byte("SELECT 1;"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, ok := latestVersion(dir); ok {
		t.Fatal("latestVersion accepted a directory with a duplicate version")
	}
}

func TestLatestVersionOnMissingDirectory(t *testing.T) {
	if _, ok := latestVersion(filepath.Join(t.TempDir(), "nope")); ok {
		t.Fatal("latestVersion claimed success for a missing directory")
	}
}

// A migration set with gaps still reports its highest version; the reader walks
// First/Next rather than assuming 1..N.
func TestLatestVersionHandlesGaps(t *testing.T) {
	dir := t.TempDir()
	for _, v := range []int{1, 7, 42} {
		for _, suffix := range []string{"up", "down"} {
			name := fmt.Sprintf("%06d_gap.%s.sql", v, suffix)
			if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	got, ok := latestVersion(dir)
	if !ok || got != 42 {
		t.Fatalf("latestVersion = %d (ok=%v), want 42", got, ok)
	}
}
