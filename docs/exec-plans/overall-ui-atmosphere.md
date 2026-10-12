# Overall FoodFlow visual atmosphere

Date: 2026-10-11. Owner: web UI. Bounded task: `overall-ui-atmosphere`.

## Request and scope

The user finds the background too pale and requests improvement to the overall UI and page appearance. Apply a richer forest-green and warm-gold backdrop throughout the application, with an opaque warm reading canvas, consistent navigation, cards, page headings and kitchen overview panels. Keep existing business behavior, quantity displays and confirmation flows.

## Acceptance

- One decorative, noninteractive backdrop across login, household onboarding, loading/error states and all authenticated pages.
- Dark header with clear active navigation and readable account/control labels; mobile navigation remains fixed to the viewport bottom.
- Warm content surfaces protect text contrast from the saturated background. Menus, shopping and records share styled headings and selected panels; today/inventory/community retain their page identities.
- Desktop, tablet, 390px and 320px login/page previews; no horizontal overflow. Check selected real text contrasts, keyboard focus and existing business scenarios with fictional acceptance accounts.
- Run full harness and isolated platform/browser acceptance, then rebuild the local web and retain development data. Deterministic fixtures only; no paid model calls or remote publication.

## Evidence

- Applied `web/src/atmosphere.css` after the foundation styles: saturated forest/gold backdrop, visible botanical decoration, dark header with gold selected navigation, warm opaque content canvas, stronger cards and page-heading surfaces. Shared root backdrop also covers onboarding and loading/error states. Login and today retain readable light text; inventory and the onboarding guide use darker text on the richer sage/gold panels. Shared cards keep caller-specific warning/selection backgrounds.
- Frontend production build passed. Isolated Edge previews at 1366, 810, 390 and 320px covered login and all six main pages without horizontal overflow or runtime errors. Visually reviewed desktop login/today/inventory/community, tablet records, mobile today/menu and narrow login.
- New selected text contrast checks reproduced a 4.127:1 inventory-panel label/body combination, below the 4.5:1 target. Darkened inventory captions and introductory text; expanded kitchen browser regression checks for actual hero/page/nav text, keyboard navigation/focus and onboarding copy. The focused rerun passed these contrasts but later reached the existing statistics scenario with the platform services stopped: missing `食材用量` heading. Full platform acceptance will start these dependencies and verify the scenario.
- Final full harness passed all 15 checks: `.cache/harness/overall-ui-full.json`, run `2c9a60ba0d5176e7d861df0655fe71a3`. Required Docker Go tests passed in 188.612 seconds; frontend tests/build, API and generated inventory freshness, architecture, formatting, vet, regression gates and repository hygiene passed.
- Final acceptance harness passed all 14 checks: `.cache/harness/overall-ui-acceptance.json`, run `4511ffb0f4c5c5d1cfb14c569eb852c0`. Platform cache/broker/recovery scenarios, kitchen flow, gateway recovery and redacted runtime diagnostics passed. Browser report records 32 expected passes, zero skipped/unexpected/flaky cases in 92.524 seconds, including the expanded contrast, keyboard-navigation and narrow-layout regressions.
- Both reports bind source digest `f54e94281c1a6d6541278b6abb5072b5ff090c6627d6404a24aff098f8fff308` and passed the final source-stability check. Local Windows browser runs used installed Edge (`PLAYWRIGHT_CHANNEL=msedge`); CI uses bundled Chromium. These are local worktree results, not a new cloud CI run or an exhaustive accessibility audit.
- Rebuilt/recreated only the development web container with `docker compose up -d --build --no-deps web`. Existing API/database remained healthy. Refreshed the user's empty login page at `http://127.0.0.1:5173/` and visually confirmed the richer backdrop and warm form. No user account or inventory was changed.
- Regenerated final previews after acceptance: `.cache/harness/ui-{login,today,inventory,menu,shopping,records,community}-{desktop,tablet,mobile,narrow}.png` at 1366, 810, 390 and 320px, no horizontal overflow or runtime errors. Previews and browser checks use fictional data on 15173, not the development database on 5173. Paid model calls and remote publication remain outside this task.
- Task mode recorded the passed outcome against the final full report. Stopped the temporary acceptance/platform containers without deleting volumes; development web (5173) and API readiness (8080) both returned HTTP 200 afterward.
