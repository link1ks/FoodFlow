# Executable engineering harness

The [engineering feedback workflow](ENGINEERING_FEEDBACK.md) adds live
evaluation, local engineering RAG and history metrics. New task contracts must
declare feedback applicability; completion bundles include a `feedback` report.
Use `-task <id>` on verification modes to retain task/run association.

The harness supplies a repository map, explicit architecture constraints, isolated runtime verification and machine-readable evidence. It does not require changing the application's model provider.

```powershell
go run ./cmd/harness -mode architecture
go run ./cmd/harness -mode fast
go run ./cmd/harness -mode full
go run ./cmd/harness -mode knowledge
go run ./cmd/harness -mode acceptance
go run ./cmd/harness -mode diagnose -out .cache/harness/diagnose-run.json
go run ./cmd/harness -mode summary -out .cache/harness/summary.json
go run ./cmd/harness -mode business
go run ./cmd/harness -mode main-site -out .cache/harness/task-main.json
go run ./cmd/harness -mode cloud -out .cache/harness/task-cloud.json
```

Full mode checks AST import rules, Go formatting, vet, all Go tests with required Docker integration, pinned sqlc generation consistency, generated frontend API type freshness, frontend tests and build. Fast mode explicitly uses `-short`; it cannot establish database or messaging acceptance. Results and bounded subprocess output are written to `.cache/harness/latest.json`. The harness clears production test-database and model/cache/service environment overrides from test subprocesses, uses fixed argument vectors and exits nonzero on failed checks.

Agent tests require Python 3.13. `TestPythonAgentContracts` runs the offline
Python unittest suite; its named Go result is the harness binding, and a failed
Python assertion fails that Go test. Model adapter/integration tests execute the
actual embedded Python runtime against local fixtures. Python sources under
`internal/agent/python` participate in source digests and the reviewed AI source
scope. `-B` prevents test imports from creating bytecode in the source tree.

Full and fast also run `update-regression` (`pwsh -File tests/update-project.ps1`). This uses disposable local Git repositories to verify check-only/fast-forward updates, preservation of configuration/photos, refusal of local changes/divergence/unfinished operations, locking and failed fetches. Docker restart arguments and retry behavior are mocked; this check does not establish real container rebuild or migration compatibility. On Windows, `powershell -NoProfile -ExecutionPolicy Bypass -File tests/update-project.ps1` additionally verifies the one-click launcher's PowerShell 5.1 compatibility.

The named Go tests `TestPythonNutritionImportContracts` and
`TestPythonEvaluationAnalysisContracts` run offline data-tool regressions through
the project Python interpreter. They bind nutrition precision/provenance and
evaluation denominator/comparison rules to full verification and CI.
[Python data tools](PYTHON_DATA_TOOLS.md) documents the hands-on workflow.

## Knowledge freshness

Full, fast and acceptance also run `quality-map` against [business-domain quality](QUALITY.md) and its [machine-readable scenarios/debts](quality/domains.json). Missing domains, owners, scenario references or incomplete debts fail. Full task evidence must include this check. OpenAPI is validated with a standard parser in Go tests, compared against actual API methods/routes, and checked against real main-flow fixture requests/responses. Catalogue photos and the quality map participate in the source digest. These checks do not establish semantic completeness of every API or real model quality.

[Source inventory](generated/contracts.md) is generated from Go route registrations (including group prefixes), direct dependencies, frontend dependencies, configuration key names, Compose images/services, migration filenames/content digests and event Go types/JSON fields. `knowledge` fails when source changes without updating the inventory; it also checks local Markdown links in repository documentation. Refresh with `go run ./cmd/harness -mode knowledge -update` and review the diff. This mechanically checks selected facts, not the semantic truth of every prose statement or OpenAPI schema completeness. Architectural ownership still requires review.

## Runtime feedback

