# Python Agent/RAG migration and measured scale

Task `python-rag-scale`, started 2026-10-21, feature, baseline unknown.
Status: migration, large evaluation and completion-gate regression passed local
verification. The user explicitly redelegated backup, deployment, publication and
acceptance. Default paired backup, deployment and read-only main-site verification
passed; publication and exact-commit cloud verification follow this recorded state.

## Scope

Five application agents already execute Python: menu, nutrition, image, speech
and community review. This task migrates remaining engineering RAG computation:
lexical/concept features, revision selection, replacement/deletion filtering,
SQLite inverted index, exact ranking, grounded prompt and strict citation/fix
validation. Go retains typed integration, authorization, transactional quantities,
allowances, leases, audit evidence storage and completion gates. These backend
controls are not a second model or retrieval implementation. No Go fallback.

The derived SQLite index is atomically replaced and binds history digest/version.
The complete Go query still scans immutable history for freshness; benchmark this
separately from Python engine time. Retain private history; subprocesses receive
an allowlisted environment without database credentials.

## Frozen measurements

- 100,000 synthetic documents; 10,000 queries; seed 20261021. Mix: 8,000 identifier,
  1,500 filtered concept/paraphrase, 500 no-answer. Generated templates and unique
  identifiers are not real incidents or independent human relevance labels.
- Hash inputs before observing results. Targets: Hit@5 >= .99, no-answer 1.0,
  mixed engine P95 <= 2000 ms. Retain denominators, MRR, build time and index size.
- Separately measure 30 broad queries and 20 full bridge queries on 100,000
  fictional audit files. No production concurrency, user-count or savings claim.
- Live: 40 new self-labelled RAG cases, 10 each answerable/conflict/no-answer/
  injection. Freeze before dispatch; overall threshold .90, P95 <= 30 seconds.
  Report strata and retain failures. Prior v0.3 failed holdout stays unchanged.
