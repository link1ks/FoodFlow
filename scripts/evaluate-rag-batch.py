"""Bounded concurrent live evaluation with a shared durable spend ledger.

Authorization for this task: cumulative <=8500 requests and <=CNY 5 at reviewed
Beijing qwen3-vl-plus list prices. Unknown outcomes retain their full reservation.
All inputs are synthetic; never use private user data with this evaluation tool.
"""
import argparse
from concurrent.futures import ThreadPoolExecutor, wait, FIRST_COMPLETED
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
import math
import os
import re
from pathlib import Path
import subprocess
import sys
import threading
import time
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parents[1]
RUNTIME = ROOT/"internal/agent/python/runtime.py"
spec = importlib.util.spec_from_file_location("agent_runtime", RUNTIME)
agent = importlib.util.module_from_spec(spec)
spec.loader.exec_module(agent)
NANO = 1000000000


def utc():
    return datetime.now(timezone.utc).isoformat()


class Ledger:
    """A single process owns a lock file; workers serialize pre-send permits."""
    def __init__(self, path, baseline_calls=85, baseline_nano=22396000, cap_calls=8500, cap_nano=5*NANO):
        self.path = Path(path)
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self.lock_path = self.path.with_suffix(".lock")
        self.file_lock = self.lock_path.open("x")
        self.file_lock.write(str(os.getpid()))
        self.file_lock.flush()
        self.mutex = threading.Lock()
        self.calls, self.spent, self.pending = 0, 0, {}
        self.seen = set()
        self.cap_calls, self.cap_nano = cap_calls, cap_nano
        self.events = []
        try:
            if self.path.exists():
                self.events = [json.loads(line) for line in self.path.read_text(encoding="utf-8").splitlines()]
                for event in self.events:
                    self._apply(event)
            self.file = self.path.open("a", encoding="utf-8")
            if not self.events:
                self._append(dict(event="baseline", calls=baseline_calls, nano=baseline_nano))
        except Exception:
            self.file_lock.close()
            self.lock_path.unlink()
            raise

    def _apply(self, e):
        if e["event"] == "baseline":
            if self.calls or self.seen:
                raise ValueError("duplicate baseline")
            self.calls, self.spent = e["calls"], e["nano"]
        elif e["event"] == "reserve":
            if e["id"] in self.seen:
                raise ValueError("duplicate request; never replay paid calls")
            self.seen.add(e["id"])
            self.pending[e["id"]] = e["nano"]
            self.calls += 1
        elif e["event"] == "settle":
            if e["id"] not in self.pending:
                raise ValueError("settlement without reservation")
            del self.pending[e["id"]]
            self.spent += e["nano"]
        else:
            raise ValueError("unknown ledger event")

    def _append(self, event):
        self.file.write(json.dumps(dict(event, at=utc()))+"\n")
        self.file.flush()
        os.fsync(self.file.fileno())
        self._apply(event)

    def reserve(self, identity, input_bound, output_bound):
        # Integer nanoyuan: 1 CNY/M input tokens, 10 CNY/M output tokens.
        nano = input_bound*1000+output_bound*10000
        with self.mutex:
            if identity in self.seen:
                raise ValueError("request already attempted")
            if self.calls >= self.cap_calls or self.spent+sum(self.pending.values())+nano > self.cap_nano:
                return False
            self._append(dict(event="reserve", id=identity, nano=nano, input_bound=input_bound, output_bound=output_bound))
            return True

    def settle(self, identity, usage, input_bound, output_bound):
        # Missing/ambiguous usage never refunds a reservation.
        if not isinstance(usage, dict):
            return False
        a, b = usage.get("prompt_tokens"), usage.get("completion_tokens")
        if type(a) is not int or type(b) is not int or not 0 <= a <= input_bound or not 0 <= b <= output_bound:
            return False
        if usage.get("total_tokens") != a+b:
            return False
        with self.mutex:
            self._append(dict(event="settle", id=identity, nano=a*1000+b*10000, input_tokens=a, output_tokens=b))
        return True

    def summary(self):
        with self.mutex:
            return dict(cumulative_calls=self.calls, estimated_list_price_cny=self.spent/NANO,
                        retained_uncertain_reservations_cny=sum(self.pending.values())/NANO,
                        max_calls=self.cap_calls, max_cny=self.cap_nano/NANO)

    def close(self):
        self.file.close()
        self.file_lock.close()
        self.lock_path.unlink()