Install browser support once: `pnpm --dir web exec playwright install chromium` (CI additionally installs OS dependencies). `acceptance` builds the isolated stack including required Redis, verifies Redis hit/readiness failure/live health/database fallback/recovery, Kafka recovery, deduplication, kitchen flow, gateway replacement, and desktop/mobile Chromium behavior, then collects diagnostics. Build mode labels and records immutable API/web image IDs with the current source digest. `-skip-build` requires this manifest and matching running base containers before business checks; platform startup then verifies relay/insights too. `go run ./cmd/harness -mode images -platform-images` exports only IDs and allowlisted labels. Missing/old/foreign bindings fail; rebuild instead of reusing unbound images. Local labels and JSON are not tamper-proof attestations. Browser tests use fresh fictional accounts on 15173; they never access a user's browser session or 5173 database. Before browser verification, `prepare-browser-fixture.ps1` checks the fixed acceptance database's Compose project/service labels and expires only its synthetic authentication budget window. Repeated runs then exercise normal application counters without disabling production limits or deleting any account, inventory, session or volume. Direct repeated `pnpm test:e2e` runs should use the same preparation when the isolated proxy budget is exhausted. Screenshots and JSON results are under `.cache/harness`; recording raw network traces is disabled.

`diagnose` collects read-only health probes, API counters, pending outbox age, job status/expired lease counts, projection/quarantine totals, per-partition committed Kafka lag, and allowlisted structured log fields. It exports no environment values, headers, payloads or household identifiers. A failed probe produces a named failed check and a nonzero exit. It is a point-in-time local snapshot, not production alerting or distributed tracing. If Chromium download is unavailable on Windows, an installed Edge can be used with `PLAYWRIGHT_CHANNEL=msedge`; record that environment difference. CI uses bundled Chromium.

## Delivery history and measurement

Every verification writes a separate run file under `.cache/harness/runs`, with timestamp, Git revision, worktree status, source digest, results and durations. A final digest check rejects edits made while verification was running. These are local development records, not tamper-proof attestations. GitHub CI attaches reports to its exact commit and uploads only explicitly selected artifacts.

For each bounded task, record its real start, bind a reviewed requirement contract before final verification, and record the final outcome:

```powershell
go run ./cmd/harness -mode task -task example-repair -action start -kind regression -baseline reproduced
# Prepare a reviewed query JSON as described in ENGINEERING_FEEDBACK.md.
go run ./cmd/harness -mode experience -action index
go run ./cmd/harness -mode experience -action query -evidence .cache/harness/example-query.json -out .cache/harness/example-retrieval.json
go run ./cmd/harness -mode task -task example-repair -action bind -contract docs/quality/tasks/example-repair.json
go run ./cmd/harness -mode full -task example-repair -out .cache/harness/example-full.json
# Also run acceptance, diagnose, main-site and (after publication/CI) cloud with -task.
# Complete applicable live/device/RAG evidence, resolve failures, then recompute metrics.
go run ./cmd/harness -mode experience -action metrics -out .cache/harness/example-metrics.json
go run ./cmd/harness -mode task -task example-repair -action finish -outcome passed -evidence .cache/harness/example-completion.json
go run ./cmd/harness -mode summary -out .cache/harness/summary.json
```

Use `feature`, `regression` or `maintenance` as the task kind; `baseline` is an operator assertion (`reproduced` or `unknown`). Failed/blocked tasks must be recorded too. Starts, bound contracts and outcomes cannot be overwritten through the command. Successful completion requires a version-2 bundle with `full`, `acceptance`, `diagnose`, `main_site` and `cloud` paths under `.cache/harness`. Every report must start after the contract is bound, match current source and include its required passing checks. Full-only, skipped, missing, stale, failed, duplicate and foreign-source evidence is rejected. Cloud must verify clean HEAD published on origin/main with successful backend/frontend/images/platform jobs. This is local evidence validation, not cryptographic attestation.

Example bundle: `{"version":2,"full":".cache/harness/example-full.json","acceptance":".cache/harness/example-acceptance.json","diagnose":".cache/harness/example-diagnose.json","main_site":".cache/harness/example-main.json","cloud":".cache/harness/example-cloud.json","feedback":".cache/harness/example-feedback.json"}`. A task contract has version 1, `task`, `criteria` and explicit `feedback` scope; each criterion has a unique `id`, user-facing `description`, `business` requirement IDs and exact named `tests`. See the committed [current contract](quality/tasks/harness-v03.json) and [feedback schemas/workflow](ENGINEERING_FEEDBACK.md). Inapplicable checks need reasons; required missing device/model evidence cannot be waived by a successful build.

