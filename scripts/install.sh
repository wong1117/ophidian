#!/bin/bash
set -euo pipefail

echo "Installing OPHIDIAN..."

# Build binaries
./scripts/build.sh

# Copy configs
mkdir -p /etc/ophidian
cp configs/*.yaml /etc/ophidian/

# Copy binaries
cp build/* /usr/local/bin/

echo "Installation complete."