def load_config():
    cfg = {}
    for line in (ROOT/".env").read_text(encoding="utf-8-sig").splitlines():
        k, sep, v = line.partition("=")
        if sep and k.strip() in ("VISION_MODEL_ENDPOINT", "VISION_MODEL_NAME", "VISION_MODEL_API_KEY"):
            cfg[k.strip()] = v.strip().strip('"').strip("'")
    url = urlsplit(cfg["VISION_MODEL_ENDPOINT"])
    if cfg["VISION_MODEL_NAME"] != "qwen3-vl-plus" or not cfg["VISION_MODEL_API_KEY"] or url.scheme != "https" or url.username or url.query or not url.hostname:
        raise ValueError("reviewed model/provider required")
    if not (url.hostname == "dashscope.aliyuncs.com" or url.hostname.endswith(".cn-beijing.maas.aliyuncs.com")):
        raise ValueError("reviewed Beijing endpoint required")
    return cfg


def call(case, cfg, ledger, runtime_path):
    request = dict(version=1, task="rag", endpoint=cfg["VISION_MODEL_ENDPOINT"], model=cfg["VISION_MODEL_NAME"],
                   key=cfg["VISION_MODEL_API_KEY"], context=case["context"], max_tokens=128, observe=True)
    body, _ = agent.prepare(request)
    input_bound = len(body)+1024
    if input_bound >= 32000:
        raise ValueError("unpriced input tier")
    if not ledger.reserve(case["id"], input_bound, 128):
        return None
    row = dict(id=case["id"], kind=case["kind"], state="dispatched", correct=False, http=0, validated=False,
               model_evidence_count=len(agent.rag_evidence(case["context"])), input_evidence_count=len(case["context"]["evidence"]))
    start = time.perf_counter()
    env = {k: os.environ[k] for k in ("SYSTEMROOT", "WINDIR", "TEMP", "TMP", "TMPDIR", "LANG", "TZ") if k in os.environ}
    child = None
    timer = None
    try:
        child = subprocess.Popen([sys.executable, "-I", "-B", "-u", str(runtime_path)], stdin=subprocess.PIPE,
                                 stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, env=env)
        timer = threading.Timer(40, child.kill)
        timer.start()
        child.stdin.write((json.dumps(request, ensure_ascii=False)+"\n").encode())
        child.stdin.flush()
        prepared = json.loads(child.stdout.readline(524288))
        if prepared.get("phase") != "prepared" or prepared.get("request_bytes") != len(body):
            raise ValueError("protocol mismatch")
        child.stdin.write(b'{"version":1,"allow":true}\n')
        child.stdin.flush()
        frame = json.loads(child.stdout.readline(524288))
        row["http"] = frame.get("status", 0)
        row["validated"] = not frame.get("error") and frame.get("phase") == "result"
        if row["validated"]:
            row["answer"] = json.loads(frame["content"])
        envelope = json.loads(frame.get("provider_response") or "{}")
        row["usage"] = envelope.get("usage")
        row["reported_model"] = envelope.get("model", "unknown")
        row["generated_text"] = ((envelope.get("choices") or [{}])[0].get("message") or {}).get("content")
        row["correct"] = row["http"] == 200 and row["validated"] and row["answer"] in case["expected"]
        row["state"] = "returned" if row["http"] == 200 else "service_failure"
        row["usage_settled"] = ledger.settle(case["id"], row["usage"], input_bound, 128)
    except Exception:
        row["state"] = "failed_or_uncertain"
    finally:
        if timer:
            timer.cancel()
        if child:
            child.kill()
            child.wait()
        row["latency_ms"] = math.ceil((time.perf_counter()-start)*1000)
    return row


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--corpus", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--ledger", default=".cache/harness/evaluations/python-rag-industrial-ledger.jsonl")
    parser.add_argument("--workers", type=int, default=16)
    parser.add_argument("--task", default="python-rag-industrial-eval")
    parser.add_argument("--paid", action="store_true")
    args = parser.parse_args()
    if not re.fullmatch(r"[a-z0-9][a-z0-9-]{0,63}", args.task):
        raise ValueError("valid task association required")
    raw = Path(args.corpus).read_bytes()
    cases = json.loads(raw)
    if not 1 <= args.workers <= 16 or not 1 <= len(cases) <= 8415 or len({c["id"] for c in cases}) != len(cases):
        raise ValueError("bounded unique corpus required")
    if not args.paid:
        print(f"offline: {len(cases)} cases")
        return
    out, ledger_path = Path(args.out).resolve(), Path(args.ledger).resolve()
    private = (ROOT/".cache/harness").resolve()
    if not out.is_relative_to(private) or not ledger_path.is_relative_to(private):
        raise ValueError("private outputs required")
    if ledger_path != private/"evaluations/python-rag-industrial-ledger.jsonl":
        raise ValueError("this authorization uses one shared ledger; cannot reset its budget")
    cfg = load_config()
    source = subprocess.run(["go", "run", "./cmd/harness", "-mode", "fingerprint"], cwd=ROOT, capture_output=True, text=True, check=True).stdout.strip()
    out.mkdir(parents=True, exist_ok=False)
    runtime_path = out/"runtime.py"
    runtime_path.write_bytes(RUNTIME.read_bytes())
    report = dict(version=1, task=args.task, domain="rag", live=True, source_digest=source,
                  started_at=utc(), corpus_digest=hashlib.sha256(raw).hexdigest(), prompt_digest=hashlib.sha256(runtime_path.read_bytes()).hexdigest(),
                  config_digest=hashlib.sha256((cfg["VISION_MODEL_ENDPOINT"]+cfg["VISION_MODEL_NAME"]).encode()).hexdigest(),
                  model=cfg["VISION_MODEL_NAME"], model_version="per-row; exact backend revision unknown", workers=args.workers,
                  samples=len(cases), cost=None, cost_status="unknown", cost_basis="List-price estimate; actual invoice and free quota unavailable",
                  scope="Synthetic self-labelled guarded pipeline permutations, not independent real-world accuracy")
    (out/"manifest.json").write_text(json.dumps(report, indent=2))
    rows, stopped, failures = [], "", 0
    ledger = Ledger(ledger_path)
    started = time.perf_counter()
    journal = (out/"results.jsonl").open("x", encoding="utf-8")
    try:
        if any(c["id"] in ledger.seen for c in cases):
            raise ValueError("corpus includes previously attempted request; no resume")
        iterator = iter(cases)
        with ThreadPoolExecutor(max_workers=args.workers) as pool:
            pending = set()
            def submit():
                c = next(iterator, None)
                if c is not None:
                    pending.add(pool.submit(call, c, cfg, ledger, runtime_path))
            for _ in range(args.workers):
                submit()
            while pending:
                done, pending = wait(pending, return_when=FIRST_COMPLETED)
                for future in done:
                    row = future.result()
                    if row is None:
                        stopped = "budget cap reached"
                    else:
                        rows.append(row)
                        journal.write(json.dumps(row, ensure_ascii=False)+"\n")
                        journal.flush()
                        os.fsync(journal.fileno())
                        if not row.get("usage_settled") or row["http"] != 200:
                            failures += 1
                        if failures >= 5:
                            stopped = "five service/unknown-usage outcomes; stop before further spend"
                        if len(rows) % 100 == 0:
                            print(json.dumps(dict(completed=len(rows), correct=sum(r["correct"] for r in rows), elapsed_s=round(time.perf_counter()-started,1), **ledger.summary())), flush=True)
                    if not stopped:
                        submit()
        report["budget"] = ledger.summary()
    finally:
        journal.close()
        ledger.close()
    report.update(recorded_at=utc(), attempted=len(rows), complete=len(rows)==len(cases), stopped_reason=stopped,
                  correct=sum(r["correct"] for r in rows), service_failures=sum(r["http"]!=200 for r in rows), rows=rows,
                  p95_ms=sorted(r["latency_ms"] for r in rows)[math.ceil(.95*len(rows))-1] if rows else 0,
                  usage={"total_tokens":sum((r.get("usage") or {}).get("total_tokens",0) for r in rows)},
                  elapsed_s=time.perf_counter()-started)
    report["accuracy"] = report["correct"]/len(cases)
    report["by_kind"] = {k:dict(samples=sum(c["kind"]==k for c in cases), correct=sum(r["kind"]==k and r["correct"] for r in rows)) for k in sorted({c["kind"] for c in cases})}
    report["passed"] = report["complete"] and report["accuracy"] >= .95 and report["p95_ms"] <= 30000 and all(v["correct"]/v["samples"] >= .90 for v in report["by_kind"].values())
    (out/"report.json").write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding="utf-8")
    print(json.dumps({k:report[k] for k in ("samples","attempted","correct","accuracy","p95_ms","budget","by_kind","stopped_reason")}))
    if not report["passed"]:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
