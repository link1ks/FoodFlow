"""Measure the real Go audit scan + Python subprocess path on synthetic files."""
import argparse
import importlib.util
import json
from pathlib import Path
import subprocess
import time

spec = importlib.util.spec_from_file_location("scale", Path(__file__).with_name("benchmark-rag.py"))
scale = importlib.util.module_from_spec(spec)
spec.loader.exec_module(scale)


def write_query(root, query):
    relative = ".cache/harness/query.json"
    target = Path(root)/relative
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(json.dumps(query), encoding="utf-8")
    return relative


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--harness", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--documents", type=int, default=100000)
    parser.add_argument("--queries", type=int, default=20)
    args = parser.parse_args()
    out = Path(args.out).resolve()
    out.mkdir(parents=True, exist_ok=False)
    records = out/".cache/harness/experience/records"
    records.mkdir(parents=True)
    (out/"synthetic-proof").write_text("Fictional benchmark provenance, no actual failures or users.\n")
    for f in scale.corpus(args.documents):
        (records/(f["id"]+"-000001.json")).write_text(json.dumps(f), encoding="utf-8")
    binary = str(Path(args.harness).resolve())
    def run(*argv):
        return subprocess.run([binary, "-mode", "experience", *argv], cwd=out, check=True, capture_output=True)
    started = time.perf_counter()
    run("-action", "index")
    build = time.perf_counter()-started
    samples = []
    for i in range(args.queries):
        q = dict(task="synthetic-scale", stage="start", query=f"incident{i*17} component{i*17//100}", disposition="synthetic measurement only")
        query_path = write_query(out, q)
        started = time.perf_counter()
        run("-action", "query", "-evidence", query_path, "-out", f"result-{i}.json")
        samples.append((time.perf_counter()-started)*1000)
        result = json.loads((out/f"result-{i}.json").read_text())
        if result["results"][0]["id"] != f"synthetic-{i*17:06}":
            raise ValueError("bridge retrieval mismatch")
        print(f"bridge {i+1}/{args.queries}: {samples[-1]:.1f} ms", flush=True)
    report = dict(documents=args.documents, queries=args.queries, latency=scale.summary(samples), build_seconds=build,
                  scope="Go executable + complete immutable JSON history scan/hash + Python process + SQLite query + citation checks + query journal; synthetic files only")
    (out/"report.json").write_text(json.dumps(report, indent=2))
    print(json.dumps(report, indent=2))


if __name__ == "__main__":
    main()
