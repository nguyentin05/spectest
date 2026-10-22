.PHONY: help install build run test vet lint format tidy clean

GO ?= go

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

install: ## Download Go module dependencies
	$(GO) mod download

build: ## Build Go binaries into ./bin
	@mkdir -p bin
	$(GO) build -o bin/ ./cmd/...

run: ## Run the CLI directly
	$(GO) run ./cmd/spectest

test: ## Run Go tests
	$(GO) test -race ./...

vet: ## Run go vet
	$(GO) vet ./...

lint: ## Run golangci-lint
	golangci-lint run

format: ## Format Go code
	gofmt -w .

tidy: ## Tidy go.mod and go.sum
	$(GO) mod tidy

clean: ## Remove build artifacts
	rm -rf bin dist reports coverage.out coverage.html site
