# Makefile for Docker Controller

.PHONY: build
build: ## Build the docker-controller binary
	go build -o docker-controller ./cmd/main.go

.PHONY: test
test: ## Run tests
	go test ./...

.PHONY: fmt
fmt: ## Format code
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: tidy
tidy: ## Tidy go modules
	go mod tidy

.PHONY: docker-build
docker-build: ## Build Docker image
	docker build -t docker-controller:latest -f Dockerfile .

.PHONY: clean
clean: ## Clean build artifacts
	rm -f docker-controller

.PHONY: help
help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

