#!/usr/bin/env python3
"""Generate the Vercel CLI canary skills catalog."""

from __future__ import annotations

import argparse
import shutil
import stat
import sys
import tempfile
import uuid
from collections.abc import Callable
from pathlib import Path


ROUTER = """---
name: feature-review
description: Route feature-review to its isolated Codex or Claude Code runtime assembly.
---

# Feature Review Runtime Router

Identify the active host, then read and follow exactly one runtime assembly:

- Codex: `runtimes/codex/SKILL.md`
- Claude Code: `runtimes/claude/SKILL.md`

Never combine, merge, or fall back to the other runtime's instructions. If the
active host is neither Codex nor Claude Code, stop and report that this package
does not support the current runtime.
"""


class CatalogError(Exception):
    """A source tree cannot produce the canary catalog."""


ReplacePath = Callable[[Path, Path], None]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    repo_root = Path(__file__).resolve().parents[1]
    parser.add_argument("--source-root", type=Path, default=repo_root)
    parser.add_argument("--output", type=Path, default=repo_root / "catalog")
    return parser.parse_args()


def copy_tree(source: Path, destination: Path) -> None:
    shutil.copytree(
        source,
        destination,
        dirs_exist_ok=True,
        copy_function=shutil.copy2,
        ignore=shutil.ignore_patterns("__pycache__", "*.pyc", "*.pyo", ".DS_Store"),
    )


def write_portable_text(path: Path, content: str) -> None:
    path.write_text(content)
    path.chmod(0o644)


def validate_sources(source_root: Path) -> None:
    required_directories = (
        source_root / "skills" / "feature-review" / "shared",
    )
    required_files = (
        source_root
        / "skills"
        / "feature-review"
        / "runtimes"
        / "codex"
        / "SKILL.md",
        source_root
        / "skills"
        / "feature-review"
        / "runtimes"
        / "claude"
        / "SKILL.md",
        source_root
        / "skills"
        / "feature-review"
        / "runtimes"
        / "codex"
        / "agents"
        / "openai.yaml",
        source_root / "third-party" / "last30days" / "SKILL.md",
        source_root / "third-party" / "ATTRIBUTION.md",
    )
    for path in required_directories:
        if not path.exists() or not stat.S_ISDIR(path.lstat().st_mode):
            raise CatalogError(f"missing required catalog source: {path}")
    for path in required_files:
        if not path.exists() or not stat.S_ISREG(path.lstat().st_mode):
            raise CatalogError(f"missing required catalog source: {path}")

    for root in (
        source_root / "skills" / "feature-review" / "shared",
        source_root / "skills" / "feature-review" / "runtimes" / "codex",
        source_root / "skills" / "feature-review" / "runtimes" / "claude",
        source_root / "third-party" / "last30days",
    ):
        validate_source_tree(root)


def validate_source_tree(root: Path) -> None:
    for path in (root, *root.rglob("*")):
        mode = path.lstat().st_mode
        if stat.S_ISLNK(mode):
            raise CatalogError(f"symlink source entry is not allowed: {path}")
        if not stat.S_ISDIR(mode) and not stat.S_ISREG(mode):
            raise CatalogError(f"special source entry is not allowed: {path}")


def validate_output(source_root: Path, output: Path) -> None:
    if output.exists() and not output.is_dir():
        raise CatalogError(f"existing output must be a directory: {output}")
    for protected_source in (
        source_root / "skills",
        source_root / "third-party",
    ):
        if (
            output == protected_source
            or output in protected_source.parents
            or protected_source in output.parents
        ):
            raise CatalogError(
                f"output overlaps catalog sources: {output} and {protected_source}"
            )


def normalize_directory_modes(catalog_root: Path) -> None:
    catalog_root.chmod(0o755)
    for path in catalog_root.rglob("*"):
        if path.is_dir() and not path.is_symlink():
            path.chmod(0o755)


def generate_first_party(source_root: Path, catalog_root: Path) -> None:
    source = source_root / "skills" / "feature-review"
    package = catalog_root / "skills" / "feature-review"
    package.mkdir(parents=True)
    write_portable_text(package / "SKILL.md", ROUTER)

    for runtime in ("codex", "claude"):
        assembly = package / "runtimes" / runtime
        copy_tree(source / "shared", assembly)
        copy_tree(source / "runtimes" / runtime, assembly)

    metadata_source = source / "runtimes" / "codex" / "agents" / "openai.yaml"
    metadata_destination = package / "agents" / "openai.yaml"
    metadata_destination.parent.mkdir(parents=True)
    shutil.copy2(metadata_source, metadata_destination)


def generate_third_party(source_root: Path, catalog_root: Path) -> None:
    source = source_root / "third-party" / "last30days"
    package = catalog_root / "skills" / "last30days"
    copy_tree(source, package)

    attribution = (source_root / "third-party" / "ATTRIBUTION.md").read_text()
    provenance_row = next(
        line for line in attribution.splitlines() if "`last30days`" in line
    )
    write_portable_text(
        package / "ATTRIBUTION.md",
        "# Attribution\n\n"
        "| Skill | Source | License |\n"
        "|---|---|---|\n"
        f"{provenance_row}\n",
    )


def replace_path(source: Path, destination: Path) -> None:
    source.replace(destination)


def publish_catalog(staging: Path, output: Path, replace: ReplacePath) -> None:
    backup: Path | None = None
    if output.exists():
        backup = output.with_name(f".{output.name}.backup.{uuid.uuid4().hex}")
        replace(output, backup)

    try:
        replace(staging, output)
    except Exception:
        if backup is not None:
            try:
                replace(backup, output)
            except Exception as rollback_error:
                raise CatalogError(
                    f"catalog publish and rollback failed; previous catalog remains at {backup}"
                ) from rollback_error
        raise

    if backup is not None:
        shutil.rmtree(backup)


def generate(
    source_root: Path,
    output: Path,
    *,
    replace: ReplacePath = replace_path,
) -> None:
    validate_sources(source_root)
    validate_output(source_root, output)
    output.parent.mkdir(parents=True, exist_ok=True)
    staging = Path(tempfile.mkdtemp(prefix=f".{output.name}.", dir=output.parent))
    try:
        generate_first_party(source_root, staging)
        generate_third_party(source_root, staging)
        normalize_directory_modes(staging)
        publish_catalog(staging, output, replace)
    except Exception:
        shutil.rmtree(staging, ignore_errors=True)
        raise


def main() -> int:
    args = parse_args()
    source_root = args.source_root.resolve()
    try:
        output_path = args.output.expanduser().absolute()
        if output_path.is_symlink():
            raise CatalogError(f"output must not be a symlink: {output_path}")
        output = output_path.resolve()
        generate(source_root, output)
    except CatalogError as error:
        print(f"catalog generation failed: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
