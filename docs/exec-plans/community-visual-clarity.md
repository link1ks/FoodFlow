# FoodFlow visual clarity correction

Date: 2026-10-11. Owner: web UI. Task: `community-visual-clarity`.

## Reproduced problem

The user reported indistinct visual hierarchy. The current 5173 browser showed the login page at a narrow viewport: field outlines were barely visible against white, and multiple secondary actions blended into the background. Community previews also used similar pale green/cream surfaces, small secondary text and subtle selected states. Its tall mobile hero delayed the actual recipes.

## Changes and acceptance

- Preserve the natural kitchen palette, with a dark green welcome panel, white form and visible field borders. Separate the primary sign-in action from SMS and other links.
- Give shared cards, secondary text and active navigation clearer contrast. Preserve explicit error, disabled and selected states.
- Separate the community introduction, filled column tabs, white recipe cards, gold weekly ranking and green contribution panel. Add a feed heading and increase readable content text.
- Keep mobile cards in one column, compact the hero, preserve keyboard tab behavior and avoid horizontal overflow at 320px.
- Verify against fictional accounts on the isolated acceptance stack. Preserve the user's development database and account; no paid model calls.

## Evidence

Completed locally. Final full and acceptance evidence below bind to the same corrected source; bounded task outcome recorded as passed.

### Visual findings

- Inspected the real 5173 login screen without signing in or reading account data. Its shared field boundary computed to `rgb(204, 214, 202)` on white: the Tailwind utility layer overrode the attempted component-layer border. Correct the field's actual base utility and retain an unfocused boundary regression assertion (minimum contrast 3:1).
- New community previews at 1366px and 390px distinguish the dark introduction, filled active tabs, white feed and gold/green side panels. The 390px first recipe image now enters the first viewport; the descriptive mobile hero paragraph and secondary feed caption are hidden to recover space.
- Added browser checks for selected visible text pairs (minimum contrast 4.5:1) and 320px login overflow. These sample actual solid surfaces/opaque gradient stops, not full accessibility certification. Existing community acceptance still checks 320px feed/publish overflow and public confirmation.

### First verification and correction

- First full harness passed all checks against the intermediate source. First acceptance run `179f1dc8aa2a55b1e4ef5eaccd091b56` passed platform, kitchen, gateway and 30 browser cases; both newly added login boundary cases failed at 1.4958:1. This accurately reproduced the inspected utility-layer override.
- Changed the actual shared Field utility to an explicit darker border and tinted surface, rather than relying on a lower-priority component rule. Removed that ineffective rule. Also bounded the desktop welcome photo container by its available width for tablet layouts.
- Final verification must bind to this corrected source, not the intermediate full report.
- The first corrected-source acceptance attempt `d3b385e1f56165133d5b8c1cbc73ade5` stopped before runtime checks because Docker Desktop was no longer running. Restarted the existing installation at `E:\Docker\Program` under the user's earlier Docker-start authorization, then retried. No database volumes or accounts were deleted.
- A corrected-source full run overlapped Docker recovery and failed `go-tests` in `internal/app`; all other checks passed. Its bounded output retained the package failure but not the earlier failing test detail, so startup causality is not established. Retrying the full check after the Docker daemon and both stacks became healthy is required; do not claim this run passed.

### Corrected-source browser and local delivery evidence

- `PLAYWRIGHT_CHANNEL=msedge go run ./cmd/harness -mode acceptance -out .cache/harness/visual-clarity-final-acceptance.json`: all 14 checks passed, run `dac06cfe5ae643be89f35f601b239d61`, source digest `0a08f9d44f707ee05ca9dc53914110bbc1e7e251036e78a933b3dbfa55a31634`.
- Browser result: 32 expected passes, zero unexpected/skipped/flaky cases, 95.048 seconds. Includes login contrast, 320px login/publish layout, public recipe confirmation, private bookmarks, likes, weekly candidates and points views. Existing kitchen/account/inventory workflows also passed.
- Anonymous login previews manually reviewed at 1366, 810, 390 and 320px; helper verified no horizontal overflow at each size. Final community desktop, 390px mobile and shop previews use fictional acceptance accounts/posts. Artifacts remain ignored under `.cache/harness/`.
- `docker compose up -d --build --no-deps web` rebuilt the development web; `docker compose up -d --no-build db api worker` restored existing stopped development services. Browser refreshed at 5173, inspected the actual new login surface and confirmed border `rgb(124, 142, 128)` on `rgb(247, 248, 244)` (3.261:1). API/database health probes were healthy.
- No database volume or account deletion, paid model call or GitHub publication. Local Windows Edge supplies browser acceptance; CI's bundled Chromium and a complete accessibility audit are outside this evidence. Task elapsed time includes the interruption before the user's resume and Docker recovery, not just coding time.

### Final outcome

- `go run ./cmd/harness -mode full -out .cache/harness/visual-clarity-final-full.json`: all 15 checks passed, run `61607c855312c2a9640bfc7b4bdefcf5`, matching the acceptance source digest above. Required Go integration took 186.404 seconds. Architecture, hygiene, knowledge, quality map, updater/release regressions, SQL generation consistency, formatting, vet, generated API freshness, frontend tests/build and source stability passed.
- The healthy-Docker retry passed; the earlier package failure remains recorded as an unresolved transient observation, not proof of a product defect or a particular startup cause.
- `git diff --check` passed. Harness task `community-visual-clarity` finished with outcome `passed` against the final full report. The temporary acceptance stack is stopped after verification, with its data retained; development 5173 remains running for the user.
