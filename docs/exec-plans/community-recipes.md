# Community recipes and contribution rewards

Date: 2026-10-11. User approved a community sharing / weekly candidates / reviewed recipe library / contribution points MVP, with a polished responsive design.

## Scope and decisions

- Global authenticated community, independent of household permissions and kitchen stock. Public author name only; no email, phone or inventory details.
- Immutable recipe posts, optional re-encoded photo, precise catalogue-bound quantities, explicit public confirmation, daily publication cap and duplicate recipe fingerprint.
- Set-state likes and saves; unique first likes award points, self-likes rejected, repeat likes cannot award again or move into a new voting week.
- Shanghai Monday–Sunday voting. Hourly worker freezes the previous week's three highest-ranked eligible recipes, one per author, deterministic tie order. Administrator review is required for immutable catalogue publication and a single 50-point award.
- Community administrator UUID allowlist is explicit server configuration; household owner does not imply global administrator.
- Append-only points, daily caps, three cosmetic rewards, owned/equipped state, atomic balance checks and request replay. A separate community write advisory lock bounds initial consistency; high-volume throughput is not claimed.
- Forest/cream visual design, responsive recipe cards, weekly gallery, accessible publishing/detail dialogs and a real rewards shop. No fabricated activity in the shipped database.

## Evidence

- `go test ./internal/app -run TestCommunity -count=1 -timeout 5m`: passed against disposable PostgreSQL 16. Tests validate actual responses with the standard OpenAPI validator; cover re-encoding, precise unit normalization, publication replay/duplicate limits, self-like rejection, private saves, daily award caps, concurrent spending, append-only points, closed-week snapshots, cross-week repeat votes, household/global-admin separation, explicit workflow review, one-time admission and retained accepted recipes after withdrawal.
- `pnpm --dir web test:e2e community.spec.ts` with `PLAYWRIGHT_CHANNEL=msedge`: desktop and Pixel 7 passed (2 tests, 15.9 seconds in a dedicated community run). Covers no-household and household entry, publishing with explicit consent, detailed quantities/steps, private bookmarks, UI likes and weekly gallery, shop/points states and 320px width. This Windows run uses installed Edge rather than bundled Chromium. The final whole-site run below also covers the later fixture corrections.
- Manually inspected desktop community/feed, shop, mobile navigation and narrow publishing screenshots under `.cache/harness/browser-artifacts`. No horizontal overflow at 320px; modal supports scrolling. Screenshots contain explicitly fictional acceptance accounts/recipes, not production activity.
- Initial browser run exposed an overly exact ledger-text selector because the row also contains its date; selector corrected to target the ledger row. Initial full harness rejected stale sqlc models after migration 033; regenerated with pinned sqlc v1.30.0. That run also correctly rejected source changes made during verification. Stable-source final evidence follows below.
- The first complete acceptance run passed platform, kitchen flow, gateway recovery and diagnostics, with 27/30 browser scenarios passing. Two old kitchen scenarios still expected five navigation items; updated them to assert six and a visible community entry. The mobile community scenario assumed its newly liked recipe would always be top three, which fails honestly after several retained fixture runs; it now verifies the gallery against the actual server shortlist without deleting old data. The final full/acceptance run below uses these stable fixture changes.
- Local development `docker compose up -d --build --wait --wait-timeout 120`: all services healthy on `http://127.0.0.1:5173`. A new ignored local JWT configuration was created because none existed; paid allowances and background market sync remain disabled. Existing development data was not deleted. Isolated acceptance uses a separate persistent database/image volume on 15173.

## Final verification

2026-10-11: stable-source full harness (`47ca1f6bb59e3cd71b374cad7fb5ad06`) and acceptance harness (`1d51e2d448d7d88ae15433752b08c261`) both passed, with the same source digest. Full includes architecture/hygiene/knowledge/quality mapping, updater/release/generation regressions, pinned sqlc consistency, formatting, vet, required Go integrations, frontend API freshness/tests/build and source stability. Acceptance includes bound images, real Redis/Kafka recovery/deduplication, kitchen flow, gateway replacement and all 30 desktop/mobile browser scenarios (0 failed/skipped/flaky, 101.776 seconds). Final diagnose mode also passed redacted runtime probes. No development volume was deleted.

Portable redacted evidence: [community-evidence.json](../validation/community-evidence.json). Local detailed reports: `.cache/harness/community-final-full.json`, `community-final-acceptance.json`, `community-diagnose.json`; final viewport screenshots: `community-desktop-preview.png`, `community-mobile-preview.png`, `community-shop-preview.png`. These use fictional acceptance data. Local application health and web endpoints both returned HTTP 200 on 8080/5173. Community administrator configuration remains empty until the user selects an account.

Harness task `community-recipes` was recorded as passed with the final full report. After diagnostics and screenshots, only the isolated acceptance services were stopped; all volumes retained. Development API/Worker/Web/database remain running, API/database health checks pass, and 5173/8080 still return HTTP 200.

## Remaining limits

- No paid model calls, cloud CI or public GitHub publication in this task. The implementation is currently a local uncommitted change, alongside the pre-existing updater work.
- No live-user trial or production performance claim. Initial writes serialize under a community-specific lock; multi-account voting abuse, reporting and moderation audit remain open in the quality map.
- Photos are PostgreSQL bytes (2MB normalized maximum); backup size grows with contributions. Full-scale storage/load acceptance is still required before broad use.
- Administrator UUIDs must be explicitly configured; none are auto-promoted. Hourly Worker handles the previous completed week; earlier missed weeks require an explicit administrator backfill.
- Existing complementary-account merge retains the source user tombstone and historical community attribution; this MVP does not migrate its personal posts, bookmarks, cosmetic ownership or points to the target account. A future merge extension needs explicit ledger transfer entries and collision/permission regressions, rather than mutating historical points.
