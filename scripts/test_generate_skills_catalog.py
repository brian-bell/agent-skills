#!/usr/bin/env python3
"""Behavior tests for the generated Vercel skills catalog."""

from __future__ import annotations

import importlib.util
import os
import subprocess
import stat
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
GENERATOR = ROOT / "scripts" / "generate-skills-catalog.py"


def load_generator_module():
    spec = importlib.util.spec_from_file_location("generate_skills_catalog", GENERATOR)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot load generator: {GENERATOR}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class CatalogGenerationTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp_dir.cleanup)
        self.fixture_root = Path(self.temp_dir.name) / "source"
        self.output = Path(self.temp_dir.name) / "catalog"
        self._write_fixture()

    def _write_fixture(self) -> None:
        skill = self.fixture_root / "skills" / "feature-review"
        (skill / "shared").mkdir(parents=True)
        (skill / "runtimes" / "codex" / "agents").mkdir(parents=True)
        (skill / "runtimes" / "claude").mkdir(parents=True)

        (skill / "shared" / "common.md").write_text("shared resource\n")
        (skill / "shared" / "collision.txt").write_text("shared\n")
        (skill / "runtimes" / "codex" / "SKILL.md").write_text(
            "---\nname: feature-review\ndescription: Codex review\n---\n\n# Codex\n"
        )
        (skill / "runtimes" / "codex" / "collision.txt").write_text("codex\n")
        (skill / "runtimes" / "codex" / "agents" / "openai.yaml").write_text(
            "interface:\n  display_name: Feature Review\n"
        )
        (skill / "runtimes" / "claude" / "SKILL.md").write_text(
            "---\nname: feature-review\ndescription: Claude review\n---\n\n# Claude\n"
        )
        (skill / "runtimes" / "claude" / "collision.txt").write_text("claude\n")

        third_party = self.fixture_root / "third-party" / "last30days"
        (third_party / "scripts" / "lib" / "__pycache__").mkdir(parents=True)
        (third_party / "SKILL.md").write_text(
            "---\nname: last30days\ndescription: Recent research\n---\n"
        )
        (third_party / "scripts" / "lib" / "__pycache__" / "local.pyc").write_bytes(
            b"local runtime cache"
        )
        (self.fixture_root / "third-party" / "ATTRIBUTION.md").write_text(
            "| Skill | Source | License |\n"
            "|---|---|---|\n"
            "| `last30days` | https://example.test/last30days | MIT |\n"
        )

    def _generate(self) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [
                sys.executable,
                str(GENERATOR),
                "--source-root",
                str(self.fixture_root),
                "--output",
                str(self.output),
            ],
            text=True,
            capture_output=True,
            check=False,
        )

    def _generate_real_catalog(self) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [
                sys.executable,
                str(GENERATOR),
                "--source-root",
                str(ROOT),
                "--output",
                str(self.output),
            ],
            text=True,
            capture_output=True,
            check=False,
        )

    def test_first_party_router_and_runtime_assemblies(self) -> None:
        result = self._generate()
        self.assertEqual(0, result.returncode, result.stderr)

        package = self.output / "skills" / "feature-review"
        router = (package / "SKILL.md").read_text()
        self.assertIn("name: feature-review", router)
        self.assertIn("runtimes/codex/SKILL.md", router)
        self.assertIn("runtimes/claude/SKILL.md", router)
        self.assertIn("Never combine", router)

        codex = package / "runtimes" / "codex"
        claude = package / "runtimes" / "claude"
        self.assertEqual("shared resource\n", (codex / "common.md").read_text())
        self.assertEqual("shared resource\n", (claude / "common.md").read_text())
        self.assertEqual("codex\n", (codex / "collision.txt").read_text())
        self.assertEqual("claude\n", (claude / "collision.txt").read_text())

    def test_feature_review_metadata_and_runtime_isolation(self) -> None:
        result = self._generate_real_catalog()
        self.assertEqual(0, result.returncode, result.stderr)

        source = ROOT / "skills" / "feature-review"
        package = self.output / "skills" / "feature-review"
        codex = package / "runtimes" / "codex"
        claude = package / "runtimes" / "claude"

        for role in (source / "shared" / "roles").iterdir():
            self.assertEqual(role.read_bytes(), (codex / "roles" / role.name).read_bytes())
            self.assertEqual(role.read_bytes(), (claude / "roles" / role.name).read_bytes())

        self.assertFalse((codex / "findings-schema.md").exists())
        self.assertTrue((claude / "findings-schema.md").is_file())
        self.assertNotIn("AskUserQuestion", (codex / "SKILL.md").read_text())

        metadata = source / "runtimes" / "codex" / "agents" / "openai.yaml"
        self.assertEqual(
            metadata.read_bytes(),
            (package / "agents" / "openai.yaml").read_bytes(),
        )

    def test_third_party_fidelity_and_provenance(self) -> None:
        result = self._generate_real_catalog()
        self.assertEqual(0, result.returncode, result.stderr)

        source = ROOT / "third-party" / "last30days"
        package = self.output / "skills" / "last30days"
        for source_file in source.rglob("*"):
            if not source_file.is_file():
                continue
            if (
                "__pycache__" in source_file.parts
                or source_file.suffix in {".pyc", ".pyo"}
                or source_file.name == ".DS_Store"
            ):
                continue
            emitted_file = package / source_file.relative_to(source)
            self.assertTrue(emitted_file.is_file(), emitted_file)
            self.assertEqual(source_file.read_bytes(), emitted_file.read_bytes())
            self.assertEqual(
                stat.S_IMODE(source_file.stat().st_mode),
                stat.S_IMODE(emitted_file.stat().st_mode),
                emitted_file,
            )

        executable = package / "scripts" / "build-skill.sh"
        self.assertTrue(executable.stat().st_mode & stat.S_IXUSR)
        self.assertFalse((package / "scripts" / "lib" / "__pycache__").exists())
        self.assertTrue((package / "agents" / "openai.yaml").is_file())
        self.assertTrue((package / "references" / "save-html-brief.md").is_file())

        source_attribution = (ROOT / "third-party" / "ATTRIBUTION.md").read_text()
        provenance_row = next(
            line for line in source_attribution.splitlines() if "`last30days`" in line
        )
        self.assertIn(provenance_row, (package / "ATTRIBUTION.md").read_text())

    def test_missing_runtime_fails_without_partial_output(self) -> None:
        shutil_target = self.fixture_root / "skills" / "feature-review" / "runtimes" / "claude"
        for child in shutil_target.iterdir():
            child.unlink()
        shutil_target.rmdir()

        result = self._generate()

        self.assertNotEqual(0, result.returncode)
        self.assertIn("skills/feature-review/runtimes/claude", result.stderr)
        self.assertFalse((self.output / "skills" / "feature-review").exists())

    def test_checked_in_catalog_matches_generation(self) -> None:
        result = self._generate_real_catalog()
        self.assertEqual(0, result.returncode, result.stderr)

        def manifest(root: Path) -> dict[str, tuple[str, bytes | None, int | None]]:
            entries: dict[str, tuple[str, bytes | None, int | None]] = {}
            for path in root.rglob("*"):
                relative = path.relative_to(root).as_posix()
                if path.is_dir():
                    entries[relative] = ("directory", None, None)
                else:
                    entries[relative] = (
                        "file",
                        path.read_bytes(),
                        stat.S_IMODE(path.stat().st_mode),
                    )
            return entries

        self.assertEqual(manifest(self.output), manifest(ROOT / "catalog"))

    def test_output_cannot_overlap_catalog_sources(self) -> None:
        source_skill = (
            self.fixture_root
            / "skills"
            / "feature-review"
            / "runtimes"
            / "codex"
            / "SKILL.md"
        )
        result = subprocess.run(
            [
                sys.executable,
                str(GENERATOR),
                "--source-root",
                str(self.fixture_root),
                "--output",
                str(self.fixture_root / "skills"),
            ],
            text=True,
            capture_output=True,
            check=False,
        )

        self.assertNotEqual(0, result.returncode)
        self.assertIn("output overlaps catalog sources", result.stderr)
        self.assertTrue(source_skill.is_file())

    def test_transient_runtime_caches_are_not_emitted(self) -> None:
        result = self._generate()
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertFalse(
            (
                self.output
                / "skills"
                / "last30days"
                / "scripts"
                / "lib"
                / "__pycache__"
            ).exists()
        )

    def test_symlinked_output_is_rejected_without_touching_target(self) -> None:
        target = Path(self.temp_dir.name) / "catalog-target"
        target.mkdir()
        sentinel = target / "sentinel.txt"
        sentinel.write_text("keep me\n")
        output_link = Path(self.temp_dir.name) / "catalog-link"
        output_link.symlink_to(target, target_is_directory=True)

        result = subprocess.run(
            [
                sys.executable,
                str(GENERATOR),
                "--source-root",
                str(self.fixture_root),
                "--output",
                str(output_link),
            ],
            text=True,
            capture_output=True,
            check=False,
        )

        self.assertNotEqual(0, result.returncode)
        self.assertIn("output must not be a symlink", result.stderr)
        self.assertEqual("keep me\n", sentinel.read_text())
        self.assertEqual([sentinel], list(target.iterdir()))
        self.assertTrue(output_link.is_symlink())

    def test_generated_file_modes_do_not_depend_on_umask(self) -> None:
        previous_umask = os.umask(0o077)
        try:
            result = self._generate()
        finally:
            os.umask(previous_umask)

        self.assertEqual(0, result.returncode, result.stderr)
        for generated_file in (
            self.output / "skills" / "feature-review" / "SKILL.md",
            self.output / "skills" / "last30days" / "ATTRIBUTION.md",
        ):
            self.assertEqual(0o644, stat.S_IMODE(generated_file.stat().st_mode))
        for generated_directory in (
            self.output,
            self.output / "skills",
            self.output / "skills" / "feature-review",
            self.output / "skills" / "feature-review" / "agents",
            self.output / "skills" / "feature-review" / "runtimes",
        ):
            self.assertEqual(
                0o755,
                stat.S_IMODE(generated_directory.stat().st_mode),
                generated_directory,
            )

    def test_missing_skill_entry_point_fails_without_output(self) -> None:
        entry_point = (
            self.fixture_root
            / "skills"
            / "feature-review"
            / "runtimes"
            / "codex"
            / "SKILL.md"
        )
        entry_point.unlink()

        result = self._generate()

        self.assertNotEqual(0, result.returncode)
        self.assertIn("skills/feature-review/runtimes/codex/SKILL.md", result.stderr)
        self.assertFalse(self.output.exists())

    def test_skill_entry_points_must_be_regular_files(self) -> None:
        entry_points = (
            Path("skills/feature-review/runtimes/codex/SKILL.md"),
            Path("skills/feature-review/runtimes/claude/SKILL.md"),
            Path("third-party/last30days/SKILL.md"),
        )
        for relative_path in entry_points:
            with self.subTest(entry_point=relative_path.as_posix()):
                entry_point = self.fixture_root / relative_path
                original = entry_point.read_bytes()
                entry_point.unlink()
                entry_point.mkdir()

                result = self._generate()

                self.assertNotEqual(0, result.returncode)
                self.assertIn(relative_path.as_posix(), result.stderr)
                self.assertFalse(self.output.exists())

                entry_point.rmdir()
                entry_point.write_bytes(original)

    def test_publish_failure_restores_previous_catalog(self) -> None:
        self.output.mkdir()
        sentinel = self.output / "sentinel.txt"
        sentinel.write_text("previous catalog\n")
        calls = 0

        def fail_second_replace(source: Path, destination: Path) -> None:
            nonlocal calls
            calls += 1
            if calls == 2:
                raise OSError("injected publish failure")
            source.replace(destination)

        generator = load_generator_module()
        with self.assertRaisesRegex(OSError, "injected publish failure"):
            generator.generate(
                self.fixture_root,
                self.output,
                replace=fail_second_replace,
            )

        self.assertEqual(3, calls)
        self.assertEqual("previous catalog\n", sentinel.read_text())
        self.assertEqual([sentinel], list(self.output.iterdir()))

    def test_existing_file_output_is_rejected_without_replacement(self) -> None:
        self.output.write_text("not a catalog\n")

        result = self._generate()

        self.assertNotEqual(0, result.returncode)
        self.assertIn("existing output must be a directory", result.stderr)
        self.assertTrue(self.output.is_file())
        self.assertEqual("not a catalog\n", self.output.read_text())
        self.assertEqual(
            [],
            list(self.output.parent.glob(f".{self.output.name}.backup.*")),
        )

    def test_source_symlink_is_rejected_without_copying_target(self) -> None:
        secret = Path(self.temp_dir.name) / "outside-secret.txt"
        secret.write_text("do not publish\n")
        source_link = (
            self.fixture_root / "third-party" / "last30days" / "leaked-secret.txt"
        )
        source_link.symlink_to(secret)

        result = self._generate()

        self.assertNotEqual(0, result.returncode)
        self.assertIn("symlink source entry is not allowed", result.stderr)
        self.assertFalse(self.output.exists())
        self.assertEqual("do not publish\n", secret.read_text())

    @unittest.skipUnless(hasattr(os, "mkfifo"), "FIFO support is required")
    def test_special_source_entry_is_rejected_before_copy(self) -> None:
        fifo = self.fixture_root / "third-party" / "last30days" / "named-pipe"
        os.mkfifo(fifo)

        result = self._generate()

        self.assertNotEqual(0, result.returncode)
        self.assertIn("special source entry is not allowed", result.stderr)
        self.assertFalse(self.output.exists())


if __name__ == "__main__":
    unittest.main()
