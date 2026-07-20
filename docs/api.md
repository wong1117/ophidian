# OPHIDIAN API

## Base URL: `/api/v1`

### Missions
- `POST /missions` - Create mission
- `GET /missions` - List missions
- `GET /missions/:id` - Get mission details
- `POST /missions/:id/start` - Start mission
- `POST /missions/:id/abort` - Abort mission

### Recon
- `POST /recon/passive` - Start passive recon
- `POST /recon/active` - Start active recon
- `GET /recon/:id` - Get recon results

### Exploit
- `POST /exploit/match` - Match exploits to services
- `POST /exploit/execute` - Execute exploit
- `GET /exploit/sessions` - List active sessions

### AI
- `POST /ai/plan` - Generate attack plan
- `GET /ai/plan/:id` - Get plan details
- `POST /ai/correlate` - Correlate findings

### Reports
- `POST /report/generate` - Generate report
- `GET /report/export/:format` - Export report
