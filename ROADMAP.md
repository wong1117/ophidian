# OPHIDIAN3 MASTER DEVELOPMENT ROADMAP

## 0. Project Identity

Name: Ophidian3
Nature: Internal, Solo-Developer Offensive AI Security Platform
Core Philosophy: AI as Advisor (Never Commander), Extreme Engineering Excellence, Stealth over SaaS-features.
Architecture: Three-Plane (Control, AI, Execution) + Clean Architecture + Event Sourcing + CQRS.

---

## 1. The Grand Objective

To build a self-hosted, highly stealthy, and architecturally perfect system that acts as a personal exoskeleton for offensive security operations. It must record every action via Event Sourcing, reason over targets using local/cloud LLMs, and execute attacks only with explicit human approval.

---

## 2. Era 1: The Foundation (COMPLETED)

Goal: Establish unbreakable architectural boundaries and core domain logic.

Phase 1.1 to 1.5: Core Setup
- Project structure (cmd, internal, pkg, configs).
- PostgreSQL setup with pgxpool.
- Basic Echo v4 HTTP server skeleton.
- Domain entities (Mission, Target, RoE).
- CQRS separation (Commands vs Queries).

Phase 1.6 to 1.8: Event Sourcing Core
- Event Store implementation in Postgres.
- Aggregate root logic (Mission aggregate).
- Snapshotting mechanism.
- Domain event definitions.

Status: 100% Complete.

---

## 3. Era 2: The Infrastructure & Planes (COMPLETED)

Goal: Build the specific components for the Three-Plane architecture and AI integration.

Phase 2.1: Control Plane
- Mission HTTP handlers (Create, Get, Start, Abort).
- Basic CLI skeleton (Cobra).
- TUI skeleton (Bubble Tea) - Read-only dashboard.

Phase 2.2: Execution Plane (Worker)
- Worker skeleton listening for jobs.
- Job queue mechanism (in-memory/HTTP bridge).

Phase 2.3: AI Plane
- LLM Client interface (Supporting Ollama/DeepSeek).
- Embedded Vector DB integration (chromem-go) for RAG.
- Basic Prompt templates.

Phase 2.4: Observability Suite
- OpenTelemetry integration.
- Prometheus metrics.
- pprof integration.

Status: 100% Complete.

---

## 4. Era 3: Engineering Excellence (COMPLETED)

Goal: Ensure the system does not collapse under real operational stress.

Phase 3.1: Performance
- Benchmarking core packages.
- Cache integration (Ristretto/Redis).
- Profiling and optimization.

Phase 3.2: Reliability
- Graceful shutdown/startup.
- Fault injection testing.
- Chaos testing.

Phase 3.3: Governance
- Architecture linters (forbidding forbidden imports).
- Dependency direction enforcement.
- Cyclic dependency detection.

Phase 3.4: Supply Chain
- Dependency vulnerability scanning.
- SBOM generation.

Status: 100% Complete.

---

## 5. Era 4: The Operational MVP (COMPLETED)

Goal: Prove the architecture works by executing a real, end-to-end attack cycle.

Phase 4.1: Minimum Viable Attack Cycle (MVAC)
- [X] Step 1: Define ReconCompletedEvent (Domain).
- [X] Step 2: Build NmapRunner (Infrastructure).
- [X] Step 3: Wire Runner into Worker Handler.
- [X] Step 4: Append ReconCompletedEvent to EventStore.
- [X] Step 5: Verify data persistence in Postgres via curl trigger.

Phase 4.2: The AI Feedback Loop
- [X] Step 1: Create a background subscriber that listens for new Domain Events (e.g., ReconCompleted).
- [X] Step 2: Pass Event context to AI Plane (DeepSeek/Ollama).
- [X] Step 3: Generate an AI Recommendation Event containing advice and confidence score.
- [X] Step 4: Save AI Recommendation Event to EventStore.

Phase 4.3: The Human-In-The-Loop (TUI Fix)
- [ ] Step 1: Fix Bubble Tea input freezing bug (implement non-blocking channels for log streams).
- [ ] Step 2: Create a new TUI view to display AI Recommendations.
- [ ] Step 3: Add [Y/n] keyboard listener to approve/reject AI suggestions.
- [ ] Step 4: Send Approval/Rejection Event back to Server/EventStore.

Phase 4.4: The Execution Trigger
- [ ] Step 1: Worker listens for Approval Events.
- [ ] Step 2: If approved, Worker triggers Exploit Execution (using a stub first, then real payload).
- [ ] Step 3: Append ExploitResultEvent to EventStore.

