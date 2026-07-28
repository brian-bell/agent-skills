#!/usr/bin/env python3
"""Integration tests for the project-scoped runtime-fork parity audit."""

from __future__ import annotations

import json
import subprocess
import tempfile
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[1]
AUDIT_SCRIPT = (
    REPO_ROOT
    / ".agents"
    / "skills"
    / "skill-parity-audit"
    / "scripts"
    / "audit_runtime_forks.py"
)


class RuntimeForkParityAuditTests(unittest.TestCase):
    def test_identical_runtime_forks_pass(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            skill = repo / "skills" / "demo"
            (skill / "shared").mkdir(parents=True)
            for runtime in ("claude", "codex"):
                overlay = skill / "runtimes" / runtime
                overlay.mkdir(parents=True)
                (overlay / "SKILL.md").write_text(
                    "---\n"
                    "name: demo\n"
                    "description: Demonstrate runtime parity.\n"
                    "---\n"
                    "\n"
                    "# Demo\n",
                    encoding="utf-8",
                )

            json_out = repo / "audit.json"
            result = subprocess.run(
                [
                    "python3",
                    str(AUDIT_SCRIPT),
                    str(repo),
                    "--json-out",
                    str(json_out),
                ],
                check=False,
                capture_output=True,
                text=True,
            )

            self.assertEqual(result.returncode, 0, result.stderr or result.stdout)
            data = json.loads(json_out.read_text(encoding="utf-8"))
            self.assertEqual(data["summary"]["skill_count"], 1)
            self.assertEqual(data["summary"]["error_count"], 0)
            self.assertEqual(data["summary"]["review_count"], 0)
            self.assertEqual(data["skills"]["demo"]["status"], "pass")

    def test_missing_runtime_fork_is_a_blocking_error(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            skill = repo / "skills" / "demo"
            (skill / "shared").mkdir(parents=True)
            claude = skill / "runtimes" / "claude"
            claude.mkdir(parents=True)
            (claude / "SKILL.md").write_text(
                "---\n"
                "name: demo\n"
                "description: Demonstrate runtime parity.\n"
                "---\n",
                encoding="utf-8",
            )

            json_out = repo / "audit.json"
            result = subprocess.run(
                [
                    "python3",
                    str(AUDIT_SCRIPT),
                    str(repo),
                    "--json-out",
                    str(json_out),
                ],
                check=False,
                capture_output=True,
                text=True,
            )

            self.assertEqual(result.returncode, 1)
            data = json.loads(json_out.read_text(encoding="utf-8"))
            self.assertEqual(data["summary"]["error_count"], 1)
            self.assertEqual(data["skills"]["demo"]["status"], "error")
            self.assertIn(
                "missing runtimes/codex/SKILL.md",
                data["skills"]["demo"]["errors"],
            )

    def test_runtime_trigger_metadata_must_match(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            skill = repo / "skills" / "demo"
            (skill / "shared").mkdir(parents=True)
            descriptions = {
                "claude": "Demonstrate Claude parity.",
                "codex": "Demonstrate Codex parity.",
            }
            for runtime in ("claude", "codex"):
                overlay = skill / "runtimes" / runtime
                overlay.mkdir(parents=True)
                (overlay / "SKILL.md").write_text(
                    "---\n"
                    "name: demo\n"
                    f"description: {descriptions[runtime]}\n"
                    "---\n",
                    encoding="utf-8",
                )

            json_out = repo / "audit.json"
            result = subprocess.run(
                [
                    "python3",
                    str(AUDIT_SCRIPT),
                    str(repo),
                    "--json-out",
                    str(json_out),
                ],
                check=False,
                capture_output=True,
                text=True,
            )

            self.assertEqual(result.returncode, 1)
            data = json.loads(json_out.read_text(encoding="utf-8"))
            detail = data["skills"]["demo"]
            self.assertEqual(detail["status"], "error")
            self.assertIn("runtime descriptions differ", detail["errors"])

    def test_each_runtime_requires_trigger_description(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            skill = repo / "skills" / "demo"
            (skill / "shared").mkdir(parents=True)
            for runtime in ("claude", "codex"):
                overlay = skill / "runtimes" / runtime
                overlay.mkdir(parents=True)
                (overlay / "SKILL.md").write_text(
                    "---\n"
                    "name: demo\n"
                    "---\n",
                    encoding="utf-8",
                )

            json_out = repo / "audit.json"
            result = subprocess.run(
                [
                    "python3",
                    str(AUDIT_SCRIPT),
                    str(repo),
                    "--json-out",
                    str(json_out),
                ],
                check=False,
                capture_output=True,
                text=True,
            )

            self.assertEqual(result.returncode, 1)
            detail = json.loads(json_out.read_text(encoding="utf-8"))["skills"][
                "demo"
            ]
            self.assertIn(
                "runtimes/claude/SKILL.md is missing description",
                detail["errors"],
            )
            self.assertIn(
                "runtimes/codex/SKILL.md is missing description",
                detail["errors"],
            )

    def test_runtime_specific_differences_are_reported_for_semantic_review(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            skill = repo / "skills" / "demo"
            (skill / "shared").mkdir(parents=True)
            bodies = {
                "claude": "# Demo\n\nUse the Agent tool.\n",
                "codex": "# Demo\n\nUse native subagents.\n",
            }
            for runtime in ("claude", "codex"):
                overlay = skill / "runtimes" / runtime
                overlay.mkdir(parents=True)
                (overlay / "SKILL.md").write_text(
                    "---\n"
                    "name: demo\n"
                    "description: Demonstrate runtime parity.\n"
                    "---\n\n"
                    f"{bodies[runtime]}",
                    encoding="utf-8",
                )
            (skill / "runtimes" / "claude" / "findings-schema.md").write_text(
                "Claude schema\n", encoding="utf-8"
            )
            codex_agents = skill / "runtimes" / "codex" / "agents"
            codex_agents.mkdir()
            (codex_agents / "openai.yaml").write_text(
                "interface:\n  display_name: Demo\n", encoding="utf-8"
            )

            json_out = repo / "audit.json"
            result = subprocess.run(
                [
                    "python3",
                    str(AUDIT_SCRIPT),
                    str(repo),
                    "--json-out",
                    str(json_out),
                ],
                check=False,
                capture_output=True,
                text=True,
            )

            self.assertEqual(result.returncode, 0, result.stderr or result.stdout)
            detail = json.loads(json_out.read_text(encoding="utf-8"))["skills"][
                "demo"
            ]
            self.assertEqual(detail["status"], "review")
            self.assertEqual(detail["changed_files"], ["SKILL.md"])
            self.assertEqual(detail["claude_only_files"], ["findings-schema.md"])
            self.assertEqual(detail["codex_only_files"], ["agents/openai.yaml"])

    def test_each_runtime_declares_the_first_party_skill_name(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            skill = repo / "skills" / "demo"
            (skill / "shared").mkdir(parents=True)
            names = {"claude": "demo", "codex": "other-name"}
            for runtime in ("claude", "codex"):
                overlay = skill / "runtimes" / runtime
                overlay.mkdir(parents=True)
                (overlay / "SKILL.md").write_text(
                    "---\n"
                    f"name: {names[runtime]}\n"
                    "description: Demonstrate runtime parity.\n"
                    "---\n",
                    encoding="utf-8",
                )

            json_out = repo / "audit.json"
            result = subprocess.run(
                [
                    "python3",
                    str(AUDIT_SCRIPT),
                    str(repo),
                    "--json-out",
                    str(json_out),
                ],
                check=False,
                capture_output=True,
                text=True,
            )

            self.assertEqual(result.returncode, 1)
            detail = json.loads(json_out.read_text(encoding="utf-8"))["skills"][
                "demo"
            ]
            self.assertIn(
                "runtimes/codex/SKILL.md declares name other-name, expected demo",
                detail["errors"],
            )

    def test_first_party_skill_requires_shared_directory(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            skill = repo / "skills" / "demo"
            for runtime in ("claude", "codex"):
                overlay = skill / "runtimes" / runtime
                overlay.mkdir(parents=True)
                (overlay / "SKILL.md").write_text(
                    "---\n"
                    "name: demo\n"
                    "description: Demonstrate runtime parity.\n"
                    "---\n",
                    encoding="utf-8",
                )

            json_out = repo / "audit.json"
            result = subprocess.run(
                [
                    "python3",
                    str(AUDIT_SCRIPT),
                    str(repo),
                    "--json-out",
                    str(json_out),
                ],
                check=False,
                capture_output=True,
                text=True,
            )

            self.assertEqual(result.returncode, 1)
            detail = json.loads(json_out.read_text(encoding="utf-8"))["skills"][
                "demo"
            ]
            self.assertIn("missing shared/", detail["errors"])

    def test_repository_has_no_blocking_runtime_parity_errors(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            json_out = Path(tmp) / "audit.json"
            result = subprocess.run(
                [
                    "python3",
                    str(AUDIT_SCRIPT),
                    str(REPO_ROOT),
                    "--json-out",
                    str(json_out),
                ],
                check=False,
                capture_output=True,
                text=True,
            )

            self.assertEqual(result.returncode, 0, result.stderr or result.stdout)
            data = json.loads(json_out.read_text(encoding="utf-8"))
            self.assertEqual(data["summary"]["error_count"], 0)
            self.assertNotIn("skill-parity-audit", data["skills"])


if __name__ == "__main__":
    unittest.main()
