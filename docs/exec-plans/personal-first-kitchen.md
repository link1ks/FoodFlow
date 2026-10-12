# Personal-first kitchen

Date: 2026-10-11. Owner: kitchen API and web. Task: `personal-first-kitchen`.

## Scope

Login opens a private personal kitchen (default servings 1) without family onboarding. Personal center offers optional family create/join/switch. Migration 034 leaves existing households as `family` and adds a unique per-user default private-kitchen mapping. Explicit PUT provisions under an account lock. Personal membership also requires ownership; inviting, accepting private invites and changing members are rejected server-side. Existing family stock is not copied or moved.

Credential-verified account merge retains private histories under the merged owner, preserving the target default when present, otherwise transferring the source default. Extra personal kitchens remain selectable. UI ignores legacy family selection once, remembers explicit choices on refresh and defaults to personal at new login. Switching remounts space-dependent UI; menu defaults follow kitchen servings. Community identity and points stay per account.

## Evidence

Frontend build passed. Initial PostgreSQL tests passed concurrent provisioning/replay, private access denial including erroneous membership, private invite rejection, forged owner input, exact legacy stock preservation and both credential-verified merge mapping cases.

Focused PostgreSQL re-run passed in 14.918s after worker permission and expiry-reminder isolation checks. Private stock reminders reach only its owner. Focused browser run passed 2 desktop/mobile cases in 7.4s; an initial fixture selector matched both a batch and its stock summary, corrected to the first matching summary before re-running. Existing family scenarios now explicitly choose their prepared shared kitchen after login; they retain their family-stock assertions. New browser scenario also checks personal procurement and monthly-report labels.

Added browser coverage for individual entry, 1-person menus, private settings/nutrition, isolated stock, optional family create/join, viewer permissions, refresh and relogin. Final verification and deployment evidence follows. Deterministic fictional fixtures on 15173; installed Edge; no paid model calls or development-data deletion.

First final full run passed all 15 checks. Initial acceptance passed service checks but browser ended 32/34: the two last mobile fixture registrations returned 429 after the new scenario pushed the shared gateway's normal 60-attempt window over budget. Personal scenario account provisioning now uses the fixed isolated API at `18080/api/register` (its separate peer budget), with every UI login still through 15173; no header spoofing, production limit changes or database/session deletion. A first direct-peer attempt omitted the `/api` prefix and failed its 200-status fixture assertion (404); corrected and focused 2-case browser re-run passed in 15.3s. A concurrently running full report rejected changed inputs at `source-stable`; its business checks passed but that report is not completion evidence.

Final frozen-source full passed all 15 checks: Go tests (163.835s), pinned sqlc consistency, generated API freshness, frontend tests/build, update/release regressions, architecture, hygiene, knowledge, quality and source stability. Run `7cb6e3ec2a640a3ba9d9be94f4af161d`, evidence `.cache/harness/personal-first-full.json`, same source digest as final acceptance below. Harness task `personal-first-kitchen` recorded `passed` against this final full report.

Final acceptance passed all 14 checks, including all 34 desktop/mobile browser cases with zero skips and no mid-suite clock resets. Run `17afab4bcfa175f2cb3be92bed8be41e`, source `e8e0007163222d6eb3d4b06acfbee99eafe2efdb016d5c70c9dccb7ba4915693`. Evidence: `.cache/harness/personal-first-acceptance.json`; `.cache/harness/browser.json`. Standard single pre-suite fixture clock preparation remains unchanged. Platform outage recovery, kitchen workflow, gateway replacement and redacted runtime diagnosis passed. API adds PUT to its preflight method allowlist.

Fictional isolated personal home/profile/spaces screenshots inspected at desktop 1366, tablet 810, mobile 390 and narrow 320px. Previews recorded no horizontal overflow or runtime script errors; full browser tests also check personal text contrast. Preview files: `.cache/harness/personal-{today,profile,spaces}-{desktop,tablet,mobile,narrow}.png`.

Development Compose updated with migration 034; original database and photo volumes retained. API ready 8080 and page 5173 return 200; live PUT preflight verified. Before/after account, household, membership, ingredient and batch counts, exact total stock and immutable stock-ledger fingerprint match. Aggregate evidence: `.cache/harness/personal-first-development-{before,after}.json`. Isolated acceptance services stopped afterward without removing volumes; development API/Worker/Web/PostgreSQL remain running.

Limitations: local installed Edge was used rather than CI bundled Chromium; cloud CI not run for this uncommitted worktree. Existing shared data is deliberately retained in optional families and is not assigned to one user's personal space. No paid model calls, personal account browser sessions, downgrade rehearsals or production deployment were performed.