Phase 4.5: The Live Dashboard
- [ ] Dispatch ReconCompletedEvent from Worker back to Server via HTTP bridge.
- [ ] Propagate events to TUI dashboard for real-time operator awareness.

Milestone: When a user submits a target via curl, sees Nmap run, gets an AI tip in the TUI, presses Y, and an exploit stub runs.

---

## 6. Era 5: Arsenal Expansion & Real Offense (CURRENT ERA)

Goal: Replace all stubs with real, high-performance offensive tools.

Phase 5.1: Web Exploitation Engine
- [ ] Integrate chromedp for dynamic XSS/CSRF/SSRF testing.
- [ ] Build HTTP request forgery modules.
- [ ] Session handling and cookie stealing mechanisms.

Phase 5.2: Advanced Reconnaissance
- [ ] Subdomain enumeration integration (Subfinder, Amass).
- [ ] Directory brute-forcing (Feroxbuster).
- [ ] JavaScript endpoint parsing.
- [ ] Web scanner integration (Nikto, WhatWeb, Gobuster).
- [ ] Multi-target parallel scanning with worker pool.
- [ ] Scan scheduling and rate limiting.

Phase 5.3: Exploit Acquisition
- [ ] Auto-fetcher for public PoCs (ExploitDB API).
- [ ] Local N-Day exploit cache manager.
- [ ] Payload template engine (reverse shells, webshells).

Phase 5.4: Evasion & Stealth
- [ ] Dynamic payload obfuscation (WAF bypass).
- [ ] Living-off-the-Land (LoLBins) integration.
- [ ] Fileless execution techniques via reflect package.

---

## 7. Era 6: The Exoskeleton Intelligence (Advanced AI)

Goal: Make the system learn from personal operational history.

Phase 6.1: Cross-Target Intelligence
- [ ] Target fingerprinting DB (Tech stack patterns).
- [ ] Automatic matching: "Target B looks like Target A, use the same exploit chain."

Phase 6.2: Autonomous Scoping
- [ ] Passive intelligence ingestion (CVE RSS, Twitter feeds).
- [ ] Alerting user if a new CVE matches a previously scanned target.

Phase 6.3: Advanced RAG
- [ ] Automatic indexing of exploit outputs.
- [ ] Error-to-solution mapping (If Nmap crashes on X, try Y next time).

---

## 8. Era 7: Infrastructure Maturity (Technical Debt Resolution)

Goal: Upgrade the temporary bridges to enterprise-grade, resilient infrastructure.

Phase 7.1: Active Event Bus
- [ ] Replace HTTP bridge with persistent NATS JetStream.
- [ ] Implement exactly-once delivery for critical events.

Phase 7.2: Service Mesh
- [ ] Implement gRPC contracts between all planes.
- [ ] Add Anti-Corruption Layers (ACL) to translate data between planes.

Phase 7.3: Resilience Patterns
- [ ] Integrate Circuit Breaker into all external calls (LLM, Ollama, external APIs).
- [ ] Implement Retry with Exponential Backoff for DB queries.
- [ ] Bulkhead isolation for Execution Plane (prevent runaway scans from crashing Control Plane).

Phase 7.4: Fix Pre-Existing Build Errors
- [ ] Fix compilation errors in saga, ai, messaging/nats, crypto, network packages.
- [ ] Ensure `go build ./...` passes cleanly (full project build).

---

## 9. Era 8: Reporting & Tradecraft

Goal: Automate the paperwork of offensive security.

Phase 8.1: Kill Chain Reporter
- [ ] Event-to-Markdown parser.
- [ ] Auto-generation of Executive Summaries.
- [ ] Technical PoC generation with curl/python snippets.

Phase 8.2: OPSEC & Cleanup
- [ ] Automated log cleansing commands for target servers.
- [ ] Ophidian self-destruct mode (wipe local Vector DB, Event Store, and caches).

---

## 10. Zen Mode Rules (Post-Era 5)

Once Era 4 and Era 5 are sufficiently stable, enter permanent Zen Mode.

Rules:
1. No new Eras or Phases will be created.
2. Code changes only occur due to one of three triggers:
   a. A bug encountered during real operational use.
   b. A performance bottleneck found via benchmarks.
   c. A dependency update breaking the build.
3. All changes must still pass Architecture Governance linters.
4. The system is now a weapon. Maintain it, do not rebuild it.
