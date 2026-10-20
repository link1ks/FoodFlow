# Python nutrition and offline evaluation analysis

Task: `python-data-analysis`, feature, baseline unknown, started 2026-10-20.

The user requests unified Python nutrition processing and AI evaluation analysis.
Routine implementation/testing is delegated; meaningful report review, publication
and CI/CD steps remain user hands-on operations. Do not push or switch deployments
on the user's behalf without renewed delegation.

## Design and scope

- Replace the Node base importer and incremental Python source/migration writer
  with one Python 3.13 standard-library entry point.
- Move reviewed original mappings/notes to JSON, retain extension mappings.
  Parse official local USDA/TFDA/CFS inputs, reject invalid decimals, duplicate
  measurements, missing mapped IDs and wrong units. Preserve absent values,
  exact decimal strings and insoluble-fibre semantics.
- Output only to a new `.cache/` review directory. Preserve current production
  profiles and migration 038. Record all source and mapping hashes.
- Read existing semantic and vision journals; produce aggregate JSON and
  standalone escaped HTML. No database/provider/network calls, no raw responses
  in reports. Separate offline rule scores from model scores. Exclude uncertain,
  unsent and HTTP-failed rows from scored accuracy, expose coverage, and reject
  incomparable or partial-run comparisons.
- Keep Go evaluation dispatch and application behavior unchanged. Add two named
  Go wrappers for eight Python regressions so current full/CI executes them.

## Actual preparation evidence

- Rebuilt 200 profiles from retained `.cache/nutrition-data`; 198 reference and
  two unknown profiles, **zero semantic changes** against committed profiles.
  Candidate: `.cache/nutrition-python-review/manifest.json`. Raw inputs remain local.
- First reproduction rejected CFS Chinese unit labels; reviewed actual labels
  千卡/克/毫克, added explicit equivalents and wrong-unit regression. No published
  file was modified by the failed run.
- Eight offline Python regressions pass; named Go wrapper tests pass.
- `cmd/intake-eval -mode scoped` produced 100 offline rule rows: 91 strict,
  9 false-noop, zero wrong-draft. This is not real model accuracy.
  HTML: `.cache/ai-eval-python-review-2/index.html`.
- Business contexts, source scopes and test bindings reviewed and refreshed;
  knowledge verification passes.

## Pending delivery

Preliminary full report `.cache/harness/python-data-full.json` passed all test
checks but correctly failed source-stability: review identified a missing CFS
100-g portion check while verification was running. Added the explicit portion
guard and wrong-portion regression; eight Python regressions and actual source
reproduction pass again (200/198, zero changes, review directory suffix `-2`).
A direct sandbox-only Python run could not create Windows temporary fixtures;
the same suite passes under the authorized test execution environment. The final
full run must be fresh and sequential with acceptance; retain the preliminary
failure rather than treating it as final evidence.

The immutable contract was bound before final verification. Final local reports
share source digest `3c1b616b0c92b68b2b2f92c8307ca08271bdb618875eb75aa820609690a97fab`:

- Full `.cache/harness/python-data-full-2.json`, run
  `a9c145f6bdaefe39e6ab0bd915f25a24`: 18/18 checks, 157 named results, no failures.
  Workload: 10 households, 20 clients, 30 seconds, 3507 cycles, 24549 requests,
  zero errors/invariant failures, p95 20.487 ms. This excludes model requests and
  does not establish production capacity.
- Acceptance `.cache/harness/python-data-acceptance.json`, run
  `c09f700afe99d4fd5041935c126d51b4`: 17/17 checks, 42 desktop/mobile results, all
  passed. Installed Edge used locally; CI retains bundled Chromium.
- `.cache/harness/python-data-diagnose.json` and
  `.cache/harness/python-data-main.json`: all required checks passed. Main-site
  used a fresh unauthenticated read-only browser and verified served build hashes.
- Standalone analysis HTML checked with Playwright at 1280x900 and 390x844, no
  horizontal overflow; desktop/mobile screenshots visually reviewed. Linked
  nutrition-basis desktop/mobile screenshots and fresh main login page reviewed.
- No application runtime/source data/schema change; default deployment retained.
  No paid model calls. Candidate records remain local; existing committed profiles
  and source records are unchanged.

Implementation and local verification are ready for user hands-on publication.
The assistant has not staged, committed or pushed these changes. Cloud verification
and version-2 task completion remain pending until the user publishes and the
exact commit passes CI. Do not claim full task completion from local evidence.

This is developer tooling only; no runtime data/schema/frontend change or default
deployment recreation is necessary to use the tools. No paid model evaluation
is authorized. Do not mark the task fully delivered before the user CI step and
completion bundle succeed.

## User hands-on steps after local acceptance passes

### CI failure and follow-up correction

The user committed/pushed `d6d19cfa697d7089dacabee4d59cae104721468e`.
GitHub run `39422137609` failed: frontend/images passed; backend failed
`TestPythonNutritionImportContracts`, and platform failed its Go/named-test gates.
The Python CLI subprocess attempted to execute an empty `sys.executable`.
The Go test launcher passed a bare `python3` argv[0] while intentionally omitting
PATH from the child environment. Linux CPython could not resolve its own path;
the native Windows venv invocation had masked this difference.

