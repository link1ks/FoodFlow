# Platform maintenance and delivery

## Maintained boundaries

- Kitchen PostgreSQL owns inventory, immutable ledger, permissions and idempotency. Inventory writes and Outbox events commit in one transaction.
- Redis caches only the public catalogue with bounded deadlines and database fallback.
- Outbox relay publishes stable event IDs at least once. Insights owns a separate PostgreSQL database, deduplicates receipts and commits offsets after its database transaction.
- Model Worker retains PostgreSQL jobs, guarded paid dispatch, leases, cancellation and explicit user confirmation.
- Local single-node middleware is an acceptance configuration. Production HA, TLS/SASL, sustained load and operations remain in the [quality ledger](../QUALITY.md).

Service ownership and recovery commands: [PLATFORM](../PLATFORM.md). Checks and evidence rules: [HARNESS](../HARNESS.md). Verified scope and historical closure provenance: [VERIFICATION](../VERIFICATION.md).

## Previous task: repository curation and application icon

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

## Previous task: preserve the mascot, refine its icon presentation

2026-10-12, task `lively-kitchen-spirit-icon`. The user likes the original pot character and clarified that the required change is software-icon presentation. An exploratory hopping/winking character render was rejected and remains outside the repository. Final selection preserves the original front-facing shape, two open eyes, smile, lid, leaves and handles; refines the rounded tile, composition, margins and restrained material depth; and adds the user's requested magical accents. The user defines magic as the model, the pot as its harness and their combination as the agent. Gold sparkles and a thin arc surround the lid without covering the face. A transparent draft's fringe was corrected with the built-in editor; the selected master is opaque with a clean rounded tile. No business code, migrations, credentials or development data changed.

Previous curation commit `fb0c7a11f01123f94c55cd70842ac3aeb6d6d11b` has now passed [CI 38214018458](https://github.com/link1ks/FoodFlow/actions/runs/38214018458). It does not establish checks for the new icon. After selection, run a new full harness, isolated browser/platform acceptance and actual served-PNG check. Record actual evidence and limitations here. Drafts and the previous PNG stay in local ignored storage; the shared application asset path remains unchanged.

Final local full `f9f3cb2e4cf7eb2448da3559cdef09bd` and isolated acceptance `38264d03f554c14114599774d8743fe5` passed on source digest `0ec15d6f9637e6a73bdd10669b5e7b165d6cc87fc8cfb6d7b7096db9463e12a8`. Reports: `.cache/harness/kitchen-icon-full.json` and `.cache/harness/kitchen-icon-acceptance.json`. Browser tests used Edge and deterministic application-model fixtures; all 36 desktop/mobile scenarios passed with no failures, skips or retries. Cloud Chromium is a separate environment.

Inspected original/refined 32/48/128/256px comparisons and GitHub Markdown desktop/mobile/light/dark previews in `.cache/kitchen-icon-refinement`. A reused-page dark screenshot missed an image repaint after a viewport change; independent page rendering with image decode and frame synchronization resolved the preview-only artifact. PNG metadata and byte identity passed on the rebuilt anonymous acceptance web. The standalone helper initially shared the harness output filename; its checks ran before the harness wrote the final report, and its output is now separated as `.cache/harness/kitchen-icon-image-acceptance.json` and `kitchen-icon-image-development.json`. The final full/acceptance reports remain intact. The tiny magical particles intentionally become secondary at favicon size. Local preview does not establish GitHub's Mermaid enrichment, native installation or model quality. Final remote/cloud state and served-image hash are recorded in `.cache/harness/kitchen-icon-publication.json`.

## Current task: README icon display reference

2026-10-12, task `readme-icon-reference-display`. The supplied reference shows a small standalone rounded-square icon centered above the project name and description. Preserve FoodFlow's approved pot and magical accents; remove the surrounding opaque square with the built-in image editor and use a 112px README display. Magic represents the model, the pot represents its harness, and their combination represents the agent.

Keep the shared PNG path and existing browser metadata. Only brand assets and documentation are in scope. Preserve the previous image and previews in ignored local storage. Inspect actual transparency and the GitHub Markdown header in desktop/mobile/light/dark views, then run a new full harness and isolated acceptance against the final source. Record actual results below; previous task evidence does not establish this change.

The first transparent output had visible outer flecks; a targeted built-in image edit removed the scattered marks. Inspected the selected 1254px PNG at its actual 112px README size using GitHub Markdown output, independent dark/light browser pages and a 390px mobile view in `.cache/readme-icon-display/preview`. The standalone tile has no rectangular outer matte. Read-only alpha inspection reports corner values `[0,0,1,0]` out of 255; the initial strict-zero probe rejected that negligible export residue. It is retained as generated, with no manual pixel modification. Full-size raster edges are not vector geometry; approval is based on the intended display size. Preview does not execute GitHub Mermaid enrichment. The preceding icon commit `a3dcc2e` passed [CI 38219831777](https://github.com/link1ks/FoodFlow/actions/runs/38219831777), separately from this task.

Final local full `ecfd09cb2ff5a54eab7389424b9d5bfa` passed 15/15 and isolated acceptance `d70bb0a55cc0cdc91445b76d54119a77` passed 14/14 on source digest `25702b6d90b143e4731cfa253784779c4b33c242754c4197baadb3466556bd96`. Reports: `.cache/harness/readme-icon-full.json` and `.cache/harness/readme-icon-acceptance.json`. Browser acceptance used Edge with deterministic model fixtures; cloud Chromium remains separate. Anonymous byte/metadata checks passed on both acceptance port 15173 and rebuilt development port 5173, recorded in `readme-icon-image-acceptance.json` and `readme-icon-image-development.json`. No development data or volume was removed. Remote identity and current cloud state are recorded separately in `.cache/harness/readme-icon-publication.json`; local success does not imply cloud success.
