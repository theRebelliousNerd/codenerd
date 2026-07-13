"""Tests for the Claude-to-Codex workspace inventory helper."""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from inventory_workspace import build_report


class InventoryWorkspaceTests(unittest.TestCase):
    """Exercise mixed roots, config parsing, and failure reporting."""

    def test_reports_divergent_duplicate_skills_and_config(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            agents_skill = root / ".agents" / "skills" / "shared"
            codex_skill = root / ".codex" / "skills" / "shared"
            agents_skill.mkdir(parents=True)
            codex_skill.mkdir(parents=True)
            agents_skill.joinpath("SKILL.md").write_text("agents", encoding="utf-8")
            codex_skill.joinpath("SKILL.md").write_text("codex", encoding="utf-8")
            root.joinpath(".codex", "config.toml").write_text(
                '[[skills.config]]\npath = ".codex/skills/shared"\nenabled = true\n'
                '\n[agents.scout]\nconfig_file = ".codex/agents/scout.toml"\n',
                encoding="utf-8",
            )
            agents = root / ".codex" / "agents"
            agents.mkdir()
            agents.joinpath("scout.toml").write_text(
                'name = "scout"\ndescription = "Scout"\n'
                'developer_instructions = "Inspect only."\nmodel = "gpt-5.6"\n',
                encoding="utf-8",
            )

            report = build_report(root)

            self.assertEqual(report["duplicate_skill_names"], ["shared"])
            self.assertEqual(report["divergent_duplicate_skill_names"], ["shared"])
            self.assertEqual(
                report["configured_skills"],
                [{"path": ".codex/skills/shared", "enabled": True}],
            )
            self.assertFalse(report["errors"])
            self.assertEqual(len(report["custom_agents"]), 1)
            self.assertEqual(report["configured_agents"][0]["name"], "scout")
            self.assertFalse(report["unregistered_custom_agents"])
            self.assertFalse(report["incomplete_custom_agents"])

    def test_reports_invalid_config_without_crashing(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            config = root / ".codex" / "config.toml"
            config.parent.mkdir(parents=True)
            config.write_text("[broken", encoding="utf-8")

            report = build_report(root)

            self.assertEqual(len(report["errors"]), 1)
            self.assertIn("cannot parse .codex/config.toml", report["errors"][0])

    def test_inventory_is_empty_for_missing_optional_surfaces(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            report = build_report(Path(temp_dir))

            self.assertFalse(report["errors"])
            self.assertEqual(report["skill_roots"][".agents/skills"], {})
            self.assertEqual(report["skill_roots"][".codex/skills"], {})


if __name__ == "__main__":
    unittest.main()
