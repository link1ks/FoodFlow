import importlib.util
from concurrent.futures import ThreadPoolExecutor
import json
from pathlib import Path
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[3]


def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


batch = load("batch", ROOT/"scripts/evaluate-rag-batch.py")
generator = load("generator", ROOT/"scripts/generate-rag-evaluation.py")


class BatchTests(unittest.TestCase):
    def test_concurrent_budget_unknowns_and_exclusive_journal(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp)/"ledger.jsonl"
            ledger = batch.Ledger(path, baseline_calls=0, baseline_nano=0, cap_calls=10, cap_nano=2000000)
            with self.assertRaises(FileExistsError):
                batch.Ledger(path)
            with ThreadPoolExecutor(max_workers=16) as pool:
                accepted = list(pool.map(lambda i: ledger.reserve(str(i), 1000, 100), range(32)))
            self.assertEqual(sum(accepted), 1)
            identity = next(iter(ledger.pending))
            self.assertFalse(ledger.settle(identity, None, 1000, 100))
            self.assertEqual(ledger.summary()["retained_uncertain_reservations_cny"], .002)
            self.assertFalse(ledger.reserve("extra", 1, 1))
            with self.assertRaises(ValueError):
                ledger.reserve(identity, 1, 1)
            self.assertTrue(ledger.settle(identity, dict(prompt_tokens=10, completion_tokens=2, total_tokens=12), 1000, 100))
            self.assertEqual(ledger.summary()["estimated_list_price_cny"], .00003)
            self.assertTrue(ledger.reserve("later", 10, 10))
            ledger.close()
            replay = batch.Ledger(path)
            self.assertEqual(replay.calls, 2)
            self.assertEqual(replay.spent, 30000)
            self.assertIn("later", replay.pending)
            replay.close()

    def test_call_limit_cannot_be_refunded_and_usage_is_bounded(self):
        with tempfile.TemporaryDirectory() as tmp:
            ledger = batch.Ledger(Path(tmp)/"ledger", baseline_calls=8499, baseline_nano=0)
            self.assertTrue(ledger.reserve("last", 100, 10))
            self.assertFalse(ledger.settle("last", dict(prompt_tokens=101, completion_tokens=0, total_tokens=101), 100, 10))
            self.assertTrue(ledger.settle("last", dict(prompt_tokens=10, completion_tokens=1, total_tokens=11), 100, 10))
            self.assertFalse(ledger.reserve("over", 1, 1))
            ledger.close()

    def test_frozen_corpus_strata_labels_and_scope_filter(self):
        cases = generator.generate(200, 937, "test")
        self.assertEqual(cases, generator.generate(200, 937, "test"))
        self.assertEqual(len({c["id"] for c in cases}), 200)
        self.assertEqual({k:sum(c["kind"]==k for c in cases) for k in generator.KINDS}, {k:20 for k in generator.KINDS})
        for case in cases:
            context = case["context"]
            request = dict(task="rag", endpoint="https://example.com/v1", model="qwen3-vl-plus", key="fixture", context=context)
            body, _ = batch.agent.prepare(request)
            transmitted = json.loads(json.loads(body)["messages"][1]["content"])
            self.assertNotIn("expected", transmitted)
            eligible = {r["id"] for r in batch.agent.rag_evidence(context)}
            for expected in case["expected"]:
                if expected["citation"]:
                    self.assertIn(expected["citation"], eligible)
                else:
                    self.assertEqual(eligible, set())
            for record in context["evidence"]:
                if record["id"] not in eligible:
                    with self.assertRaises(ValueError):
                        batch.agent.validate(json.dumps(dict(citation=record["id"],answer=record["fix"])), "rag", context)


if __name__ == "__main__":
    unittest.main()
