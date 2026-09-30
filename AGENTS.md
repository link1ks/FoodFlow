# FoodFlow repository map

Read `docs/HARNESS.md` for verification commands and `docs/PLATFORM.md` for service ownership.

- Product and startup: `README.md`.
- Business invariants: `ARCHITECTURE.md`.
- Current delivery plan: `docs/exec-plans/platform.md`.
- Generated contract/configuration inventory: `docs/generated/contracts.md`; refresh with harness knowledge mode when source facts change.
- Harness hardening plan: `docs/exec-plans/harness-hardening.md`.
- API contract: `openapi.yaml`; event contract: `internal/events/stock.go`.
- Pure algorithms: `internal/engine`; never import database, HTTP or messaging clients there.
- Transactional kitchen writes: `internal/app`; keep inventory, immutable ledger and outbox in one PostgreSQL transaction.
- Public read cache: `internal/cache`; do not cache household permissions or authoritative inventory.
- Event relay: `internal/outbox`; publish committed events with stable IDs; delivery can duplicate.
- Statistics service: `internal/insights`; owns its database and inbox; never query kitchen tables.
- UI: `web/src/pages`, shared application state in `web/src/app`.

Before declaring a change complete, run the harness and relevant integration scenarios. Write actual evidence and limitations into the execution plan. Use deterministic model fixtures by default; real model calls need explicit task authorization because they can incur fees.

Record bounded task start/outcome with harness task mode, including failed/blocked outcomes. Use diagnose mode for redacted acceptance-stack feedback and acceptance mode for browser/platform scenarios. Do not claim productivity improvement from a single successful run. Keep generated knowledge current and convert reproduced failures into regression checks.

Never include `.env`, local database volumes, credentials or raw authenticated logs in commits. Do not delete development data for tests. Preserve request idempotency, server-side household authorization, precise quantities and explicit user confirmation. Architecture checks enforce these import boundaries in CI.
