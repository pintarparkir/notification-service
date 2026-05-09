.PHONY: help run build dlq dlq-list dlq-replay test test-unit fmt vet lint proto \
        docker-build clean

PROJECT_ENV ?= local

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Run / build ────────────────────────────────────────────────────────────────
run: ## Run the consumer (subscribes to parkirpintar.events, dispatches SMS)
	PROJECT_ENV=$(PROJECT_ENV) go run ./cmd/notification

build: ## Compile both binaries (notification + dlq tool)
	CGO_ENABLED=0 go build -o bin/notification ./cmd/notification
	CGO_ENABLED=0 go build -o bin/dlq          ./cmd/dlq

# ── DLQ tool ───────────────────────────────────────────────────────────────────
dlq: ## Pass-through: `make dlq -- list --limit 5`
	@PROJECT_ENV=$(PROJECT_ENV) go run ./cmd/dlq $(filter-out $@,$(MAKECMDGOALS))

dlq-list: ## Inspect first 20 messages in the DLQ
	PROJECT_ENV=$(PROJECT_ENV) go run ./cmd/dlq list --limit 20

dlq-replay: ## Replay up to 50 DLQ messages back to the main exchange
	PROJECT_ENV=$(PROJECT_ENV) go run ./cmd/dlq replay --limit 50

# ── Tests ──────────────────────────────────────────────────────────────────────
test: ## Run all tests (template renderer suite, etc.)
	go test ./...

test-unit: ## Unit tests only — pure logic in template/ + grpcclient cache
	go test -short -race -count=1 ./pkg/... ./internal/...

# ── Quality ────────────────────────────────────────────────────────────────────
fmt: ## Format Go code
	gofmt -s -w .

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (requires install)
	golangci-lint run ./...

# ── Code generation ────────────────────────────────────────────────────────────
proto: ## Regenerate api/proto/user/v1/{user.pb.go, user_grpc.pb.go} (consumed via gRPC)
	@which protoc >/dev/null || (echo "protoc not installed (brew install protobuf)" && exit 1)
	@which protoc-gen-go >/dev/null || (echo "protoc-gen-go not installed (go install google.golang.org/protobuf/cmd/protoc-gen-go@latest)" && exit 1)
	protoc --go_out=. --go_opt=paths=source_relative \
	       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	       api/proto/user/v1/user.proto

# ── Container ──────────────────────────────────────────────────────────────────
docker-build: ## Build the service container image
	docker build -t notification-service:local .

# ── Housekeeping ───────────────────────────────────────────────────────────────
clean: ## Remove build artefacts
	rm -rf bin/ coverage.out
