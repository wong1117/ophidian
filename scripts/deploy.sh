#!/bin/bash
set -euo pipefail

echo "Deploying OPHIDIAN..."

# Run database migrations
# TODO: implement

# Start services
echo "Starting control plane..."
nohup ophidian-server > /var/log/ophidian/server.log 2>&1 &

echo "Deployment complete."
