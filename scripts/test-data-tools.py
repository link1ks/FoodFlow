"""Deterministic offline regressions, called by named Go harness tests."""
import copy
import csv
import importlib.util
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent


def module(name):
    spec = importlib.util.spec_from_file_location(name, HERE / (name + ".py"))
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


nutrition = module("import-nutrition")
evaluation = module("analyze-ai-eval")


class NutritionTests(unittest.TestCase):
    def fixture(self, root):
        def table(path, headers, rows):
            path.parent.mkdir(parents=True, exist_ok=True)
            with path.open("w", encoding="utf-8", newline="") as f:
                writer = csv.writer(f)
                writer.writerow(headers)
                writer.writerows(rows)
        table(root / "usda/food.csv", ["fdc_id", "description"], [["1", 'food, "raw"']])
        table(root / "usda/food_nutrient.csv", ["fdc_id", "nutrient_id", "amount"],
              [["1", "1008", "10.00"], ["1", "1003", "1.234567890123456789"],
               ["1", "1004", "0"], ["1", "1093", ""]])
        table(root / "tfda/20_2.csv", ["整合編號", "分析項", "含量單位", "每100克含量", "樣品名稱", "內容物描述"],
              [["T1", "熱量", "kcal", "10", "fixture", "raw"],
               ["T1", "粗蛋白", "g", "1", "fixture", "raw"], ["T1", "鈉", "mg", "0", "fixture", "raw"]])
        rows = [("分量:", "100 克可食用部份"), ("資料來源:", "CFC2002Ed2"), ("食物名稱:", "fixture"),
                ("能量*", "千卡", "10"), ("蛋白質", "克", "1"), ("膳食纖維*", "克", "2"),
                ("脂肪", "克", "Trace")]
        (root / "cfs-C1.html").write_text("".join("<tr>" + "".join(f"<td>{c}</td>" for c in r) + "</tr>" for r in rows), encoding="utf-8")
        return ({"mapping": {"base": 1}, "notes": {}, "missing": {"unknown": "unknown"}},
                {"usda": {}, "tfda": {"tfda": "T1"}, "cfs": {"cfs": "C1"}, "notes": {}})

    def test_exact_values_unknown_units_fibre_and_provenance(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            base, ext = self.fixture(root)
            profiles, records, hashes = nutrition.build(root, base, ext)
            self.assertEqual(profiles["base"]["nutrients"]["protein_g"], "1.234567890123456789")
            self.assertEqual(profiles["base"]["nutrients"]["fat_g"], "0")
            self.assertNotIn("sodium_mg", profiles["base"]["nutrients"])
            self.assertEqual(profiles["unknown"]["status"], "unavailable")
            self.assertEqual(profiles["cfs"]["nutrients"]["insoluble_fiber_g"], "2")
            self.assertNotIn("fiber_g", profiles["cfs"]["nutrients"])
            self.assertNotIn("fat_g", profiles["cfs"]["nutrients"])
            self.assertEqual(len(records), 3)
            self.assertEqual(len(hashes), 4)
            self.assertEqual((profiles, records, hashes), nutrition.build(root, base, ext))

    def test_reject_bad_numbers_duplicates_missing_ids_and_units(self):
        for value in ("-1", "NaN", "Infinity", "1e3", "1,200", "１２"):
            with self.assertRaises(ValueError):
                nutrition.number(value)
        with self.assertRaises(ValueError):
            nutrition.put({"x": "1"}, "x", "2")
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            base, ext = self.fixture(root)
            with self.assertRaises(ValueError):
                nutrition.usda(root / "usda", [999])
            path = root / "tfda/20_2.csv"
            original = path.read_text(encoding="utf-8")
            path.write_text(original.replace(",mg,", ",g,"), encoding="utf-8")
            with self.assertRaises(ValueError):
                nutrition.build(root, base, ext)
            path.write_text(original, encoding="utf-8")
            path = root / "cfs-C1.html"
            path.write_text(path.read_text(encoding="utf-8").replace("千卡", "kJ"), encoding="utf-8")
            with self.assertRaises(ValueError):
                nutrition.build(root, base, ext)
            path.write_text(path.read_text(encoding="utf-8").replace("kJ", "千卡").replace("100 克", "200 克"), encoding="utf-8")
            with self.assertRaises(ValueError):
                nutrition.build(root, base, ext)

    def test_cli_cannot_write_sources_or_migrations(self):
        result = subprocess.run([sys.executable, "-B", str(HERE / "import-nutrition.py"),
                                 "--sources", "missing", "--out", str(HERE.parent / "sql/schema")],
                                capture_output=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b"new directory", result.stderr)


def semantic():
    row = dict(id="one", category="correction", state="", usage={"prompt_tokens": 0, "completion_tokens": 0},
               valid=True, strict=True, wrong_draft=False, safe_fallback=False, false_noop=False)
    return {"corpus_sha256": "a" * 64, "scope": "offline bounded production interpreter; synthetic implementer-labelled cases",
            "rows": [row]}


class EvaluationTests(unittest.TestCase):
    def test_offline_origin_and_redacted_output(self):
        source = semantic()
        source.update(prompt="SECRET_PROMPT", model_content="SECRET_CONTENT")
        source["rows"][0]["model_content"] = "SECRET_RESPONSE"
        report = evaluation.analyze(source)
        self.assertEqual(report["origin"], "offline_rules")
        self.assertEqual(report["coverage"], 1)
        self.assertEqual(report["accuracy_on_scored"], 1)
        self.assertIsNone(report["latency_p95_ms"])
        self.assertNotIn("SECRET", evaluation.render(report))

    def test_failures_are_not_successful_rejections(self):
        rows = [dict(id="bad", sha256="b" * 64, state="uncertain", http_status=0,
                     expected_name="", accepted=False, correct=True, latency_ms=100, usage={}),
                dict(id="ok", sha256="c" * 64, state="returned", http_status=200,
                     expected_name="", accepted=True, correct=False, latency_ms=20, usage={"prompt_tokens": 12})]
        report = evaluation.analyze(dict(model="fixture", manifest_sha256="a" * 64, rows=rows))
        self.assertEqual(report["coverage"], .5)
        self.assertEqual(report["accuracy_on_scored"], 0)
        self.assertEqual(report["false_accept_rate"], 1)
        self.assertEqual(report["latency_p95_ms"], 20)
        self.assertEqual(report["counts"]["input_tokens"], 12)

    def test_comparison_refuses_different_corpus_subset_origin_and_incomplete(self):
        first = evaluation.analyze(semantic())
        self.assertEqual(evaluation.compare(first, first)["accuracy_delta"], 0)
        for key, value in (("dataset_sha256", "b" * 64), ("sample_set_sha256", "c" * 64),
                           ("origin", "model_journal"), ("coverage", .5)):
            other = copy.deepcopy(first)
            other[key] = value
            with self.assertRaises(ValueError):
                evaluation.compare(first, other)

    def test_empty_duplicate_invalid_boolean_and_nonfinite_rejected(self):
        for rows in ([], semantic()["rows"] * 2, [dict(semantic()["rows"][0], strict="true")]):
            with self.assertRaises(ValueError):
                evaluation.analyze(dict(semantic(), rows=rows))
        for raw in ('{"a":1,"a":2}', '{"a":NaN}'):
            with self.assertRaises(ValueError):
                evaluation.strict_json(raw)

    def test_html_escapes_untrusted_text(self):
        self.assertNotIn("<script>alert", evaluation.render({"limits": ["<script>alert(1)</script>"]}))


if __name__ == "__main__":
    unittest.main()
