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

# --- sim harness (Full-Scale Completion program, Wave 0) --------------------
# Thin wrappers only; each is exactly the command the wave plan documents.
# The sim overlay adds mailhog + a second judge-worker and rebinds the
# frontend to :8081 so it never fights local dev servers on :80.
SIM = docker compose -f docker-compose.yml -f docker-compose.sim.yml --profile sim

.PHONY: sim-up sim-down sim-reset sim-seed sim-logs sim-ps e2e

sim-up:
	$(SIM) up -d --build

sim-down:
	$(SIM) down

sim-reset:
	$(SIM) down -v && $(SIM) up -d --build

sim-seed:
	$(SIM) run --rm seeder

sim-ps:
	$(SIM) ps

sim-logs:
	$(SIM) logs -f backend judge-worker

e2e:
	cd web && npx playwright test
