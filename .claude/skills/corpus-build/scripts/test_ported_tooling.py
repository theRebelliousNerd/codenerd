from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
SCRIPTS = Path(__file__).resolve().parent


class PortedCorpusToolingTests(unittest.TestCase):
    def run_script(self, name: str, *args: str) -> subprocess.CompletedProcess:
        return subprocess.run(
            [sys.executable, str(SCRIPTS / name), *args],
            cwd=ROOT,
            capture_output=True,
            text=True,
            timeout=30,
            check=False,
        )

    def test_dag_accepts_depends_on_and_rejects_unknown_dependencies(self):
        with tempfile.TemporaryDirectory() as raw:
            path = Path(raw) / "plan.json"
            path.write_text(json.dumps({"work_units": [
                {"id": "WU-1", "depends_on": [], "write_paths": ["internal/example/a.go"]},
                {"id": "WU-2", "depends_on": ["WU-1"], "write_paths": ["internal/example/b.go"]},
            ]}), encoding="utf-8")
            valid = self.run_script("build_dag.py", str(path), "--json")
            self.assertEqual(valid.returncode, 0, valid.stderr)
            self.assertEqual(json.loads(valid.stdout)["total_levels"], 2)
            path.write_text(json.dumps({"work_units": [{"id": "WU-1", "depends_on": ["missing"]}]}), encoding="utf-8")
            invalid = self.run_script("build_dag.py", str(path), "--json")
            self.assertNotEqual(invalid.returncode, 0)
            self.assertIn("Unknown dependencies", invalid.stderr)

    def test_compatibility_cost_script_never_estimates(self):
        with tempfile.TemporaryDirectory() as raw:
            path = Path(raw) / "plan.json"
            path.write_text(json.dumps({"work_units": [{"id": "WU-1", "level": 0, "owner": "corpus_builder", "acceptance": ["go test ./..."]}]}), encoding="utf-8")
            result = self.run_script("cost_estimate.py", str(path), "--json")
            self.assertEqual(result.returncode, 0, result.stderr)
            payload = json.loads(result.stdout)
            self.assertIsNone(payload["estimated_tokens"])
            self.assertIsNone(payload["estimated_cost_usd"])

    def test_port_residue_and_common_reference_checks_pass(self):
        residue = self.run_script("mutate_ecosystem_for_codenerd.py", "--json")
        self.assertEqual(residue.returncode, 0, residue.stdout)
        self.assertFalse(json.loads(residue.stdout)["write_mode"])
        sync = self.run_script("sync_common_refs.py", "--check")
        self.assertEqual(sync.returncode, 0, sync.stdout + sync.stderr)

    def test_surface_registry_is_live_and_bounded(self):
        result = self.run_script("verify_surfaces.py", "--json")
        self.assertEqual(result.returncode, 0, result.stderr)
        payload = json.loads(result.stdout)
        self.assertGreaterEqual(len(payload["surfaces"]), 15)
        self.assertTrue(all(item["verdict"] in {"DISCOVERED", "UNAVAILABLE"} for item in payload["surfaces"]))


if __name__ == "__main__":
    unittest.main()
