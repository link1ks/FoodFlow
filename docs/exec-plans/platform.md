# Harness and event-driven service migration

Date: 2026-09-30. Scope: executable engineering harness, Redis catalog cache, Kafka stock events, independently deployed kitchen statistics service.

## Acceptance

1. Harness checks real package dependencies, formatting and tests, and emits machine-readable evidence.
2. Redis serves authenticated public catalog reads; failures fall back to PostgreSQL within bounded time.
3. Inventory commit atomically creates its outbox event; rollback creates neither; Kafka outage does not block stock writes.
4. Stable event IDs, consumer inbox and transaction protect statistics against repeated delivery. Offset commits occur after local database commit.
5. Insights owns a separate database. User requests pass existing household authorization in API; internal service requires a separate secret.
6. The UI displays statistics, unit boundaries and projection freshness. Existing kitchen flows continue to work without the new deployment.

## Decisions

- Keep inventory and procurement together because they share strong transactions. Extract a read projection first; this is incremental microservice adoption.
- Redis is an optional public-read cache, not a source of inventory truth or a replacement for database idempotency.
- Keep PostgreSQL model jobs and leases. Kafka transports committed stock facts, not billable model work.
- Single-node Kafka and Redis Compose are local acceptance configurations, not highly available production topology.

## Evidence

Implementation and final local verification completed on 2026-09-30. No external model calls were made as part of this migration.

- Initial full harness passed: AST architecture, format, vet, required PostgreSQL integration, 8 frontend tests and build. Go tests took 65.211 seconds.
- Real Compose acceptance passed Redis hit/auth/fallback, broker outage stock write, independent consumer restart, repeated event delivery and household isolation.
- Existing registration → planning → procurement → consumption smoke passed against the extended stack.
- Browser verified 100g inbound, 10g consumed and 5g wasted for the isolated fixture, with no unnecessary decimal zeroes.
- Current deployment is local single-node middleware and one extracted projection service. Production HA, TLS/SASL, sustained-load performance and fully distributed kitchen writes are not claimed.
- Final full harness passed after canonical event hashing: semantically identical JSON replays deduplicate, conflicting event content is rejected. JSON evidence is summarized in `docs/validation/platform-harness.json`.
- Final real Kafka acceptance also passed malformed-event quarantine. Evidence: `docs/validation/platform-evidence.json`.
- sqlc v1.30.0 generated the outbox model successfully. The CI workflow now includes architecture/format rules and a real platform integration job; this new workflow has not yet run on GitHub for the current working-tree changes.

## Login regression correction

The existing web container retained the old API IP after API replacement, returning HTML 502 pages to login. Nginx now resolves Docker service DNS dynamically. Frontend reports gateway failures as service unavailability and preserves valid credential errors. `tests/gateway-recovery.ps1` verifies API replacement without restarting the web gateway; 3 frontend regression cases cover HTML failure, JSON credential error and successful JSON.

The original account database is served by development `http://127.0.0.1:5173/` (API 8080); `15173` is an isolated acceptance database. Original API and Vite processes were restored; no account or password migration was performed.

Login repair verification passed: a valid existing fixture account logged in through 5173; replacing the acceptance API with a different container IP preserved gateway operation without restarting web. The complete harness passed architecture, formatting, vet, Go integration tests, all 11 frontend tests and production build. Local evidence: `.cache/harness/login-fix.json`.
