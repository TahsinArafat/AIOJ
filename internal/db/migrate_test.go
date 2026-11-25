package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	dbstub "github.com/golang-migrate/migrate/v4/database/stub"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/stub"
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

// stubMigrate builds a Migrate over the in-memory stub drivers, so the repair
// rules can be asserted without a database.
//
// Two sources have to agree, because RepairDirty consults both: the stub source
// drives Force, and the on-disk directory is what latestVersion reads to decide
// whether a version is the newest. Seeding only one of them makes the refusal
// fire for the wrong reason — "the migration set could not be read" instead of
// the rule under test.
func stubMigrate(t *testing.T, onDisk []uint, recorded int, dirty bool) (*migrate.Migrate, string) {
	t.Helper()

	dir := t.TempDir()
	if n := len(onDisk); n > 0 {
		// writeMigrations lays out 1..n, so the versions must be contiguous —
		// which every case here uses.
		writeMigrations(t, dir, n)
	}

	src := &stub.Stub{Migrations: source.NewMigrations()}
	for _, v := range onDisk {
		src.Migrations.Append(&source.Migration{Version: v, Identifier: fmt.Sprintf("m%d", v)})
	}
	drv := &dbstub.Stub{}
	if err := drv.SetVersion(recorded, dirty); err != nil {
		t.Fatalf("seed recorded version: %v", err)
	}

	m, err := migrate.NewWithInstance("stub", src, "stub", drv)
	if err != nil {
		t.Fatalf("new migrate: %v", err)
	}
	t.Cleanup(func() { _, _ = m.Close() })
	return m, dir
}

// A dirty version below the newest is the dangerous case: the migrations after
// it may or may not have run, so clearing the flag asserts a schema shape nobody
// checked. It must be refused.
func TestRepairDirtyRefusesVersionBelowNewest(t *testing.T) {
	m, dir := stubMigrate(t, []uint{1, 2, 3}, 2, true)
	err := RepairDirty(m, dir, 2)
	if err == nil {
		t.Fatal("RepairDirty accepted a dirty version below the newest")
	}
}

// The newest version is the interrupted-migration case: it rolled back, so the
// schema sits at version-1 and the migration can run again.
func TestRepairDirtyRetriesNewestVersion(t *testing.T) {
	m, dir := stubMigrate(t, []uint{1, 2, 3}, 3, true)
	if err := RepairDirty(m, dir, 3); err != nil {
		t.Fatalf("RepairDirty(newest) = %v, want nil", err)
	}
	v, dirty, err := m.Version()
	if err != nil {
		t.Fatalf("version after repair: %v", err)
	}
	if dirty {
		t.Error("still dirty after repairing the newest version")
	}
	if v != 2 {
		t.Errorf("version = %d, want 2 (one below the interrupted migration)", v)
	}
}

// A version with no migration on disk was renumbered or deleted. Nothing to
// retry, so it clears to the newest.
func TestRepairDirtyClearsVersionNotOnDisk(t *testing.T) {
	m, dir := stubMigrate(t, []uint{1, 2, 3}, 99, true)
	if err := RepairDirty(m, dir, 99); err != nil {
		t.Fatalf("RepairDirty(unknown) = %v, want nil", err)
	}
	v, dirty, err := m.Version()
	if err != nil {
		t.Fatalf("version after repair: %v", err)
	}
	if dirty {
		t.Error("still dirty after clearing a version that no longer exists")
	}
	if v != 3 {
		t.Errorf("version = %d, want 3 (the newest on disk)", v)
	}
}
