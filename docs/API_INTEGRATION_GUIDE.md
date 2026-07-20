# Ophidian API Integration Guide

## Server

| Property | Value |
|----------|-------|
| Protocol | HTTP/1.1 REST |
| Host | localhost |
| Port | 8443 |
| Content-Type | application/json |

## Quickstart

```bash
# Start the server
./build/ophidian-server

# In another terminal:
# Health check
curl http://localhost:8443/health

# Create and execute a mission
curl -s -X POST http://localhost:8443/api/v1/missions \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Pentest Ground SQLi",
    "target": {"name": "pg","domains":["pentest-ground.com"],"ips":[]},
    "objectives": [{"description": "SQL injection on login","priority":1}],
    "roe": {"max_severity":"HIGH","allow_exfiltration":true,"max_targets":1},
    "started_by": "op"
  }' | jq
```

## Endpoints

### Missions

```bash
# Create
curl -X POST http://localhost:8443/api/v1/missions -H 'Content-Type: application/json' \
  -d '{"name":"X","target":{"name":"T","domains":["example.com"]},"objectives":[{"description":"D","priority":1}],"roe":{"max_severity":"HIGH","max_targets":1},"started_by":"op"}'

# List
curl http://localhost:8443/api/v1/missions

# Get
curl http://localhost:8443/api/v1/missions/<ID>

# Start (if DRAFT)
curl -X POST http://localhost:8443/api/v1/missions/<ID>/start

# Abort
curl -X POST http://localhost:8443/api/v1/missions/<ID>/abort
```

### Recon

```bash
# Passive
curl -X POST http://localhost:8443/api/v1/recon/passive \
  -H 'Content-Type: application/json' \
  -d '{"target_id":"T1","type":"passive"}'

# Active
curl -X POST http://localhost:8443/api/v1/recon/active \
  -H 'Content-Type: application/json' \
  -d '{"target_id":"T1","ports":"80,443","type":"active"}'
```

### One-Liner for Target: pentest-ground.com

```bash
M=$(curl -s -X POST http://localhost:8443/api/v1/missions \
  -H 'Content-Type: application/json' \
  -d '{"name":"PG SQLi","target":{"name":"pg","domains":["pentest-ground.com"]},"objectives":[{"description":"SQLi","priority":1}],"roe":{"max_severity":"HIGH","allow_exfiltration":true,"max_targets":1},"started_by":"op"}')
ID=$(echo $M | jq -r .id)
echo "Mission: $ID"
curl -s "http://localhost:8443/api/v1/missions/$ID" | jq
curl -s -X POST http://localhost:8443/api/v1/recon/active \
  -H 'Content-Type: application/json' \
  -d "{\"target_id\":\"$ID\",\"ports\":\"80,443,4280\",\"type\":\"active\"}" | jq
```

## Note

The server stores missions **in-memory** by default (no PostgreSQL required for development). Data is lost on restart. For production persistence, configure PostgreSQL via environment variables or config file.

## Architecture Notes

- Server communicates with Worker via in-process dispatch (NATS/gRPC stubs pending)
- Recon/Exploit handlers are **stubs** (return acknowledgement but don't execute real tools)
- CLI (`ophidian-cli`) is infrastructure-only: `dashboard`, `events`, `metrics`, `plugins`, `workflow`, `scaffold`
- To execute real offensive tools, use the `ophidian-worker` binary with the queue infrastructure
