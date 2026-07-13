import importlib.util
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("inject-subagent-memory-context.py")
SPEC = importlib.util.spec_from_file_location("inject_subagent_memory_context", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC and SPEC.loader
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class InjectSubagentMemoryContextTests(unittest.TestCase):
    def test_injects_exact_memory_for_hyphenated_agent(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            path = root / ".claude" / "agent-memory" / "corpus-builder" / "MEMORY.md"
            path.parent.mkdir(parents=True)
            path.write_text("# Builder memory\n\nUse attention \u2192 ONNX \u2192 simple.\n", encoding="utf-8")

            output = MODULE.hook_output({"agent_type": "corpus-builder"}, root)

            self.assertEqual("SubagentStart", output["hookSpecificOutput"]["hookEventName"])
            self.assertEqual(path.read_bytes().decode("utf-8-sig"), output["hookSpecificOutput"]["additionalContext"])

    def test_maps_registered_underscore_name_to_hyphenated_memory_directory(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            path = root / ".claude" / "agent-memory" / "corpus-packetizer" / "MEMORY.md"
            path.parent.mkdir(parents=True)
            path.write_text("packet memory", encoding="utf-8")

            output = MODULE.hook_output({"agent_type": "corpus_packetizer"}, root)

            self.assertEqual("packet memory", output["hookSpecificOutput"]["additionalContext"])

    def test_missing_or_unsafe_agent_type_is_a_quiet_noop(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            self.assertEqual({}, MODULE.hook_output({"agent_type": "missing"}, root))
            self.assertEqual({}, MODULE.hook_output({"agent_type": "../escape"}, root))
            self.assertEqual({}, MODULE.hook_output({}, root))

    def test_oversized_memory_is_not_injected(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            path = root / ".claude" / "agent-memory" / "large-agent" / "MEMORY.md"
            path.parent.mkdir(parents=True)
            path.write_bytes(b"x" * (MODULE.MAX_MEMORY_BYTES + 1))

            output = MODULE.hook_output({"agent_type": "large-agent"}, root)

            self.assertIn("systemMessage", output)
            self.assertNotIn("hookSpecificOutput", output)


if __name__ == "__main__":
    unittest.main()
