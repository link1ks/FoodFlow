"""Offline analysis of FoodFlow semantic/scope evaluation journals; no provider calls."""
import argparse
import hashlib
import html
import json
import math
import re
from pathlib import Path

HERE = Path(__file__).resolve().parent


def strict_json(raw):
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                raise ValueError("duplicate JSON key")
            result[key] = value
        return result
    return json.loads(raw, object_pairs_hook=pairs,
                      parse_constant=lambda _: (_ for _ in ()).throw(ValueError("nonfinite value")))


def integer(value):
    if type(value) is not int or value < 0:
        raise ValueError("expected nonnegative integer")
    return value


def flag(row, key):
    value = row[key]
    if type(value) is not bool:
        raise ValueError("expected boolean metric")
    return value


def ratio(numerator, denominator):
    return numerator / denominator if denominator else None


def analyze(report):
    vision = "manifest_sha256" in report
    digest = report.get("manifest_sha256" if vision else "corpus_sha256", "")
    if not isinstance(digest, str) or not re.fullmatch("[0-9a-f]{64}", digest):
        raise ValueError("missing valid corpus/manifest SHA256")
    rows = report.get("rows")
    if not isinstance(rows, list) or not 1 <= len(rows) <= 10000:
        raise ValueError("expected 1–10000 evaluation rows")
    offline = not vision and "model" not in report and str(report.get("scope", "")).startswith("offline bounded production interpreter;")
    if not offline and (not isinstance(report.get("model"), str) or not re.fullmatch(r"[\w./:-]{1,160}", report["model"])):
        raise ValueError("unknown evaluation origin")
    totals = dict(rows=len(rows), scored=0, correct=0, accepted=0, false_accept=0,
                  negative_cases=0, wrong_draft=0, safe_fallback=0, false_noop=0,
                  input_tokens=0, output_tokens=0, usage_rows=0)
    states, categories, latencies, seen, identities = {}, {}, [], set(), []
    for row in rows:
        if not isinstance(row, dict):
            raise ValueError("invalid sample row")
        ident = row.get("id")
        if not isinstance(ident, str) or not ident or ident in seen:
            raise ValueError("missing/duplicate sample ID")
        seen.add(ident)
        if vision and not re.fullmatch("[0-9a-f]{64}", str(row.get("sha256", ""))):
            raise ValueError("missing image digest")
        identities.append((ident, row.get("sha256", "") if vision else row.get("category", "")))
        state = "offline" if offline else row.get("state")
        if state not in ("offline", "returned", "uncertain", "dispatched", "not_dispatched"):
            raise ValueError("unknown dispatch state")
        states[state] = states.get(state, 0) + 1
        status = integer(row.get("http_status", 0))
        scored = state == "offline" or state == "returned" and status == 200
        usage = row.get("usage", {})
        if type(usage) is not dict:
            raise ValueError("invalid usage")
        input_tokens = integer(usage.get("prompt_tokens", 0))
        output_tokens = integer(usage.get("completion_tokens", 0))
        totals["input_tokens"] += input_tokens
        totals["output_tokens"] += output_tokens
        totals["usage_rows"] += int(input_tokens + output_tokens > 0)
        if vision:
            integer(row.get("latency_ms", 0))
        if not scored:
            continue  # Transport failures, ambiguous and unsent rows are not correct rejections.
        totals["scored"] += 1
        correct = flag(row, "correct" if vision else "strict")
        totals["correct"] += int(correct)
        if vision:
            if not isinstance(row.get("expected_name"), str):
                raise ValueError("missing vision ground truth")
            accepted = flag(row, "accepted")
            negative = row["expected_name"] == ""
            totals["accepted"] += int(accepted)
            totals["negative_cases"] += int(negative)
            totals["false_accept"] += int(negative and accepted)
            latencies.append(row["latency_ms"])
            category = "negative_image" if negative else "ingredient_image"
        else:
            for key in ("wrong_draft", "safe_fallback", "false_noop"):
                totals[key] += int(flag(row, key))
            flag(row, "valid")
            category = row.get("category")
            if not isinstance(category, str) or not re.fullmatch(r"[\w .-]{1,80}", category):
                raise ValueError("invalid category")
        bucket = categories.setdefault(category, {"scored": 0, "correct": 0})
        bucket["scored"] += 1
        bucket["correct"] += int(correct)
    latencies.sort()
    identity = hashlib.sha256(json.dumps(sorted(identities), ensure_ascii=False).encode()).hexdigest()
    provenance = {}
    for key in ("prompt_sha256", "agent_runtime_sha256"):
        if key in report:
            if not isinstance(report[key], str) or not re.fullmatch("[0-9a-f]{64}", report[key]):
                raise ValueError("invalid provenance hash")
            provenance[key] = report[key]
    return dict(version=1, kind="vision" if vision else "semantic",
                origin="offline_rules" if offline else "model_journal",
                model="none" if offline else report["model"], provenance=provenance,
                dataset_sha256=digest, sample_set_sha256=identity,
                counts=totals, states=states, categories=categories,
                coverage=ratio(totals["scored"], len(rows)),
                accuracy_on_scored=ratio(totals["correct"], totals["scored"]),
                correct_over_all_rows=ratio(totals["correct"], len(rows)),
                false_accept_rate=ratio(totals["false_accept"], totals["negative_cases"]) if vision else None,
                latency_p95_ms=latencies[math.ceil(len(latencies) * .95) - 1] if latencies else None,
                latency_samples=len(latencies),
                limits=["Existing labels are trusted, not independently re-labelled.",
                        "Tokens are reported usage, not a verified bill; zero may mean unavailable.",
                        "Accuracy excludes unsent/uncertain/non-200 rows; inspect coverage.",
                        "Offline rule results do not measure real model quality."])


