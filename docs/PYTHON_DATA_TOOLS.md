# Python nutrition processing and offline AI analysis

These developer tools use Python 3.13 standard library only. They do not read
`.env`, connect to databases, download data, or invoke models. Go continues to
own online nutrition reads, business rules and evaluation dispatch guards.

## Nutrition reproduction

Run from the repository root, after retaining the reviewed official downloads:

```powershell
$py = ".\.cache\python-agent-venv\Scripts\python.exe"
& $py -B scripts/import-nutrition.py --sources .cache/nutrition-data --out .cache/nutrition-review-01
Get-Content .cache/nutrition-review-01/manifest.json
```

The source directory contains exactly one `usda/**/food.csv` and adjacent
`food_nutrient.csv`, `tfda/20_2.csv`, and the mapped `cfs-<id>.html` responses.
`nutrition-base-mapping.json` preserves the original explicitly reviewed USDA
IDs and notes. `nutrition-reference-mapping.json` holds reviewed extensions.
No fuzzy food-name matching is allowed.

The output directory must be new and under this repository's `.cache/`.
`profiles.json` is the candidate complete profile set. `reference-records.json`
covers all reference profiles, including the original USDA entries; the old
committed records covered only extensions. `manifest.json` records original
file hashes (including both USDA food and nutrient tables), mapping hashes,
counts and names differing from current profiles.

Decimal values remain strings; blanks/NA/ND/Trace remain absent, never zero.
Duplicate measurements, wrong units and CFS portions other than exactly 100 g
edible portion fail. CFC2002Ed2 insoluble fibre remains
distinct from total dietary fibre. Missing generic fish/shellfish stays unknown.
The importer does not touch production profiles, database migrations or ledger
snapshots. A new reference update needs semantic review, copying reviewed
artifacts and a **new** database migration; never rewrite migration 038.
The retired Node importer and incremental Python writer have no remaining
executable entry points.

## AI evaluation analysis

Generate a fresh **offline rule** report, then analyze it:

```powershell
go run ./cmd/intake-eval -mode scoped -out .cache/harness/data-tools-scoped-01.json
& $py -B scripts/analyze-ai-eval.py --input .cache/harness/data-tools-scoped-01.json --out .cache/ai-analysis-01
Start-Process .cache/ai-analysis-01/index.html
```

The same analyzer supports existing semantic-model journals from
`cmd/intake-eval` and image-scope journals from `cmd/intake-scope-eval`. It only
reads these files; it cannot dispatch or resume model calls. Original private
journals stay local. JSON and standalone HTML output contain aggregate metrics,
provenance hashes and category names, excluding raw prompts, model responses,
sample IDs, media, URLs and authenticated logs. Review category names before
sharing because those labels come from the input corpus. HTML escapes input
text and loads no network resources or JavaScript.

Read **coverage before accuracy**. Only offline scored rows or returned HTTP
200 model rows enter scored accuracy. Unsent, uncertain and HTTP-error rows
remain in coverage denominators, never become successful image rejections.
The report also shows correct/all-records, negative-image false-accept rate,
semantic wrong-draft/false-noop counts, category accuracy and reported tokens.
Image p95 uses nearest-rank over scored rows with recorded latency.
Semantic journals do not record per-row latency, so no latency is invented.
Zero tokens can mean unavailable; this report does not estimate invoices.
It trusts existing correctness labels and does not independently re-score them.

Optional `--baseline <report-input.json>` compares two **input journals**.
Comparison requires identical task kind, origin, corpus/manifest hash, sample
set and complete scored coverage. Different prompts/models are allowed, with
their input hashes retained; differences are descriptive, not causal or
statistically significant claims. Offline rules cannot be compared as though
they were live model results. Empty or malformed input fails.

## Verification and delivery

```powershell
& $py -B scripts/test-data-tools.py
go test ./internal/agent -run 'TestPython(NutritionImport|EvaluationAnalysis)Contracts' -count=1
```

The two named Go tests run the offline Python contracts in local full verification
and existing CI. They cover exact decimals, missing data, source units, fibre
meaning, source-write refusal, duplicate samples, partial runs, incompatible
comparisons and HTML escaping. Actual source reproduction is recorded separately
because private official downloads are not CI fixtures.

Implementation, verification, report inspection, publication and CI checks are
delegated to the agent under the current [delivery convention](../AGENTS.md).
The agent must inspect actual results before claiming success. These tools change
no application data or UI; no database migration or application switch is needed
merely to inspect their candidate files and HTML.
