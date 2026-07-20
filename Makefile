.PHONY: all build build-server build-cli build-agent build-worker test test-race test-coverage lint arch-lint arch-check clean run-server run-cli fuzz fuzz-cron fuzz-feature test-integration quality check dev-setup dev-reset dev-logs docs godoc examples scaffold bench bench-full bench-save bench-cmp

APP_NAME := ophidian
BUILD_DIR := build
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags "-X main.version=0.1.0 -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)"

all: build

build: build-server build-cli build-agent build-worker

build-server:
	go build $(LDFLAGS) -o $(BUILD_DIR)/ophidian-server ./cmd/ophidian-server

build-cli:
	go build $(LDFLAGS) -o $(BUILD_DIR)/ophidian-cli ./cmd/ophidian-cli

build-agent:
	go build $(LDFLAGS) -o $(BUILD_DIR)/ophidian-agent ./cmd/ophidian-agent

build-worker:
	go build $(LDFLAGS) -o $(BUILD_DIR)/ophidian-worker ./cmd/ophidian-worker

test:
	go test ./... -v -count=1

test-race:
	go test -race ./... -count=1

test-coverage:
	go test -coverprofile=coverage.out -covermode=atomic ./... -count=1 2>&1 | tee test-results.txt
	grep -E "^(ok|FAIL|\?)" test-results.txt || true
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage: $$(go tool cover -func=coverage.out | grep total | awk '{print $$3}')"

test-integration:
	go test -tags=integration -v -count=1 ./test/integration/...

fuzz:
	go test -fuzz=Fuzz -fuzztime=30s ./internal/infrastructure/scheduler/... ./internal/infrastructure/queue/... ./internal/infrastructure/secrets/...

fuzz-cron:
	go test -fuzz=FuzzNextCronTime -fuzztime=30s ./internal/infrastructure/scheduler/...

fuzz-feature:
	go test -fuzz=FuzzIsInRollout -fuzztime=30s ./internal/application/feature/...

lint:
	golangci-lint run ./... --timeout=10m

arch-lint:

arch-check:
	go run ./cmd/archlint . --json
	@echo "Architecture compliance report: arch-compliance-report.json"
	go-arch-lint check

clean:
	rm -rf build coverage.out coverage.html test-results.txt

run-server:
	go run ./cmd/ophidian-server

run-cli:
	go run ./cmd/ophidian-cli

dev-setup:
	@echo "Setting up development environment..."
	go mod download
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	@mkdir -p configs
	@test -f configs/config.local.yaml || cp configs/.env.example configs/config.local.yaml 2>/dev/null || echo "server:\n  port: 8080\ndatabase:\n  host: localhost\n  port: 5432\n  user: ophidian\n  password: dev\n  database: ophidian\n  ssl_mode: disable" > configs/config.local.yaml
	@echo "Development environment ready. Run 'make run-server' to start."

dev-reset:
	@echo "Resetting development environment..."
	rm -rf build coverage.out coverage.html test-results.txt
	go clean -cache -testcache
	go mod tidy
	@echo "Reset complete. Run 'make dev-setup' to reinitialize."

dev-logs:
	@echo "=== Watching for changes ==="
	@which air >/dev/null 2>&1 && air || (echo "Install air for hot reload: go install github.com/air-verse/air@latest")

docs:
	@echo "Generating API documentation..."
	swag init -g cmd/ophidian-server/main.go -o docs/api --parseDependency --parseInternal
	@echo "API docs generated at docs/api/"

godoc:
	@echo "Starting godoc server at http://localhost:6060"
	godoc -http=:6060

examples:
	@echo "Running examples..."
	cd examples && go run .

scaffold:
	@test -n "$(NAME)" || (echo "Usage: make scaffold NAME=my-service TEMPLATE=service"; exit 1)
	go run ./cmd/ophidian-cli scaffold $(NAME) --template $(TEMPLATE)

bench:
	go test -bench=. -benchmem -benchtime=2s ./internal/infrastructure/... ./internal/application/... ./pkg/... -run '^$$' 2>&1 | tee bench-results.txt

bench-full:
	go test -bench=BenchmarkSuite -benchmem -benchtime=5s ./cmd/benchmarks/ -run '^$$' 2>&1 | tee bench-full.txt

bench-save:
	go test -bench=BenchmarkSuite -benchmem -benchtime=5s -count=10 ./cmd/benchmarks/ -run '^$$' 2>&1 | tee bench-history.txt

bench-cmp:
	@test -f bench-history.txt || (echo "Run make bench-save first"; exit 1)
	@echo "Comparing current benchmarks with saved baseline..."
	go test -bench=BenchmarkSuite -benchmem -benchtime=2s ./cmd/benchmarks/ -run '^$$' 2>&1 | tee bench-current.txt
	@echo "Diff saved baseline vs current:"
	diff <(grep "ns/op" bench-history.txt | head -20 | cut -d'/' -f3 | sort) <(grep "ns/op" bench-current.txt | head -20 | cut -d'/' -f3 | sort) || true

quality: lint test-race test-coverage arch-check dep-check maintain-check

maintain-check:
	go run ./cmd/maintain
	@echo "Maintainability report: maintainability-report.json"
	@echo "Quality checks complete."

check: build lint test-race
	@echo "CI check complete."
