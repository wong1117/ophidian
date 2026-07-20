# OPHIDIAN Operations

## Monitoring
- Health check: `GET /health`
- Metrics: `GET /metrics`

## Logging
- Structured JSON logs via zerolog
- Log levels: debug, info, warn, error

## Audit Trail
- All events stored immutably in PostgreSQL
- 100% reconstructable from event log

## Backup
- PostgreSQL: `pg_dump`
- Event store: append-only, continuous archiving
