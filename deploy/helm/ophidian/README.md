# Ophidian Helm Chart

## Prerequisites

- Kubernetes 1.24+
- Helm 3.8+
- PostgreSQL 14+ (external)
- Redis 7+ (external)

## Quick Start

```bash
helm repo add ophidian https://charts.ophidian.io
helm install ophidian ophidian/ophidian \
  --set config.database.host=postgres.example.com \
  --set secrets.databasePassword=your-db-password \
  --set secrets.jwtSecret=your-jwt-secret
```

## Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `3` |
| `image.repository` | Container image | `ophidian/ophidian-server` |
| `image.tag` | Image tag | `latest` |
| `service.type` | Service type | `ClusterIP` |
| `service.port` | Service port | `8080` |
| `config.server.host` | Server bind address | `0.0.0.0` |
| `config.database.host` | PostgreSQL host | `""` |
| `config.database.port` | PostgreSQL port | `5432` |
| `config.redis.host` | Redis host | `""` |
| `secrets.databasePassword` | DB password | Random 32 chars |
| `secrets.jwtSecret` | JWT signing key | Random 64 chars |
| `secrets.aiApiKey` | AI provider API key | `""` |
| `autoscaling.enabled` | Enable HPA | `true` |
| `autoscaling.minReplicas` | Min pods | `2` |
| `autoscaling.maxReplicas` | Max pods | `10` |
| `podDisruptionBudget.enabled` | Enable PDB | `true` |
| `podDisruptionBudget.minAvailable` | Min available pods | `1` |
| `resources.limits.cpu` | CPU limit | `1000m` |
| `resources.limits.memory` | Memory limit | `512Mi` |
| `resources.requests.cpu` | CPU request | `200m` |
| `resources.requests.memory` | Memory request | `256Mi` |

## Security

- All secrets stored in Kubernetes Secrets (not ConfigMaps)
- ConfigMap hashe checksum triggers pod restart on changes
- PodSecurityContext: non-root user, read-only root filesystem, dropped capabilities
- Network TLS via Ingress annotations (configurable)

## High Availability

- HPA with CPU and memory targets
- PDB with minAvailable
- RollingUpdate with 1 maxUnavailable, 1 maxSurge
- Liveness probe: `/health`
- Readiness probe: `/ready`

## Upgrading

```bash
helm upgrade ophidian ophidian/ophidian --reuse-values --set image.tag=v0.2.0
```

## Resource Requirements

| Component | CPU Request | Memory Request | CPU Limit | Memory Limit |
|-----------|------------|----------------|-----------|-------------|
| ophidian-server | 200m | 256Mi | 1000m | 512Mi |
| PostgreSQL (external) | — | — | — | — |
| Redis (external) | — | — | — | — |
