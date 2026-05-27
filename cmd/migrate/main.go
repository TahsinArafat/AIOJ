package main

import (
	"flag"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://aioj:aioj_dev@localhost:5432/aioj?sslmode=disable"
	}
	dir := flag.String("dir", "internal/store/migrations", "migrations directory")
	flag.Parse()
	cmd := flag.Arg(0)
	m, err := migrate.New("file://"+*dir, dsn)
	if err != nil {
		log.Fatalf("migrate new: %v", err)
	}
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
	default:
		log.Fatalf("unknown command: %s (use up or down)", cmd)
	}
}