def compare(current, baseline):
    for key in ("kind", "origin", "dataset_sha256", "sample_set_sha256"):
        if current[key] != baseline[key]:
            raise ValueError("comparison requires matching kind, origin, corpus and sample set")
    # Prevent survivorship bias when one run has missing results.
    if current["coverage"] != 1 or baseline["coverage"] != 1:
        raise ValueError("comparison requires complete scored coverage in both reports")
    return {"accuracy_delta": current["accuracy_on_scored"] - baseline["accuracy_on_scored"]}


def load(path):
    raw = path.read_bytes()
    if len(raw) > 32 << 20:
        raise ValueError("evaluation input exceeds 32 MiB")
    result = analyze(strict_json(raw))
    result["input_sha256"] = hashlib.sha256(raw).hexdigest()
    return result


def render(report):
    # Never embed raw input, model output, sample IDs, prompts or media.
    payload = html.escape(json.dumps(report, ensure_ascii=False, indent=2))
    def percent(value):
        return "无数据" if value is None else f"{value:.1%}"
    cards = ""
    if "counts" in report:
        values = [("评测类型", "离线规则测试" if report["origin"] == "offline_rules" else "已有模型评测记录"),
                  ("记录条数", report["counts"]["rows"]), ("评分覆盖率", percent(report["coverage"])),
                  ("已评分样本准确率", percent(report["accuracy_on_scored"])),
                  ("错误生成草稿数", report["counts"]["wrong_draft"] if report["kind"] == "semantic" else "不适用"),
                  ("错误不操作数", report["counts"]["false_noop"] if report["kind"] == "semantic" else "不适用"),
                  ("负例误接受率", percent(report["false_accept_rate"])),
                  ("图片延迟 p95 (ms)", report["latency_p95_ms"] if report["latency_samples"] else "无数据")]
        cards = "<dl>" + "".join(f"<dt>{html.escape(k)}</dt><dd>{html.escape(str(v))}</dd>" for k, v in values) + "</dl>"
        cards += "<h2>分类结果</h2><table><tr><th>类别</th><th>评分样本</th><th>正确</th><th>准确率</th></tr>"
        for key, bucket in sorted(report["categories"].items()):
            cards += (f"<tr><td>{html.escape(key)}</td><td>{bucket['scored']}</td><td>{bucket['correct']}</td>"
                      f"<td>{percent(ratio(bucket['correct'], bucket['scored']))}</td></tr>")
        cards += "</table>"
    return ('<!doctype html><html lang="zh-CN"><meta charset="utf-8">'
            '<meta name="viewport" content="width=device-width,initial-scale=1">'
            '<meta http-equiv="Content-Security-Policy" content="default-src \'none\'; style-src \'unsafe-inline\'">'
            '<title>FoodFlow AI 评测分析</title><style>body{max-width:960px;margin:32px auto;padding:0 20px;'
            'font:16px/1.6 system-ui;background:#f6f8f7;color:#18392e}pre{white-space:pre-wrap;'
            'overflow-wrap:anywhere;background:white;padding:20px;border-radius:12px}'
            'dl{display:grid;grid-template-columns:1fr 1fr;background:white;padding:20px}'
            'dd{font-weight:bold}td,th{text-align:left;padding:8px;border-bottom:1px solid #ccd8d0}'
            'table{width:100%;background:white}details{margin-top:24px}</style>'
            '<h1>FoodFlow AI 评测分析</h1><p>只分析已有记录，无网络请求。'
            'offline_rules 表示规则测试，不代表真实模型效果。先看 coverage，再看 accuracy_on_scored。</p>'
            '<p>延迟仅统计有记录的已评分图片请求，p95 使用 nearest-rank；'
            '准确率差值为比例差（0.01 = 1 个百分点）。报告不包含原始回答或样本内容。</p>'
            f'{cards}<details><summary>完整统计与口径说明（JSON）</summary><pre>{payload}</pre></details></html>')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", type=Path, required=True)
    parser.add_argument("--baseline", type=Path)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    target = args.out.resolve()
    cache = HERE.parent / ".cache"
    if target == cache or not target.is_relative_to(cache) or target.exists():
        parser.error("--out must be a new directory under this repository's .cache/")
    report = load(args.input)
    if args.baseline:
        baseline = load(args.baseline)
        report["comparison"] = compare(report, baseline)
        report["baseline_input_sha256"] = baseline["input_sha256"]
    target.mkdir(parents=True)
    (target / "report.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    (target / "index.html").write_text(render(report), encoding="utf-8")
    print(json.dumps({"origin": report["origin"], "rows": report["counts"]["rows"], "coverage": report["coverage"]}))


if __name__ == "__main__":
    main()