Reproduced the exact failure using a cross-compiled Go test binary in a local
Linux Python 3.13 image, network disabled and repository mounted read-only.
Corrected only the test launcher: resolve the interpreter using the parent
environment and pass its absolute path before sanitizing child environment.
Force bare-name selection in the existing named regression even on Windows and
assert absolute `sys.executable` before testing the child CLI. No production
runtime, environment allowlist, provider behavior or nutrition data changes.
Windows targeted tests and both Linux data-tool contracts now pass. Local Linux
evidence: `.cache/harness/python-data-linux-fixed.txt`; redacted cloud failure:
`.cache/harness/python-data-ci-failure.json`. The failed run remains on GitHub.

Fresh correction evidence shares source digest
`0432cab66243bf39c2459cfc8163468797cf250dd0c4415f1ce50f102dddd73e`:

- `.cache/harness/python-data-ci-fix-full.json`, run
  `3a9019f6fb9fe9933fe3b8f113ee3c9f`: 18 checks and 157 named results passed.
  Workload: 10 households/20 clients/30 seconds, 3206 cycles, 22442 requests,
  zero errors/invariant failures, p95 24.828 ms.
- `.cache/harness/python-data-ci-fix-acceptance.json`, run
  `d4872987abb07c879f8285028d3b2768`: 17 checks and all 42 desktop/mobile
  browser results passed, using installed Edge.
- `.cache/harness/python-data-ci-fix-diagnose.json` and
  `.cache/harness/python-data-ci-fix-main.json`: all checks passed, including
  fresh unauthenticated read-only main-page build verification.

The user subsequently explicitly authorized the assistant to create the local
fix commit. Push and GitHub Actions webpage inspection remain user hands-on
steps. The failed cloud run is retained; no fixed-commit cloud success or final
version-2 completion is claimed before publication and successful CI.

Run in PowerShell from `E:\FoodFlow`. This sequence deliberately keeps publication
and the CI webpage check with the user. The assistant must first supply actual
local verification results; these commands alone do not imply verification.

1. Reproduce candidate nutrition files and inspect the change manifest:

```powershell
Set-Location E:\FoodFlow
$py = ".\.cache\python-agent-venv\Scripts\python.exe"
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$nutritionOut = ".cache/nutrition-hands-on-$stamp"
& $py -B scripts/import-nutrition.py --sources .cache/nutrition-data --out $nutritionOut
if ($LASTEXITCODE -ne 0) { throw "Nutrition import failed" }
Get-Content "$nutritionOut/manifest.json" | ConvertFrom-Json | Select-Object profiles, reference, changed_profiles
```

Expected: 200 profiles, 198 references, empty changed_profiles. The source files
are the retained official downloads; this does not download or publish anything.

2. Analyze an existing offline evaluation and open the standalone page:

```powershell
$analysisOut = ".cache/ai-hands-on-$stamp"
& $py -B scripts/analyze-ai-eval.py --input .cache/harness/python-data-offline-eval.json --out $analysisOut
if ($LASTEXITCODE -ne 0) { throw "Analysis failed" }
Start-Process "$analysisOut/index.html"
```

Check offline rule origin, 100 samples, coverage 100%, accuracy 91%, wrong-draft
0 and false-noop 9. Expand JSON to inspect provenance and limits. These numbers
describe the existing rule interpreter, not a new live model benchmark.

3. Review and publish the exact prepared source files:

```powershell
git status --short
git diff --stat
$taskFiles = @(
  "AGENTS.md", "docs/BUSINESS_GUIDE.md", "docs/HARNESS.md",
  "docs/NUTRITION_REFERENCES.md", "docs/PYTHON_DATA_TOOLS.md",
  "docs/exec-plans/platform.md", "docs/exec-plans/python-data-analysis.md",
  "docs/quality/business.json", "docs/quality/tasks/python-data-analysis.json",
  "internal/agent/data_tools_test.go", "internal/nutrition/README.md",
  "scripts/analyze-ai-eval.py", "scripts/import-nutrition.py",
  "scripts/nutrition-base-mapping.json", "scripts/test-data-tools.py",
  "scripts/extend-nutrition.py", "scripts/import-nutrition.cjs"
)
git add -- $taskFiles
if ($LASTEXITCODE -ne 0) { throw "Staging failed" }
git diff --cached --stat
git diff --cached --check
if ($LASTEXITCODE -ne 0) { throw "Staged diff check failed" }
git commit -m "Unify nutrition imports in Python and add offline AI analysis"
if ($LASTEXITCODE -ne 0) { throw "Commit failed" }
git push origin main
if ($LASTEXITCODE -ne 0) { throw "Push failed; stop and share the error" }
git rev-parse HEAD
Start-Process "https://github.com/link1ks/FoodFlow/actions"
```

The deleted old scripts must be staged too. `.cache/` candidates, raw downloads,
journals and screenshots are intentionally excluded. On GitHub select the run
whose commit matches `git rev-parse HEAD`, open backend/frontend/images/platform,
inspect step details and wait for all four to pass. Platform artifacts contain
bounded harness evidence. This workflow validates and builds images; it does not
automatically deploy to this PC. Send the commit/run URL and actual outcome back
so the assistant can verify cloud evidence and record version-2 completion.
