.PHONY: build run dev test clean docker-up docker-down

# Build the binary
build:
	go build -o bin/syntheticworlds ./cmd/syntheticworlds

# Run locally (requires Postgres + Redis running)
run: build
	./bin/syntheticworlds

# Run with live reload (requires air: go install github.com/air-verse/air@latest)
dev:
	air -c .air.toml || go run ./cmd/syntheticworlds

# Run tests
test:
	go test ./... -v

# Clean build artifacts
clean:
	rm -rf bin/

# Start all services with Docker Compose
docker-up:
	docker compose up --build -d

# Stop all services
docker-down:
	docker compose down

# View logs
logs:
	docker compose logs -f syntheticworlds

# Run migrations manually (Postgres must be running)
migrate:
	psql "$$DATABASE_URL" -f migrations/000001_init.up.sql

# Format code
fmt:
	go fmt ./...
	go vet ./...
