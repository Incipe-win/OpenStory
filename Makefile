.PHONY: dev dev-infra stop build test lint migrate migrate-down fmt help

# ─── Variables ────────────────────────────────────────
DATABASE_URL ?= postgres://openstory:openstory@localhost:15432/openstory?sslmode=disable

# ─── Help ─────────────────────────────────────────────
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

# ─── Development ──────────────────────────────────────
dev: ## Start all services with Docker Compose
	docker compose up --build -d

dev-infra: ## Start only infrastructure (DB, Redis, Kafka, MinIO)
	docker compose up -d postgres redis kafka minio minio-init

stop: ## Stop all services
	docker compose down

# ─── Build ────────────────────────────────────────────
build: ## Build all Go binaries locally
	go build -o bin/api       ./cmd/api
	go build -o bin/worker    ./cmd/worker
	go build -o bin/outbox-relay ./cmd/outbox-relay
	go build -o bin/consumer  ./cmd/consumer

# ─── Test ─────────────────────────────────────────────
test: ## Run all tests
	go test ./... -v -race -count=1

# ─── Lint ─────────────────────────────────────────────
lint: ## Run golangci-lint
	golangci-lint run ./...

# ─── Database Migration ──────────────────────────────
migrate: ## Run database migrations up
	goose -dir migrations postgres "$(DATABASE_URL)" up

migrate-down: ## Rollback last migration
	goose -dir migrations postgres "$(DATABASE_URL)" down

migrate-status: ## Show migration status
	goose -dir migrations postgres "$(DATABASE_URL)" status

# ─── Format ───────────────────────────────────────────
fmt: ## Format Go code
	gofmt -s -w .
	goimports -w .
