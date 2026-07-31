#!/usr/bin/env python3
"""Behavior tests for the generated Vercel skills catalog."""

from __future__ import annotations

import importlib.util
import os
import shutil
import subprocess
import stat
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
GENERATOR = ROOT / "scripts" / "generate-skills-catalog.py"
FIRST_PARTY = {
    "autofix",
    "chrome-reading-list",
    "docs",
    "feature-review",
    "go-review",
    "product-manager",
    "ship",
    "slice-issues",
    "tdd",
    "tdd-with-review",
}
THIRD_PARTY = {
    "autoreview",
    "batch-grill-me",
    "grill-me",
    "improve-codebase-architecture",
    "last30days",
    "prd-to-issues",
    "prd-to-plan",
    "review-loop",
    "teach",
    "wizard",
    "write-a-prd",
}
TRANSIENT_NAMES = {"__pycache__", ".DS_Store"}
TRANSIENT_SUFFIXES = {".pyc", ".pyo"}


def has_source_material(root: Path) -> bool:
    return any(
        path.name not in TRANSIENT_NAMES
        and path.suffix not in TRANSIENT_SUFFIXES
        and "__pycache__" not in path.parts
        for path in root.rglob("*")
        if path.is_file()
    )


def source_files(root: Path) -> dict[str, tuple[bytes, int]]:
    entries: dict[str, tuple[bytes, int]] = {}
    for path in root.rglob("*"):
        if not path.is_file() or path.is_symlink():
            continue
        if (
            path.name in TRANSIENT_NAMES
            or path.suffix in TRANSIENT_SUFFIXES
            or "__pycache__" in path.parts
        ):
            continue
        entries[path.relative_to(root).as_posix()] = (
            path.read_bytes(),
            stat.S_IMODE(path.stat().st_mode),
        )
    return entries


