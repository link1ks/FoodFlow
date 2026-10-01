# Kitchen experience revision — 2026-10-01

## Authorized scope

Implement the approved five-entry navigation, daily-action home, contextual generation results, lighter repeated stock entry and configuration-aware capability states. Preserve authorization, exact quantities, explicit confirmation, idempotent writes and development data. No paid model requests or public deployment.

## Acceptance criteria

- Desktop and mobile share Today, Ingredients, Menus, Shopping and Records; mobile uses a bottom navigation. Prices belong to Shopping; household configuration and help belong to the profile.
- Empty new households receive dismissible guidance. Dismissal persists per household, existing households default to daily tasks, help remains accessible. Loading and error states are distinct from empty data.
- Home shows recent tasks, expired/spoiled batches separately from urgent usable batches, meals and outstanding procurement. Image/advice results are accessible in Ingredients and plan results in Menus; generation history is accessible in Records.
- Nutrition history and quantity-only usage are separate. Configured-but-failing statistics offer retry; disabled statistics make no projection requests. Model demo is announced before generation.
- Continuous catalog entry retains the picker and displays this-session additions, resets quantities between ingredients, and leaves dates unknown unless entered. Repeat stock entry clears previous quantities/dates. Cooking completion has an explicit confirmation step.
- Current-source full harness and real isolated desktop/mobile acceptance pass; diagnostics report actual stack status.

## Limits

Capabilities describe API-side configuration, not a dependency-health guarantee. Deployment must keep API and Worker model configuration consistent. Existing APIs return the latest 50 generation jobs, and the new household-authorized ledger view returns the latest 100 immutable stock rows; neither is an unlimited historical search. Missing historical snapshot names/units are shown as unknown. Task source links identify the operation type, rather than a deep link to a particular task. Entry-time improvement and longer-term user retention require actual household trials.

## Evidence

Task start recorded through harness task mode before implementation. Acceptance uses isolated fictional accounts and deterministic model configuration, without touching the development account database.

- Initial full/acceptance runs correctly failed because Docker Desktop was stopped. Started the existing Docker Desktop installation; no volumes were deleted. These failed reports remain in harness run history.
- The next full run passed static and frontend checks but identified a fixture error: the new ledger regression expected HTTP 201 for stock input, whose actual contract is HTTP 200. This was corrected without changing stock-write behavior; subsequent full verification passed.

## Final verification

On 2026-10-01, the final current-source full harness passed architecture, knowledge, formatting, vet, required Go integration tests, frontend tests, production build and source stability. The new ledger regression verifies exact quantities, historical names after ingredient rename, unauthenticated rejection and cross-household rejection.

Final `acceptance -skip-build` passed read-recovery regression, Redis/Kafka platform recovery, kitchen workflow, gateway replacement, six desktop/mobile Chromium cases, read-only diagnostics and source stability. Existing images already contained the final application code; the last changes only corrected the new test's required menu revision/idempotency parameters. Full and acceptance source digests match. [Selected machine-readable evidence](../validation/kitchen-ux.json).

Browser cases verify five-entry navigation, no horizontal page overflow, profile-to-household access, guide dismissal across reload, continuous catalog entry with cleared quantities and unknown dates, prices round-trip, returning from home to a generated menu task, history display, disabled statistics making no projection requests, configured service failure with retry, and cooking confirmation: cancelling makes zero completion requests; explicit confirmation makes one. The insufficient-stock response remains permitted and visible; the kitchen smoke separately verifies successful stocked consumption.

Desktop home and mobile inventory screenshots were visually reviewed. Empty contextual-result panels are omitted; pending results open in their originating page. Cooking confirmation uses a focus-managed dialog so the confirmation remains visible after scrolling through meal cards.

During development the first navigation fixture asserted a nonexistent “家庭名称” label instead of the actual settings heading, and the new cooking fixture omitted the required menu revision/idempotency key. Both fixture errors were corrected; final runs have no failed, skipped or flaky browser cases. Failed run history is retained. No new cloud CI, real model quality evaluation, public rollout or household usability measurement is claimed.
