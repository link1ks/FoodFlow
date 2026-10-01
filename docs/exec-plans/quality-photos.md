# Contract, browser, quality and ingredient-photo delivery

Date: 2026-10-01 (Asia/Shanghai). Task: `quality-photos-20261001`.

## Authorized scope

Implement the first three project-audit recommendations: valid/matching API contracts and main-flow schemas, successful UI kitchen workflow, business-domain quality/debt mapping. Clean demonstrably unused files and replace picker illustrations with matching real photographs. Preserve development data, backups, permissions, quantities, idempotency and explicit confirmation. No paid model calls or public deployment.

## Acceptance

1. Standard OpenAPI validation, exact API route/method parity and real fixture request/response checks run in Go tests; missing required fields and numeric quantities fail regressions.
2. Desktop/mobile UI perform stock entry, menu confirmation, shortage purchase and successful cooking; balances, FEFO ledger rows, refresh and replay show no repeated deduction. Household switching and member downgrade have browser/server checks.
3. Nine domains have module ownership, acceptance, check references and debts. Harness rejects lost mapping/scenarios and incomplete completion evidence. Task failures have categories without rewriting history. Fuzz exercises quantity/conversion/scaling/allocation conservation.
4. Every current catalogue ingredient uses a reviewed local photograph with source, author and license; uploads keep precedence and missing/failed photos have an explicit fallback.
5. Cleanup removes only checked unused transient files; source/fixtures, data, volumes, uploads, credentials and backups are retained.

## Decisions

`kin-openapi` v0.149.0 is test-only; real authorization remains application middleware. Schemas cover the main kitchen flow; remaining endpoints and generated frontend types are explicit debts. Quality-map checks discoverability, not actual execution; actual results come from full/acceptance. Default verification uses deterministic models. Licensed Wikimedia photographs are local, with individual attribution and visual review.

## Development feedback

- Newly written schemas initially assumed wrong response keys. Register uses `user_id`; confirmation returns `shopping_list_id`/`shortages_rechecked`; stocking returns `batch_id`/`quantity`; completion returns `plan_meal_id`. Schemas now match actual responses; application behavior is preserved.
- Initial UI fixture relied on a new label against an old image; option-based selection fixes the fixture, and final acceptance rebuilds images. Purchase fields preserve `200.000`; expected values were corrected. FEFO splits lettuce consumption into 100g and 200g rows, so expected ledger count is six.
- Photo sourcing encountered Wikimedia rate limiting; cached metadata, bounded retry and reduced concurrency prevent repeated downloads. Botanical illustrations, live animals and unrelated prepared dishes were rejected during review; real food/reference photos retain their individual source context.
- Scheduler fuzz initially assumed every feasible recipe must be recommended. The default scoring policy can prefer an empty plan for tiny unselected demand; the fixture now selects the ingredient explicitly. The failing input remains a regression seed; no algorithm defect was established.
- The asset integrity test needed explicit Node.js type definitions for filesystem/crypto imports; these are development-only. The production build includes local assets, not filesystem checks.
- First full/acceptance runs correctly rejected the AI quality-map anchor (`param(` was not present). It now references the actual durable-enqueue failure assertion. Both runs were retained as failed evidence, including source-stability failure after correction during verification; final reports are rerun against stable inputs.
- The browser runner on Node 24 requires a JSON import attribute; the photo fixture now uses explicit filesystem JSON parsing, matching the existing browser fixture convention. No application behavior changed.

## Verification

Task start recorded before edits. Final full passed all 9 checks, including required Docker integration, contract fixtures, quality mapping, frontend tests/build and stable source. Rebuilt acceptance passed all 11 checks, including read-recovery regression, platform recovery, kitchen API smoke, gateway replacement, browser and redacted diagnostics. Both reports have the same source digest `b4e8f9df020b1225781680e5c7d28b6db3179ce594fa92d33294823b3177ee75`.

Full run: `30959c4787381750fccee17a148bc5f0`; passing full report `.cache/harness/quality-photos-full.json`. Acceptance report `.cache/harness/quality-photos-acceptance.json`; no skip-build flag was used. The 6 browser scenarios ran on desktop and mobile: 12 passed, zero skipped/unexpected/flaky (47.994 seconds). New scenarios verify successful UI stock/menu/procurement/cooking and exact FEFO ledger/replay, household switching/member downgrade with server denial, and real reference-photo loading/upload precedence/removal/failure fallback. Frontend has 12 passing unit tests; production build succeeds with a 484.99 kB application JS bundle (152.33 kB gzip). The compact photo index avoids bundling full provenance metadata.

Portable redacted results and remaining limits: [delivery evidence](../validation/quality-photos-20261001.json). Screenshots and final photo-review contact sheets remain under `.cache/harness`; historical failed run files are preserved. Task completion is recorded with the passing current full report.

Photo visual review covered all 91 current catalogue entries in three contact sheets under `.cache/harness/ingredient-photo-review-*.jpg`. Each source/author/license and transformed-file SHA-256 is in `web/public/ingredient-photos/manifest.json`; public attribution is `web/public/ingredient-photos/credits.html`. The client imports a compact index with only name/URL/credit, avoiding provenance metadata in the application bundle. WebP assets total 2,775,998 bytes versus 6,332,470 bytes for the selected source thumbnails (56.2% reduction); maximum image is 90,138 bytes and dimensions do not exceed 480 px. Photos are references, not claims about actual household stock.

Initial cleanup: 56 unreferenced one-off scripts, superseded binaries and empty historical logs removed (224,134,188 bytes). A read-only process check confirmed no executable under the project was running. Candidate-photo cleanup removed 85 unselected/superseded source files. Exact manifests are under `.cache/harness/cleanup-quality-photos.json` and `.cache/harness/photo-candidate-cleanup.json`. Historical evidence, backups, configuration and development data are retained.

After provenance and final visual-review sheets were saved, 664 temporary photo search/download candidates, cached API metadata and one-off helpers were removed (42,646,134 bytes). Exact manifest: `.cache/harness/photo-temporary-cleanup.json`. Selected WebP files, public credits/manifest and final review evidence remain.

Bounded fuzz: quantity round-trip passed 6,623,587 executions; conversion/serving conservation passed 7,360,068; corrected stock allocation/FEFO conservation passed 6,509,634. Each ran for 10 seconds; these observations are not formal proofs or productivity measurements. Frontend targeted tests passed 12 tests.

## Limits

No real SMS/S3/public production acceptance, paid AI evaluation, capacity/SLO or household retention improvement is claimed. Existing untracked cleanup/audit documents are retained.
