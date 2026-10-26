"""Real compiled graphs, deterministic model fixtures, no business DB/network."""
import importlib.util
import json
from pathlib import Path
import unittest
from unittest.mock import Mock, patch

spec = importlib.util.spec_from_file_location("foodflow_graph", Path(__file__).with_name("runtime.py"))
agent = importlib.util.module_from_spec(spec)
spec.loader.exec_module(agent)


def request(recipe="known"):
    return {"task": "plan", "key": "fictional", "model": "fixture", "endpoint": "http://127.0.0.1",
            "context": {"household_preferences": "少盐", "available_inventory": [],
                        "allowed_recipes": [{"ID": recipe}], "database_password": "must-not-appear"}}


class GraphContracts(unittest.TestCase):
    def test_offline_learning_example_has_explicit_memory_and_isolation(self):
        spec = importlib.util.spec_from_file_location("lesson", Path(__file__).with_name("learn.py"))
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        with patch.object(agent.http.client, "HTTPConnection") as connection:
            result = module.lesson()
            connection.assert_not_called()
        self.assertEqual(result["model_calls"], 1)
        self.assertEqual(result["remembered_preference"]["source"], "explicit_user_save")
        self.assertFalse(result["other_household_has_memory"])
        self.assertFalse(result["model_output_saved_as_memory"])
        self.assertEqual(result["steps"][-1]["node"], "validate_result")

    def test_tool_loop_observations_reach_model_then_validate(self):
        sent = []
        def transport(req, body, progress):
            sent.append(body)
            self.assertEqual(json.loads(body)["messages"][1]["content"], agent.dump({
                "household_preferences": "少盐", "available_inventory": [], "allowed_recipes": [{"ID": "known"}]}))
            return '{"recipe_id":"known"}'
        permission = Mock(return_value=True)
        graph = agent.build_agent(request(), {}, permission, transport)
        with agent.tracing_context(enabled=False):
            updates = list(graph.stream({}, {"recursion_limit": 12}))
        self.assertEqual([next(iter(u)) for u in updates], ["load_context", "read_tool", "read_tool", "read_tool",
                         "prepare_model", "authorize", "call_model", "validate_result"])
        self.assertEqual(updates[-1]["validate_result"]["content"], '{"recipe_id":"known"}')
        self.assertEqual(len(sent), 1)
        permission.assert_called_once_with(sent[0])
        self.assertNotIn(b"must-not-appear", sent[0])

    def test_tools_reject_scope_override_and_copy_snapshots(self):
        source = request()
        readers = agent.context_tools(source)
        source["context"]["allowed_recipes"][0]["ID"] = "changed"
        reader = readers["read_recipes"]
        with agent.tracing_context(enabled=False):
            for arguments in ({"scope": "foreign"}, {"scope": "current_request", "household_id": "foreign"},
                              {"scope": "current_request", "sql": "delete"}, {"scope": "current_request", "url": "https://example.org"}, {}):
                with self.subTest(arguments=arguments), self.assertRaises(ValueError):
                    reader.invoke(arguments)
            result = reader.invoke({"scope": "current_request"})
            result["allowed_recipes"][0]["ID"] = "mutated"
            self.assertEqual(reader.invoke({"scope": "current_request"})["allowed_recipes"], [{"ID": "known"}])
        self.assertNotIn("write_inventory", readers)

    def test_rejection_and_invalid_output_never_retry(self):
        for permit in (False, 1, None):
            transport = Mock()
            with agent.tracing_context(enabled=False):
                state = agent.build_agent(request(), {}, lambda _: permit, transport).invoke({})
            transport.assert_not_called()
            self.assertFalse(state["authorized"])
        for answer in ('{"recipe_id":"invented"}', '{"recipe_id":"known","extra":true}'):
            transport = Mock(return_value=answer)
            with agent.tracing_context(enabled=False), self.assertRaises(ValueError):
                agent.build_agent(request(), {}, lambda _: True, transport).invoke({})
            transport.assert_called_once()

    def test_request_state_and_preferences_do_not_leak_across_runs(self):
        for recipe in ("household-a", "household-b"):
            transport = Mock(return_value=agent.dump({"recipe_id": recipe}))
            with agent.tracing_context(enabled=False):
                graph = agent.build_agent(request(recipe), {}, lambda _: True, transport)
                state = graph.invoke({})
            self.assertEqual(state["context"]["allowed_recipes"], [{"ID": recipe}])
            self.assertEqual(state["tool_steps"], 3)
            self.assertIsNone(graph.checkpointer)
            self.assertNotIn("key", state)

    def test_memory_retrieval_requires_verified_unconflicted_matching_evidence(self):
        good = {"id": "verified", "fix": "reviewed repair", "verified": True, "conflicts": [],
                "error_code": "E1", "applicable_versions": ["v1"]}
        context = {"question": "help", "error_code": "E1", "applicable_version": "v1",
                   "evidence": [good, {**good, "id": "guess", "verified": False},
                                {**good, "id": "conflict", "conflicts": ["missing"]},
                                {**good, "id": "old", "applicable_versions": ["v0"]}]}
        req = {**request(), "task": "rag", "context": context}
        with agent.tracing_context(enabled=False):
            observed = agent.context_tools(req)["read_verified_memory"].invoke({"scope": "current_request"})
        self.assertEqual(observed["evidence"], [good])
        with self.assertRaises(ValueError):
            agent.validate('{"citation":"guess","answer":"reviewed repair"}', "rag", observed)

    def test_provider_tool_calls_are_rejected_with_known_debit(self):
        progress = {"known_complete": False}
        response = Mock(status=200)
        response.read.return_value = json.dumps({"choices": [{"message": {
            "content": '{"recipe_id":"known"}', "tool_calls": [{"name": "write_inventory"}]}}]}).encode()
        with patch.object(agent.http.client, "HTTPConnection") as factory:
            factory.return_value.getresponse.return_value = response
            with agent.tracing_context(enabled=False), self.assertRaises(ValueError):
                agent.build_agent(request(), progress, lambda _: True).invoke({})
            self.assertEqual(factory.return_value.request.call_count, 1)
            factory.return_value.close.assert_called_once()
        self.assertTrue(progress["known_complete"])
        self.assertEqual(progress["status"], 200)

    def test_tool_failure_stops_before_permission_or_network(self):
        permission, transport = Mock(), Mock()
        reader = Mock()
        reader.invoke.side_effect = ValueError("fixture failure")
        with patch.object(agent, "context_tools", return_value={"read_recipes": reader}):
            with agent.tracing_context(enabled=False), self.assertRaises(ValueError):
                agent.build_agent(request(), {}, permission, transport).invoke({})
        permission.assert_not_called()
        transport.assert_not_called()

    def test_demo_graph_never_dispatches_or_requests_allowance(self):
        for req in ({"task": "demo", "recipe_id": "known"},
                    {"task": "demo_advice", "context": {"allowed_recipes": []}}):
            permission, transport = Mock(), Mock()
            with agent.tracing_context(enabled=False):
                state = agent.build_agent(req, {}, permission, transport).invoke({})
            permission.assert_not_called()
            transport.assert_not_called()
            self.assertTrue(state["demo"])
            self.assertTrue(state["content"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