## Requirement and business feedback

[Business guide](BUSINESS_GUIDE.md) and [explicit mappings](quality/business.json) retain reviewed behavior, implementation paths, bounded data profiles and browser rechecks. `business-map` runs in full/fast/acceptance. A changed guide or source digest requires semantic review followed by `-mode business -update`; refresh does not infer correctness or invent test mappings. Implementation files under internal/cmd/web/src/sql must belong to reviewed source scopes; newly added files cannot silently evade review. New named Go tests/fuzz entrypoints, frontend tests, browser scenarios and executable PowerShell test scripts must have explicit bindings. Table rows and subtests inherit their named parent. Unsupported modified JavaScript registrations fail inventory discovery until an adapter is added.

Full captures only named Go/Vitest statuses in its evidence; acceptance captures each desktop/mobile Playwright result. Missing, skipped and retried/flaky results cannot satisfy named coverage. Script-level tests are bound to business purpose; their execution is represented by harness checks and the platform CI job, not fabricated per-assertion statuses. Task criteria must name directly recorded Go/frontend/browser cases.

`data-scale` requires the fresh full-run workload: 10 households, 20 clients, 30 seconds, at least 100 completed cycles/700 requests, zero errors/invariant failures and p95 <= 2000 ms. Requirements additionally describe relevant empty/boundary/negative/replay fixtures; volume alone is not a quality measure. Full/fast/acceptance take an exclusive `.cache/harness/verification.lock`; overlapping runs fail before starting. After an interrupted process, inspect its recorded PID and remove only the stale lock after confirming no verification is running. Never clear a live lock to force parallel execution.

`main-site` opens a fresh browser on localhost:5173, permits only same-origin GET/HEAD, verifies login controls, readiness, HTML and JS/CSS hashes against web/dist, and saves an unauthenticated screenshot. It does not sign into private accounts, mutate stock or make paid calls. Actual business interactions are rechecked in the linked isolated acceptance pages. Main-site success establishes frontend build identity and entry availability, not every backend behavior or a completed deployment backup; application updates still require the documented backup/rebuild procedure.

Summary separates verification runs by mode from explicitly completed tasks. Durations include waiting and interruptions. Samples start when this mechanism is adopted; historic runs are not backfilled as fictional task records. Pass counts and task wall-clock time do not establish AI productivity gains or user regression rate. Compare later tasks only with stated scope, environment and baseline. Failures should produce a reproducible case and a regression rule/test, as the login gateway repair did.

## Cloud evidence

CI runs on pushes to `main`, all pull requests, and explicit manual dispatch. Development-branch pushes do not start another suite alongside their PR; open/update the PR or dispatch manually to verify a branch. PR opened/reopened/synchronize events retain GitHub's defaults. Concurrency is scoped by workflow and PR number (falling back to branch ref), and newer revisions cancel older runs in the same group without cancelling another PR. Main's post-integration verification is retained. `TestCIEventPolicyAvoidsDuplicateBranchRuns` checks the actual YAML event matrix against the duplicate branch-push regression. See [GitHub workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax).

The platform CI job runs the full harness and acceptance harness on the checked-out commit, then uploads an explicit artifact allowlist for 14 days. `pwsh -File scripts/ci-status.ps1 -Revision <full-sha>` or `-RunId <id>` reads workflow/job outcomes with existing Git Credential Manager authorization. Credentials remain in memory and are never included in reports. A local green run does not substitute for a completed cloud run.

CI additionally runs race detection, sqlc generation consistency, container builds and the event pipeline acceptance test. Docker unavailability must fail required checks. Real model calls are not included; existing deterministic fixtures verify confirmation, cancellation and schema constraints without charges.

