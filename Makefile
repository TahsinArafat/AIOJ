.PHONY: build run test migrate-up migrate-down migrate-status migrate-version migrate-force

MIGRATE_DSN ?= postgres://aioj:aioj_dev@localhost:5432/aioj?sslmode=disable
MIGRATE_DIR  = internal/store/migrations

build:
	go build -o aioj ./cmd/aioj

run:
	go run ./cmd/aioj

test:
	go test ./... -v -count=1

migrate-up:
	DB_DSN="$(MIGRATE_DSN)" go run ./cmd/migrate -dir $(MIGRATE_DIR) up

migrate-down:
	DB_DSN="$(MIGRATE_DSN)" go run ./cmd/migrate -dir $(MIGRATE_DIR) down

migrate-status:
	DB_DSN="$(MIGRATE_DSN)" go run ./cmd/migrate -dir $(MIGRATE_DIR) status

migrate-version:
	DB_DSN="$(MIGRATE_DSN)" go run ./cmd/migrate -dir $(MIGRATE_DIR) version

migrate-force:
	@test -n "$(V)" || (echo "usage: make migrate-force V=<version>"; exit 1)
	DB_DSN="$(MIGRATE_DSN)" go run ./cmd/migrate -dir $(MIGRATE_DIR) force $(V)

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f backend
