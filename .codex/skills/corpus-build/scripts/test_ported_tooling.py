from __future__ import annotations

import contextlib
import importlib.util
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


ROOT = Path(__file__).resolve().parents[4]
SCRIPTS = Path(__file__).resolve().parent

VALIDATOR_SPEC = importlib.util.spec_from_file_location(
    "codenerd_architecture_validator",
    SCRIPTS / "validate_architecture_corpora.py",
)
assert VALIDATOR_SPEC is not None and VALIDATOR_SPEC.loader is not None
VALIDATOR = importlib.util.module_from_spec(VALIDATOR_SPEC)
VALIDATOR_SPEC.loader.exec_module(VALIDATOR)


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

    @contextlib.contextmanager
    def validator_repo(self, root: Path):
        arch = root / "Docs" / "architecture"
        with mock.patch.object(VALIDATOR, "ROOT", root), mock.patch.object(
            VALIDATOR, "ARCH", arch
        ):
            yield arch

    def write_corpus(
        self,
        root: Path,
        corpus_id: str = "alpha",
        *,
        kind: str = "realized",
        roots: list[str] | None = None,
    ) -> Path:
        corpus = root / "Docs" / "architecture" / corpus_id
        corpus.mkdir(parents=True)
        canonical = VALIDATOR.CLI_CANONICAL if corpus_id == "cli" else VALIDATOR.CANONICAL
        for name in canonical:
            (corpus / name).write_text(f"# {name.removesuffix('.md')}\n", encoding="utf-8")
        readme = "# Alpha\n\n" + "\n\n".join(
            f"## {section}\n\nPackage-specific evidence."
            for section in VALIDATOR.README_SECTIONS
        )
        (corpus / "README.md").write_text(readme + "\n", encoding="utf-8")
        (corpus / "TODO.md").write_text(
            """# TODO

<!-- NERD_FEATURE
id: alpha-safe-uplift
owner: alpha
status: proposed
kind: truth-gap
depends_on: []
affects: [alpha]
-->
""".replace("alpha", corpus_id),
            encoding="utf-8",
        )
        if kind == "realized":
            roots = roots or [f"internal/{corpus_id}"]
            for source in roots:
                source_path = root / source
                source_path.mkdir(parents=True, exist_ok=True)
                (source_path / "main.go").write_text(
                    f"package {corpus_id.replace('-', '')}\n\ntype Alpha struct{{}}\n",
                    encoding="utf-8",
                )
            roots_literal = ", ".join(json.dumps(item) for item in roots)
            manifest = f"""schema_version = 1
id = {json.dumps(corpus_id)}
kind = "realized"
source_roots = [{roots_literal}]
entrypoint = "README.md"
implemented_spec = "IMPLEMENTED_SPEC.md"
verified_on = "2026-07-13"
"""
        else:
            manifest = f"""schema_version = 1
id = {json.dumps(corpus_id)}
kind = "proposed"
planned_source_roots = ["internal/{corpus_id}"]
entrypoint = "README.md"
"""
        (corpus / "corpus.toml").write_text(manifest, encoding="utf-8")
        return corpus

    def write_portfolio(
        self,
        root: Path,
        corpus_ids: list[str],
        *,
        schema_version: int = 1,
        exclusion: str = """[[exclusions]]
path = "conductor"
classification = "ecosystem-governance"
reason = "Governance evidence is not a runtime subsystem."
review_on = "2026-10-13"
""",
    ) -> None:
        (root / "conductor").mkdir(exist_ok=True)
        ids = ", ".join(json.dumps(item) for item in corpus_ids)
        (root / "Docs" / "architecture" / "portfolio.toml").write_text(
            f"""schema_version = {schema_version}
coverage_patterns = ["internal/*"]
corpus_ids = [{ids}]

{exclusion}
""",
            encoding="utf-8",
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

    def test_architecture_validator_skips_support_directories(self):
        result = self.run_script("validate_architecture_corpora.py", "--json")
        payload = json.loads(result.stdout)
        ids = {item["id"] for item in payload["corpora"]}
        self.assertNotIn("_rebuild", ids)
        self.assertEqual(payload["measurements"]["corpora"], 38)

    def test_strict_validator_exposes_superstar_migration_gaps(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            with self.validator_repo(root):
                corpus = self.write_corpus(root)
                (corpus / "01-DOMAIN-MODEL.md").write_text(
                    "# Legacy redirect\n\nSee [README](README.md).\n",
                    encoding="utf-8",
                )
                (corpus / "README.md").write_text(
                    "# Alpha\n\n## Vision Alignment\n\nIncomplete migration fixture.\n",
                    encoding="utf-8",
                )
                payload = VALIDATOR.validate(corpus, strict=True)
        self.assertFalse(payload["valid"])
        measurements = payload["measurements"]
        self.assertGreater(measurements["legacy_documents"], 0)
        self.assertGreater(measurements["missing_readme_sections"], 0)

    def test_cli_canonical_documents_are_not_legacy(self):
        result = self.run_script(
            "validate_architecture_corpora.py",
            "--corpus",
            str(ROOT / "Docs" / "architecture" / "cli"),
            "--json",
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        payload = json.loads(result.stdout)
        self.assertEqual(payload["corpora"][0]["measurements"]["legacy_documents"], 0)

    def test_check_is_an_explicit_read_only_alias_and_subset_stays_supported(self):
        result = self.run_script(
            "validate_architecture_corpora.py",
            "--check",
            "--corpus",
            str(ROOT / "Docs" / "architecture" / "cli"),
            "--json",
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(json.loads(result.stdout)["measurements"]["corpora"], 1)

    def test_valid_portfolio_owns_every_expanded_surface_exactly_once(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            with self.validator_repo(root):
                corpus = self.write_corpus(root)
                self.write_portfolio(root, ["alpha"])
                payload = VALIDATOR.portfolio([corpus], strict=True)
        self.assertTrue(payload["valid"], payload)
        self.assertEqual(payload["measurements"]["covered_source_surfaces"], 1)

    def test_portfolio_schema_and_registered_id_parity_are_hard_errors(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            with self.validator_repo(root):
                corpus = self.write_corpus(root)
                self.write_portfolio(root, ["beta"], schema_version=2)
                payload = VALIDATOR.portfolio([corpus], strict=False)
        joined = "\n".join(payload["errors"])
        self.assertIn("schema_version must be 1", joined)
        self.assertIn("registered corpus directories missing: beta", joined)
        self.assertIn("unregistered corpus directories: alpha", joined)
        self.assertIn("do not match corpus.toml ids", joined)

    def test_unowned_nested_and_duplicate_source_roots_fail(self):
        nested = VALIDATOR._ownership_errors(
            [
                {"id": "alpha", "source_roots": ["internal/alpha"], "planned_source_roots": []},
                {"id": "beta", "source_roots": ["internal/alpha/nested"], "planned_source_roots": []},
            ],
            None,
            None,
        )
        duplicate = VALIDATOR._ownership_errors(
            [
                {"id": "alpha", "source_roots": ["internal/shared"], "planned_source_roots": []},
                {"id": "beta", "source_roots": ["internal/shared"], "planned_source_roots": []},
            ],
            None,
            None,
        )
        self.assertTrue(any("nested source-root ownership" in error for error in nested))
        self.assertTrue(any("duplicate source-root ownership" in error for error in duplicate))

        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            with self.validator_repo(root):
                corpus = self.write_corpus(root)
                (root / "internal" / "unowned").mkdir()
                self.write_portfolio(root, ["alpha"])
                payload = VALIDATOR.portfolio([corpus], strict=False)
        self.assertTrue(
            any("no owner or exclusion: internal/unowned" in error for error in payload["errors"]),
            payload,
        )

    def test_exclusions_are_typed_complete_existing_and_reviewable(self):
        bad_exclusion = """[[exclusions]]
path = "missing"
classification = "whatever"
reason = "short"
review_on = "not-a-date"
extra = "typo"
"""
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            with self.validator_repo(root):
                corpus = self.write_corpus(root)
                self.write_portfolio(root, ["alpha"], exclusion=bad_exclusion)
                payload = VALIDATOR.portfolio([corpus], strict=False)
        joined = "\n".join(payload["errors"])
        self.assertIn("unsupported fields: extra", joined)
        self.assertIn("exclusion path does not exist: missing", joined)
        self.assertIn("classification is unsupported", joined)
        self.assertIn("specific non-empty explanation", joined)
        self.assertIn("must be an ISO date", joined)

    def test_realized_and_proposed_manifest_shapes_are_distinct(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            with self.validator_repo(root):
                realized = self.write_corpus(root, "alpha")
                proposed = self.write_corpus(root, "future", kind="proposed")
                _, realized_errors = VALIDATOR._manifest(realized)
                proposed_data, proposed_errors = VALIDATOR._manifest(proposed)
                self.assertEqual(realized_errors, [])
                self.assertEqual(proposed_errors, [])
                self.assertEqual(proposed_data["planned_source_roots"], ["internal/future"])

                (proposed / "corpus.toml").write_text(
                    """schema_version = 1
id = "future"
kind = "proposed"
source_roots = ["internal/future"]
entrypoint = "README.md"
verified_on = "2026-07-13"
""",
                    encoding="utf-8",
                )
                _, malformed = VALIDATOR._manifest(proposed)
                (realized / "corpus.toml").write_text(
                    """schema_version = 1
id = "alpha"
kind = "realized"
source_roots = []
planned_source_roots = ["internal/future"]
entrypoint = "README.md"
""",
                    encoding="utf-8",
                )
                _, malformed_realized = VALIDATOR._manifest(realized)
        self.assertTrue(any("must use planned_source_roots" in error for error in malformed))
        self.assertTrue(any("must not claim verified_on" in error for error in malformed))
        self.assertTrue(any("requires implemented_spec" in error for error in malformed_realized))
        self.assertTrue(any("source_roots must not be empty" in error for error in malformed_realized))
        self.assertTrue(any("must not declare planned_source_roots" in error for error in malformed_realized))
        self.assertTrue(any("verified_on must be" in error for error in malformed_realized))

    def test_feature_cards_require_exact_schema_owner_and_todo_authority(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            with self.validator_repo(root):
                corpus = self.write_corpus(root)
                valid = VALIDATOR.validate(corpus, strict=True)
                self.assertTrue(valid["valid"], valid)

                (corpus / "README.md").write_text(
                    (corpus / "README.md").read_text(encoding="utf-8")
                    + """
<!-- NERD_FEATURE
id: alpha-wrong-place
owner: beta
status: building
kind: vague
depends_on: []
unexpected: yes
-->
""",
                    encoding="utf-8",
                )
                invalid = VALIDATOR.validate(corpus, strict=False)
        joined = "\n".join(invalid["errors"])
        self.assertIn("authoritative only in TODO.md", joined)
        self.assertIn("missing fields", joined)
        self.assertIn("unsupported fields", joined)
        self.assertIn("owner must be 'alpha'", joined)
        self.assertIn("invalid or missing status", joined)
        self.assertIn("invalid or missing kind", joined)

    def test_source_evidence_supports_symbols_and_skips_planned_examples(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            with self.validator_repo(root):
                corpus = self.write_corpus(root)
                evidence = corpus / "10-TESTING-ALIGNMENT.md"
                evidence.write_text(
                    """# Evidence

Live: `internal/alpha/main.go#Alpha`.
Future: `planned:internal/future/new.go#Thing`.
Illustration: `example:cmd/future#Run`.
""",
                    encoding="utf-8",
                )
                positive = VALIDATOR.validate(corpus, strict=True)
                self.assertTrue(positive["valid"], positive)

                evidence.write_text(
                    evidence.read_text(encoding="utf-8")
                    + "\nFalse live claim: `internal/alpha/main.go#MissingSymbol`.\n",
                    encoding="utf-8",
                )
                negative = VALIDATOR.validate(corpus, strict=True)
        self.assertFalse(negative["valid"])
        self.assertTrue(
            any("MissingSymbol" in error for error in negative["errors"]), negative
        )

    def test_source_evidence_accepts_live_globs_and_rejects_empty_globs(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            with self.validator_repo(root):
                corpus = self.write_corpus(root)
                evidence = corpus / "10-TESTING-ALIGNMENT.md"
                evidence.write_text(
                    "# Evidence\n\nLive family: `internal/alpha/**/*.go`.\n",
                    encoding="utf-8",
                )
                positive = VALIDATOR.validate(corpus, strict=True)
                self.assertTrue(positive["valid"], positive)

                evidence.write_text(
                    "# Evidence\n\nMissing family: `internal/alpha/**/*.rs`.\n",
                    encoding="utf-8",
                )
                negative = VALIDATOR.validate(corpus, strict=True)
        self.assertFalse(negative["valid"])
        self.assertTrue(
            any("internal/alpha/**/*.rs" in error for error in negative["errors"]),
            negative,
        )

    def test_markdown_anchor_checks_distinguish_valid_and_broken_targets(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            with self.validator_repo(root):
                corpus = self.write_corpus(root)
                spec = corpus / "IMPLEMENTED_SPEC.md"
                spec.write_text("# Implemented spec\n", encoding="utf-8")
                readme = corpus / "README.md"
                readme.write_text(
                    readme.read_text(encoding="utf-8")
                    + "\n[Proof](IMPLEMENTED_SPEC.md#implemented-spec)\n",
                    encoding="utf-8",
                )
                self.assertTrue(VALIDATOR.validate(corpus, strict=True)["valid"])
                readme.write_text(
                    readme.read_text(encoding="utf-8")
                    + "\n[Broken](IMPLEMENTED_SPEC.md#not-there)\n",
                    encoding="utf-8",
                )
                negative = VALIDATOR.validate(corpus, strict=False)
        self.assertTrue(any("broken anchor" in error for error in negative["errors"]))

    def test_verify_profiles_are_fixed_to_owned_go_packages(self):
        self.assertEqual(
            VALIDATOR._verification_command(
                {"id": "core", "kind": "realized", "source_roots": ["internal/core"]}
            ),
            ["go", "test", "-count=1", "./internal/core/..."],
        )
        self.assertEqual(
            VALIDATOR._verification_command(
                {"id": "cli", "kind": "realized", "source_roots": ["cmd/nerd"]}
            ),
            ["go", "test", "-count=1", "./cmd/nerd/..."],
        )
        self.assertIsNone(
            VALIDATOR._verification_command(
                {"id": "future", "kind": "proposed", "planned_source_roots": ["internal/future"]}
            )
        )

    def test_verify_receipt_is_bounded_and_records_discriminator(self):
        completed = subprocess.CompletedProcess(
            ["go", "test"],
            0,
            stdout="ok\n" + ("x" * 5000),
            stderr="",
        )
        git_commit = subprocess.CompletedProcess(["git"], 0, stdout="abc123\n", stderr="")
        git_status = subprocess.CompletedProcess(["git"], 0, stdout=" M file.go\n", stderr="")
        with mock.patch.object(
            VALIDATOR.subprocess,
            "run",
            side_effect=[git_commit, git_status, completed],
        ):
            receipt = VALIDATOR._verify_corpus(
                {"id": "core", "kind": "realized", "source_roots": ["internal/core"]},
                30,
            )
        self.assertTrue(receipt["valid"], receipt)
        self.assertEqual(receipt["exit_code"], 0)
        self.assertEqual(receipt["commit"], "abc123")
        self.assertEqual(len(receipt["dirty_fingerprint"]), 64)
        self.assertIn("truncated", receipt["stdout"])


if __name__ == "__main__":
    unittest.main()
