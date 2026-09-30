# Executable engineering harness

The harness supplies a repository map, explicit architecture constraints, isolated runtime verification and machine-readable evidence. It does not require changing the application's model provider.

```powershell
go run ./cmd/harness -mode architecture
go run ./cmd/harness -mode fast
go run ./cmd/harness -mode full
go run ./cmd/harness -mode knowledge
go run ./cmd/harness -mode acceptance
go run ./cmd/harness -mode diagnose -out .cache/harness/diagnose-run.json
go run ./cmd/harness -mode summary -out .cache/harness/summary.json
```

Full mode checks AST import rules, Go formatting, vet, all Go tests with required Docker integration, frontend tests and build. Fast mode explicitly uses `-short`; it cannot establish database or messaging acceptance. Results and bounded subprocess output are written to `.cache/harness/latest.json`. The harness clears production test-database and model/cache/service environment overrides from test subprocesses, uses fixed argument vectors and exits nonzero on failed checks.

## Knowledge freshness

[Source inventory](generated/contracts.md) is generated from Go route registrations (including group prefixes), direct dependencies, frontend dependencies, configuration key names, Compose images/services, migration filenames/content digests and event Go types/JSON fields. `knowledge` fails when source changes without updating the inventory; it also checks local Markdown links in repository documentation. Refresh with `go run ./cmd/harness -mode knowledge -update` and review the diff. This mechanically checks selected facts, not the semantic truth of every prose statement or OpenAPI schema completeness. Architectural ownership still requires review.

## Runtime feedback

Install browser support once: `pnpm --dir web exec playwright install chromium` (CI additionally installs OS dependencies). `acceptance` builds the isolated stack, verifies cache fallback, Kafka recovery, deduplication, kitchen flow, gateway replacement, and desktop/mobile Chromium behavior, then collects diagnostics. Use `-skip-build` only for existing current images. Browser tests use fresh fictional accounts on 15173; they never access a user's browser session or 5173 database. Screenshots and JSON results are under `.cache/harness`; recording raw network traces is disabled.

`diagnose` collects read-only health probes, API counters, pending outbox age, job status/expired lease counts, projection/quarantine totals, per-partition committed Kafka lag, and allowlisted structured log fields. It exports no environment values, headers, payloads or household identifiers. A failed probe produces a named failed check and a nonzero exit. It is a point-in-time local snapshot, not production alerting or distributed tracing. If Chromium download is unavailable on Windows, an installed Edge can be used with `PLAYWRIGHT_CHANNEL=msedge`; record that environment difference. CI uses bundled Chromium.

## Delivery history and measurement

Every verification writes a separate run file under `.cache/harness/runs`, with timestamp, Git revision, worktree status, source digest, results and durations. A final digest check rejects edits made while verification was running. These are local development records, not tamper-proof attestations. GitHub CI attaches reports to its exact commit and uploads only explicitly selected artifacts.

For each bounded task, record its real start and final outcome:

```powershell
go run ./cmd/harness -mode task -task example-repair -action start -kind regression -baseline reproduced
go run ./cmd/harness -mode full -out .cache/harness/example-full.json
go run ./cmd/harness -mode task -task example-repair -action finish -outcome passed -evidence .cache/harness/example-full.json
go run ./cmd/harness -mode summary -out .cache/harness/summary.json
```

Use `feature`, `regression` or `maintenance` as the task kind; `baseline` is an operator assertion (`reproduced` or `unknown`). Failed/blocked tasks must be recorded too. Starts and outcomes cannot be overwritten through the command. Successful completion requires a passing full report recorded after task start with the current source digest. Reports that skipped frontend or Docker checks cannot satisfy this gate. This does not automatically reproduce a bug or prove every acceptance criterion; run relevant acceptance scenarios as well.

Summary separates verification runs by mode from explicitly completed tasks. Durations include waiting and interruptions. Samples start when this mechanism is adopted; historic runs are not backfilled as fictional task records. Pass counts and task wall-clock time do not establish AI productivity gains or user regression rate. Compare later tasks only with stated scope, environment and baseline. Failures should produce a reproducible case and a regression rule/test, as the login gateway repair did.

## Cloud evidence

The platform CI job runs the full harness and acceptance harness on the checked-out commit, then uploads an explicit artifact allowlist for 14 days. `pwsh -File scripts/ci-status.ps1 -Revision <full-sha>` or `-RunId <id>` reads workflow/job outcomes with existing Git Credential Manager authorization. Credentials remain in memory and are never included in reports. A local green run does not substitute for a completed cloud run.

CI additionally runs race detection, sqlc generation consistency, container builds and the event pipeline acceptance test. Docker unavailability must fail required checks. Real model calls are not included; existing deterministic fixtures verify confirmation, cancellation and schema constraints without charges.

## Agent workflow

1. Read `AGENTS.md`, relevant contracts and current execution plan.
2. Reproduce the business failure in an isolated fixture.
3. Make a bounded change within the documented service owner.
4. Run the relevant scenarios and full harness before claiming completion.
5. Record commands, results, known limits and service recovery evidence in the plan.

Architecture rules forbid I/O dependencies in pure algorithms and forbid kitchen-table imports in the independent projection. A new rule needs a test proving that a violating import is rejected. Tests check business invariants; implementation-shaped snapshots are insufficient.

Read [platform ownership](PLATFORM.md) and [execution plan](exec-plans/platform.md). Harness engineering here means this development feedback environment; it is not a claim that all future development runs autonomously or that model output is inherently correct.
