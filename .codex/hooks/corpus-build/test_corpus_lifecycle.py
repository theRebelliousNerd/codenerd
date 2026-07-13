from __future__ import annotations

import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path


HOOKS = Path(__file__).resolve().parent


class CorpusLifecycleHookTests(unittest.TestCase):
    def invoke(self, script: str, root: Path, payload: dict) -> subprocess.CompletedProcess:
        env = os.environ.copy()
        env["CODEX_HOOK_REPO_ROOT"] = str(root)
        return subprocess.run(
            [
                "powershell",
                "-NoProfile",
                "-NonInteractive",
                "-ExecutionPolicy",
                "Bypass",
                "-File",
                str(HOOKS / script),
            ],
            input=json.dumps(payload),
            capture_output=True,
            text=True,
            env=env,
            timeout=20,
            check=False,
        )

    def test_start_and_stop_are_scoped_and_usage_is_explicit(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            ledger = root / ".corpus-build" / "ledger"
            ledger.mkdir(parents=True)
            (ledger / "session-1.active").write_text(
                json.dumps({"run_id": "run-1", "phase": "build", "skill": "corpus-build"}),
                encoding="utf-8",
            )
            base = {"session_id": "session-1", "agent_type": "corpus_builder", "agent_id": "agent-1"}
            started = self.invoke("corpus-fleet-start.ps1", root, base)
            stopped = self.invoke("corpus-token-meter.ps1", root, base)
            self.assertEqual(started.returncode, 0, started.stderr)
            self.assertEqual(stopped.returncode, 0, stopped.stderr)
            events = [json.loads(line) for line in (ledger / "fleet_events.jsonl").read_text(encoding="utf-8-sig").splitlines()]
            self.assertEqual([item["event"] for item in events], ["start", "stop"])
            self.assertEqual(events[1]["token_measurement"], "unavailable")

            with_usage = {**base, "agent_id": "agent-2", "usage": {"input_tokens": 12, "output_tokens": 5}}
            measured = self.invoke("corpus-token-meter.ps1", root, with_usage)
            self.assertEqual(measured.returncode, 0, measured.stderr)
            last = json.loads((ledger / "fleet_events.jsonl").read_text(encoding="utf-8-sig").splitlines()[-1])
            self.assertEqual(last["token_measurement"], "hook_payload")
            self.assertEqual(last["billable_total"], 17)

    def test_no_active_run_is_a_quiet_noop(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            result = self.invoke(
                "corpus-fleet-start.ps1",
                root,
                {"session_id": "missing", "agent_type": "corpus_builder"},
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertFalse((root / ".corpus-build").exists())


if __name__ == "__main__":
    unittest.main()
