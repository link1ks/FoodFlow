# FoodFlow v0.2.0 — 2026-10-01

## Authorized scope

Synchronize the completed kitchen experience revision and its evidence to the existing GitHub repository. Advance the existing 0.1.0 frontend/API metadata to 0.2.0, add a changelog, expose the frontend version in the profile, and create an annotated version tag. The user explicitly authorized GitHub synchronization and versioning. No production deployment or paid model calls.

## Repository and version decision

Origin is `https://github.com/link1ks/FoodFlow.git`, branch `main`. Initial fetch found no remote divergence and no existing version tags. v0.2.0 is a feature increment within the current pre-1.0 application; it does not assert production readiness. Version metadata remains in `web/package.json` and `openapi.yaml`; the profile imports the package version to avoid an independent display constant.

## Verification and synchronization

Task start was recorded before version edits. Current-source full harness passed all eight checks, including required database integration, frontend tests and production build. Isolated acceptance passed platform recovery, kitchen flow, gateway replacement, six desktop/mobile Chromium cases, diagnostics and source stability. Full and acceptance source digests match; profile version display is checked in both browser projects. [Local packaging evidence](../validation/release-0.2.0.json).

The initial version-display browser test imported package JSON directly, which Node's ESM loader rejected because it requires a JSON import attribute. The fixture now reads and parses the package file through Node's filesystem API. Test discovery and subsequent complete verification passed. Application JSON imports are handled by Vite and production builds passed throughout; this was a test-runtime import correction. Final acceptance reused the already-built v0.2.0 images after this test-only edit.

Only application code, contracts, documentation and selected redacted evidence are included in publication. Caches, credentials, environment files and data volumes are excluded. The annotated `v0.2.0` tag identifies the published commit. Exact remote references are verified after push, and cloud results are inspected through `scripts/ci-status.ps1`; the [GitHub release](https://github.com/link1ks/FoodFlow/releases/tag/v0.2.0) records observed commit-specific CI outcomes separately. This checked-in document records local packaging validation, without treating it as proof of completed cloud CI or public deployment.
