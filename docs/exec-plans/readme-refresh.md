# README refresh — 2026-09-30

## Scope

Update the project entry point to match the delivered API/Worker, optional Redis/Kafka event platform, independent insights database, and executable engineering harness. Remove duplicate usage/testing text and the obsolete validation date. Preserve the distinction between development and isolated acceptance accounts.

## Changes

- Add the main-branch CI badge, optional event architecture and service ownership.
- Document reproducible dependency/browser setup and full, acceptance and diagnose commands.
- Link existing commit-specific cloud evidence and deployment/recovery documentation.
- State actual boundaries: eventual consistency, quantity-only monthly projections, missing integrations and single-node platform deployment.

## Verification

On 2026-09-30, `git diff --check` passed; `go run ./cmd/harness -mode knowledge` passed architecture, generated inventory, local Markdown links and source stability. `go run ./cmd/harness -mode full -out .cache/harness/readme-full.json` passed all eight checks: architecture, knowledge, formatting, vet, required Go integration tests, frontend tests, frontend build and source stability.

The task start was recorded before editing. Its completion references the current-source full report through harness task mode. Runtime code was unchanged, so no additional local fault injection or paid model requests were performed. README links the previously verified cloud revision; the badge tracks subsequent main-branch runs. No production readiness or AI productivity gain is claimed.
