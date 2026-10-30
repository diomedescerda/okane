SHELL := /bin/bash
BIN   := bin/okane

.PHONY: help dev db-up db-down migrate build test vet fmt run seed web-install web-dev tidy up down status api web db logs

up: ## Start the whole dev stack (postgres, api, web)
	@./scripts/dev.sh up

down: ## Stop the dev stack (data is kept)
	@./scripts/dev.sh down

status: ## Show what is running (postgres, api, web)
	@./scripts/dev.sh status

logs: ## Tail logs: make logs [api|web|db] (default api)
	@./scripts/dev.sh logs $(filter-out $@,$(MAKECMDGOALS))

api web db: ## used by `make logs api` etc.
	@:

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

db-up: ## Start local Postgres
	docker compose up -d postgres

db-down: ## Stop local Postgres
	docker compose down

migrate: ## Apply database migrations
	go run ./cmd/okane migrate up

seed: ## Create the demo@okane.dev dataset (idempotent)
	go run ./cmd/okane seed

build: ## Build the okane binary
	go build -o $(BIN) ./cmd/okane

run: ## Run the API (applies migrations first)
	go run ./cmd/okane serve

test: ## Run tests
	go test ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format Go code
	gofmt -w .

tidy: ## Tidy Go modules
	go mod tidy

web-install: ## Install web dependencies
	cd web && npm install

web-dev: ## Run web dev server
	cd web && npm run dev

dev: db-up migrate ## Start db + migrations, then print next steps
	@echo "Postgres is up and migrations applied. Run 'make run' and 'make web-dev' in separate terminals."
