# Cross-computer source updates

Date: 2026-10-10. Scope: download FoodFlow into `E:\FoodFlow` and provide one-click updates after the initial clone on each computer.

## Decisions

- Windows launchers use Windows PowerShell 5.1 and Git. No extra application dependency is needed to update source.
- Follow the current branch's configured remote upstream and apply only a fast-forward to the fetched commit.
- Reject dirty/untracked source, local-only/divergent commits, unfinished Git operations, concurrent updates and incoming private configuration/data paths.
- Preserve ignored configuration, photos and Docker volumes. Code updates do not synchronize business data between independent computers.
- Docker rebuild is explicit with `-Restart` for the default Compose deployment. Custom/trial/learning deployments retain their existing startup/release procedures.

## Verification

- Cloned the requested GitHub repository into `E:\FoodFlow`; the original `main` checkout was clean before this task.
- `powershell -NoProfile -ExecutionPolicy Bypass -File tests/update-project.ps1` passed on Windows PowerShell 5.1. Covers check-only updates, successful fast-forward, repeat update, ignored configuration/photos, dirty/untracked files, concurrent lock, unfinished Git operations, detached/no-upstream branches, ahead/diverged history, failed fetch, private-path protection and mocked Compose rebuild/retry.
- The same update regression passed under PowerShell 7 through the harness. Regression testing caught Windows native-stderr handling and wrapped lock exceptions; both are handled in the final script.
- The real `scripts/update.ps1 -Action check` successfully reached GitHub and found local/remote `b1d1eb49` equal. The actual update entrypoint refused the current uncommitted task changes with exit code 1.
- Docker Desktop was started after the user authorized it. A disposable Compose/Git fixture using the existing `node:22-alpine` image passed real image rebuilding, container replacement, HTTP response change from v1 to v2, named-volume sentinel preservation and repeat restart. Evidence: `.cache/harness/update-docker.json`; the isolated fixture was stopped afterward. This does not prove FoodFlow database migration or older-release compatibility.
- `go run ./cmd/harness -mode fast -out .cache/harness/cross-computer-update-fast-final.json` passed every check, including the update regression, architecture, hygiene, knowledge, quality map, formatting, vet and short Go suite. Fast mode explicitly skips database integration.
- `go run ./cmd/harness -mode full -out .cache/harness/cross-computer-update-full-final.json` passed every check except `go-tests`: PostgreSQL 16 image pulls returned Docker Hub HTTP 401 / missing-image errors and the integration suite timed out. SQL generation, release checks, frontend API freshness, frontend tests/build and source stability passed. Earlier attempts also encountered transient Go dependency download EOF errors; those checks passed in the final full attempt.
- Manual preparation of `testcontainers/ryuk:0.13.0` succeeded. The separate `postgres:16-alpine` pull stalled and was cancelled. No database test failure has been presented as a successful acceptance result. The harness task outcome is recorded as infrastructure-blocked because passing full evidence is unavailable.
- `git diff --check` passed. Changes remain local and uncommitted; no GitHub push or publication occurred. Other computers can obtain the new launchers after these changes are committed and pushed.

## Remaining boundary

Rerun the full harness after Docker Hub can supply `postgres:16-alpine`. The code-update feature is verified, but full database acceptance remains blocked by image access. Code updates deliberately do not transfer household databases or photos between independent deployments.
