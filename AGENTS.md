# FoodFlow repository map

## Delivery collaboration

The user delegates complete delivery to the agent, including routine commands,
CI/CD verification, GitHub publication, backups and default deployment updates
(clarified 2026-10-22, replacing the earlier hands-on learning handoff). Explain
significant design choices and prepare concrete, reviewed changes, then execute
the required delivery steps without handing them back by default. Preserve
configuration, credentials and development data; back up the database and photos
before recreating the default deployment. Inspect actual results before claiming
verification, deployment or publication succeeded. Paid model calls still require
explicit task authorization.

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

## Verification proportional to the change

User clarification on 2026-10-26: do not repeat long tests or CI/CD acceptance for
small follow-up changes shortly after a successful run. Compare the actual diff
with the most recent verified commit and record that baseline. Recency alone does
not establish safety; inspect whether the change affects the previously tested
behavior. This policy takes precedence over unconditional full-verification
requirements in older harness, business-guide and execution-plan documentation.

For documentation, agent instructions, and other bounded changes that do not
change long-test scenarios or the behavior those scenarios cover, run only the
relevant short tests and static/documentation checks. Use targeted named tests
with `-short`, or harness `-mode fast` when its broader short suite is warranted.
Do not rerun full, the bounded load, browser/platform acceptance, diagnose,
main-site or cloud gates merely because the source digest or commit changed.
Do not rebuild/redeploy the application for documentation-only changes.

Escalate only when the diff changes business behavior, permissions, persistence,
schema/migrations, queues/idempotency, model dispatch/accounting, dependency or
deployment behavior, the long-test suite itself, or when a reproduced failure or
explicit user request requires it. Run the affected integration/long scenarios;
use full delivery acceptance when the change crosses service/deployment boundaries
or leaves material coverage uncertain. Keep full and acceptance sequential.

For lightweight follow-ups to a recent green CI commit, publication does not
require another full CI run. A documentation/instructions-only push may use
GitHub's documented `[skip ci]` commit marker; record why it is applicable. Do not
disable workflows globally or use skipped CI as evidence that the new commit
passed. If a short test fails, diagnose it before broadening verification.

Write the changed scope, baseline commit/run, exact short checks and actual
results, skipped long checks and reason into the execution plan. Preserve prior
and interrupted evidence without relabeling it as a fresh pass. A lightweight
task can finish with this reviewed record and its short-test evidence; it does
not require a version-2 full completion bundle. If an existing immutable harness
contract was bound under the old policy, retain it and document the user's scope
change separately; do not forge a full-gate success or broaden implementation
just to satisfy the old completion validator.

Use deterministic model fixtures by default; real model calls need explicit task
authorization because they can incur fees.

For full-delivery tasks, bind each user requirement to named tests in an immutable task contract before final verification (`task -action bind -contract ...`). New tests require explicit business bindings. Full verification must record named results and the bounded 10-household/20-client/30-second workload; keep boundary/error/idempotency cases. When full acceptance is required, recheck linked business pages in isolated desktop/mobile browsers, then open the default main page read-only in a fresh browser and verify served build identity. Never run synthetic business writes or paid model probes against the user's default database.

Follow `docs/ENGINEERING_FEEDBACK.md`: query engineering experience before binding
full-delivery tasks, explicitly declare feedback applicability and thresholds, and pass
`-task <id>` on verification commands. Record failures with reviewed redaction,
classification and evidence; only resolved, regression-verified experience may
recommend a fix. Keep error-code/applicable-version filters explicit. Real
model evaluation requires task authorization and exclusive journals; report
unknown costs honestly. Recompute metrics from complete retained history after
full-delivery verification, including RAG history. Lightweight documentation tasks
do not require another RAG evaluation or complete-history metrics recomputation.
Required pending device acceptance blocks
overall completion; a disclosed prerelease does not override this gate.

When full delivery acceptance is required, successful harness task finish requires one version-2 completion bundle containing fresh source-matching full, acceptance, diagnose, main-site, current published-commit cloud reports and the task's feedback report. No full-only completion for that scope. Retain failed evidence and fix reproduced bugs with regressions before rerunning. Lightweight tasks use the reviewed short-verification record above.

Record full-delivery task start/outcome with harness task mode, including failed/blocked outcomes; record lightweight task scope and outcome in the execution plan with retained short-test evidence. Use diagnose mode for redacted acceptance-stack feedback and acceptance mode for browser/platform scenarios only when applicable. Do not claim productivity improvement from a single successful run. Keep generated knowledge current and convert reproduced failures into regression checks.

Never include `.env`, local database volumes, credentials or raw authenticated logs in commits. Do not delete development data for tests. Preserve request idempotency, server-side household authorization, precise quantities and explicit user confirmation. Architecture checks enforce these import boundaries in CI.

After each task, synchronize important verified changes and findings to the existing GitHub remote. Promptly remove obsolete task files, while retaining required evidence, credentials, backups and development data locally; never include private artifacts in publication.

Use `main` directly unless the user explicitly requests another branch. After delivering an application update, rebuild and start the existing default Docker deployment, preserve its configuration and data volumes, and verify `http://localhost:5173` serves the new build. Back up the default database and photo volume before recreation; isolated acceptance success alone does not establish that the main site was updated.
