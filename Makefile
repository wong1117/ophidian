.PHONY: all build build-server build-cli build-agent build-worker test test-race test-coverage lint arch-lint clean run-server run-cli

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
	go test ./... -race -count=1

test-coverage:
	go test ./... -coverprofile=coverage.out -count=1
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

arch-lint:
	go-arch-lint check

clean:
	rm -rf build coverage.out coverage.html

run-server:
	go run ./cmd/ophidian-server

run-cli:
	go run ./cmd/ophidian-cli