- User authorization: up to 100 requests, 100,000 tokens, CNY 5, existing models.
  Use text-only qwen3-vl-plus, durable pre-dispatch reservation, no retries.
  Forty cases reserve 78,032 tokens and CNY 2.341 at a conservative 30/million.
  Free quota applicability and invoice cost remain unknown. Official Beijing
  <=32K list prices: CNY 1 input / 10 output per million tokens.
  [Official pricing](https://help.aliyun.com/zh/model-studio/qwen3-vl-plus).

## Verification and delivery

Engineering history queried before binding. Freeze named requirements, run full
and acceptance sequentially, then diagnose/RAG lifecycle and complete-history
metrics. Completion also requires default deployment, fresh read-only main-site
identity and current published-commit cloud evidence. Under current learning
collaboration, publication, paired backup and default Docker recreation remain
user steps unless redelegated. No default database synthetic writes or new
physical-device acceptance are included; earlier pending device gates remain.

## Evidence

Initial Go harness/agent suites and four Python RAG regressions passed: exact
scores against exhaustive oracle, lifecycle/filters, grounding and budget limits.
First scale report `.cache/harness/python-rag-scale-benchmark-1/report.json`:
100,000 documents / 10,000 queries; build 10.875 s; SQLite 121,954,304 bytes;
mixed engine P95 32.003 ms, P99 34.160 ms, sequential 200.369 queries/s.
Hit@5 9500/9500, no-answer 500/500, MRR@5 1.0 on identifier-heavy templates.
Broad query P95 1271.839 ms (30 queries). Warm engine excludes history scan,
process startup, HTTP, model and concurrency. Windows 11, Python 3.13.11,
20 logical processors. Full bridge/live/final integration evidence pending.


## Retained failures and final guarded evaluation

First live evaluation: 30/40 (75%), with conflict abstention 0/10. Every unsupported
conflict answer was blocked by output validation. Strengthening the prompt alone
passed only 3/5 previously failed conflict cases. Neither result was discarded.
The observed model contract failure is established; its internal model cause is
unknown. The implemented mitigation makes evidence eligibility deterministic in
Python before dispatch as well as validating citations after generation. Existing
harness no-evidence handling avoids a paid call altogether. The live test deliberately
still dispatches empty eligible contexts to measure model abstention; these probes
are distinguished from the no-call application path.

A newly frozen holdout passed 40/40 after deterministic filtering, 10/10 in each
stratum; P95 7,984 ms. This is **guarded pipeline correctness**, not raw model
accuracy and not an independent human-labelled benchmark. Wilson 95% interval
for 40/40 is approximately 91.24–100%, conditional on these samples; it does not
correct selection bias. Journal: `.cache/harness/evaluations/python-rag-scale-holdout-3.json`.
Its source digest matches the code before the later scope expansion:
`ac6d9e30a6890322407dc25ec74771fbc1599046f3cb5883d2736ef492ed6aec`.

Across all three live runs: 85 requests, 11,848 provider-reported tokens. Public
list-price estimate CNY 0.022396; invoice cost and free-quota application unknown.
At that stage no additional calls were planned; the subsequent explicit user
authorization below expands this scope. All failed/returned rows remain.

Final engine run: 100,000 records, 10,000 queries, build 10.845 s, index
121,954,304 bytes; mixed P95 31.410 ms, P99 34.310 ms, sequential 200.916 queries/s.
Broad unfiltered P95 is 1,525.880 ms, not 31.410 ms. Synthetic Hit@5 9500/9500,
no-answer 500/500. Final manifest preserves hashes and all per-query outcomes.

The first full-bridge benchmark was rejected because its query fixture was placed
outside `.cache/harness`. Fixed the fixture writer, added a private-path regression,
and retained the failure. This was a benchmark fixture failure, not an application
outage. Final bridge and full/acceptance results follow after execution.


Final full bridge: 100,000 immutable synthetic history files, 20 queries, build
20.949 s; P50 8,182.848 ms, P95 13,090.621 ms, max 21,433.766 ms. Docker Desktop
was started during later samples, so this includes local contention and is not an
uncontended latency claim. Complete JSON scan/hash and interpreter startup are
included. This CLI audit design is not a low-latency online service at 100K history
files; no engine-only figure should be described as its end-to-end performance.
A future online design needs an explicitly versioned snapshot/transactional history
store and independently tested freshness semantics; this task does not weaken
freshness checks to improve its benchmark.

Public redacted numerical evidence: [python-rag-scale.json](../evidence/python-rag-scale.json).
Default Docker was initially stopped. Docker Desktop was started for isolated tests;
no default application recreation, backup or publication has been performed.


## User-authorized large live continuation

User expanded the real-call allowance to **up to 8,500 cumulative requests** while
retaining the CNY 5 cap. The earlier 100K-token limit is superseded by this larger
explicit call/budget authorization. New bounded task: `python-rag-industrial-eval`.
The original task outcome is retained as scope-expanded/blocked, not rewritten as
passed. Its source-matching full did pass all checks before the expansion.

A shared durable ledger starts with the actual 85 calls and CNY 0.022396 list-price
estimate. Up to 16 workers serialize pre-dispatch reservations; unknown outcomes
keep their full allowance and never trigger retry. Settlement uses actual reported
input/output tokens at reviewed Beijing rates. An exclusive ledger lock and fixed
ledger path prevent accidentally resetting this task budget in another run.
Free quota and actual invoice remain unknown. Stop on budget exhaustion or five
service/unknown-usage outcomes; never silently skip them from denominators.

Development set: 200/200 passed across ten strata, 20 each. Cumulative 285 calls,
CNY 0.104546 estimated list-price usage; no uncertain reservation remained.
The new 8,000-case holdout is frozen before final verification with seed 2026102197,
SHA256 `658afb155b530d8001d6824cd1a754966841b6fe58e14283898f20a9b9555b1e`.
It contains **20 self-labelled semantic issue families**, permuted IDs, ordering,
versions and injection text across ten strata. It is not 8,000 independent human
annotations or new semantic problems. Each stratum has 800 calls. Thresholds:
all 8,000 attempted, overall >=95%, each stratum >=90%, P95 <=30 seconds.
These calls start with supplied candidate evidence. They evaluate deterministic
eligibility, model generation and strict citation validation; they do not retrieve
from the separate 100K-document scale corpus. Do not combine the two experiments
into an end-to-end RAG accuracy or latency claim.

The runtime now applies explicit error-code and applicable-version eligibility
both before dispatch and during citation validation, matching the existing RAG
query scope. Old-version or unrelated-code citations cannot pass by guessing an
ID hidden from model context. Regression tests include this cross-boundary case.
No model budget is spent by ordinary full/CI checks. Final full and acceptance
must be rerun against this expanded source; earlier full remains historical.


## Reviewed delivery procedure

The following procedure was prepared under AGENTS.md learning collaboration.
The user subsequently explicitly authorized the agent to execute these steps.
Final full, acceptance and live reports were inspected before default deployment.

```powershell
Set-Location E:\FoodFlow
# Start existing dependencies only; do not recreate or remove their volumes.
docker start foodflow-dev-postgres foodflow-redis-1
# Private reviewed helper: builds first, validates a paired database/photo backup,
# preserves exact existing configuration and mounts, then recreates API/Worker/Web.
pwsh -File .cache/deploy-python-rag.ps1 -ConfirmSwitch
$env:PLAYWRIGHT_CHANNEL = 'msedge'
go run ./cmd/harness -mode main-site -task python-rag-metrics-closure -out .cache/harness/python-rag-metrics-main.json
# Review the source diff and redacted evidence before staging only those changes.
git diff --check
git diff --stat
# After a reviewed commit is on main, publish it:
git push origin main
# Inspect the exact commit's Actions results, then record cloud evidence:
go run ./cmd/harness -mode cloud -task python-rag-metrics-closure -out .cache/harness/python-rag-metrics-cloud.json
# Recompute after main-site/cloud added new retained runs, then validate all gates.
go run ./cmd/harness -mode experience -action metrics -task python-rag-metrics-closure -out .cache/harness/python-rag-metrics-metrics.json
go run ./cmd/harness -mode task -action finish -task python-rag-metrics-closure -outcome passed -evidence .cache/harness/python-rag-metrics-completion.json
```

The private helper is deliberately not a public artifact: it references this
machine's existing default deployment and preserves private configuration under
protected backups. Its authorized deployment execution passed as recorded below.
Do not include `.cache`, `.env`, backup archives, raw authenticated logs, or private
provider journals in commits. [CI dashboard](https://github.com/link1ks/FoodFlow/actions).
Final completion still requires fresh matching full/acceptance/diagnose/main-site/
cloud/feedback reports and a validated version-2 completion bundle.

## Expanded final local verification

Full run `52c763d8672ced1849548034d9f391ec` passed 18/18 checks and all 168 named
results. Source digest:
`021bc46f7cc890966a353570a518305fa3364c5825c60b0ff0d3741e6269fc4a`.
The bounded workload used 10 households and 20 clients over 30.163 seconds:
3,459 completed cycles, 24,213 requests, zero errors and invariant failures;
P50 9.137 ms, P95 21.583 ms, P99 29.427 ms. This is an isolated in-process API
plus real PostgreSQL test; it excludes gateway, cache, Kafka and model calls.
Report: `.cache/harness/python-rag-industrial-full.json`.

Acceptance run `17b01f29c0af84eb7700f6b4620e9262` passed 17/17 checks and all
44 browser results (22 desktop, 22 mobile), including platform ownership,
kitchen flow, gateway recovery and runtime diagnostics. Desktop/mobile home and
the editable voice-intake draft screenshots were inspected. Diagnose passed 5/5;
the offline RAG lifecycle evaluation passed 8/8. All bind the same source digest.
The original eligibility and benchmark-path failures now have append-only resolved
revisions linked to the passing named full regression and browser acceptance.
An exact error-code/version repair query confirms the verified mitigation is
retrievable; no claim is made about the model's internal cause.

The expanded live run binds the same source digest and completed all 8,000 calls:
7,999 correct (99.9875%), P95 9,985 ms, zero service failures. Nine strata passed
800/800; multiple-sources passed 799/800. All frozen thresholds passed. One model
response omitted a character from its citation ID. Existing exact citation
validation rejected it; offline replay confirmed rejection and acceptance of both
legitimate citations. This remains a counted model error, not a repaired model or
100% correctness claim. The synthetic failed row and its classification were
reviewed for redaction; no private user data is present. No retry was sent.

Across all five retained live runs: 8,285 requests, 1,965,345 provider-reported
tokens, CNY 3.394338 list-price estimate, zero unsettled reservations. Actual
invoice/free-quota application is unknown. The longer contexts account for a
higher average estimate than extrapolating the initial 85-call sample. All calls
remain below the authorized 8,500-request/CNY-5 limits; unused capacity is not spent.

Frozen input corpora/manifests are retained under `.cache/harness/live-inputs/`.
The exact aggregate reports are registered as
`.cache/harness/evaluations/python-rag-development-run-1.json` and
`.cache/harness/evaluations/python-rag-industrial-holdout-run-1.json` so the harness
includes them in its complete history. Their original run folders retain the
exclusive JSONL rows and runtime snapshots. No outcomes or labels were changed.
The reconciliation report `.cache/harness/python-rag-industrial-audit.json` checks
all 8,000 identities/labels, JSONL vs aggregate rows, P95, provider usage and the
cumulative reservation ledger. Complete-history metrics and the task feedback
report are prepared. The version-2 completion bundle still lacks current main-site
and published-commit cloud evidence; it must not be marked passed yet.

The Go integration layer, service-side transaction/authorization checks and
synthetic diagnostic harnesses remain Go. Production Agent execution and RAG
computation run in Python. Retaining typed DTO parsing and synthetic modelcheck
fixtures in Go is not an alternative production model/retrieval implementation.

## Completion-gate regression continuation

Validating the prepared bundle reproduced a pre-existing metrics comparison bug:
the retained open `v03-rag-conflict-holdout` makes `improvement_backlog` nonempty.
Fresh values contain typed structs, whose JSON field order differs from decoded
maps. Hashing these representations rejected semantically identical complete
metrics. Regenerating metrics did not fix this. The old failure was not deleted
or marked resolved to bypass the gate.

`TestFeedbackMetricsCanonicalizationKeepsUnrelatedOpenFailures` failed before the
fix and passed afterwards. The gate now normalizes the freshly computed JSON
representation before content comparison. The regression retains an unrelated
open failure and rejects missing backlog, run count and RAG history. Other source,
task, time, threshold and unresolved-current-task checks are unchanged.

The original 8,000-sample contract/result remains immutable; that task was recorded
blocked by the completion bug. Continuation `python-rag-metrics-closure` binds the
new regression and a separate 200-call post-fix check. Its smaller sample is not a
replacement or reinterpretation of the large experiment. Both versions have the
identical Python runtime SHA256
`b36e752c4e75312f38540dedd08fb5badd61ebb07bdf8368b708f37bd526625b`.
Only completion comparison, task association in the evaluation CLI, its regression
and documentation/bindings changed in this phase.

Post-fix corpus seed 2026102103, SHA256
`ec75b61d68a789d6f50438c38890b50fde17a864db25d3bc34c133404ed3adcf`:
200/200 passed, P95 13,262 ms. Do not combine it with the large holdout to conceal
the latter's one failure or claim a quality improvement. Shared-ledger final total:
8,485 requests, 2,012,922 reported tokens, CNY 3.476439 estimated list price,
zero pending reservations. No further paid calls are planned. Actual invoice and
free-quota application remain unknown. `.cache/harness/python-rag-metrics-audit.json`
reconciles the three batch journals with frozen inputs, runtime snapshots and all
8,400 reservations/settlements in addition to the original 85-call baseline.

The closure source digest is
`bc857cf89612080a0af0e7aa7fde3cae6e16097a876ac8a6da583c78c9a474ce`.
Final full run `675314f8f14270f0304d41e81a717ea1` passed 18/18 checks and all 169
named results. Its 10-household/20-client/30-second load completed 23,037 requests,
zero errors and invariant failures, P95 23.434 ms; the earlier load observation is
retained separately, not discarded as a slower/faster result. The test still
excludes gateway/cache/Kafka/model calls.

Final acceptance run `c5c034a879befd0210fc7d39174c3e88` passed 17/17 checks and
44 named browser results (22 desktop, 22 mobile). Diagnose passed 5/5 and the
offline RAG lifecycle suite passed 8/8, all against the closure source digest.
The metrics failure has a verified resolved revision linked to the full and
acceptance reports. The old unrelated v0.3 model failure remains open.

The completion bundle recheck now passes feedback, full, acceptance and diagnose
validation and stops specifically at the absent main-site report. Current default
containers were observed stopped; no default backup/recreation, Git commit/push,
main-site update or cloud verification was performed. The task remains awaiting
the user-owned delivery steps above or explicit redelegation. It is not marked
successfully finished at that point. After those steps, recompute complete-history
metrics and rerun the same version-2 bundle gate; do not repeat paid calls solely
to close it.

## Authorized default delivery

The user explicitly redelegated paired backup, default Docker deployment, GitHub
publication and acceptance on 2026-10-21. The reviewed helper completed the paired
database/photo backup, verified archive contents and preserved original database
identity, configuration, mounts, data and photos while recreating API/Worker/Web.
Default deployment reports ready with Python 3.13.10 and the closure source digest.
Private backup: `.cache/default-backups/pre-python-rag-metrics-20261021T053857Z`.
Report: `.cache/harness/python-rag-metrics-deployment.json`. No paid probe ran.

Fresh read-only Edge main-site run `97ef8179c3e7a853203236294e0c3552` passed all
five checks, including served build identity at `http://localhost:5173`, against
source `bc857cf89612080a0af0e7aa7fde3cae6e16097a876ac8a6da583c78c9a474ce`.
Report: `.cache/harness/python-rag-metrics-main.json`. Publication and successful
backend/frontend/images/platform jobs for the exact published commit must still
be observed and recorded by cloud mode before final completion; this document
does not predict that result. The private version-2 bundle retains that final
cloud report, recomputed complete-history metrics and task finish outcome.
