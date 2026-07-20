# OPHIDIAN Deployment

## Prerequisites
- Go 1.22+
- PostgreSQL 15+
- Redis 7+
- NATS or RabbitMQ
- Ollama (for AI Plane)

## Build
```bash
make build
```

## Configuration
Edit configs in `/etc/ophidian/`:
- `control-plane.yaml`
- `ai-plane.yaml`
- `execution-plane.yaml`

## Run
```bash
./build/ophidian-server
```
