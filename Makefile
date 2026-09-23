.PHONY: build run test openapi cli test-frontend lint fmt fmt-check build-image migrate-up migrate-down migrate-status migrate-version migrate-force

MIGRATE_DSN ?= postgres://aioj:aioj_dev@localhost:5432/aioj?sslmode=disable
TEST_DSN    ?= $(MIGRATE_DSN)
MIGRATE_DIR  = internal/store/migrations

build:
	go build -o aioj ./cmd/aioj

run:
	go run ./cmd/aioj

test:
	AIOJ_TEST_DSN="$${AIOJ_TEST_DSN:-$(TEST_DSN)}" go test ./cmd/... ./internal/... -count=1

# CI parity: frontend typecheck+build+vitest
test-frontend:
	cd web && npm test

# Local lint: go vet + gofmt -l + eslint (mirrors .github/workflows/lint.yml)
lint: fmt-check
	go vet ./cmd/... ./internal/...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout=10m; \
	else \
		echo "golangci-lint not installed; skipped (CI runs it)"; \
	fi
	cd web && npm run lint

GO_FILES = $(shell find cmd internal -type f -name '*.go')

fmt:
	gofmt -w $(GO_FILES)

fmt-check:
	@unformatted=$$(gofmt -l $(GO_FILES)); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed:"; echo "$$unformatted"; exit 1; \
	fi

build-image:
	docker build -t aioj:local .

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


# --- OpenAPI / CLI (Phase D) -----------------------------------------------
openapi:
	@test -f docs/openapi/openapi.yaml
	@command -v oapi-codegen >/dev/null 2>&1 && oapi-codegen -generate types -package apitypes docs/openapi/openapi.yaml > web/src/lib/api-types.ts || echo "spec ready: docs/openapi/openapi.yaml (install oapi-codegen for types)"

cli:
	go build -o bin/aioj-cli ./cmd/aioj-cli
