"""Reproducible synthetic SCALE measurement, never a real-user accuracy claim."""
import argparse
import hashlib
import importlib.util
import json
import math
import os
from pathlib import Path
import platform
import random
import statistics
import time

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("rag", ROOT/"internal/agent/python/rag.py")
rag = importlib.util.module_from_spec(spec)
spec.loader.exec_module(rag)


def sha(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, ensure_ascii=False).encode()).hexdigest()


def corpus(count):
    for i in range(count):
        yield dict(id=f"synthetic-{i:06}", revision=1, status="resolved", verified=True,
                   symptom=f"component{i//100} incident{i} duplicate inventory", conditions="request replay",
                   root_cause="missing idempotency", fix=f"Check transaction key in component{i//100} incident{i}.",
                   business=["STOCK"], tests=["synthetic#replay"], category="code",
                   error_code=f"GROUP{i//100}", applicable_versions=["synthetic-v1"], code_version="synthetic-v1",
                   scope="fictional scale fixture", evidence=["synthetic-proof"])


def summary(values):
    ordered = sorted(values)
    return {"count": len(values), "p50_ms": statistics.median(values),
            "p95_ms": ordered[math.ceil(.95*len(values))-1], "p99_ms": ordered[math.ceil(.99*len(values))-1],
            "max_ms": max(values)}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--documents", type=int, default=100000)
    parser.add_argument("--queries", type=int, default=10000)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()
    if not 100 <= args.documents <= 1000000 or not 100 <= args.queries <= 100000:
        raise ValueError("bounded scale required")
    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=False)
    records = list(corpus(args.documents))
    digest = sha(records)
    rng = random.Random(20261021)
    cases = []
    for n in range(args.queries):
        i = rng.randrange(args.documents)
        if n % 20 == 0:
            cases.append(dict(kind="no-answer", query=dict(query=f"astronomy{n}"), expected=""))
        elif n % 5 == 0:
            cases.append(dict(kind="filtered-paraphrase", query=dict(query=f"incident{i} 重放", error_code=f"GROUP{i//100}", applicable_version="synthetic-v1"), expected=f"synthetic-{i:06}"))
        else:
            cases.append(dict(kind="identifier", query=dict(query=f"incident{i} component{i//100}"), expected=f"synthetic-{i:06}"))
    # Freeze inputs and denominators before building or observing any result.
    manifest = dict(seed=20261021, documents=args.documents, queries=len(cases), corpus_sha256=digest,
                    queries_sha256=sha(cases), scope="synthetic templates with unique identifiers; not independent human relevance labels",
                    thresholds=dict(hit_at_5=.99, no_answer_accuracy=1, p95_ms=2000),
                    engine_sha256=hashlib.sha256((ROOT/"internal/agent/python/rag.py").read_bytes()).hexdigest())
    (out/"manifest.json").write_text(json.dumps(manifest, indent=2), encoding="utf-8")
    (out/"queries.json").write_text(json.dumps(cases, ensure_ascii=False), encoding="utf-8")
    started = time.perf_counter()
    rag.build(out/"index.sqlite", records, digest)
    build_s = time.perf_counter()-started
    del records
    index = rag.Index(out/"index.sqlite", digest)
    rows, timings, hit, rr, negative, negative_ok = [], {}, 0, 0., 0, 0
    started = time.perf_counter()
    with (out/"queries-results.jsonl").open("x", encoding="utf-8") as journal:
        for n, case in enumerate(cases):
            before = time.perf_counter()
            result = index.query(case["query"])
            elapsed = (time.perf_counter()-before)*1000
            timings.setdefault(case["kind"], []).append(elapsed)
            ids = [r["id"] for r in result]
            if case["expected"]:
                if case["expected"] in ids:
                    hit += 1
                    rr += 1/(ids.index(case["expected"])+1)
            else:
                negative += 1
                negative_ok += not ids
            journal.write(json.dumps(dict(case=n, ids=ids, latency_ms=elapsed))+"\n")
            if (n+1) % 1000 == 0:
                print(f"completed {n+1}/{len(cases)}", flush=True)
    elapsed_s = time.perf_counter()-started
    broad = []
    for _ in range(30):
        before = time.perf_counter()
        index.query(dict(query="duplicate inventory"))
        broad.append((time.perf_counter()-before)*1000)
    index.close()
    positive = len(cases)-negative
    all_ms = [v for vs in timings.values() for v in vs]
    report = dict(manifest=manifest, python=platform.python_version(), platform=platform.platform(), cpu_count=os.cpu_count(),
                  retrieval_version=rag.VERSION, build_seconds=build_s, index_bytes=(out/"index.sqlite").stat().st_size,
                  query_seconds=elapsed_s, sequential_queries_per_second=len(cases)/elapsed_s,
                  latency=summary(all_ms), by_query_kind={k: summary(v) for k,v in timings.items()},
                  broad_unfiltered_latency=summary(broad), hit_at_5=dict(numerator=hit, denominator=positive, value=hit/positive),
                  mrr_at_5=rr/positive, no_answer=dict(numerator=negative_ok, denominator=negative, value=negative_ok/negative),
                  scope="warm read-only Python SQLite engine; excludes Go audit scan/process startup, HTTP, LLM and concurrent users")
    report["passed"] = hit/positive >= .99 and negative_ok == negative and report["latency"]["p95_ms"] <= 2000
    (out/"report.json").write_text(json.dumps(report, indent=2), encoding="utf-8")
    print(json.dumps(report, indent=2))
    if not report["passed"]:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
