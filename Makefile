.DEFAULT_GOAL := help
.PHONY: help be-run be-build be-test be-tidy be-vet fe-install fe-dev fe-build fe-lint dev up down

help: ## List available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

## --- Backend (Go) ---
be-run: ## Run the backend server
	cd backend && go run ./cmd/server

be-build: ## Build the backend binary into backend/bin/server
	cd backend && go build -o bin/server ./cmd/server

be-test: ## Run backend tests
	cd backend && go test ./...

be-tidy: ## Tidy go.mod/go.sum
	cd backend && go mod tidy

be-vet: ## Vet backend code
	cd backend && go vet ./...

## --- Frontend (Vite + React) ---
fe-install: ## Install frontend dependencies (pnpm)
	cd frontend && pnpm install

fe-dev: ## Start the Vite dev server
	cd frontend && pnpm dev

fe-build: ## Type-check and build the frontend
	cd frontend && pnpm build

fe-lint: ## Lint the frontend
	cd frontend && pnpm lint

## --- Full stack ---
dev: ## Print how to run both apps for local dev
	@echo "Run in two terminals:"
	@echo "  make be-run"
	@echo "  make fe-dev   # http://localhost:5173"

up: ## Build & start the demo stack via docker compose
	docker compose up --build

down: ## Stop the demo stack
	docker compose down
