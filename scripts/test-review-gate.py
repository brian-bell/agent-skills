#!/usr/bin/env python3
from __future__ import annotations

import json
import os
import shutil
import signal
import stat
import subprocess
import tempfile
import textwrap
import time
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
REVIEW_GATE = ROOT / "skills/review-gate/shared/scripts/review-gate"


def run(
    args: list[str],
    *,
    cwd: Path,
    env: dict[str, str] | None = None,
    check: bool = True,
) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(
        args,
        cwd=cwd,
        env={**os.environ, **(env or {})},
        text=True,
        capture_output=True,
        check=False,
    )
    if check and result.returncode != 0:
        raise AssertionError(
            f"command failed ({result.returncode}): {' '.join(args)}\n"
            f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
        )
    return result


def write_executable(path: Path, contents: str) -> None:
    path.write_text(textwrap.dedent(contents).lstrip(), encoding="utf-8")
    path.chmod(path.stat().st_mode | stat.S_IXUSR)


class ReviewGateTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory(prefix="review-gate-test-")
        self.temp = Path(self.temp_dir.name)
        self.repo = self.temp / "repo"
        self.repo.mkdir()
        run(["git", "init", "-q", "-b", "main"], cwd=self.repo)
        run(["git", "config", "user.name", "Review Gate Test"], cwd=self.repo)
        run(["git", "config", "user.email", "review-gate@example.test"], cwd=self.repo)
        (self.repo / "app.txt").write_text("before\n", encoding="utf-8")
        (self.repo / "delete.txt").write_text("delete me\n", encoding="utf-8")
        (self.repo / "rename.txt").write_text("rename me\n", encoding="utf-8")
        (self.repo / "consumer.txt").write_text(
            "unchanged consumer\n", encoding="utf-8"
        )
        write_executable(
            self.repo / "verify.sh",
            """
            #!/bin/sh
            set -eu
            test "$(cat app.txt)" = "after"
            printf 'verification passed\\n'
            """,
        )
        run(
            [
                "git",
                "add",
                "app.txt",
                "consumer.txt",
                "delete.txt",
                "rename.txt",
                "verify.sh",
            ],
            cwd=self.repo,
        )
        run(["git", "commit", "-qm", "initial"], cwd=self.repo)
        (self.repo / "app.txt").write_text("after\n", encoding="utf-8")
        run(["git", "add", "app.txt"], cwd=self.repo)
        run(["git", "commit", "-qm", "change app"], cwd=self.repo)

        self.native_log = self.temp / "native.log"
        self.challenger_input_log = self.temp / "challenger-input.log"
        self.adjudicator_input_log = self.temp / "adjudicator-input.log"
        self.native = self.temp / "native"
        write_executable(
            self.native,
            """
            #!/bin/sh
            set -eu
            printf '%s\\n' "$*" > "$NATIVE_LOG"
            printf '{"findings":[],"checked_clean":true}\\n'
            """,
        )

        self.challenger = self.temp / "challenger"
        write_executable(
            self.challenger,
            """
            #!/usr/bin/env python3
            import json
            import os
            import pathlib
            import sys

            args = sys.argv[1:]
            prompt = sys.stdin.read()
            output = pathlib.Path(args[args.index("-o") + 1])
            if os.environ.get("CHALLENGER_STARTED"):
                pathlib.Path(os.environ["CHALLENGER_STARTED"]).write_text(
                    "started",
                    encoding="utf-8",
                )
            if (
                output.parent.name != "native"
                and os.environ.get("CHALLENGER_INPUT_LOG")
            ):
                pathlib.Path(os.environ["CHALLENGER_INPUT_LOG"]).write_text(
                    prompt,
                    encoding="utf-8",
                )
            environment_key = (
                "NATIVE_NORMALIZED_RESULT"
                if output.parent.name == "native"
                else "CHALLENGER_RESULT"
            )
            payload = json.loads(
                os.environ.get(
                    environment_key,
                    '{"findings":[],"checked_clean":true}',
                )
            )
            output.write_text(
                json.dumps(payload),
                encoding="utf-8",
            )
            """,
        )

        self.gh = self.temp / "gh"
        write_executable(
            self.gh,
            """
            #!/usr/bin/env python3
            import os
            print(os.environ["GH_PAYLOAD"])
            """,
        )
        self.command_env = {
            "NATIVE_LOG": str(self.native_log),
            "CHALLENGER_INPUT_LOG": str(self.challenger_input_log),
            "ADJUDICATOR_INPUT_LOG": str(self.adjudicator_input_log),
        }

        self.adjudicator = self.temp / "adjudicator"
        write_executable(
            self.adjudicator,
            """
            #!/usr/bin/env python3
            import json
            import os
            import pathlib
            import sys

            args = sys.argv[1:]
            prompt = sys.stdin.read()
            if os.environ.get("ADJUDICATOR_INPUT_LOG"):
                pathlib.Path(os.environ["ADJUDICATOR_INPUT_LOG"]).write_text(
                    prompt,
                    encoding="utf-8",
                )
            output = pathlib.Path(args[args.index("-o") + 1])
            payload = json.loads(
                os.environ.get(
                    "ADJUDICATOR_RESULT",
                    '{"decisions":[],"native_raw_coverage":'
                    '{"complete":true,"rationale":"Normalized output matches."},'
                    '"native_coverage":[],'
                    '"challenger_coverage":[],"checked_clean":true}',
                )
            )
            output.write_text(
                json.dumps(payload),
                encoding="utf-8",
            )
            """,
        )

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def invoke(
        self,
        *args: str,
        output_format: str = "json",
    ) -> subprocess.CompletedProcess[str]:
        return run(
            [
                str(REVIEW_GATE),
                "--repo",
                str(self.repo),
                *args,
                "--native-bin",
                str(self.native),
                "--challenger-bin",
                str(self.challenger),
                "--adjudicator-bin",
                str(self.adjudicator),
                "--format",
                output_format,
            ],
            cwd=self.repo,
            env=self.command_env,
            check=False,
        )

    def test_reviews_an_immutable_commit_end_to_end(self) -> None:
        commit = run(["git", "rev-parse", "HEAD"], cwd=self.repo).stdout.strip()

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
        )

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        report = json.loads(result.stdout)
        self.assertEqual(report["status"], "clean")
        self.assertEqual(report["exit_code"], 0)
        self.assertEqual(report["accepted_findings"], [])
        self.assertEqual(report["rejected_findings"], [])
        self.assertEqual(report["target"]["mode"], "commit")
        self.assertEqual(report["target"]["commit"], commit)
        self.assertIn("app.txt", report["target"]["changed_paths"])
        self.assertEqual(
            report["stages"]["native_normalization"]["result"],
            {"findings": [], "checked_clean": True},
        )
        self.assertTrue(report["stages"]["native"]["cwd"].endswith("/snapshot"))
        self.assertEqual(report["stages"]["verification"]["status"], "passed")
        self.assertEqual(
            set(report["stages"]),
            {
                "target",
                "snapshot",
                "native",
                "native_normalization",
                "challenger",
                "verification",
                "adjudication",
                "freshness",
            },
        )
        self.assertIn(
            "verification passed",
            report["stages"]["verification"]["runs"][0]["stdout"],
        )
        verification_run = report["stages"]["verification"]["runs"][0]
        self.assertEqual(verification_run["command"], ["./verify.sh"])
        self.assertTrue(verification_run["cwd"].endswith("/snapshot"))
        self.assertEqual(verification_run["returncode"], 0)
        self.assertIsInstance(verification_run["duration_seconds"], float)
        native_args = self.native_log.read_text(encoding="utf-8")
        self.assertIn("review", native_args)
        self.assertIn("--commit", native_args)
        self.assertIn(commit, native_args)
        challenger_command = report["stages"]["challenger"]["command"]
        adjudicator_command = report["stages"]["adjudication"]["command"]
        challenger_output = Path(challenger_command[challenger_command.index("-o") + 1])
        adjudicator_output = Path(
            adjudicator_command[adjudicator_command.index("-o") + 1]
        )
        self.assertEqual(challenger_output.parent.name, "challenger")
        self.assertEqual(adjudicator_output.parent.name, "adjudication")
        self.assertEqual(
            Path(report["stages"]["native"]["ephemeral_artifact_directory"]).name,
            "native",
        )
        self.assertEqual(
            Path(report["stages"]["challenger"]["ephemeral_artifact_directory"]).name,
            "challenger",
        )
        adjudicator_prompt = self.adjudicator_input_log.read_text(encoding="utf-8")
        self.assertIn("Verbatim native review output", adjudicator_prompt)
        self.assertIn("Normalized native result", adjudicator_prompt)
        self.assertIn('"findings": []', adjudicator_prompt)
        self.assertIn('"checked_clean": true', adjudicator_prompt)
        self.assertIn("verification passed", adjudicator_prompt)
        self.assertIn(report["target"]["target_id"], adjudicator_prompt)
        self.assertFalse(Path(report["stages"]["native"]["cwd"]).exists())
        self.assertFalse(
            Path(report["stages"]["native"]["ephemeral_artifact_directory"]).exists()
        )
        self.assertFalse(
            Path(
                report["stages"]["challenger"]["ephemeral_artifact_directory"]
            ).exists()
        )

    def test_reviews_merge_commit_against_first_parent(self) -> None:
        run(["git", "checkout", "-qb", "side", "HEAD^"], cwd=self.repo)
        (self.repo / "side.txt").write_text("side\n", encoding="utf-8")
        run(["git", "add", "side.txt"], cwd=self.repo)
        run(["git", "commit", "-qm", "side change"], cwd=self.repo)
        run(["git", "checkout", "-q", "main"], cwd=self.repo)
        (self.repo / "main.txt").write_text("main\n", encoding="utf-8")
        run(["git", "add", "main.txt"], cwd=self.repo)
        run(["git", "commit", "-qm", "main change"], cwd=self.repo)
        run(["git", "merge", "--no-ff", "-qm", "merge side", "side"], cwd=self.repo)

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verification-not-applicable",
            "scope-only merge test",
        )

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        report = json.loads(result.stdout)
        self.assertEqual(report["target"]["changed_paths"], ["side.txt"])
        self.assertEqual(
            report["target"]["parent"],
            run(["git", "rev-parse", "HEAD^1"], cwd=self.repo).stdout.strip(),
        )

    def test_human_report_contains_target_commands_stages_and_evidence(self) -> None:
        result = run(
            [
                str(REVIEW_GATE),
                "--repo",
                str(self.repo),
                "--mode",
                "commit",
                "--commit",
                "HEAD",
                "--verify",
                "./verify.sh",
                "--native-bin",
                str(self.native),
                "--challenger-bin",
                str(self.challenger),
                "--adjudicator-bin",
                str(self.adjudicator),
                "--format",
                "human",
            ],
            cwd=self.repo,
            env=self.command_env,
            check=False,
        )

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("Final status: clean", result.stdout)
        self.assertIn("Target", result.stdout)
        self.assertIn("commit", result.stdout)
        self.assertIn("Stages", result.stdout)
        self.assertIn("native", result.stdout)
        self.assertIn("challenger", result.stdout)
        self.assertIn("verification", result.stdout)
        self.assertIn("adjudication", result.stdout)
        self.assertIn("freshness", result.stdout)
        self.assertIn("Command:", result.stdout)
        self.assertIn("verification passed", result.stdout)
        self.assertIn("Accepted findings", result.stdout)
        self.assertIn("Rejected findings", result.stdout)

    def test_report_write_failure_is_incomplete(self) -> None:
        output_directory = self.temp / "report-directory"
        output_directory.mkdir()

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
            "--json-output",
            str(output_directory),
        )

        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        report = json.loads(result.stdout)
        self.assertEqual(report["status"], "incomplete")
        self.assertIn("report emission failed", report["reason"])

    def test_stdout_failure_rewrites_json_report_as_incomplete(self) -> None:
        output_file = self.temp / "report.json"
        process = subprocess.Popen(
            [
                str(REVIEW_GATE),
                "--repo",
                str(self.repo),
                "--mode",
                "commit",
                "--commit",
                "HEAD",
                "--verify",
                "./verify.sh",
                "--native-bin",
                str(self.native),
                "--challenger-bin",
                str(self.challenger),
                "--adjudicator-bin",
                str(self.adjudicator),
                "--format",
                "json",
                "--json-output",
                str(output_file),
            ],
            cwd=self.repo,
            env={**os.environ, **self.command_env},
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        assert process.stdout is not None
        process.stdout.close()
        assert process.stderr is not None
        stderr = process.stderr.read()
        process.stderr.close()
        returncode = process.wait(timeout=10)

        self.assertEqual(returncode, 2, stderr)
        report = json.loads(output_file.read_text(encoding="utf-8"))
        self.assertEqual(report["status"], "incomplete")
        self.assertIn("report emission failed", report["reason"])

    def test_human_report_shows_verification_not_applicable_reason(self) -> None:
        reason = "documentation-only change has no executable behavior"

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verification-not-applicable",
            reason,
            output_format="human",
        )

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("Status: not_applicable", result.stdout)
        self.assertIn(f"Reason: {reason}", result.stdout)

    def test_freezes_staged_unstaged_deleted_renamed_and_untracked_work(self) -> None:
        (self.repo / "app.txt").write_text("local-after\n", encoding="utf-8")
        (self.repo / "delete.txt").unlink()
        run(["git", "mv", "rename.txt", "renamed.txt"], cwd=self.repo)
        (self.repo / "untracked.txt").write_text("untracked\n", encoding="utf-8")
        write_executable(
            self.repo / "verify-local.sh",
            """
            #!/bin/sh
            set -eu
            test "$(cat app.txt)" = "local-after"
            test ! -e delete.txt
            test -f renamed.txt
            test -f untracked.txt
            printf 'local snapshot passed\\n'
            """,
        )
        before_status = run(
            ["git", "status", "--porcelain=v1", "-z"],
            cwd=self.repo,
        ).stdout
        before_head = run(["git", "rev-parse", "HEAD"], cwd=self.repo).stdout
        before_refs = run(["git", "show-ref"], cwd=self.repo).stdout
        before_index = (self.repo / ".git/index").read_bytes()
        before_config = (self.repo / ".git/config").read_bytes()

        result = self.invoke(
            "--mode",
            "local",
            "--verify",
            "./verify-local.sh",
        )

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        report = json.loads(result.stdout)
        self.assertEqual(report["target"]["mode"], "local")
        self.assertEqual(
            set(report["target"]["changed_paths"]),
            {
                "app.txt",
                "delete.txt",
                "renamed.txt",
                "untracked.txt",
                "verify-local.sh",
            },
        )
        self.assertIn(
            "local snapshot passed",
            report["stages"]["verification"]["runs"][0]["stdout"],
        )
        native_args = self.native_log.read_text(encoding="utf-8")
        self.assertIn("--uncommitted", native_args)
        self.assertEqual(
            run(["git", "status", "--porcelain=v1", "-z"], cwd=self.repo).stdout,
            before_status,
        )
        self.assertEqual(
            run(["git", "rev-parse", "HEAD"], cwd=self.repo).stdout,
            before_head,
        )
        self.assertEqual(run(["git", "show-ref"], cwd=self.repo).stdout, before_refs)
        self.assertEqual((self.repo / ".git/index").read_bytes(), before_index)
        self.assertEqual((self.repo / ".git/config").read_bytes(), before_config)

    def test_preserves_staged_change_hidden_by_worktree_reversion(self) -> None:
        (self.repo / "app.txt").write_text("staged-only\n", encoding="utf-8")
        run(["git", "add", "app.txt"], cwd=self.repo)
        (self.repo / "app.txt").write_text("after\n", encoding="utf-8")
        write_executable(
            self.repo / "verify-local.sh",
            """
            #!/bin/sh
            set -eu
            test "$(git show :app.txt)" = "staged-only"
            test "$(cat app.txt)" = "after"
            """,
        )

        result = self.invoke(
            "--mode",
            "local",
            "--verify",
            "./verify-local.sh",
        )

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        report = json.loads(result.stdout)
        self.assertIn("app.txt", report["target"]["changed_paths"])
        self.assertEqual(report["stages"]["verification"]["status"], "passed")

    def test_fingerprints_untracked_executable_mode(self) -> None:
        write_executable(
            self.repo / "untracked-tool",
            """
            #!/bin/sh
            exit 0
            """,
        )

        result = self.invoke(
            "--mode",
            "local",
            "--verification-not-applicable",
            "mode fingerprint test",
        )

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        report = json.loads(result.stdout)
        record = next(
            item
            for item in report["target"]["untracked"]
            if item["path"] == "untracked-tool"
        )
        self.assertEqual(record["mode"], "100755")

    def test_rejects_empty_local_scope_before_reviewers_run(self) -> None:
        result = self.invoke(
            "--mode",
            "local",
            "--verification-not-applicable",
            "empty-scope check",
        )

        self.assertEqual(result.returncode, 2)
        report = json.loads(result.stdout)
        self.assertEqual(report["status"], "incomplete")
        self.assertIn("no staged, unstaged, or untracked changes", report["reason"])
        self.assertEqual(report["stages"], {})

    def test_freezes_a_complete_branch_delta_against_exact_base(self) -> None:
        base = run(["git", "rev-parse", "HEAD^"], cwd=self.repo).stdout.strip()
        head = run(["git", "rev-parse", "HEAD"], cwd=self.repo).stdout.strip()
        before_refs = run(["git", "show-ref"], cwd=self.repo).stdout
        before_config = (self.repo / ".git/config").read_bytes()

        result = self.invoke(
            "--mode",
            "branch",
            "--base",
            base,
            "--head",
            "HEAD",
            "--verify",
            "./verify.sh",
        )

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        report = json.loads(result.stdout)
        target = report["target"]
        self.assertEqual(target["mode"], "branch")
        self.assertEqual(target["base"], base)
        self.assertEqual(target["head"], head)
        self.assertEqual(target["merge_base"], base)
        self.assertEqual(target["changed_paths"], ["app.txt"])
        native_args = self.native_log.read_text(encoding="utf-8")
        self.assertIn("--base", native_args)
        self.assertIn("review-gate-base", native_args)
        self.assertEqual(run(["git", "show-ref"], cwd=self.repo).stdout, before_refs)
        self.assertEqual((self.repo / ".git/config").read_bytes(), before_config)

    def test_reads_guidance_from_frozen_commit_not_dirty_checkout(self) -> None:
        (self.repo / "AGENTS.md").write_text(
            "FROZEN-GUIDANCE\n",
            encoding="utf-8",
        )
        run(["git", "add", "AGENTS.md"], cwd=self.repo)
        run(["git", "commit", "-qm", "add frozen guidance"], cwd=self.repo)
        (self.repo / "AGENTS.md").write_text(
            "DIRTY-GUIDANCE\n",
            encoding="utf-8",
        )

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
        )

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        prompt = self.challenger_input_log.read_text(encoding="utf-8")
        self.assertIn("FROZEN-GUIDANCE", prompt)
        self.assertNotIn("DIRTY-GUIDANCE", prompt)

    def test_rejects_symlinked_guidance_that_escapes_snapshot(self) -> None:
        outside_guidance = self.temp / "outside-guidance"
        outside_guidance.write_text("HOST-ONLY-SECRET\n", encoding="utf-8")
        (self.repo / "AGENTS.md").symlink_to(outside_guidance)
        run(["git", "add", "AGENTS.md"], cwd=self.repo)
        run(["git", "commit", "-qm", "add escaping guidance"], cwd=self.repo)

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
        )

        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        report = json.loads(result.stdout)
        self.assertEqual(report["status"], "incomplete")
        self.assertIn("guidance must not be a symlink", report["reason"])
        if self.challenger_input_log.exists():
            self.assertNotIn(
                "HOST-ONLY-SECRET",
                self.challenger_input_log.read_text(encoding="utf-8"),
            )

    def test_freezes_pr_shas_intent_and_files(self) -> None:
        (self.repo / "AGENTS.md").write_text(
            "PR-FROZEN-GUIDANCE\n",
            encoding="utf-8",
        )
        run(["git", "add", "AGENTS.md"], cwd=self.repo)
        run(["git", "commit", "--amend", "--no-edit", "-q"], cwd=self.repo)
        base = run(["git", "rev-parse", "HEAD^"], cwd=self.repo).stdout.strip()
        head = run(["git", "rev-parse", "HEAD"], cwd=self.repo).stdout.strip()
        self.command_env["GH_PAYLOAD"] = json.dumps(
            {
                "number": 42,
                "baseRefName": "main",
                "baseRefOid": base,
                "headRefName": "feature",
                "headRefOid": head,
                "title": "Preserve complete review scope",
                "body": "Review unchanged consumers too.",
                "isCrossRepository": False,
                "url": "https://github.example/pull/42",
                "files": [],
            }
        )

        result = self.invoke(
            "--mode",
            "pr",
            "--pr",
            "42",
            "--gh-bin",
            str(self.gh),
            "--verify",
            "./verify.sh",
        )

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        report = json.loads(result.stdout)
        target = report["target"]
        self.assertEqual(target["mode"], "pr")
        self.assertEqual(target["pr"]["number"], 42)
        self.assertEqual(target["base"], base)
        self.assertEqual(target["head"], head)
        self.assertEqual(target["changed_paths"], ["AGENTS.md", "app.txt"])
        challenger_prompt = self.challenger_input_log.read_text(encoding="utf-8")
        self.assertIn("Preserve complete review scope", challenger_prompt)
        self.assertIn("Review unchanged consumers too.", challenger_prompt)
        self.assertIn("PR-FROZEN-GUIDANCE", challenger_prompt)
        self.assertEqual(target["guidance"][0]["path"], "AGENTS.md")

    def test_fetches_missing_pr_objects_only_into_isolated_clones(self) -> None:
        source = self.temp / "pr-source"
        source.mkdir()
        run(["git", "init", "-q", "-b", "main"], cwd=source)
        run(["git", "config", "user.name", "Review Gate Test"], cwd=source)
        run(
            ["git", "config", "user.email", "review-gate@example.test"],
            cwd=source,
        )
        (source / "app.txt").write_text("before\n", encoding="utf-8")
        write_executable(
            source / "verify.sh",
            """
            #!/bin/sh
            set -eu
            test "$(cat app.txt)" = "after"
            """,
        )
        run(["git", "add", "app.txt", "verify.sh"], cwd=source)
        run(["git", "commit", "-qm", "base"], cwd=source)
        base = run(["git", "rev-parse", "HEAD"], cwd=source).stdout.strip()

        builder = self.temp / "pr-builder"
        run(["git", "clone", "-q", str(source), str(builder)], cwd=self.temp)
        run(["git", "config", "user.name", "Review Gate Test"], cwd=builder)
        run(
            ["git", "config", "user.email", "review-gate@example.test"],
            cwd=builder,
        )
        run(["git", "switch", "-qc", "feature"], cwd=builder)
        (builder / "app.txt").write_text("after\n", encoding="utf-8")
        run(["git", "add", "app.txt"], cwd=builder)
        run(["git", "commit", "-qm", "head"], cwd=builder)
        head = run(["git", "rev-parse", "HEAD"], cwd=builder).stdout.strip()

        remote = self.temp / "remote.git"
        run(["git", "clone", "-q", "--bare", str(builder), str(remote)], cwd=self.temp)
        run(
            ["git", "--git-dir", str(remote), "update-ref", "refs/heads/main", base],
            cwd=self.temp,
        )
        run(
            [
                "git",
                "--git-dir",
                str(remote),
                "update-ref",
                "refs/pull/88/head",
                head,
            ],
            cwd=self.temp,
        )
        run(["git", "remote", "add", "origin", str(remote)], cwd=source)
        self.command_env["GH_PAYLOAD"] = json.dumps(
            {
                "number": 88,
                "baseRefName": "main",
                "baseRefOid": base,
                "headRefName": "feature",
                "headRefOid": head,
                "title": "Remote-only head",
                "body": "The source clone has not fetched this head.",
                "isCrossRepository": False,
                "url": "https://github.example/pull/88",
                "files": [],
            }
        )
        before_config = (source / ".git/config").read_bytes()
        before_refs = run(["git", "show-ref"], cwd=source).stdout
        missing_before = run(
            ["git", "cat-file", "-e", f"{head}^{{commit}}"],
            cwd=source,
            check=False,
        )
        self.assertNotEqual(missing_before.returncode, 0)

        original_repo = self.repo
        self.repo = source
        try:
            result = self.invoke(
                "--mode",
                "pr",
                "--pr",
                "88",
                "--gh-bin",
                str(self.gh),
                "--verify",
                "./verify.sh",
            )
        finally:
            self.repo = original_repo

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        report = json.loads(result.stdout)
        self.assertEqual(report["target"]["head"], head)
        self.assertEqual((source / ".git/config").read_bytes(), before_config)
        self.assertEqual(run(["git", "show-ref"], cwd=source).stdout, before_refs)
        missing_after = run(
            ["git", "cat-file", "-e", f"{head}^{{commit}}"],
            cwd=source,
            check=False,
        )
        self.assertNotEqual(missing_after.returncode, 0)

    def test_rejects_dirty_branch_scope_instead_of_narrowing_to_local(self) -> None:
        (self.repo / "dirty.txt").write_text("dirty\n", encoding="utf-8")

        result = self.invoke(
            "--mode",
            "branch",
            "--base",
            "HEAD^",
            "--verify",
            "./verify.sh",
        )

        self.assertEqual(result.returncode, 2)
        report = json.loads(result.stdout)
        self.assertEqual(report["status"], "incomplete")
        self.assertIn("ambiguous", report["reason"])
        self.assertEqual(report["stages"], {})

    def test_rejects_dirty_pr_scope_instead_of_narrowing_to_local(self) -> None:
        (self.repo / "dirty.txt").write_text("dirty\n", encoding="utf-8")

        result = self.invoke(
            "--mode",
            "pr",
            "--pr",
            "42",
            "--gh-bin",
            str(self.gh),
            "--verification-not-applicable",
            "mixed-scope check",
        )

        self.assertEqual(result.returncode, 2)
        report = json.loads(result.stdout)
        self.assertEqual(report["status"], "incomplete")
        self.assertIn("pull request target is ambiguous", report["reason"])
        self.assertEqual(report["stages"], {})

    def test_runs_challenger_concurrently_without_native_findings(self) -> None:
        challenger_started = self.temp / "challenger-started"
        self.command_env["CHALLENGER_STARTED"] = str(challenger_started)
        write_executable(
            self.native,
            """
            #!/bin/sh
            set -eu
            count=0
            while [ ! -f "$CHALLENGER_STARTED" ]; do
              count=$((count + 1))
              [ "$count" -lt 100 ] || {
                printf 'challenger never started\\n' >&2
                exit 9
              }
              sleep 0.01
            done
            printf '%s\\n' "$*" > "$NATIVE_LOG"
            printf '{"findings":[],"checked_clean":true}\\n'
            """,
        )

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
        )

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        report = json.loads(result.stdout)
        self.assertEqual(report["stages"]["native"]["status"], "completed")
        challenger_prompt = self.challenger_input_log.read_text(encoding="utf-8")
        self.assertNotIn("NATIVE-ONLY-MARKER", challenger_prompt)

    def test_missing_challenger_output_is_incomplete(self) -> None:
        write_executable(
            self.challenger,
            """
            #!/bin/sh
            exit 0
            """,
        )

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
        )

        self.assertEqual(result.returncode, 2)
        report = json.loads(result.stdout)
        self.assertIn("did not produce structured output", report["reason"])
        self.assertNotIn("adjudication", report["stages"])

    def test_failed_challenger_process_is_incomplete(self) -> None:
        write_executable(
            self.challenger,
            """
            #!/bin/sh
            printf 'challenger infrastructure failed\\n' >&2
            exit 9
            """,
        )

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
        )

        self.assertEqual(result.returncode, 2)
        report = json.loads(result.stdout)
        self.assertEqual(report["stages"]["challenger"]["status"], "failed")
        self.assertIn("challenger review did not complete", report["reason"])
        self.assertNotIn("adjudication", report["stages"])

    def test_discovers_focused_go_verification_from_changed_package(self) -> None:
        (self.repo / "go.mod").write_text(
            "module example.test/reviewgate\n\ngo 1.22\n",
            encoding="utf-8",
        )
        package = self.repo / "pkg/check"
        package.mkdir(parents=True)
        (package / "check.go").write_text(
            "package check\n\nfunc Value() int { return 1 }\n",
            encoding="utf-8",
        )
        run(["git", "add", "go.mod", "pkg/check/check.go"], cwd=self.repo)
        run(["git", "commit", "-qm", "add go package"], cwd=self.repo)
        (package / "check.go").write_text(
            "package check\n\nfunc Value() int { return 2 }\n",
            encoding="utf-8",
        )
        run(["git", "add", "pkg/check/check.go"], cwd=self.repo)
        run(["git", "commit", "-qm", "change go package"], cwd=self.repo)

        fake_bin = self.temp / "fake-bin"
        fake_bin.mkdir()
        go_log = self.temp / "go.log"
        write_executable(
            fake_bin / "go",
            """
            #!/bin/sh
            set -eu
            printf '%s\\n' "$*" > "$GO_LOG"
            printf 'focused go verification passed\\n'
            """,
        )
        self.command_env["GO_LOG"] = str(go_log)
        self.command_env["PATH"] = f"{fake_bin}{os.pathsep}{os.environ['PATH']}"

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--discover-verification",
        )

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        report = json.loads(result.stdout)
        verification = report["stages"]["verification"]
        self.assertEqual(verification["status"], "passed")
        self.assertTrue(verification["discovered"])
        self.assertEqual(
            verification["runs"][0]["command"],
            ["go", "test", "./pkg/check"],
        )
        self.assertEqual(go_log.read_text(encoding="utf-8").strip(), "test ./pkg/check")

    def test_discovers_focused_go_verification_from_test_only_change(self) -> None:
        (self.repo / "go.mod").write_text(
            "module example.test/reviewgate\n\ngo 1.22\n",
            encoding="utf-8",
        )
        package = self.repo / "pkg/check"
        package.mkdir(parents=True)
        (package / "check_test.go").write_text(
            "package check\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) {}\n",
            encoding="utf-8",
        )
        run(["git", "add", "go.mod", "pkg/check/check_test.go"], cwd=self.repo)
        run(["git", "commit", "-qm", "add go test"], cwd=self.repo)
        (package / "check_test.go").write_text(
            "package check\n\nimport \"testing\"\n\nfunc TestChanged(t *testing.T) {}\n",
            encoding="utf-8",
        )
        run(["git", "add", "pkg/check/check_test.go"], cwd=self.repo)
        run(["git", "commit", "-qm", "change go test"], cwd=self.repo)

        fake_bin = self.temp / "fake-bin"
        fake_bin.mkdir()
        go_log = self.temp / "go.log"
        write_executable(
            fake_bin / "go",
            """
            #!/bin/sh
            set -eu
            printf '%s\\n' "$*" > "$GO_LOG"
            """,
        )
        self.command_env["GO_LOG"] = str(go_log)
        self.command_env["PATH"] = f"{fake_bin}{os.pathsep}{os.environ['PATH']}"

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--discover-verification",
        )

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(go_log.read_text(encoding="utf-8").strip(), "test ./pkg/check")

    def test_requires_opt_in_before_untrusted_pr_verification(self) -> None:
        base = run(["git", "rev-parse", "HEAD^"], cwd=self.repo).stdout.strip()
        head = run(["git", "rev-parse", "HEAD"], cwd=self.repo).stdout.strip()
        self.command_env["GH_PAYLOAD"] = json.dumps(
            {
                "number": 77,
                "baseRefName": "main",
                "baseRefOid": base,
                "headRefName": "outside-feature",
                "headRefOid": head,
                "title": "External contribution",
                "body": "Candidate code is untrusted.",
                "isCrossRepository": True,
                "url": "https://github.example/pull/77",
                "files": [{"path": "app.txt"}],
            }
        )

        blocked = self.invoke(
            "--mode",
            "pr",
            "--pr",
            "77",
            "--gh-bin",
            str(self.gh),
            "--verify",
            "./verify.sh",
        )

        self.assertEqual(blocked.returncode, 2)
        blocked_report = json.loads(blocked.stdout)
        self.assertIn("untrusted", blocked_report["reason"])
        self.assertEqual(set(blocked_report["stages"]), {"target"})

        allowed = self.invoke(
            "--mode",
            "pr",
            "--pr",
            "77",
            "--gh-bin",
            str(self.gh),
            "--verify",
            "./verify.sh",
            "--allow-untrusted-execution",
        )
        self.assertEqual(allowed.returncode, 0, allowed.stdout + allowed.stderr)

    def test_rejects_adjudication_that_omits_native_finding(self) -> None:
        write_executable(
            self.native,
            """
            #!/usr/bin/env python3
            import json

            print(
                json.dumps(
                    {
                        "findings": [
                            {
                                "id": "native-broken-contract",
                                "title": "Changed value breaks a contract",
                                "body": "The changed value is incompatible.",
                                "severity": "P1",
                                "confidence": 0.99,
                                "evidence_location": {"path": "app.txt", "line": 1},
                                "caused_by": {"path": "app.txt", "line": 1},
                                "scenario": "Read the changed app value.",
                            }
                        ],
                        "checked_clean": False,
                    }
                )
            )
            """,
        )
        self.command_env["NATIVE_NORMALIZED_RESULT"] = json.dumps(
            {
                "findings": [
                    {
                        "id": "native-broken-contract",
                        "title": "Changed value breaks a contract",
                        "body": "The changed value is incompatible.",
                        "severity": "P1",
                        "confidence": 0.99,
                        "evidence_location": {"path": "app.txt", "line": 1},
                        "caused_by": {"path": "app.txt", "line": 1},
                        "scenario": "Read the changed app value.",
                    }
                ],
                "checked_clean": False,
            }
        )

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
        )

        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        report = json.loads(result.stdout)
        self.assertIn("native finding", report["reason"])

    def test_rejects_native_normalization_that_drops_raw_finding(self) -> None:
        write_executable(
            self.native,
            """
            #!/bin/sh
            printf 'Finding: changed app value breaks the contract\\n'
            """,
        )
        self.command_env["ADJUDICATOR_RESULT"] = json.dumps(
            {
                "decisions": [],
                "native_raw_coverage": {
                    "complete": False,
                    "rationale": "The raw finding is absent from normalization.",
                },
                "native_coverage": [],
                "challenger_coverage": [],
                "checked_clean": True,
            }
        )

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
        )

        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        report = json.loads(result.stdout)
        self.assertIn("does not cover", report["reason"])

    def test_rejects_origins_without_matching_source_ids(self) -> None:
        challenger_finding = {
            "id": "challenger-contract",
            "title": "Changed value breaks a contract",
            "body": "The changed value is incompatible.",
            "severity": "P1",
            "confidence": 0.99,
            "evidence_location": {"path": "app.txt", "line": 1},
            "caused_by": {"path": "app.txt", "line": 1},
            "scenario": "Read the changed app value.",
        }
        self.command_env["CHALLENGER_RESULT"] = json.dumps(
            {"findings": [challenger_finding], "checked_clean": False}
        )
        self.command_env["ADJUDICATOR_RESULT"] = json.dumps(
            {
                "decisions": [
                    {
                        "source_ids": ["challenger-contract"],
                        "origins": ["native", "challenger"],
                        "title": challenger_finding["title"],
                        "severity": "P1",
                        "confidence": 0.99,
                        "evidence_location": {"path": "app.txt", "line": 1},
                        "caused_by": {"path": "app.txt", "line": 1},
                        "scenario": challenger_finding["scenario"],
                        "verification_evidence": "Repository contents.",
                        "verification_run_indexes": [],
                        "disposition": "accepted",
                        "rationale": "The changed value is incompatible.",
                        "suggested_fix": "Restore the contract.",
                    }
                ],
                "native_raw_coverage": {
                    "complete": True,
                    "rationale": "Normalized output matches.",
                },
                "native_coverage": [],
                "challenger_coverage": ["challenger-contract"],
                "checked_clean": False,
            }
        )

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
        )

        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        report = json.loads(result.stdout)
        self.assertIn("origins must exactly match", report["reason"])

    def test_accepts_unchanged_consumer_finding_with_changed_cause(self) -> None:
        write_executable(
            self.native,
            """
            #!/bin/sh
            set -eu
            printf '%s\\n' "$*" > "$NATIVE_LOG"
            printf 'Finding: changed app value breaks consumer.txt\\n'
            """,
        )
        write_executable(
            self.challenger,
            """
            #!/usr/bin/env python3
            import json
            import pathlib
            import sys

            args = sys.argv[1:]
            output = pathlib.Path(args[args.index("-o") + 1])
            finding_id = (
                "native-stale-consumer"
                if output.parent.name == "native"
                else "stale-consumer"
            )
            output.write_text(
                json.dumps(
                    {
                        "findings": [
                            {
                                "id": finding_id,
                                "title": "Changed value breaks an unchanged consumer",
                                "body": "The consumer still expects the old value.",
                                "severity": "P1",
                                "confidence": 0.99,
                                "evidence_location": {
                                    "path": "consumer.txt",
                                    "line": 1,
                                },
                                "caused_by": {"path": "app.txt", "line": 1},
                                "scenario": "Read app.txt and then validate consumer.txt.",
                            }
                        ],
                        "checked_clean": False,
                    }
                ),
                encoding="utf-8",
            )
            """,
        )
        write_executable(
            self.adjudicator,
            """
            #!/usr/bin/env python3
            import json
            import os
            import pathlib
            import sys

            args = sys.argv[1:]
            cause_path = os.environ.get("ADJUDICATOR_CAUSE", "app.txt")
            verification_run_indexes = json.loads(
                os.environ.get("ADJUDICATOR_VERIFICATION_INDEXES", "[]")
            )
            output = pathlib.Path(args[args.index("-o") + 1])
            output.write_text(
                json.dumps(
                    {
                        "decisions": [
                            {
                                "source_ids": [
                                    "native-stale-consumer",
                                    "stale-consumer",
                                ],
                                "origins": ["native", "challenger"],
                                "title": "Changed value breaks an unchanged consumer",
                                "severity": "P1",
                                "confidence": 0.99,
                                "evidence_location": {
                                    "path": "consumer.txt",
                                    "line": 1,
                                },
                                "caused_by": {"path": cause_path, "line": 1},
                                "scenario": "Read app.txt and then validate consumer.txt.",
                                "verification_evidence": "Repository contents show the mismatch.",
                                "verification_run_indexes": verification_run_indexes,
                                "disposition": "accepted",
                                "rationale": "The unchanged consumer is reachable.",
                                "suggested_fix": "Update the consumer contract.",
                            }
                        ],
                        "native_raw_coverage": {
                            "complete": True,
                            "rationale": "Both raw findings were normalized.",
                        },
                        "native_coverage": ["native-stale-consumer"],
                        "challenger_coverage": ["stale-consumer"],
                        "checked_clean": False,
                    }
                ),
                encoding="utf-8",
            )
            """,
        )

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
        )

        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        report = json.loads(result.stdout)
        self.assertEqual(report["status"], "findings")
        decision = report["decisions"][0]
        self.assertEqual(decision["origins"], ["native", "challenger"])
        self.assertEqual(len(report["decisions"]), 1)
        self.assertEqual(decision["evidence_location"]["path"], "consumer.txt")
        self.assertEqual(decision["caused_by"]["path"], "app.txt")
        self.assertNotIn("consumer.txt", report["target"]["changed_paths"])

        human = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
            output_format="human",
        )
        self.assertEqual(human.returncode, 1, human.stdout + human.stderr)
        self.assertIn("Changed value breaks an unchanged consumer", human.stdout)
        self.assertIn("Origins: native, challenger", human.stdout)
        self.assertIn("Evidence: consumer.txt:1", human.stdout)
        self.assertIn("Caused by: app.txt:1", human.stdout)
        self.assertIn("Repository contents show the mismatch.", human.stdout)

        unlinked_failure = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "/usr/bin/false",
        )
        self.assertEqual(unlinked_failure.returncode, 2)
        self.assertIn(
            "not causally attributed",
            json.loads(unlinked_failure.stdout)["reason"],
        )

        self.command_env["ADJUDICATOR_VERIFICATION_INDEXES"] = "[1]"
        linked_failure = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "/usr/bin/false",
        )
        self.assertEqual(
            linked_failure.returncode,
            1,
            linked_failure.stdout + linked_failure.stderr,
        )
        self.command_env.pop("ADJUDICATOR_VERIFICATION_INDEXES")

        self.command_env["ADJUDICATOR_CAUSE"] = "consumer.txt"
        invalid = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
        )
        self.assertEqual(invalid.returncode, 2)
        invalid_report = json.loads(invalid.stdout)
        self.assertIn(
            "accepted finding must identify a causal location in the change",
            invalid_report["reason"],
        )

    def test_rejects_malformed_challenger_finding_before_adjudication(self) -> None:
        write_executable(
            self.challenger,
            """
            #!/usr/bin/env python3
            import json
            import pathlib
            import sys

            args = sys.argv[1:]
            output = pathlib.Path(args[args.index("-o") + 1])
            output.write_text(
                json.dumps(
                    {
                        "findings": [{"id": "missing-causal-evidence"}],
                        "checked_clean": False,
                    }
                ),
                encoding="utf-8",
            )
            """,
        )

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
        )

        self.assertEqual(result.returncode, 2)
        report = json.loads(result.stdout)
        self.assertEqual(report["status"], "incomplete")
        self.assertIn("challenger finding is missing", report["reason"])
        self.assertNotIn("adjudication", report["stages"])

    def test_rejects_malformed_adjudication_as_incomplete(self) -> None:
        write_executable(
            self.adjudicator,
            """
            #!/usr/bin/env python3
            import json
            import pathlib
            import sys

            args = sys.argv[1:]
            output = pathlib.Path(args[args.index("-o") + 1])
            output.write_text(
                json.dumps(
                    {
                        "decisions": [],
                        "challenger_coverage": [],
                        "checked_clean": True,
                    }
                ),
                encoding="utf-8",
            )
            """,
        )

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
        )

        self.assertEqual(result.returncode, 2)
        report = json.loads(result.stdout)
        self.assertIn("adjudication output has unexpected fields", report["reason"])
        self.assertEqual(report["stages"]["adjudication"]["status"], "completed")

    def test_failed_adjudicator_process_is_incomplete(self) -> None:
        write_executable(
            self.adjudicator,
            """
            #!/bin/sh
            printf 'adjudicator infrastructure failed\\n' >&2
            exit 9
            """,
        )

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
        )

        self.assertEqual(result.returncode, 2)
        report = json.loads(result.stdout)
        self.assertEqual(report["stages"]["adjudication"]["status"], "failed")
        self.assertIn("adjudication did not complete", report["reason"])

    def test_unattributed_verification_failure_is_incomplete(self) -> None:
        verification_failure = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "/usr/bin/false",
        )
        self.assertEqual(
            verification_failure.returncode,
            2,
            verification_failure.stdout + verification_failure.stderr,
        )
        verification_report = json.loads(verification_failure.stdout)
        self.assertEqual(verification_report["status"], "incomplete")
        self.assertIn("not causally attributed", verification_report["reason"])
        self.assertEqual(
            verification_report["stages"]["verification"]["status"],
            "failed",
        )

        write_executable(
            self.native,
            """
            #!/bin/sh
            printf 'native infrastructure failed\\n' >&2
            exit 9
            """,
        )
        reviewer_failure = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
        )
        self.assertEqual(reviewer_failure.returncode, 2)
        reviewer_report = json.loads(reviewer_failure.stdout)
        self.assertEqual(reviewer_report["status"], "incomplete")
        self.assertEqual(
            reviewer_report["stages"]["native"]["status"],
            "failed",
        )
        self.assertFalse(Path(reviewer_report["stages"]["native"]["cwd"]).exists())
        self.assertFalse(
            Path(
                reviewer_report["stages"]["native"]["ephemeral_artifact_directory"]
            ).exists()
        )

    def test_verification_launch_failure_is_incomplete(self) -> None:
        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./does-not-exist",
        )

        self.assertEqual(result.returncode, 2)
        report = json.loads(result.stdout)
        verification = report["stages"]["verification"]
        self.assertEqual(verification["status"], "incomplete")
        self.assertIsNone(verification["runs"][0]["returncode"])
        self.assertIn("No such file or directory", verification["runs"][0]["stderr"])

    def test_verification_can_mutate_snapshot_without_mutating_source(self) -> None:
        write_executable(
            self.repo / "mutate-snapshot.sh",
            """
            #!/bin/sh
            set -eu
            printf 'snapshot-only\\n' > app.txt
            printf 'snapshot mutation completed\\n'
            """,
        )
        run(["git", "add", "mutate-snapshot.sh"], cwd=self.repo)
        run(["git", "commit", "-qm", "add mutating verification"], cwd=self.repo)
        before_status = run(
            ["git", "status", "--porcelain=v1", "-z"],
            cwd=self.repo,
        ).stdout
        before_refs = run(["git", "show-ref"], cwd=self.repo).stdout
        before_config = (self.repo / ".git/config").read_bytes()

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./mutate-snapshot.sh",
        )

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        report = json.loads(result.stdout)
        self.assertIn(
            "snapshot mutation completed",
            report["stages"]["verification"]["runs"][0]["stdout"],
        )
        self.assertEqual(
            (self.repo / "app.txt").read_text(encoding="utf-8"),
            "after\n",
        )
        self.assertEqual(
            run(["git", "status", "--porcelain=v1", "-z"], cwd=self.repo).stdout,
            before_status,
        )
        self.assertEqual(run(["git", "show-ref"], cwd=self.repo).stdout, before_refs)
        self.assertEqual((self.repo / ".git/config").read_bytes(), before_config)

    def test_rejects_conflicting_verification_selection(self) -> None:
        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
            "--verification-not-applicable",
            "conflicts with explicit verification",
        )

        self.assertEqual(result.returncode, 2)
        report = json.loads(result.stdout)
        self.assertIn("choose exactly one verification strategy", report["reason"])
        self.assertEqual(report["stages"], {})

    def test_rejects_local_target_that_drifts_during_review(self) -> None:
        (self.repo / "app.txt").write_text("local-after\n", encoding="utf-8")
        self.command_env["MUTATE_SOURCE"] = str(self.repo / "app.txt")
        write_executable(
            self.native,
            """
            #!/bin/sh
            set -eu
            printf 'drifted again\\n' >> "$MUTATE_SOURCE"
            printf '%s\\n' "$*" > "$NATIVE_LOG"
            printf '{"findings":[],"checked_clean":true}\\n'
            """,
        )

        result = self.invoke(
            "--mode",
            "local",
            "--verification-not-applicable",
            "target-drift test has no executable behavior",
        )

        self.assertEqual(result.returncode, 2)
        report = json.loads(result.stdout)
        self.assertEqual(report["status"], "incomplete")
        self.assertIn("changed before finalization", report["reason"])
        self.assertEqual(
            report["stages"]["verification"]["status"],
            "not_applicable",
        )

    def test_interrupt_stops_reviewers_reports_incomplete_and_cleans_up(self) -> None:
        native_started = self.temp / "native-started"
        challenger_started = self.temp / "challenger-started"
        run_temp = self.temp / "run-temp"
        run_temp.mkdir()
        write_executable(
            self.native,
            """
            #!/bin/sh
            set -eu
            touch "$NATIVE_STARTED"
            sleep 2
            """,
        )
        write_executable(
            self.challenger,
            """
            #!/bin/sh
            set -eu
            touch "$CHALLENGER_STARTED"
            sleep 2
            """,
        )
        env = {
            **os.environ,
            **self.command_env,
            "NATIVE_STARTED": str(native_started),
            "CHALLENGER_STARTED": str(challenger_started),
            "TMPDIR": str(run_temp),
        }
        process = subprocess.Popen(
            [
                str(REVIEW_GATE),
                "--repo",
                str(self.repo),
                "--mode",
                "commit",
                "--commit",
                "HEAD",
                "--verify",
                "./verify.sh",
                "--native-bin",
                str(self.native),
                "--challenger-bin",
                str(self.challenger),
                "--adjudicator-bin",
                str(self.adjudicator),
                "--format",
                "json",
            ],
            cwd=self.repo,
            env=env,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        for _ in range(200):
            if native_started.exists() and challenger_started.exists():
                break
            time.sleep(0.01)
        self.assertTrue(native_started.exists())
        self.assertTrue(challenger_started.exists())

        interrupted_at = time.monotonic()
        process.send_signal(signal.SIGINT)
        stdout, stderr = process.communicate(timeout=5)
        elapsed = time.monotonic() - interrupted_at

        self.assertEqual(process.returncode, 2, stdout + stderr)
        self.assertLess(elapsed, 1.5)
        report = json.loads(stdout)
        self.assertEqual(report["status"], "incomplete")
        self.assertEqual(report["reason"], "review-gate was interrupted")
        self.assertEqual(list(run_temp.glob("review-gate-*")), [])

    def test_sigterm_stops_reviewers_reports_incomplete_and_cleans_up(self) -> None:
        native_started = self.temp / "native-started"
        challenger_started = self.temp / "challenger-started"
        run_temp = self.temp / "run-temp"
        run_temp.mkdir()
        write_executable(
            self.native,
            """
            #!/bin/sh
            set -eu
            touch "$NATIVE_STARTED"
            sleep 5
            """,
        )
        write_executable(
            self.challenger,
            """
            #!/bin/sh
            set -eu
            touch "$CHALLENGER_STARTED"
            sleep 5
            """,
        )
        env = {
            **os.environ,
            **self.command_env,
            "NATIVE_STARTED": str(native_started),
            "CHALLENGER_STARTED": str(challenger_started),
            "TMPDIR": str(run_temp),
        }
        process = subprocess.Popen(
            [
                str(REVIEW_GATE),
                "--repo",
                str(self.repo),
                "--mode",
                "commit",
                "--commit",
                "HEAD",
                "--verify",
                "./verify.sh",
                "--native-bin",
                str(self.native),
                "--challenger-bin",
                str(self.challenger),
                "--adjudicator-bin",
                str(self.adjudicator),
                "--format",
                "json",
            ],
            cwd=self.repo,
            env=env,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        for _ in range(200):
            if native_started.exists() and challenger_started.exists():
                break
            time.sleep(0.01)
        self.assertTrue(native_started.exists())
        self.assertTrue(challenger_started.exists())

        terminated_at = time.monotonic()
        process.send_signal(signal.SIGTERM)
        stdout, stderr = process.communicate(timeout=5)
        elapsed = time.monotonic() - terminated_at

        self.assertEqual(process.returncode, 2, stdout + stderr)
        self.assertLess(elapsed, 1.5)
        report = json.loads(stdout)
        self.assertEqual(report["status"], "incomplete")
        self.assertEqual(report["reason"], "review-gate was terminated")
        self.assertEqual(list(run_temp.glob("review-gate-*")), [])

    def test_snapshot_materialization_failure_is_incomplete(self) -> None:
        git_wrapper = self.temp / "git-wrapper"
        write_executable(
            git_wrapper,
            """
            #!/bin/sh
            set -eu
            if [ "${1:-}" = "clone" ]; then
              printf 'snapshot clone failed\\n' >&2
              exit 9
            fi
            exec "$REAL_GIT" "$@"
            """,
        )
        self.command_env["REAL_GIT"] = shutil.which("git") or "git"

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
            "--git-bin",
            str(git_wrapper),
        )

        self.assertEqual(result.returncode, 2)
        report = json.loads(result.stdout)
        self.assertIn("snapshot clone failed", report["reason"])
        self.assertEqual(set(report["stages"]), {"target"})

    def test_reviewer_timeout_is_incomplete_and_cleans_up(self) -> None:
        write_executable(
            self.native,
            """
            #!/bin/sh
            sleep 2
            """,
        )

        result = self.invoke(
            "--mode",
            "commit",
            "--commit",
            "HEAD",
            "--verify",
            "./verify.sh",
            "--timeout",
            "1",
        )

        self.assertEqual(result.returncode, 2)
        report = json.loads(result.stdout)
        native = report["stages"]["native"]
        self.assertEqual(native["status"], "incomplete")
        self.assertIsNone(native["returncode"])
        self.assertIn("timed out", native["stderr"])
        self.assertFalse(Path(native["cwd"]).exists())

    def test_reports_native_challenger_and_union_historical_recovery(self) -> None:
        case_ids = [
            "retirement-upgrade-state",
            "stale-consumers",
            "non-unix-build-tags",
            "cancellation-aware-locking",
            "root-permission-behavior",
            "runtime-parity",
        ]
        runs = []
        for index, case_id in enumerate(case_ids):
            runs.append(
                {
                    "case_id": case_id,
                    "reviewer": "native",
                    "status": "incomplete" if index == 3 else "completed",
                    "found": [case_id] if index < 3 else [],
                    "false_positives": [],
                }
            )
            runs.append(
                {
                    "case_id": case_id,
                    "reviewer": "challenger",
                    "status": "completed",
                    "found": [case_id] if index >= 2 else [],
                    "false_positives": ["speculative-extra"] if index == 5 else [],
                }
            )
        results = self.temp / "historical-results.json"
        results.write_text(json.dumps({"runs": runs}), encoding="utf-8")
        before_status = run(
            ["git", "status", "--porcelain=v1", "-z"],
            cwd=self.repo,
        ).stdout
        before_refs = run(["git", "show-ref"], cwd=self.repo).stdout
        before_config = (self.repo / ".git/config").read_bytes()

        evaluated = run(
            [
                str(REVIEW_GATE),
                "evaluate",
                "--results",
                str(results),
                "--format",
                "json",
            ],
            cwd=self.repo,
            check=False,
        )

        self.assertEqual(
            evaluated.returncode,
            0,
            evaluated.stdout + evaluated.stderr,
        )
        report = json.loads(evaluated.stdout)
        self.assertFalse(report["gating"])
        self.assertEqual(report["summary"]["native"]["recovered"], 3)
        self.assertEqual(report["summary"]["native"]["incomplete_runs"], 1)
        self.assertEqual(report["summary"]["challenger"]["recovered"], 4)
        self.assertEqual(report["summary"]["union"]["recovered"], 6)
        self.assertEqual(report["summary"]["union"]["recovery_rate"], 1.0)
        self.assertEqual(report["summary"]["challenger"]["false_positives"], 1)
        last_case = report["cases"][-1]
        self.assertEqual(
            last_case["recovery"],
            {"native": 0, "challenger": 1, "union": 1},
        )
        self.assertEqual(
            last_case["false_positives"]["challenger"],
            ["speculative-extra"],
        )
        interrupted_case = report["cases"][3]
        self.assertTrue(interrupted_case["incomplete"]["native"])
        self.assertFalse(interrupted_case["incomplete"]["challenger"])
        self.assertEqual(
            run(["git", "status", "--porcelain=v1", "-z"], cwd=self.repo).stdout,
            before_status,
        )
        self.assertEqual(run(["git", "show-ref"], cwd=self.repo).stdout, before_refs)
        self.assertEqual((self.repo / ".git/config").read_bytes(), before_config)

        human = run(
            [
                str(REVIEW_GATE),
                "evaluate",
                "--results",
                str(results),
                "--format",
                "human",
            ],
            cwd=self.repo,
            check=False,
        )
        self.assertEqual(human.returncode, 0, human.stdout + human.stderr)
        for case_id in case_ids:
            self.assertIn(case_id, human.stdout)
        self.assertIn("native recovery", human.stdout)
        self.assertIn("challenger recovery", human.stdout)
        self.assertIn("union recovery", human.stdout)
        self.assertIn("speculative-extra", human.stdout)
        self.assertIn("incomplete", human.stdout)


if __name__ == "__main__":
    unittest.main(verbosity=2)
