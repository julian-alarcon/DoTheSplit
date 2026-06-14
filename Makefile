.PHONY: help gen gen-go gen-ts-app migrate-up migrate-down test test-go test-app dev dev-api dev-app lint lint-go build build-app embed-app tidy up licenses sbom compliance

# Where the built Vue SPA is copied so the Go binary can //go:embed it.
WEBUI_DIST := api/internal/webui/dist

SHELL := /bin/bash

DATABASE_URL ?= postgres://dts:dts@localhost:5432/dts?sslmode=disable

help:
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

gen: gen-go gen-ts-app ## Regenerate server + client bindings from docs/openapi.yaml

gen-go: ## Generate Go models + embedded spec from docs/openapi.yaml
	cd api && go generate ./...

gen-ts-app: ## Generate TypeScript types for the Vue SPA client
	cd app && npm run gen:api

migrate-up: ## Apply all migrations
	cd api && go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate \
		-path ./migrations -database "$(DATABASE_URL)" up

migrate-down: ## Roll back the most recent migration
	cd api && go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate \
		-path ./migrations -database "$(DATABASE_URL)" down 1

test: test-go test-app ## Run all tests

test-go: ## Run Go unit + integration tests
	cd api && go test ./... -race

test-app: ## Run Vue SPA unit tests
	cd app && npm test --silent || true

dev-api: ## Run the API locally
	cd api && go run ./cmd/api

dev-app: ## Run the Vue SPA dev server (proxies /v1 to the local API)
	cd app && npm run dev

dev: ## Run api + app concurrently (requires two terminals)
	@echo "Run 'make dev-api' and 'make dev-app' in separate terminals."

tidy: ## go mod tidy
	cd api && go mod tidy

lint: lint-go ## Lint everything

lint-go: ## Lint Go
	cd api && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./...

build-app: ## Build the Vue SPA static bundle (app/dist)
	cd app && npm ci && npm run build

embed-app: build-app ## Copy the built SPA into the Go embed dir
	mkdir -p $(WEBUI_DIST)
	find $(WEBUI_DIST) -mindepth 1 ! -name .gitkeep ! -name .gitignore -delete
	cp -r app/dist/. $(WEBUI_DIST)/

build: embed-app ## Build Go binaries with the SPA embedded
	cd api && go build -o bin/api ./cmd/api && go build -o bin/worker ./cmd/worker

up: ## Rebuild + start the full stack, baking git SHA + app/package.json version into images
	BUILD_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo dev) \
	BUILD_VERSION=$$(node -p "require('./app/package.json').version" 2>/dev/null || echo dev) \
		docker compose up -d --build

licenses: ## Generate THIRD_PARTY_LICENSES.md and app/src/lib/credits.json
	./scripts/generate-licenses.sh

sbom: ## Generate CycloneDX SBOMs into ./sbom/
	./scripts/generate-sbom.sh

compliance: licenses sbom ## Run all license + SBOM generation
