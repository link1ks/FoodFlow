# Platform maintenance and delivery

## Maintained boundaries

- Kitchen PostgreSQL owns inventory, immutable ledger, permissions and idempotency. Inventory writes and Outbox events commit in one transaction.
- Redis caches only the public catalogue with bounded deadlines and database fallback.
- Outbox relay publishes stable event IDs at least once. Insights owns a separate PostgreSQL database, deduplicates receipts and commits offsets after its database transaction.
- Model Worker retains PostgreSQL jobs, guarded paid dispatch, leases, cancellation and explicit user confirmation.
- Local single-node middleware is an acceptance configuration. Production HA, TLS/SASL, sustained load and operations remain in the [quality ledger](../QUALITY.md).

Service ownership and recovery commands: [PLATFORM](../PLATFORM.md). Checks and evidence rules: [HARNESS](../HARNESS.md). Verified scope and historical closure provenance: [VERIFICATION](../VERIFICATION.md).

## Current task: repository curation and application icon

Started 2026-10-12 (Asia/Shanghai), harness task `repository-curation-app-icon`. User requested removing nonessential uploaded text, improving README and redesigning the existing mascot as a software icon.

Removed 20 completed execution logs, 24 old JSON run summaries, two historical report pages and the superseded mascot asset. Verified local copies are in `.cache/repository-curation/originals`; `.cache/repository-curation/manifest.json` lists the exact files. Git history remains recoverable. Required platform/harness plans, tests, licences, operational guides and generated contracts remain. Closed-debt provenance is consolidated in `docs/VERIFICATION.md`, and the machine quality ledger and all document links follow it.

README now leads with product features, first run and incremental updates. One shared PNG supplies the README and browser application icon; only `web/index.html` changes application behavior. Generation used the built-in image tool; final prompt set and asset location are in [brand assets](../assets/README.md). No business code, migrations, credentials or development data are changed. The opaque master uses a full square safe area; icon shape can be masked by the host. No native installer, PWA/offline support or paid application-model call is introduced.

Run a new full harness and isolated acceptance against frozen inputs before publishing. Inspect GitHub Markdown rendering, mobile/desktop icon appearance, asset requests and browser metadata. Record the actual run IDs, source digest and cloud result below; no earlier run proves this task.

### Evidence

- Final local full `7116252cb15de484e7e61fe269c93817`: 15/15 checks passed, including required Docker database integrations, generation, quality, frontend and source stability. Report: `.cache/harness/repository-curation-full.json`.
- Final isolated acceptance `ebdf4dca571b4ca2775383aafff95ff2`: 14/14 checks passed; 36 desktop/mobile cases, zero failures/skips/retries, 126.7 seconds. Platform recovery, kitchen flow, gateway replacement and redacted diagnostics passed. Local browser used installed Edge; cloud uses Chromium. Report: `.cache/harness/repository-curation-acceptance.json`.
- Both runs match source digest `ab16ca6b2e4ffb2db87de645e9b0673f89f213a1e3b688952eed33e979d8b782`.
- GitHub Markdown preserved the PNG; inspected desktop/mobile/dark README and 32/48/128/256px icon views. Preview JSON/screenshots: `.cache/repository-curation/preview`. Preview helper initially serialized PowerShell file metadata and depended on its legacy HTML parser; reading a plain UTF-8 string and using basic parsing corrected these fixture-only issues. No project tests or business protections were weakened. Local preview does not establish GitHub Mermaid enrichment.
- Anonymous browser verification on the rebuilt acceptance web checked favicon, home-screen metadata, theme color and exact PNG bytes. Evidence: `.cache/harness/repository-curation-icon-acceptance.json`.
- Final remote identities/cloud state and local main synchronization are recorded in `.cache/harness/repository-curation-publication.json`. Local pass does not imply cloud pass. The archive is intentionally ignored; it is not uploaded as replacement clutter.