def tree_manifest(root: Path) -> dict[str, tuple[str, bytes | None, int]]:
    entries: dict[str, tuple[str, bytes | None, int]] = {
        ".": ("directory", None, stat.S_IMODE(root.lstat().st_mode))
    }
    for path in root.rglob("*"):
        relative = path.relative_to(root).as_posix()
        mode = stat.S_IMODE(path.lstat().st_mode)
        if path.is_symlink():
            entries[relative] = ("symlink", os.readlink(path).encode(), mode)
        elif path.is_dir():
            entries[relative] = ("directory", None, mode)
        else:
            entries[relative] = ("file", path.read_bytes(), mode)
    return entries


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

    def _generate(self, *, check: bool = False) -> subprocess.CompletedProcess[str]:
        command = [
                sys.executable,
                str(GENERATOR),
                "--source-root",
                str(self.fixture_root),
                "--output",
                str(self.output),
            ]
        if check:
            command.append("--check")
        return subprocess.run(
            command,
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

    def _assert_failure_preserves_output(self, diagnostic: str) -> None:
        if not self.output.exists():
            self.output.mkdir()
            (self.output / "sentinel.txt").write_text("keep me\n")
        before = source_files(self.output)

        result = self._generate()

        self.assertNotEqual(0, result.returncode)
        self.assertIn(diagnostic, result.stderr)
        self.assertEqual(before, source_files(self.output))
        self.assertEqual([], list(self.output.parent.glob(f".{self.output.name}.*")))

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

    def test_real_inventory_contains_every_portable_skill_exactly_once(self) -> None:
        result = self._generate_real_catalog()
        self.assertEqual(0, result.returncode, result.stderr)

        source_first_party = {
            path.name
            for path in (ROOT / "skills").iterdir()
            if path.is_dir() and has_source_material(path)
        }
        source_third_party = {
            path.name
            for path in (ROOT / "third-party").iterdir()
            if path.is_dir()
        }
        generated = {
            path.name for path in (self.output / "skills").iterdir() if path.is_dir()
        }

        self.assertEqual(FIRST_PARTY, source_first_party)
        self.assertEqual(THIRD_PARTY, source_third_party)
        self.assertEqual(FIRST_PARTY | THIRD_PARTY, generated)
        self.assertEqual(21, len(generated))

        for name in FIRST_PARTY:
            router = (self.output / "skills" / name / "SKILL.md").read_text()
            self.assertIn(f"name: {name}\n", router)
            self.assertIn(f"# {name} Runtime Router", router)
            self.assertIn("runtimes/codex/SKILL.md", router)
            self.assertIn("runtimes/claude/SKILL.md", router)
            self.assertIn("does not support the current runtime", router)

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

    def test_every_first_party_runtime_is_shared_plus_its_overlay(self) -> None:
        result = self._generate_real_catalog()
        self.assertEqual(0, result.returncode, result.stderr)

        for name in FIRST_PARTY:
            with self.subTest(skill=name):
                source = ROOT / "skills" / name
                package = self.output / "skills" / name
                shared = source_files(source / "shared")
                for runtime in ("codex", "claude"):
                    expected = shared | source_files(source / "runtimes" / runtime)
                    actual = source_files(package / "runtimes" / runtime)
                    self.assertEqual(expected, actual)

                metadata = (
                    source / "runtimes" / "codex" / "agents" / "openai.yaml"
                )
                promoted = package / "agents" / "openai.yaml"
                self.assertEqual(metadata.is_file(), promoted.is_file())
                if metadata.is_file():
                    self.assertEqual(metadata.read_bytes(), promoted.read_bytes())
                    self.assertEqual(
                        stat.S_IMODE(metadata.stat().st_mode),
                        stat.S_IMODE(promoted.stat().st_mode),
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

    def test_every_third_party_package_is_faithful_and_attributed(self) -> None:
        result = self._generate_real_catalog()
        self.assertEqual(0, result.returncode, result.stderr)
        attribution_lines = (ROOT / "third-party" / "ATTRIBUTION.md").read_text().splitlines()

        for name in THIRD_PARTY:
            with self.subTest(skill=name):
                source = ROOT / "third-party" / name
                package = self.output / "skills" / name
                expected = source_files(source)
                actual = source_files(package)
                provenance = actual.pop("ATTRIBUTION.md")
                self.assertEqual(expected, actual)
                rows = [
                    line for line in attribution_lines if line.startswith(f"| `{name}` |")
                ]
                self.assertEqual(1, len(rows))
                self.assertIn(rows[0], provenance[0].decode())

    def test_missing_runtime_fails_without_partial_output(self) -> None:
        shutil_target = self.fixture_root / "skills" / "feature-review" / "runtimes" / "claude"
        for child in shutil_target.iterdir():
            child.unlink()
        shutil_target.rmdir()

        result = self._generate()

        self.assertNotEqual(0, result.returncode)
        self.assertIn("skills/feature-review/runtimes/claude", result.stderr)
        self.assertFalse((self.output / "skills" / "feature-review").exists())

    def test_frontmatter_name_must_match_package_directory(self) -> None:
        entry_point = (
            self.fixture_root
            / "skills"
            / "feature-review"
            / "runtimes"
            / "codex"
            / "SKILL.md"
        )
        entry_point.write_text(
            "---\nname: wrong-name\ndescription: Codex review\n---\n\n# Codex\n"
        )
        self.output.mkdir()
        sentinel = self.output / "sentinel.txt"
        sentinel.write_text("keep me\n")

        result = self._generate()

        self.assertNotEqual(0, result.returncode)
        self.assertIn("frontmatter name 'wrong-name' must match directory 'feature-review'", result.stderr)
        self.assertEqual("keep me\n", sentinel.read_text())
        self.assertEqual([sentinel], list(self.output.iterdir()))
        self.assertEqual([], list(self.output.parent.glob(f".{self.output.name}.*")))

    def test_invalid_required_frontmatter_fails_closed(self) -> None:
        entry_point = (
            self.fixture_root
            / "skills"
            / "feature-review"
            / "runtimes"
            / "codex"
            / "SKILL.md"
        )
        original = entry_point.read_text()
        cases = {
            "missing required frontmatter": "# no frontmatter\n",
            "unterminated or oversized required frontmatter": (
                "---\nname: feature-review\ndescription: missing close\n"
            ),
            "missing required frontmatter name": "---\ndescription: review\n---\n",
            "missing required frontmatter description": "---\nname: feature-review\n---\n",
            "duplicate required frontmatter name": (
                "---\nname: feature-review\nname: feature-review\ndescription: review\n---\n"
            ),
            "duplicate required frontmatter description": (
                "---\nname: feature-review\ndescription: one\ndescription: two\n---\n"
            ),
            "invalid name scalar": (
                "---\nname: [feature-review]\ndescription: review\n---\n"
            ),
            "empty required frontmatter description": (
                "---\nname: feature-review\ndescription: ''\n---\n"
            ),
            "unsafe install name": (
                "---\nname: ../feature-review\ndescription: review\n---\n"
            ),
        }
        try:
            for diagnostic, content in cases.items():
                with self.subTest(diagnostic=diagnostic):
                    entry_point.write_text(content)
                    self._assert_failure_preserves_output(diagnostic)
        finally:
            entry_point.write_text(original)

    def test_first_party_runtime_shape_and_cross_inventory_names_are_strict(self) -> None:
        extra_runtime = (
            self.fixture_root / "skills" / "feature-review" / "runtimes" / "other"
        )
        extra_runtime.mkdir()
        (extra_runtime / "SKILL.md").write_text(
            "---\nname: feature-review\ndescription: Other\n---\n"
        )
        self._assert_failure_preserves_output("exactly codex and claude runtime variants")
        shutil.rmtree(extra_runtime)

        legacy = self.fixture_root / "skills" / "feature-review" / "SKILL.md"
        legacy.write_text("legacy\n")
        self._assert_failure_preserves_output("must not contain root legacy SKILL.md")
        legacy.unlink()

        duplicate = self.fixture_root / "skills" / "last30days"
        shutil.copytree(self.fixture_root / "skills" / "feature-review", duplicate)
        for entry_point in (
            duplicate / "runtimes" / "codex" / "SKILL.md",
            duplicate / "runtimes" / "claude" / "SKILL.md",
        ):
            entry_point.write_text(
                "---\nname: last30days\ndescription: Duplicate\n---\n"
            )
        self._assert_failure_preserves_output("duplicate install name 'last30days'")

    def test_attribution_table_must_map_packages_one_to_one(self) -> None:
        attribution = self.fixture_root / "third-party" / "ATTRIBUTION.md"
        original = attribution.read_text()
        cases = {
            "missing attribution row for third-party package 'last30days'": (
                "| Skill | Source | License |\n|---|---|---|\n"
            ),
            "duplicate attribution row for third-party package 'last30days'": (
                original + "| `last30days` | https://duplicate.test | MIT |\n"
            ),
            "orphan attribution row for unknown package 'orphan'": (
                original + "| `orphan` | https://orphan.test | MIT |\n"
            ),
        }
        try:
            for diagnostic, content in cases.items():
                with self.subTest(diagnostic=diagnostic):
                    attribution.write_text(content)
                    self._assert_failure_preserves_output(diagnostic)
        finally:
            attribution.write_text(original)

        collision = self.fixture_root / "third-party" / "last30days" / "ATTRIBUTION.md"
        collision.write_text("source-owned attribution\n")
        self._assert_failure_preserves_output("source package reserves generated ATTRIBUTION.md")

    def test_checked_in_catalog_matches_generation(self) -> None:
        result = subprocess.run(
            [sys.executable, str(GENERATOR), "--check"],
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertEqual(0, result.returncode, result.stderr)

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

    def test_repeated_generation_is_byte_for_byte_deterministic_across_umasks(self) -> None:
        first = Path(self.temp_dir.name) / "first"
        second = Path(self.temp_dir.name) / "second"

        def generate_at(output: Path, umask: int) -> subprocess.CompletedProcess[str]:
            previous_umask = os.umask(umask)
            try:
                return subprocess.run(
                    [
                        sys.executable,
                        str(GENERATOR),
                        "--source-root",
                        str(self.fixture_root),
                        "--output",
                        str(output),
                    ],
                    text=True,
                    capture_output=True,
                    check=False,
                )
            finally:
                os.umask(previous_umask)

        first_result = generate_at(first, 0o022)
        second_result = generate_at(second, 0o077)
        self.assertEqual(0, first_result.returncode, first_result.stderr)
        self.assertEqual(0, second_result.returncode, second_result.stderr)
        self.assertEqual(tree_manifest(first), tree_manifest(second))

    def test_check_mode_reports_drift_without_modifying_output(self) -> None:
        result = self._generate()
        self.assertEqual(0, result.returncode, result.stderr)

        matching = self._generate(check=True)
        self.assertEqual(0, matching.returncode, matching.stderr)
        self.assertEqual([], list(self.output.parent.glob(f".{self.output.name}.*")))

        changed = self.output / "skills" / "feature-review" / "SKILL.md"
        changed.write_text("stale content\n")
        extra = self.output / "extra.txt"
        extra.write_text("extra\n")
        missing = self.output / "skills" / "last30days" / "SKILL.md"
        missing.unlink()
        mode_only = self.output / "skills" / "feature-review" / "runtimes" / "codex" / "common.md"
        mode_only.chmod(0o600)
        self.output.chmod(0o700)
        before = {
            path.relative_to(self.output).as_posix(): (
                path.read_bytes() if path.is_file() else b"",
                stat.S_IMODE(path.stat().st_mode),
            )
            for path in self.output.rglob("*")
        }

        drift = self._generate(check=True)

        self.assertNotEqual(0, drift.returncode)
        self.assertIn("stale-content: skills/feature-review/SKILL.md", drift.stderr)
        self.assertIn("extra: extra.txt", drift.stderr)
        self.assertIn("missing: skills/last30days/SKILL.md", drift.stderr)
        self.assertIn(
            "stale-mode: skills/feature-review/runtimes/codex/common.md",
            drift.stderr,
        )
        self.assertIn("stale-mode: .", drift.stderr)
        after = {
            path.relative_to(self.output).as_posix(): (
                path.read_bytes() if path.is_file() else b"",
                stat.S_IMODE(path.stat().st_mode),
            )
            for path in self.output.rglob("*")
        }
        self.assertEqual(before, after)

    def test_check_mode_rejects_missing_or_symlinked_output_without_repair(self) -> None:
        missing = self._generate(check=True)
        self.assertNotEqual(0, missing.returncode)
        self.assertIn("missing output", missing.stderr)
        self.assertFalse(self.output.exists())

        target = Path(self.temp_dir.name) / "target"
        target.mkdir()
        (target / "sentinel.txt").write_text("keep\n")
        self.output.symlink_to(target, target_is_directory=True)
        unsafe = self._generate(check=True)
        self.assertNotEqual(0, unsafe.returncode)
        self.assertIn("output must not be a symlink", unsafe.stderr)
        self.assertEqual("keep\n", (target / "sentinel.txt").read_text())

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
