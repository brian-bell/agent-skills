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
SKILL_MD = AUDIT_SCRIPT.parents[1] / "SKILL.md"


class RuntimeForkParityAuditTests(unittest.TestCase):
    def test_skill_workflow_is_root_anchored_and_reads_reported_files(self) -> None:
        instructions = SKILL_MD.read_text(encoding="utf-8")

        self.assertIn(
            'repo_root="$(git rev-parse --show-toplevel)"',
            instructions,
        )
        self.assertIn(
            'python3 "$repo_root/.agents/skills/skill-parity-audit/'
            'scripts/audit_runtime_forks.py"',
            instructions,
        )
        self.assertIn('"$repo_root/scripts/test-skill-parity-audit.py"', instructions)
        self.assertIn(
            "every changed, runtime-only, and shared-source-candidate file",
            instructions,
        )

    def test_repository_has_no_blocking_parity_errors(self) -> None:
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

    def test_runtime_trigger_metadata_differences_require_review(self) -> None:
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

            self.assertEqual(result.returncode, 0, result.stderr or result.stdout)
            data = json.loads(json_out.read_text(encoding="utf-8"))
            detail = data["skills"]["demo"]
            self.assertEqual(detail["status"], "review")
            self.assertEqual(detail["errors"], [])
            self.assertEqual(
                detail["metadata_differences"],
                ["runtime descriptions differ"],
            )

    def test_block_scalar_descriptions_are_parsed_before_comparison(self) -> None:
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
                    "description: >\n"
                    f"  {descriptions[runtime]}\n"
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

            self.assertEqual(result.returncode, 0, result.stderr or result.stdout)
            detail = json.loads(json_out.read_text(encoding="utf-8"))["skills"][
                "demo"
            ]
            self.assertEqual(
                detail["metadata"]["claude"]["description"],
                descriptions["claude"],
            )
            self.assertEqual(
                detail["metadata"]["codex"]["description"],
                descriptions["codex"],
            )
            self.assertEqual(
                detail["metadata_differences"],
                ["runtime descriptions differ"],
            )

    def test_indented_delimiter_is_preserved_as_block_scalar_content(self) -> None:
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
                    "description: |\n"
                    "  ---\n"
                    "  Demonstrate runtime parity.\n"
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

            self.assertEqual(result.returncode, 0, result.stderr or result.stdout)
            detail = json.loads(json_out.read_text(encoding="utf-8"))["skills"][
                "demo"
            ]
            self.assertEqual(
                detail["metadata"]["claude"]["description"],
                "---\nDemonstrate runtime parity.",
            )
            self.assertEqual(detail["status"], "pass")

    def test_block_scalar_indentation_indicators_are_parsed(self) -> None:
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
                    "description: >2-\n"
                    f"  {descriptions[runtime]}\n"
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

            self.assertEqual(result.returncode, 0, result.stderr or result.stdout)
            detail = json.loads(json_out.read_text(encoding="utf-8"))["skills"][
                "demo"
            ]
            self.assertEqual(
                detail["metadata"]["claude"]["description"],
                descriptions["claude"],
            )
            self.assertEqual(
                detail["metadata"]["codex"]["description"],
                descriptions["codex"],
            )
            self.assertEqual(
                detail["metadata_differences"],
                ["runtime descriptions differ"],
            )

    def test_folded_block_preserves_more_indented_line_breaks(self) -> None:
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
                    "description: >\n"
                    "  Use:\n"
                    "    command\n"
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

            self.assertEqual(result.returncode, 0, result.stderr or result.stdout)
            detail = json.loads(json_out.read_text(encoding="utf-8"))["skills"][
                "demo"
            ]
            self.assertEqual(
                detail["metadata"]["claude"]["description"],
                "Use:\n  command",
            )
            self.assertEqual(detail["status"], "pass")

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

    def test_executable_mode_differences_require_semantic_review(self) -> None:
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
                    "---\n",
                    encoding="utf-8",
                )
                script = overlay / "scripts" / "run.sh"
                script.parent.mkdir()
                script.write_text("#!/bin/sh\n", encoding="utf-8")
            (skill / "runtimes" / "codex" / "scripts" / "run.sh").chmod(0o755)

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
            self.assertEqual(detail["changed_files"], ["scripts/run.sh"])

    def test_symlink_targets_are_compared_without_dereferencing(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            skill = repo / "skills" / "demo"
            (skill / "shared").mkdir(parents=True)
            targets = {}
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
                targets[runtime] = repo / f"{runtime}-target.sh"
                targets[runtime].write_text("#!/bin/sh\n", encoding="utf-8")
                link = overlay / "scripts" / "run.sh"
                link.parent.mkdir()
                link.symlink_to(targets[runtime])

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
            self.assertEqual(detail["changed_files"], ["scripts/run.sh"])

    def test_identical_support_files_are_shared_source_candidates(self) -> None:
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
                    "---\n",
                    encoding="utf-8",
                )
                reference = overlay / "references" / "contract.md"
                reference.parent.mkdir()
                reference.write_text("Shared contract\n", encoding="utf-8")

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
            self.assertEqual(
                detail["shared_source_candidates"],
                ["references/contract.md"],
            )

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

if __name__ == "__main__":
    unittest.main()
