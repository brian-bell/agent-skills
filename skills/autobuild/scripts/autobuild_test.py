#!/usr/bin/env python3
"""Unit suite for the autobuild orchestrator.

Hermetic: uses temporary git repositories and a fake Claude stub, no model calls
and no network. Run with `python3 autobuild_test.py -v`.
"""
import os
import subprocess
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("autobuild")

# A task string with a unique sentinel so a test can prove the prompt was
# delivered on stdin and never leaked onto the engine argv.
SENTINEL_TASK = "Build the SENTINEL_RESET workflow"

# Body shared by the happy-path fakes: commit for the required phases, then
# report completion for every phase.
STANDARD_BODY = """
if phase in {"implementation", "review-loop"}:
    target = Path("phase-" + phase + ".txt")
    target.write_text("done " + phase + "\\n", encoding="utf-8")
    subprocess.check_call(["git", "add", str(target)])
    subprocess.check_call(["git", "commit", "-m", "work " + phase])
print("AUTOBUILD_REPORT: " + phase + ": completed - ok")
"""


def run(args, cwd, check=True):
    result = subprocess.run(
        args, cwd=cwd, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE
    )
    if check and result.returncode != 0:
        raise AssertionError(
            f"command failed ({result.returncode}): {' '.join(args)}\n"
            f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
        )
    return result


