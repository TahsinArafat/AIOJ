.PHONY: build run test migrate-up migrate-down

build:
	go build -o aioj ./cmd/aioj
run:
	go run ./cmd/aioj
test:
	go test ./... -v -count=1
migrate-up:
	DB_DSN="postgres://aioj:aioj_dev@localhost:5432/aioj?sslmode=disable" \
	go run ./cmd/migrate -dir internal/store/migrations up
migrate-down:
	DB_DSN="postgres://aioj:aioj_dev@localhost:5432/aioj?sslmode=disable" \
	go run ./cmd/migrate -dir internal/store/migrations down
