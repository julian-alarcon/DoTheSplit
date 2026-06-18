.PHONY: help gen gen-go gen-ts-frontend migrate-up migrate-down test test-go test-frontend dev dev-api dev-frontend lint lint-go build build-frontend embed-frontend tidy up licenses sbom compliance

# Where the built Vue SPA is copied so the Go binary can //go:embed it.
WEBUI_DIST := api/internal/webui/dist

SHELL := /bin/bash

DATABASE_URL ?= postgres://dts:dts@localhost:5432/dts?sslmode=disable

help:
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

gen: gen-go gen-ts-frontend ## Regenerate server + client bindings from docs/openapi.yaml

gen-go: ## Generate Go models + embedded spec from docs/openapi.yaml
	cd api && go generate ./...

gen-ts-frontend: ## Generate TypeScript types for the Vue SPA client
	cd frontend && npm run gen:api

migrate-up: ## Apply all migrations
	cd api && go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate \
		-path ./migrations -database "$(DATABASE_URL)" up

migrate-down: ## Roll back the most recent migration
	cd api && go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate \
		-path ./migrations -database "$(DATABASE_URL)" down 1

test: test-go test-frontend ## Run all tests

test-go: ## Run Go unit + integration tests
	cd api && go test ./... -race

test-frontend: ## Run Vue SPA unit tests
	cd frontend && npm test --silent || true

dev-api: ## Run the API locally
	cd api && go run ./cmd/api

dev-frontend: ## Run the Vue SPA dev server (proxies /v1 to the local API)
	cd frontend && npm run dev

dev: ## Run api + frontend concurrently (requires two terminals)
	@echo "Run 'make dev-api' and 'make dev-frontend' in separate terminals."

tidy: ## go mod tidy
	cd api && go mod tidy

lint: lint-go ## Lint everything

lint-go: ## Lint Go
	cd api && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./...

build-frontend: ## Build the Vue SPA static bundle (frontend/dist)
	cd frontend && npm ci && npm run build

embed-frontend: build-frontend ## Copy the built SPA into the Go embed dir
	mkdir -p $(WEBUI_DIST)
	find $(WEBUI_DIST) -mindepth 1 ! -name .gitkeep ! -name .gitignore -delete
	cp -r frontend/dist/. $(WEBUI_DIST)/

build: embed-frontend ## Build Go binaries with the SPA embedded
	cd api && go build -trimpath -ldflags="-s -w" -o bin/api ./cmd/api && go build -trimpath -ldflags="-s -w" -o bin/worker ./cmd/worker

up: ## Rebuild + start the full stack, baking git SHA + frontend/package.json version into images
	BUILD_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo dev) \
	BUILD_VERSION=$$(node -p "require('./frontend/package.json').version" 2>/dev/null || echo dev) \
		docker compose up -d --build

licenses: ## Generate THIRD_PARTY_LICENSES.md and frontend/src/lib/credits.json
	./scripts/generate-licenses.sh

sbom: ## Generate CycloneDX SBOMs into ./sbom/
	./scripts/generate-sbom.sh

compliance: licenses sbom ## Run all license + SBOM generation
