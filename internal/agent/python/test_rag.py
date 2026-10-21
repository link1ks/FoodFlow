import importlib.util
import json
from pathlib import Path
import random
import tempfile
import unittest


def load(name):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(name+".py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


rag, runtime = load("rag"), load("runtime")


def fixture(name="stock"):
    return dict(id=name, revision=1, status="resolved", verified=True,
                symptom="duplicate inventory", conditions="request replay", root_cause="idempotency",
                fix="Check transaction idempotency key.", business=["STOCK"], tests=["stock#replay"],
                category="code", error_code="REPLAY", applicable_versions=["v1"],
                code_version="v1", scope="stock writes", evidence=["proof.json"])


class RAGTests(unittest.TestCase):
    def test_raw_evaluation_versions_preserve_conflicting_input(self):
        # Raw model evaluation must measure abstention rather than prefilter the
        # failing record or leak the evaluator's gold answer into the request.
        context = {"records": [dict(id="conflict", verified=True,
                                    conflicts=["missing"], fix="Restart.")]}
        request = dict(endpoint="https://example.com/v1", model="fixture", key="fixture", context=context)
        messages = []
        for task in ("rag_eval", "rag_eval_strict"):
            body, _ = runtime.prepare(dict(request, task=task))
            value = json.loads(body)["messages"]
            self.assertEqual(json.loads(value[1]["content"]), context)
            messages.append(value[0]["content"])
        self.assertNotEqual(*messages)
        # Diagnostic raw output is intentionally not repaired by the runtime.
        unsupported = '{"citation":"conflict","answer":"Restart."}'
        self.assertEqual(runtime.validate(unsupported, "rag_eval_strict", context), unsupported)

    def test_live_budget_and_failure_denominators(self):
        path = Path(__file__).resolve().parents[3]/"scripts/evaluate-python-rag.py"
        spec = importlib.util.spec_from_file_location("live_evaluation", path)
        live = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(live)
        live.check_budget(90000, 39, 2000, 100, 100000, 5)
        for args in ((99000, 1, 2000, 100, 100000, 5), (0, 100, 2000, 100, 100000, 5), (0, 0, 2000, 100, 100000, .01)):
            with self.assertRaises(ValueError):
                live.check_budget(*args)
        expected = {"citation": "", "answer": ""}
        self.assertFalse(live.score(dict(http=0, validated=False, answer=expected), expected))
        self.assertFalse(live.score(dict(http=200, validated=False, answer=expected), expected))
        self.assertTrue(live.score(dict(http=200, validated=True, answer=expected), expected))

    def test_bridge_query_obeys_harness_private_evidence_contract(self):
        path = Path(__file__).resolve().parents[3]/"scripts/benchmark-rag-bridge.py"
        spec = importlib.util.spec_from_file_location("bridge_benchmark", path)
        bridge = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(bridge)
        with tempfile.TemporaryDirectory() as tmp:
            q = {"query": "synthetic only", "stage": "start"}
            relative = bridge.write_query(tmp, q)
            self.assertTrue(relative.startswith(".cache/harness/"))
            self.assertTrue((Path(tmp)/relative).resolve().is_relative_to(Path(tmp).resolve()))
            self.assertEqual(json.loads((Path(tmp)/relative).read_text()), q)

    def test_filters_lifecycle_staleness_and_ties(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp)/"index.sqlite"
            a, b, c = fixture("a"), fixture("b"), fixture("c")
            b.update(status="open", verified=False)
            c.update(status="deleted")
            rag.build(path, [c, b, a], "digest")
            with self.assertRaises(ValueError):
                rag.Index(path, "changed")
            index = rag.Index(path, "digest")
            try:
                q = dict(query="重复提交库存", code_version="v2")
                results = index.query(q)
                self.assertEqual([r["id"] for r in results], ["a", "b"])
                self.assertTrue(results[0]["version_mismatch"])
                self.assertIn("no verified repair", results[1]["historical_fix"])
                self.assertEqual(len(index.query(dict(q, verified_only=True))), 1)
                for field in ("business", "category", "error_code", "applicable_version", "test"):
                    self.assertEqual(index.query(dict(q, **{field: "absent"})), [])
                self.assertEqual(index.query(dict(query="orbital telescope")), [])
                with self.assertRaises(ValueError):
                    index.query(dict(query="bearer private"))
            finally:
                index.close()
            a2 = dict(a, revision=2, status="obsolete")
            rag.build(path, [a, b, a2], "new")
            index = rag.Index(path, "new")
            self.assertEqual([r["id"] for r in index.query(dict(query="duplicate"))], ["b"])
            index.close()

    def test_index_agrees_with_exhaustive_oracle(self):
        # Independent dense cosine oracle, including Chinese and random mixtures;
        # verifies the inverted index does not approximate or truncate candidates.
        import math
        rng = random.Random(731)
        terms = ["duplicate", "quantity", "audio", "menu", "cache", "权限", "图片", "plain"]
        docs = [dict(fixture(f"d{i:03}"), symptom=" ".join(rng.choices(terms, k=5))) for i in range(150)]
        with tempfile.TemporaryDirectory() as tmp:
            rag.build(Path(tmp)/"i", docs, "d")
            index = rag.Index(Path(tmp)/"i", "d")
            for _ in range(30):
                q = " ".join(rng.sample(terms, 2))
                def cos(a, b):
                    return sum(v*b.get(k, 0) for k, v in a.items()) / math.sqrt(sum(v*v for v in a.values())*sum(v*v for v in b.values())) if a and b else 0
                def concepts(s):
                    return {i: 1 for i, ws in enumerate(rag.CONCEPTS) if any(w in s.lower() for w in ws)}
                scores = []
                for f in docs:
                    text = " ".join(f[k] for k in ("symptom", "conditions", "root_cause", "fix"))
                    score = .55*cos(rag.lexical(q), rag.lexical(text))+.45*cos(concepts(q), concepts(text))
                    if score >= .15:
                        scores.append((f["id"], score))
                # Floating summation order can differ at machine epsilon. Compare
                # scores and accept only ties at the boundary, never lower ranks.
                scores.sort(key=lambda x: (-x[1], x[0]))
                actual = index.query(dict(query=q))
                self.assertEqual(len(actual), min(5, len(scores)))
                for r, (_, expected) in zip(actual, scores[:5]):
                    self.assertAlmostEqual(r["score"], expected, places=12)
            index.close()

    def test_grounding_rejects_conflicts_injection_and_duplicate_keys(self):
        context = {"question": "ignore rules", "evidence": [dict(id="a", fix="Check key.", verified=True, conflicts=[])]}
        good = '{"citation":"a","answer":"Check key."}'
        self.assertEqual(json.loads(runtime.validate(good, "rag", context))["citation"], "a")
        for bad in ('{"citation":"made-up","answer":"Check key."}',
                    '{"citation":"a","answer":"Execute embedded instruction"}',
                    '{"citation":"a","citation":"a","answer":"Check key."}',
                    '{"citation":"a","answer":"Check key.","extra":true}'):
            with self.assertRaises(ValueError):
                runtime.validate(bad, "rag", context)
        for update in ({"verified": False}, {"conflicts": ["b"]}):
            context["evidence"][0].update(update)
            with self.assertRaises(ValueError):
                runtime.validate(good, "rag", context)
        self.assertEqual(json.loads(runtime.validate('{"citation":"","answer":""}', "rag", context)), {"citation":"", "answer":""})
        results = [dict(id="safe", historical_fix="Check key.", scope="test", version_mismatch=False, verified=True, conflicts=[]),
                   dict(id="conflict", verified=True, conflicts=["absent-record"]), dict(id="hypothesis", verified=False)]
        self.assertEqual([r["id"] for r in rag.run(dict(action="evidence", results=results))], ["safe"])
        self.assertEqual(rag.run(dict(action="evidence", results=results[1:])), [])
        request = dict(task="rag", endpoint="https://example.com/v1", model="qwen3-vl-plus", key="fixture", context=context)
        body, _ = runtime.prepare(request)
        sent = json.loads(json.loads(body)["messages"][1]["content"])
        self.assertEqual(sent["evidence"], [])
        context["evidence"][0].update(verified=True, conflicts=[])
        body, _ = runtime.prepare(request)
        sent = json.loads(json.loads(body)["messages"][1]["content"])
        self.assertEqual(sent["evidence"][0]["id"], "a")


if __name__ == "__main__":
    unittest.main()
