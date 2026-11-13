// Package db owns the schema migration step that runs at process startup.
//
// Migrations used to be applied inline in cmd/aioj with a bare m.Up() and a
// log.Fatalf on error. That is safe when a human watches the deploy, and unsafe
// when a platform auto-deploys on every push: a migration that cannot run, or a
// database left dirty by one that failed halfway, turns every restart into the
// same fatal error. With restart: unless-stopped that is an unrecoverable
// crash-loop — reverting the image does not help, because the failure lives in
// the schema, not the binary.
//
// Two properties of golang-migrate make that recoverable:
//
//   - A migration runs as a single statement, so a failed one is rolled back
//     atomically. What survives is the version row marked dirty, not a
//     half-applied schema.
//   - Every migration is a file that ships next to this code, so "would this
//     apply cleanly" can be answered before touching the real database.
package db

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/file"
)

// MigrationDir is where the .sql files live, relative to the process working
// directory (the container sets it to the app root).
const MigrationDir = "internal/store/migrations"

// Result describes what the startup migration step did.
type Result struct {
	// From and To are the schema versions before and after.
	From uint
	To   uint
	// Applied is how many migrations ran.
	Applied int
	// Recovered is true when a dirty version was repaired before migrating.
	Recovered bool
	// RecoveredFrom is the dirty version that was repaired.
	RecoveredFrom uint
}

// Migrate applies every pending migration, optionally recovering first from a
// version left dirty by an interrupted run.
//
// autoRecover is off by default and must be enabled deliberately: clearing the
// dirty flag asserts that the migration it names is not half-applied. The two
// cases it accepts are both provably safe given golang-migrate's atomicity:
//
//   - The version is the highest that exists on disk. The migration was
//     attempted and rolled back, so the schema is still at the previous version;
//     clearing the flag lets it be retried.
//   - The version does not exist on disk at all. A migration was removed or
//     renumbered after being recorded, so there is nothing to retry.
//
// Anything else is refused, because it means a migration in the middle of the
// sequence is marked dirty and the schema's real shape is unknown.
func Migrate(m *migrate.Migrate, dir string, autoRecover bool) (Result, error) {
	var res Result

	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return res, fmt.Errorf("read schema version: %w", err)
	}
	res.From = version

	if dirty {
		if !autoRecover {
			return res, fmt.Errorf(
				"database is dirty at version %d: a migration failed partway and must be "+
					"resolved by hand, or set database.migrate_auto_recover to retry it", version)
		}
		if repairErr := repairDirty(m, dir, version); repairErr != nil {
			return res, repairErr
		}
		res.Recovered = true
		res.RecoveredFrom = version
		slog.Warn("repaired a dirty schema version before migrating",
			"version", version, "detail", "the migration was rolled back, so it is safe to retry")
	}

	// Count what actually runs: Up() reports ErrNoChange for a no-op, which is
	// the common case on every deploy that adds no migration.
	before := version
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		// Name the version we were on, so the operator knows where the schema
		// actually stopped instead of only that something failed.
		after, dirtyNow, verr := m.Version()
		if verr == nil {
			return res, fmt.Errorf("apply migrations from version %d: %w (now at %d, dirty=%v)",
				before, err, after, dirtyNow)
		}
		return res, fmt.Errorf("apply migrations from version %d: %w", before, err)
	}

	final, _, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return res, fmt.Errorf("read schema version after migrating: %w", err)
	}
	res.To = final
	if final > before {
		res.Applied = int(final - before)
	}
	return res, nil
}

// repairDirty clears a dirty flag, but only for the two shapes that are known to
// be safe. See Migrate.
func repairDirty(m *migrate.Migrate, dir string, version uint) error {
	latest, ok := latestVersion(dir)
	if !ok {
		// Without knowing the newest migration there is no way to tell a safe
		// repair from a guess, so this refuses.
		return fmt.Errorf(
			"database is dirty at version %d and the migration set could not be read; "+
				"resolve it by hand", version)
	}

	switch {
	case version == latest:
		// The interrupt hit the newest migration; it rolled back, so the schema
		// sits at version-1 and the migration can run again. Force(-1) covers
		// the first-migration case, where there is no version below it.
		slog.Warn("dirty version is the newest migration on disk; retrying it", "version", version)
		return m.Force(int(version) - 1)

	case version > latest:
		// No such migration any more (renumbered or deleted). Nothing to retry.
		slog.Warn("dirty version has no migration on disk; clearing it", "version", version, "latest", latest)
		return m.Force(int(latest))

	default:
		// A migration in the middle of the sequence is dirty: the ones after it
		// may or may not have been recorded. Refuse rather than guess.
		return fmt.Errorf(
			"database is dirty at version %d but %d is the newest migration: this is a "+
				"partially-applied sequence and must be resolved by hand, not automatically",
			version, latest)
	}
}

// latestVersion reports the highest migration version present on disk.
//
// It opens the source directory itself rather than reaching into the Migrate
// instance: Migrate exposes no source driver, and every one of its own helpers
// requires a clean database — exactly what is in doubt when this is called.
func latestVersion(dir string) (uint, bool) {
	src, err := (&file.File{}).Open(dir)
	if err != nil {
		return 0, false
	}
	defer src.Close()

	v, err := src.First()
	if err != nil {
		return 0, false
	}
	for {
		next, err := src.Next(v)
		if err != nil {
			break
		}
		v = next
	}
	return v, true
}
