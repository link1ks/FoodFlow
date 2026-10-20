# FoodFlow repository map

## Learning collaboration

The user delegates routine, repetitive commands to the agent. Explain significant
design choices and prepare concrete, reviewed changes first. Leave meaningful
CI/CD, publication, backup and deployment steps to the user with command-line and
web instructions for hands-on learning (clarified 2026-10-20). Do not execute these
learning steps on their behalf unless they delegate them again. Inspect actual
results before claiming verification, deployment or publication succeeded.

Read `docs/HARNESS.md` for verification commands and `docs/PLATFORM.md` for service ownership.

- Product and startup: `README.md`.
- Business invariants: `docs/ARCHITECTURE.md`.
- Current business context and update procedure: `docs/BUSINESS_GUIDE.md`; requirement/test/browser/data bindings: `docs/quality/business.json`. Review affected context after source changes; never refresh review digests without semantic review.
- Current delivery plan: `docs/exec-plans/platform.md`.
- Generated contract/configuration inventory: `docs/generated/contracts.md`; refresh with harness knowledge mode when source facts change.
- Harness hardening plan: `docs/exec-plans/harness-hardening.md`.
- Business quality and technical debt: `docs/QUALITY.md`, machine-checked `docs/quality/domains.json`.
- API contract: `api/openapi.yaml`; event contract: `internal/events/stock.go`.
- Pure algorithms: `internal/engine`; never import database, HTTP or messaging clients there.
- Transactional kitchen writes: `internal/app`; keep inventory, immutable ledger and outbox in one PostgreSQL transaction.
- Public read cache: `internal/cache`; do not cache household permissions or authoritative inventory.
- Event relay: `internal/outbox`; publish committed events with stable IDs; delivery can duplicate.
- Statistics service: `internal/insights`; owns its database and inbox; never query kitchen tables.
- UI: `web/src/pages`, shared application state in `web/src/app`.

Before declaring a change complete, run the harness and relevant integration scenarios. Write actual evidence and limitations into the execution plan. Use deterministic model fixtures by default; real model calls need explicit task authorization because they can incur fees.

Bind each user requirement to named tests in an immutable task contract before final verification (`task -action bind -contract ...`). New tests require explicit business bindings. Full verification must record named results and the bounded 10-household/20-client/30-second workload; keep boundary/error/idempotency cases. Recheck linked business pages in isolated desktop/mobile browsers, then open the default main page read-only in a fresh browser and verify served build identity. Never run synthetic business writes or paid model probes against the user's default database.

Successful task finish requires one version-2 completion bundle containing fresh source-matching full, acceptance, diagnose, main-site and current published-commit cloud reports. No full-only completion. Run memory-heavy full and acceptance sequentially; retain failed evidence and fix reproduced bugs with regressions before rerunning.

Record bounded task start/outcome with harness task mode, including failed/blocked outcomes. Use diagnose mode for redacted acceptance-stack feedback and acceptance mode for browser/platform scenarios. Do not claim productivity improvement from a single successful run. Keep generated knowledge current and convert reproduced failures into regression checks.

Never include `.env`, local database volumes, credentials or raw authenticated logs in commits. Do not delete development data for tests. Preserve request idempotency, server-side household authorization, precise quantities and explicit user confirmation. Architecture checks enforce these import boundaries in CI.

After each task, synchronize important verified changes and findings to the existing GitHub remote. Promptly remove obsolete task files, while retaining required evidence, credentials, backups and development data locally; never include private artifacts in publication.

Use `main` directly unless the user explicitly requests another branch. After delivering an application update, rebuild and start the existing default Docker deployment, preserve its configuration and data volumes, and verify `http://localhost:5173` serves the new build. Back up the default database and photo volume before recreation; isolated acceptance success alone does not establish that the main site was updated.
