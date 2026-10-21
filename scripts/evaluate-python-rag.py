"""Explicit, budget-reserved live RAG evaluation against synthetic frozen cases.

No database, automatic retries, resume or environment changes. Private journals
are exclusively created before sending. Unknown outcomes keep their reservation.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
import math
import os
from pathlib import Path
import subprocess
import sys
import time
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parents[1]
RUNTIME = ROOT/"internal/agent/python/runtime.py"
spec = importlib.util.spec_from_file_location("agent_runtime", RUNTIME)
agent = importlib.util.module_from_spec(spec)
spec.loader.exec_module(agent)


def utc():
    return datetime.now(timezone.utc).isoformat()


def budget(request):
    body, _ = agent.prepare(request)
    # Text-only UTF-8 byte upper estimate plus a deliberately conservative
    # envelope allowance and max output. No image/audio token assumptions.
    bound = len(body)+1024+request["max_tokens"]
    if bound >= 32000 or request["model"] != "qwen3-vl-plus":
        raise ValueError("unpriced model or input tier")
    return bound


def check_budget(reserved, calls, tokens, max_calls, max_tokens, max_cny):
    # Beijing official <=32K input/output rates are 1/10 CNY per million.
    # Reserve all estimated tokens at 30 CNY/million (the highest Beijing tier).
    if calls >= max_calls or reserved+tokens > max_tokens or (reserved+tokens)*30/1000000 > max_cny:
        raise ValueError("authorized budget would be exceeded")


def score(row, expected):
    # Network/parse/schema failures never count as a correct abstention.
    return row.get("http") == 200 and row.get("validated") is True and row.get("answer") == expected


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--corpus", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--paid", action="store_true")
    parser.add_argument("--max-calls", type=int, default=100)
    parser.add_argument("--max-tokens", type=int, default=100000)
    parser.add_argument("--max-cny", type=float, default=5)
    args = parser.parse_args()
    cases_raw = Path(args.corpus).read_bytes()
    cases = json.loads(cases_raw)
    if not cases or len({c["id"] for c in cases}) != len(cases):
        raise ValueError("invalid corpus")
    if not args.paid:
        print(f"offline: {len(cases)} frozen cases; no dispatch")
        return
    if not 1 <= args.max_calls <= 100 or not 1 <= args.max_tokens <= 100000 or not 0 < args.max_cny <= 5:
        raise ValueError("task authorization limit")
    target = Path(args.out).resolve()
    if not target.is_relative_to((ROOT/".cache/harness").resolve()):
        raise ValueError("private journal required")
    cfg = {}
    for line in (ROOT/".env").read_text(encoding="utf-8-sig").splitlines():
        key, sep, value = line.partition("=")
        if sep and key.strip() in ("VISION_MODEL_ENDPOINT", "VISION_MODEL_NAME", "VISION_MODEL_API_KEY"):
            cfg[key.strip()] = value.strip().strip('"').strip("'")
    endpoint = cfg["VISION_MODEL_ENDPOINT"]
    url = urlsplit(endpoint)
    if url.scheme != "https" or url.username or url.query or not url.hostname or not (url.hostname == "dashscope.aliyuncs.com" or url.hostname.endswith(".cn-beijing.maas.aliyuncs.com")):
        raise ValueError("reviewed Beijing HTTPS provider required")
    requests = [dict(version=1, task="rag", endpoint=endpoint, model=cfg["VISION_MODEL_NAME"],
                     key=cfg["VISION_MODEL_API_KEY"], context=c["context"], max_tokens=256, observe=True) for c in cases]
    bounds = [budget(request) for request in requests]
    total = 0
    for n, bound in enumerate(bounds):
        check_budget(total, n, bound, args.max_calls, args.max_tokens, args.max_cny)
        total += bound
    fingerprint = subprocess.run(["go", "run", "./cmd/harness", "-mode", "fingerprint"], cwd=ROOT, check=True, capture_output=True, text=True).stdout.strip()
    target.parent.mkdir(parents=True, exist_ok=True)
    journal = target.open("x", encoding="utf-8")
    report = dict(version=1, task="python-rag-scale", domain="rag", live=True, source_digest=fingerprint,
                  started_at=utc(), corpus_digest=hashlib.sha256(cases_raw).hexdigest(), prompt_digest=hashlib.sha256(RUNTIME.read_bytes()).hexdigest(),
                  config_digest=hashlib.sha256((endpoint+cfg["VISION_MODEL_NAME"]).encode()).hexdigest(),
                  model=cfg["VISION_MODEL_NAME"], model_version="per-row provider response; exact backend revision unknown",
                  cost=None, cost_status="unknown", cost_basis="invoice/free quota unavailable; conservative reservation and list-price estimate only",
                  authorized=dict(max_calls=args.max_calls, max_tokens=args.max_tokens, max_cny=args.max_cny),
                  price_source="https://help.aliyun.com/zh/model-studio/qwen3-vl-plus", reserved_tokens=0,
                  scope="synthetic self-labelled guarded RAG pipeline, including deterministic eligibility filtering; not raw model accuracy, physical devices or production capacity", rows=[])
    def save():
        journal.seek(0)
        journal.write(json.dumps(report, ensure_ascii=False, indent=2))
        journal.truncate()
        journal.flush()
        os.fsync(journal.fileno())
    save()
    environment = {k: os.environ[k] for k in ("SYSTEMROOT", "WINDIR", "TEMP", "TMP", "TMPDIR", "LANG", "TZ") if k in os.environ}
    for case, request, bound in zip(cases, requests, bounds):
        check_budget(report["reserved_tokens"], len(report["rows"]), bound, args.max_calls, args.max_tokens, args.max_cny)
        row = dict(id=case["id"], kind=case["kind"], state="prepared", reserved_tokens=bound, correct=False)
        prepared_body = json.loads(agent.prepare(request)[0])
        row["model_evidence_count"] = len(json.loads(prepared_body["messages"][1]["content"])["evidence"])
        row["input_evidence_count"] = len(case["context"]["evidence"])
        report["rows"].append(row)
        save()
        start = time.perf_counter()
        child = subprocess.Popen([sys.executable, "-I", "-B", "-u", str(RUNTIME)], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, env=environment)
        try:
            child.stdin.write((json.dumps(request, ensure_ascii=False)+"\n").encode())
            child.stdin.flush()
            prepared = json.loads(child.stdout.readline(524288))
            if prepared.get("phase") != "prepared" or prepared.get("request_bytes") != len(agent.prepare(request)[0]):
                raise ValueError("pre-dispatch protocol")
            report["reserved_tokens"] += bound
            row["state"] = "dispatched"
            save()  # Durable intent before sending permit; no retry after ambiguity.
            child.stdin.write(b'{"version":1,"allow":true}\n')
            child.stdin.flush()
            frame = json.loads(child.stdout.readline(524288))
            row["http"] = frame.get("status", 0)
            row["validated"] = not frame.get("error") and frame.get("phase") == "result"
            if row["validated"]:
                row["answer"] = json.loads(frame["content"])
            envelope = json.loads(frame.get("provider_response") or "{}")
            row["usage"] = envelope.get("usage")
            row["model"] = envelope.get("model", "unknown")
            # Only generated text on this reviewed synthetic corpus, never headers,
            # keys, environment, real household content or arbitrary provider logs.
            row["generated_text"] = ((envelope.get("choices") or [{}])[0].get("message") or {}).get("content")
            row["correct"] = score(row, case["expected"])
            row["state"] = "returned" if row["http"] == 200 else "failed_or_uncertain"
        except Exception:
            row["state"] = "failed_or_uncertain"
        finally:
            child.kill()
            child.wait()
            row["latency_ms"] = (time.perf_counter()-start)*1000
            save()
        print(f"{case['id']}: {row['state']}, correct={row['correct']}", flush=True)
        usage = row.get("usage") or {}
        if not isinstance(usage.get("total_tokens"), int) or usage["total_tokens"] > bound:
            report["stopped_reason"] = "missing usage or exceeded reservation; no further dispatch"
            break
    rows = report["rows"]
    tokens = sum((r.get("usage") or {}).get("total_tokens", 0) for r in rows)
    report.update(recorded_at=utc(), samples=len(cases), dispatched=len(rows), correct=sum(r["correct"] for r in rows),
                  service_failures=sum(r.get("http") != 200 for r in rows),
                  p95_ms=math.ceil(sorted(r["latency_ms"] for r in rows)[math.ceil(.95*len(rows))-1]),
                  usage={"total_tokens": tokens}, complete=len(rows)==len(cases),
                  estimated_list_price_cny=sum(((r.get("usage") or {}).get("prompt_tokens", 0)+10*(r.get("usage") or {}).get("completion_tokens", 0))/1000000 for r in rows))
    report["accuracy"] = report["correct"]/report["samples"]
    report["by_kind"] = {kind: dict(samples=sum(c["kind"]==kind for c in cases), correct=sum(r["kind"]==kind and r["correct"] for r in rows)) for kind in sorted({c["kind"] for c in cases})}
    save()
    journal.close()
    print(json.dumps({k: report[k] for k in ("samples", "dispatched", "correct", "accuracy", "p95_ms", "usage", "estimated_list_price_cny", "cost_status", "by_kind")}))


if __name__ == "__main__":
    main()