class AutobuildTest(unittest.TestCase):
    def make_repo(self, root):
        repo = root / "repo"
        repo.mkdir()
        run(["git", "init"], repo)
        run(["git", "branch", "-M", "main"], repo)
        run(["git", "config", "user.email", "autobuild@example.invalid"], repo)
        run(["git", "config", "user.name", "Autobuild Test"], repo)
        (repo / "README.md").write_text("initial\n", encoding="utf-8")
        run(["git", "add", "README.md"], repo)
        run(["git", "commit", "-m", "initial"], repo)
        run(["git", "switch", "-c", "feature"], repo)
        return repo

    def make_fake_engine(self, root, name, body, label=None):
        fake = root / name
        fake.write_text(
            "#!/usr/bin/env python3\n"
            "import os, subprocess, sys\n"
            "from pathlib import Path\n"
            f"LABEL = {repr(label or name)}\n"
            "phase = os.environ['AUTOBUILD_PHASE']\n"
            "argv = sys.argv\n"
            "if os.environ.get('AUTOBUILD_TEST_ASSERT_CLAUDE') == '1' and '-p' not in argv:\n"
            "    raise SystemExit('claude engine did not use -p: ' + repr(argv))\n"
            "prompt = sys.stdin.read()\n"
            "if os.environ.get('AUTOBUILD_TEST_ASSERT_STDIN') == '1':\n"
            "    if not prompt:\n"
            "        raise SystemExit('prompt was not passed on stdin')\n"
            "    if 'SENTINEL_RESET' in ' '.join(argv):\n"
            "        raise SystemExit('prompt leaked through argv: ' + repr(argv))\n"
            "if os.environ.get('AUTOBUILD_TEST_ASSERT_NO_LEAK') == '1':\n"
            "    leaked = [k for k in os.environ if k.startswith('AUTOBUILD_') "
            "and not k.startswith('AUTOBUILD_TEST_') and k != 'AUTOBUILD_PHASE']\n"
            "    if 'PARENT_SECRET' in os.environ:\n"
            "        leaked.append('PARENT_SECRET')\n"
            "    if leaked:\n"
            "        raise SystemExit('leaked parent env: ' + ','.join(sorted(leaked)))\n"
            "Path(os.environ['AUTOBUILD_TEST_PHASE_LOG']).open('a', encoding='utf-8')"
            ".write(LABEL + ' ' + phase + '\\n')\n"
            "Path(os.environ['AUTOBUILD_TEST_PROMPT_LOG']).open('a', encoding='utf-8')"
            ".write('--- ' + LABEL + ' ' + phase + ' ---\\n' + prompt + '\\n')\n"
            + textwrap.dedent(body),
            encoding="utf-8",
        )
        fake.chmod(0o755)
        return fake

    def make_skill_root(self, root, name, skills):
        """Create a fake engine skills dir with the given skill subdirs."""
        skill_root = root / name
        for skill in skills:
            sk = skill_root / skill
            sk.mkdir(parents=True)
            (sk / "SKILL.md").write_text(f"---\nname: {skill}\n---\n", encoding="utf-8")
        return skill_root

    def base_env(self, root, phase_log, prompt_log, *, claude_skills=None):
        env = os.environ.copy()
        env["AUTOBUILD_TEST_PHASE_LOG"] = str(phase_log)
        env["AUTOBUILD_TEST_PROMPT_LOG"] = str(prompt_log)
        # Sentinel that must be scrubbed from the child environment.
        env["PARENT_SECRET"] = "1"
        env["AUTOBUILD_PARENT_RUN"] = "parent"
        if claude_skills is not None:
            env["AUTOBUILD_CLAUDE_SKILLS_DIR"] = str(claude_skills)
        return env

    def invoke(self, repo, fake_args, env, extra=None):
        args = [
            sys.executable,
            str(SCRIPT),
            "--repo",
            str(repo),
            "--task",
            SENTINEL_TASK,
            "--base",
            "origin/main",
        ] + fake_args + (extra or [])
        return subprocess.run(
            args, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env
        )

    # --- happy paths -------------------------------------------------------

    def test_runs_all_phases_and_commits(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = self.make_repo(root)
            phase_log = root / "phases.log"
            prompt_log = root / "prompts.log"
            reports = root / "reports"
            skills = self.make_skill_root(root, "claude-skills", ["review-loop", "autoreview"])
            fake = self.make_fake_engine(root, "claude", STANDARD_BODY)

            env = self.base_env(root, phase_log, prompt_log, claude_skills=skills)
            env.update(
                {
                    "AUTOBUILD_TEST_ASSERT_CLAUDE": "1",
                    "AUTOBUILD_TEST_ASSERT_STDIN": "1",
                    "AUTOBUILD_TEST_ASSERT_NO_LEAK": "1",
                }
            )
            result = self.invoke(
                repo,
                ["--claude-bin", str(fake), "--report-dir", str(reports)],
                env,
            )

            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            self.assertEqual(
                phase_log.read_text(encoding="utf-8").splitlines(),
                [
                    "claude implementation",
                    "claude review-loop",
                    "claude autoreview",
                    "claude pr-creation",
                ],
            )
            prompts = prompt_log.read_text(encoding="utf-8")
            self.assertIn(SENTINEL_TASK, prompts)
            self.assertIn("Commit changes before reporting completion.", prompts)
            log = run(["git", "log", "--oneline"], repo).stdout
            self.assertIn("work implementation", log)
            self.assertIn("work review-loop", log)
            self.assertEqual(run(["git", "status", "--porcelain"], repo).stdout, "")
            for phase in ["implementation", "review-loop", "autoreview", "pr-creation"]:
                report = reports / f"{phase}.txt"
                self.assertTrue(report.exists(), phase)
                self.assertEqual(report.stat().st_mode & 0o777, 0o600)
            self.assertEqual(reports.stat().st_mode & 0o777, 0o700)

    def test_codex_and_openai_env_forwarded(self):
        # autobuild drives Claude, but the autoreview phase delegates to the
        # autoreview skill which runs Codex; its auth env must be forwarded.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = self.make_repo(root)
            phase_log = root / "phases.log"
            prompt_log = root / "prompts.log"
            skills = self.make_skill_root(root, "claude-skills", ["review-loop", "autoreview"])
            body = (
                "for _var in ('CODEX_FOO', 'OPENAI_BAR'):\n"
                "    if _var not in os.environ:\n"
                "        raise SystemExit('missing forwarded env: ' + _var)\n"
                + STANDARD_BODY
            )
            fake = self.make_fake_engine(root, "claude", body)
            env = self.base_env(root, phase_log, prompt_log, claude_skills=skills)
            env["CODEX_FOO"] = "1"
            env["OPENAI_BAR"] = "1"
            result = self.invoke(repo, ["--claude-bin", str(fake)], env)
            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            self.assertEqual(len(phase_log.read_text().splitlines()), 4)

    def _argv_body(self):
        return (
            "Path(os.environ['AUTOBUILD_TEST_ARGV_LOG']).open('a', encoding='utf-8')"
            ".write(phase + '\\t' + ' '.join(argv) + '\\n')\n" + STANDARD_BODY
        )

    def _run_with_argv_log(self, extra):
        """Run the happy path while capturing each phase's engine argv.

        Returns (result, {phase: argv_string}).
        """
        tmp = tempfile.mkdtemp()
        root = Path(tmp)
        repo = self.make_repo(root)
        phase_log = root / "phases.log"
        prompt_log = root / "prompts.log"
        argv_log = root / "argv.log"
        skills = self.make_skill_root(root, "claude-skills", ["review-loop", "autoreview"])
        fake = self.make_fake_engine(root, "claude", self._argv_body())
        env = self.base_env(root, phase_log, prompt_log, claude_skills=skills)
        env["AUTOBUILD_TEST_ARGV_LOG"] = str(argv_log)
        result = self.invoke(repo, ["--claude-bin", str(fake)], env, extra=extra)
        by_phase = {}
        if argv_log.exists():
            for line in argv_log.read_text(encoding="utf-8").splitlines():
                phase, argv = line.split("\t", 1)
                by_phase[phase] = argv
        return result, by_phase

    def test_model_and_effort_passed_to_engine(self):
        result, by_phase = self._run_with_argv_log(["--model", "opus", "--effort", "high"])
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        for phase, argv in by_phase.items():
            self.assertIn("--model opus", argv, phase)
            self.assertIn("--effort high", argv, phase)

    # --- claude permissions ------------------------------------------------

    def test_defaults_to_bypass_permissions(self):
        result, by_phase = self._run_with_argv_log([])
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        for phase, argv in by_phase.items():
            self.assertIn("--permission-mode bypassPermissions", argv, phase)

    def test_permission_mode_override_applies_to_all_phases(self):
        result, by_phase = self._run_with_argv_log(["--claude-permission-mode", "acceptEdits"])
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        for phase, argv in by_phase.items():
            self.assertIn("--permission-mode acceptEdits", argv, phase)

    def test_phase_permission_override_targets_single_phase(self):
        result, by_phase = self._run_with_argv_log(["--phase-permission", "autoreview=default"])
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        self.assertIn("--permission-mode default", by_phase["autoreview"])
        self.assertIn("--permission-mode bypassPermissions", by_phase["implementation"])

    def test_allowed_tools_passthrough(self):
        result, by_phase = self._run_with_argv_log(["--claude-allowed-tools", "Bash(git:*)"])
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        for phase, argv in by_phase.items():
            self.assertIn("--allowedTools Bash(git:*)", argv, phase)

    def test_unknown_phase_permission_rejected(self):
        result, _ = self._run_with_argv_log(["--phase-permission", "nope=default"])
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("nope", result.stderr)

    # --- git gating --------------------------------------------------------

    def test_required_phase_without_commit_halts(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = self.make_repo(root)
            phase_log = root / "phases.log"
            prompt_log = root / "prompts.log"
            skills = self.make_skill_root(root, "claude-skills", ["review-loop", "autoreview"])
            fake = self.make_fake_engine(
                root, "claude", 'print("AUTOBUILD_REPORT: " + phase + ": completed - no changes")\n'
            )
            env = self.base_env(root, phase_log, prompt_log, claude_skills=skills)
            result = self.invoke(repo, ["--claude-bin", str(fake)], env)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("implementation did not create a commit", result.stderr)
            self.assertEqual(phase_log.read_text().splitlines(), ["claude implementation"])

    def test_phase_changing_branch_halts(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = self.make_repo(root)
            phase_log = root / "phases.log"
            prompt_log = root / "prompts.log"
            skills = self.make_skill_root(root, "claude-skills", ["review-loop", "autoreview"])
            body = """
if phase == "implementation":
    subprocess.check_call(["git", "switch", "-c", "other"])
    target = Path("x.txt")
    target.write_text("x\\n", encoding="utf-8")
    subprocess.check_call(["git", "add", str(target)])
    subprocess.check_call(["git", "commit", "-m", "wrong branch"])
print("AUTOBUILD_REPORT: " + phase + ": completed - moved")
"""
            fake = self.make_fake_engine(root, "claude", body)
            env = self.base_env(root, phase_log, prompt_log, claude_skills=skills)
            result = self.invoke(repo, ["--claude-bin", str(fake)], env)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("changed branch from feature to other", result.stderr)

    def test_phase_rewinding_branch_halts(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = self.make_repo(root)
            phase_log = root / "phases.log"
            prompt_log = root / "prompts.log"
            skills = self.make_skill_root(root, "claude-skills", ["review-loop", "autoreview"])
            body = """
if phase in {"implementation", "review-loop"}:
    target = Path("phase-" + phase + ".txt")
    target.write_text("done " + phase + "\\n", encoding="utf-8")
    subprocess.check_call(["git", "add", str(target)])
    subprocess.check_call(["git", "commit", "-m", "work " + phase])
elif phase == "autoreview":
    subprocess.check_call(["git", "reset", "--hard", "HEAD~1"])
print("AUTOBUILD_REPORT: " + phase + ": completed - ok")
"""
            fake = self.make_fake_engine(root, "claude", body)
            env = self.base_env(root, phase_log, prompt_log, claude_skills=skills)
            result = self.invoke(repo, ["--claude-bin", str(fake)], env)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("non-descendant commit", result.stderr)

    def test_dirty_worktree_after_phase_halts(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = self.make_repo(root)
            phase_log = root / "phases.log"
            prompt_log = root / "prompts.log"
            skills = self.make_skill_root(root, "claude-skills", ["review-loop", "autoreview"])
            body = """
Path("leftover.txt").write_text("dirty\\n", encoding="utf-8")
print("AUTOBUILD_REPORT: " + phase + ": completed - left a mess")
"""
            fake = self.make_fake_engine(root, "claude", body)
            env = self.base_env(root, phase_log, prompt_log, claude_skills=skills)
            result = self.invoke(repo, ["--claude-bin", str(fake)], env)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("left a dirty worktree", result.stderr)

    def test_git_location_env_is_scrubbed(self):
        # GIT_DIR/GIT_WORK_TREE in the launching env (e.g. from a git hook) must
        # not redirect the helper's or engine's git commands away from --repo.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = self.make_repo(root)  # on branch 'feature'
            phase_log = root / "phases.log"
            prompt_log = root / "prompts.log"
            skills = self.make_skill_root(root, "claude-skills", ["review-loop", "autoreview"])
            # Decoy repo left on the protected 'main' branch.
            decoy = root / "decoy"
            decoy.mkdir()
            run(["git", "init"], decoy)
            run(["git", "branch", "-M", "main"], decoy)
            run(["git", "config", "user.email", "decoy@example.invalid"], decoy)
            run(["git", "config", "user.name", "Decoy"], decoy)
            (decoy / "README.md").write_text("decoy\n", encoding="utf-8")
            run(["git", "add", "README.md"], decoy)
            run(["git", "commit", "-m", "decoy init"], decoy)

            fake = self.make_fake_engine(root, "claude", STANDARD_BODY)
            env = self.base_env(root, phase_log, prompt_log, claude_skills=skills)
            env["GIT_DIR"] = str(decoy / ".git")
            env["GIT_WORK_TREE"] = str(decoy)
            result = self.invoke(repo, ["--claude-bin", str(fake)], env)

            # Without scrubbing, validate_work_branch would read the decoy's
            # 'main' and refuse, or commits would land in the decoy.
            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            self.assertIn("work implementation", run(["git", "log", "--oneline"], repo).stdout)
            self.assertEqual(run(["git", "log", "--oneline"], decoy).stdout.count("\n"), 1)

    def test_missing_report_marker_halts(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = self.make_repo(root)
            phase_log = root / "phases.log"
            prompt_log = root / "prompts.log"
            skills = self.make_skill_root(root, "claude-skills", ["review-loop", "autoreview"])
            fake = self.make_fake_engine(root, "claude", 'print("no marker here")\n')
            env = self.base_env(root, phase_log, prompt_log, claude_skills=skills)
            result = self.invoke(repo, ["--claude-bin", str(fake)], env)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("exactly one AUTOBUILD_REPORT line", result.stderr)

    # --- retries -----------------------------------------------------------

    def test_blocked_phase_retries_then_halts(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = self.make_repo(root)
            phase_log = root / "phases.log"
            prompt_log = root / "prompts.log"
            skills = self.make_skill_root(root, "claude-skills", ["review-loop", "autoreview"])
            body = """
if phase in {"implementation", "review-loop"}:
    target = Path("phase-" + phase + ".txt")
    target.write_text("done " + phase + "\\n", encoding="utf-8")
    subprocess.check_call(["git", "add", str(target)])
    subprocess.check_call(["git", "commit", "-m", "work " + phase])
    print("AUTOBUILD_REPORT: " + phase + ": completed - ok")
elif phase == "autoreview":
    print("AUTOBUILD_REPORT: autoreview: blocked - review failed")
else:
    print("AUTOBUILD_REPORT: " + phase + ": completed - should not run")
"""
            fake = self.make_fake_engine(root, "claude", body)
            env = self.base_env(root, phase_log, prompt_log, claude_skills=skills)
            result = self.invoke(repo, ["--claude-bin", str(fake), "--max-retries", "1"], env)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("autoreview reported blocked", result.stderr)
            lines = phase_log.read_text().splitlines()
            self.assertEqual(lines.count("claude autoreview"), 2)
            self.assertNotIn("claude pr-creation", lines)
            prompts = prompt_log.read_text(encoding="utf-8")
            self.assertIn("Previous attempt reported", prompts)

    # --- report dir & branch guards ---------------------------------------

    def test_report_dir_inside_repo_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = self.make_repo(root)
            phase_log = root / "phases.log"
            prompt_log = root / "prompts.log"
            fake = self.make_fake_engine(root, "claude", STANDARD_BODY)
            env = self.base_env(root, phase_log, prompt_log)
            result = self.invoke(
                repo, ["--claude-bin", str(fake), "--report-dir", str(repo / "reports")], env
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("--report-dir must be outside the repository", result.stderr)
            self.assertFalse(phase_log.exists())

    def _expect_protected(self, switch_to, base):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = self.make_repo(root)
            run(["git", "switch", switch_to] if switch_to in ("main",) else ["git", "switch", "-c", switch_to], repo)
            phase_log = root / "phases.log"
            prompt_log = root / "prompts.log"
            fake = self.make_fake_engine(root, "claude", STANDARD_BODY)
            env = self.base_env(root, phase_log, prompt_log)
            args = [
                sys.executable, str(SCRIPT), "--repo", str(repo), "--task", SENTINEL_TASK,
                "--base", base, "--claude-bin", str(fake),
            ]
            result = subprocess.run(args, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn(f"refusing to run on protected branch {switch_to}", result.stderr)
            self.assertFalse(phase_log.exists())

    def test_protected_branch_bare_main(self):
        self._expect_protected("main", "origin/main")

    def test_protected_branch_full_remote_ref(self):
        self._expect_protected("develop", "refs/remotes/origin/develop")

    def test_protected_branch_custom_remote_prefix(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = self.make_repo(root)
            run(["git", "remote", "add", "fork", "https://example.invalid/x.git"], repo)
            run(["git", "switch", "-c", "release"], repo)
            phase_log = root / "phases.log"
            prompt_log = root / "prompts.log"
            fake = self.make_fake_engine(root, "claude", STANDARD_BODY)
            env = self.base_env(root, phase_log, prompt_log)
            args = [
                sys.executable, str(SCRIPT), "--repo", str(repo), "--task", SENTINEL_TASK,
                "--base", "fork/release", "--claude-bin", str(fake),
            ]
            result = subprocess.run(args, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("refusing to run on protected branch release", result.stderr)

    def test_slashed_local_base_branch_protected(self):
        # A local work branch named exactly like the base (with a slash) must be
        # refused, but the slash must not be misread as a remote prefix.
        self._expect_protected("hotfix/login", "hotfix/login")

    def test_detached_head_refused(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = self.make_repo(root)
            head = run(["git", "rev-parse", "HEAD"], repo).stdout.strip()
            run(["git", "checkout", head], repo)
            phase_log = root / "phases.log"
            prompt_log = root / "prompts.log"
            fake = self.make_fake_engine(root, "claude", STANDARD_BODY)
            env = self.base_env(root, phase_log, prompt_log)
            result = self.invoke(repo, ["--claude-bin", str(fake)], env)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("detached HEAD", result.stderr)

    # --- dependency check --------------------------------------------------

    def test_missing_dependency_halts_with_install_message(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = self.make_repo(root)
            phase_log = root / "phases.log"
            prompt_log = root / "prompts.log"
            # skills dir exists but is missing 'autoreview'.
            skills = self.make_skill_root(root, "claude-skills", ["review-loop"])
            fake = self.make_fake_engine(root, "claude", STANDARD_BODY)
            env = self.base_env(root, phase_log, prompt_log, claude_skills=skills)
            result = self.invoke(repo, ["--claude-bin", str(fake)], env)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("autoreview", result.stderr)
            self.assertIn("brian-bell/agent-skills", result.stderr)
            self.assertNotIn("review-loop (looked", result.stderr)  # present one not listed
            self.assertFalse(phase_log.exists())

    def test_skill_check_accepts_claude_skills_dir(self):
        # No AUTOBUILD_CLAUDE_SKILLS_DIR override: fall back to ~/.claude/skills.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = self.make_repo(root)
            phase_log = root / "phases.log"
            prompt_log = root / "prompts.log"
            home = root / "home"
            home.mkdir()
            self.make_skill_root(home / ".claude", "skills", ["review-loop", "autoreview"])
            fake = self.make_fake_engine(root, "claude", STANDARD_BODY)
            env = self.base_env(root, phase_log, prompt_log)
            env["HOME"] = str(home)
            result = self.invoke(repo, ["--claude-bin", str(fake)], env)
            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            self.assertEqual(len(phase_log.read_text().splitlines()), 4)

    def test_skip_skill_check_bypasses_missing_dependency(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = self.make_repo(root)
            phase_log = root / "phases.log"
            prompt_log = root / "prompts.log"
            skills = self.make_skill_root(root, "claude-skills", [])  # nothing installed
            fake = self.make_fake_engine(root, "claude", STANDARD_BODY)
            env = self.base_env(root, phase_log, prompt_log, claude_skills=skills)
            result = self.invoke(repo, ["--claude-bin", str(fake), "--skip-skill-check"], env)
            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            self.assertEqual(len(phase_log.read_text().splitlines()), 4)

    # --- arg validation ----------------------------------------------------

    def test_requires_task(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = self.make_repo(root)
            args = [sys.executable, str(SCRIPT), "--repo", str(repo), "--base", "origin/main"]
            result = subprocess.run(args, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("--task", result.stderr)

    def test_dry_run_prints_prompts_without_launching(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = self.make_repo(root)
            phase_log = root / "phases.log"
            prompt_log = root / "prompts.log"
            # No skills installed and no fake engine: dry-run must skip both the
            # dependency check and any launch.
            env = self.base_env(root, phase_log, prompt_log)
            result = self.invoke(repo, ["--dry-run"], env)
            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            self.assertIn("implementation", result.stdout)
            self.assertIn("pr-creation", result.stdout)
            self.assertFalse(phase_log.exists())


if __name__ == "__main__":
    unittest.main()
