#!/bin/bash
set -euo pipefail

APP_NAME="ophidian"
BUILD_DIR="build"
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS="-X main.version=0.1.0 -X main.commit=$COMMIT -X main.buildTime=$BUILD_TIME"

echo "Building $APP_NAME..."

mkdir -p $BUILD_DIR

go build $LDFLAGS -o $BUILD_DIR/ophidian-server ./cmd/ophidian-server
go build $LDFLAGS -o $BUILD_DIR/ophidian-cli ./cmd/ophidian-cli
go build $LDFLAGS -o $BUILD_DIR/ophidian-agent ./cmd/ophidian-agent
go build $LDFLAGS -o $BUILD_DIR/ophidian-worker ./cmd/ophidian-worker

echo "Build complete: $(ls $BUILD_DIR/)"
