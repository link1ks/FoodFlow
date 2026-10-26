"""Offline lesson: synthetic households, fake model, process-local memory only."""
import importlib.util
import json
from pathlib import Path

from langgraph.store.memory import InMemoryStore

spec = importlib.util.spec_from_file_location("foodflow_lesson_runtime", Path(__file__).with_name("runtime.py"))
runtime = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runtime)


def lesson():
    store = InMemoryStore()
    namespace = ("fictional-household-a", "preferences")
    # Represents an explicit user save, NOT a model-inferred fact.
    store.put(namespace, "confirmed", {"text": "少盐，不吃花生", "source": "explicit_user_save"})
    remembered = store.get(namespace, "confirmed").value
    request = {"task": "plan", "endpoint": "http://fixture.invalid", "model": "offline-fixture", "key": "fictional",
               "context": {"user_preference": "做一顿快手晚餐", "household_preferences": remembered["text"],
                           "available_inventory": [{"name": "番茄", "quantity": "500", "unit": "g"}],
                           "allowed_recipes": [{"ID": "tomato-eggs", "Title": "番茄炒蛋", "Minutes": 10}]}}
    calls = []
    def fake_model(_request, body, _progress):
        calls.append(len(body))
        return '{"recipe_id":"tomato-eggs"}'
    graph = runtime.build_agent(request, {}, lambda _: True, fake_model)
    steps = []
    with runtime.tracing_context(enabled=False):
        for update in graph.stream({}, {"recursion_limit": 12}):
            for node, state in update.items():
                steps.append({"node": node, "context_keys": sorted(state.get("context", {})),
                              "tool_steps": state.get("tool_steps"), "content": state.get("content")})
    return {"mode": "offline_fixture_no_network", "steps": steps, "model_calls": len(calls),
            "remembered_preference": remembered,
            "other_household_has_memory": store.get(("fictional-household-b", "preferences"), "confirmed") is not None,
            "model_output_saved_as_memory": store.get(namespace, "model_output") is not None,
            "persistence": "InMemoryStore is lost when this process exits; production preferences live in Go-owned PostgreSQL."}


if __name__ == "__main__":
    print(json.dumps(lesson(), ensure_ascii=False, indent=2))
