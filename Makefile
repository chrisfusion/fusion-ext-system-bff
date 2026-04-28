IMG     ?= fusion-ext-system-bff:latest
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test lint lint-fix docker-build run tidy vet fmt clean help

build: ## Build the binary
	CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o bin/fusion-ext-system-bff ./cmd/server

test: ## Run unit tests
	go test ./... -v -count=1 -race

lint: ## Run golangci-lint
	golangci-lint run ./...

lint-fix: ## Run golangci-lint with auto-fix
	golangci-lint run --fix ./...

docker-build: ## Build Docker image (IMG=fusion-ext-system-bff:local)
	docker build --build-arg VERSION=$(VERSION) -t $(IMG) .

run: ## Run locally (reads .env if present)
	@set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/server

tidy: ## Tidy go.mod
	go mod tidy

vet: ## Run go vet
	go vet ./...

fmt: ## Format source
	gofmt -w .

clean: ## Remove build artifacts
	rm -rf bin/

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
