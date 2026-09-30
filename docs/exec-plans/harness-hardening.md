# Harness feedback hardening

## Scope

Requested on 2026-09-30: resolve knowledge freshness, fragmented runtime feedback, missing continuous evidence, and unverified current GitHub CI. Preserve development accounts and transactional business boundaries. This increment includes existing unpushed platform and login repairs when synchronizing the repository.

## Acceptance criteria

1. Selected documentation facts are generated from source and drift/broken links fail verification; tests demonstrate rejection.
2. A fixed-command acceptance run covers real backend flow, middleware recovery and desktop/mobile browser behavior.
3. Redacted diagnostics expose health, counters, queues, consumer lag and structured logs in one machine-readable report.
4. Verification history identifies code inputs; task completion rejects stale or incomplete reports, without inventing productivity or historic outcomes.
5. Full local harness and acceptance pass; exact pushed revision passes actual GitHub Actions. Cloud failures must be investigated and corrected before claiming success.

## Decisions and limits

- Use repository tooling and existing services; no additional production middleware.
- Generated inventory and local-link checks cover selected facts. Semantic documentation review remains necessary.
- Browser screenshots use isolated fictional users; network traces and videos are disabled.
- Local histories are editable files; CI commit-linked artifacts are stronger evidence, not signed attestations.
- Task measurement starts now; wall-clock elapsed time includes idle time and is not human labor or a causal AI performance metric.
- No unattended merge, production deployment or autonomous document modification is enabled.

## Evidence

- Knowledge regression tests reject changed route group prefixes, new configuration keys, broken/escaping links, stale/incomplete task evidence and outcome overwrite. Configuration values are excluded from generated documentation.
- First full run passed business/frontend checks but correctly failed the source-stability gate because a new script was added during verification. A subsequent stable full run passed all checks.
- First desktop browser run observed an empty asynchronous projection within the default 10-second assertion window; mobile and gateway cases passed. The projection assertion now explicitly waits up to 30 seconds across its 10-second UI refresh interval. All four desktop/mobile cases passed afterward, with screenshots and no runtime/API 5xx errors. Local browser used installed Edge while Chromium download proceeded; CI uses bundled Chromium.
- Unified acceptance passed real Redis fallback, Kafka recovery, consumer restart/deduplication/quarantine, household isolation, kitchen flow, changed-IP gateway recovery, browser tests and read-only diagnostics. All nine diagnosis probes succeeded; observed Kafka lag was zero for each of three partitions.
- Evidence is in `.cache/harness/hardening-full.json`, `.cache/harness/hardening-acceptance.json`, `.cache/harness/browser.json`, and `.cache/harness/diagnostics.json`. Final verification and exact GitHub CI revision/run will be recorded after synchronization.
