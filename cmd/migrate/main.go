package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	schemadb "github.com/tahsinarafat/aioj/internal/db"
)

func main() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://aioj:aioj_dev@localhost:5432/aioj?sslmode=disable"
	}
	dir := flag.String("dir", "internal/store/migrations", "migrations directory")
	flag.Parse()

	cmd := flag.Arg(0)
	if cmd == "" {
		log.Fatal("usage: migrate [-dir <path>] <up|down|steps N|version|status|force N|repair>")
	}

	m, err := migrate.New("file://"+*dir, dsn)
	if err != nil {
		log.Fatalf("migrate new: %v", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			log.Printf("source close error: %v", srcErr)
		}
		if dbErr != nil {
			log.Printf("db close error: %v", dbErr)
		}
	}()

	switch cmd {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate up: %v", err)
		}
		log.Println("migration up complete")

	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate down: %v", err)
		}
		log.Println("migration down complete")

	case "steps":
		arg := flag.Arg(1)
		if arg == "" {
			log.Fatal("steps requires an integer argument, e.g. 'steps 2' or 'steps -1'")
		}
		n, err := strconv.Atoi(arg)
		if err != nil || n == 0 {
			log.Fatalf("steps: invalid argument %q (must be non-zero integer)", arg)
		}
		if err := m.Steps(n); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate steps %d: %v", n, err)
		}
		log.Printf("migration steps(%d) complete", n)

	case "version":
		v, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("migrate version: %v", err)
		}
		if dirty {
			fmt.Printf("version: %d (DIRTY)\n", v)
		} else {
			fmt.Printf("version: %d\n", v)
		}

	case "status":
		v, dirty, err := m.Version()
		if err == migrate.ErrNilVersion {
			fmt.Println("status: no migrations applied")
			return
		}
		if err != nil {
			log.Fatalf("migrate status: %v", err)
		}
		state := "clean"
		if dirty {
			state = "DIRTY — run 'force <version>' to recover"
		}
		fmt.Printf("current version: %d (%s)\n", v, state)

	case "force":
		arg := flag.Arg(1)
		if arg == "" {
			log.Fatal("force requires a version integer argument, e.g. 'force 45'")
		}
		n, err := strconv.Atoi(arg)
		if err != nil {
			log.Fatalf("force: invalid version %q", arg)
		}
		if err := m.Force(n); err != nil {
			log.Fatalf("migrate force %d: %v", n, err)
		}
		log.Printf("forced version to %d", n)

	case "repair":
		// The startup path repairs a dirty version before failing, but only when
		// the operator has opted in; a deployment that has not is left with a
		// fatal error and no command to run. `migrate force N` is the documented
		// remedy in the usage string, but N is the one thing an operator does not
		// know — and guessing it wrong writes a false version row.
		//
		// This applies the same two provably-safe cases as the startup repair,
		// in the same order, and refuses everything else. The refusal is the
		// feature: it is the only thing standing between a guess and a schema
		// whose real shape nobody checked.
		if !dirtyAt(m) {
			log.Fatal("repair: schema is not dirty; nothing to do")
		}
		v, _, err := m.Version()
		if err != nil {
			log.Fatalf("repair: read version: %v", err)
		}
		if err := schemadb.RepairDirty(m, *dir, v); err != nil {
			log.Fatalf("repair: %v", err)
		}
		log.Printf("repaired dirty version %d; run 'up' next", v)

	default:
		log.Fatalf("unknown command: %q\nUsage: migrate [-dir path] <up|down|steps N|version|status|force N|repair>", cmd)
	}
}

// dirtyAt reports the dirty flag without treating "no version yet" as an error:
// a database with no migrations applied is clean, and repair has nothing to do
// there.
func dirtyAt(m *migrate.Migrate) bool {
	_, dirty, err := m.Version()
	if err != nil {
		return false
	}
	return dirty
}