The platform job also runs isolated learning security and [paired database/photo recovery](BACKUP_RECOVERY.md) with fictional accounts. Recovery checks password login, precise stock, photo bytes, restored request replay, new writes and corrupt-artifact rejection before target creation. Only its redacted summary is uploaded; private backups/configurations and raw authenticated logs are excluded. These checks require the learning stack separately; the ordinary full harness does not imply recovery acceptance.

Full Go integration additionally checks offline recovery-code rotation/expiry/credential epochs, simultaneous one-time reset, session revocation and persistent authentication limits. Paid-call fixtures use local HTTP servers: exact PostgreSQL allowance debits, shared monthly/daily/concurrency decisions, restart/old-month ambiguity, same-job fencing and pre-dispatch rejection. Browser acceptance exercises recovery-code generation and email-account reset on desktop/mobile. This proves application allowance limits, not provider invoice totals; see [learning account/AI scope](ACCOUNT_RECOVERY_AI.md).

## Agent workflow

Failed or blocked task outcomes can add `-failure-category product|fixture|infrastructure|unknown`; summary reports category counts. Existing uncategorized records remain unknown. See [quality maintenance](QUALITY.md) for bounded fuzz commands and remaining business-domain debt.

1. Read `AGENTS.md`, relevant contracts and current execution plan.
2. Reproduce the business failure in an isolated fixture.
3. Make a bounded change within the documented service owner.
4. Run the relevant scenarios and full harness before claiming completion.
5. Record commands, results, known limits and service recovery evidence in the plan.

Architecture rules forbid I/O dependencies in pure algorithms and forbid kitchen-table imports in the independent projection. A new rule needs a test proving that a violating import is rejected. Tests check business invariants; implementation-shaped snapshots are insufficient.

Architecture also rejects passing a handler's pooled `*gin.Context` directly to object-store Put/Open/Delete. Storage HTTP transports can inspect cancellation after a handler returns, when Gin may already reuse that wrapper. Pass `c.Request.Context()` or a derived standard context instead. This is a direct-argument AST guard, not complete context escape analysis; the S3 voice integration remains covered by race detection.

Read [platform ownership](PLATFORM.md) and [execution plan](exec-plans/platform.md). Harness engineering here means this development feedback environment; it is not a claim that all future development runs autonomously or that model output is inherently correct.

## Generated contracts

Every verification mode also checks the Git-visible file inventory (tracked and untracked, excluding ignored local files). Forced additions of environment configurations, private/cache/dependency directories, backups, credentials and generated browser/build output fail `repository-hygiene`. The check never reads their contents or reports private filenames. `.env.example`, regression seeds, licensed assets and redacted evidence remain valid delivery files. This path policy is not a secret-content scanner or a substitute for reviewing the staged diff. Contribution and security documentation links are checked with the other repository documents.

`pnpm --dir web api:generate` refreshes `web/src/generated/api.d.ts` using pinned [openapi-typescript CLI](https://openapi-ts.dev/cli); `api:check` rejects stale types without rewriting them. Inventory, recipes, account, nutrition and monthly-report UI use these types. All successful API responses declare JSON/binary/SSE media; common isolated API fixtures validate actual responses with kin-openapi, including expected errors. Model job payload/result remain explicitly open JSON objects; schemas alone do not validate every model-specific payload.

Full mode runs `scripts/check-dbgen.ps1`: regenerate with sqlc v1.30.0 in a bounded scratch copy of only SQL/config files, compare the complete generated file set/content with normalized line endings, then remove only its checked scratch directory. It does not open databases or rewrite working source. This gate also works before committing deliberate generated changes.

The full Go suite includes the bounded [family workload](FAMILY_TRIAL.md); its redacted metrics are `.cache/harness/family-load.json`. List and object API fixture responses both use the standard contract validator. Real model calls remain disabled by default.

Full additionally runs `release-regression` against private/traversal/duplicate source archives, corruption, missing or stale verification, and schema/history/topology/image drift. The platform CI prepares a bound learning release and performs a same-version API/Worker/Web recreation with a paired pre-switch backup. It checks retained database identity, immutable business fingerprints, photos and security, and uploads only `.cache/harness/learning-release.json`. [Learning release procedure](LEARNING_RELEASE.md) distinguishes this rehearsal from older-version compatibility and public deployment.
