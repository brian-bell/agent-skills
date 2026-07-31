#!/usr/bin/env python3
"""Generate the Vercel CLI canary skills catalog."""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import stat
import sys
import tempfile
import uuid
from collections.abc import Callable
from pathlib import Path


class CatalogError(Exception):
    """A source tree cannot produce the canary catalog."""


ReplacePath = Callable[[Path, Path], None]
SAFE_INSTALL_NAME = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
TRANSIENT_NAMES = {"__pycache__", ".DS_Store"}
TRANSIENT_SUFFIXES = {".pyc", ".pyo"}
MAX_FRONTMATTER_BYTES = 64 * 1024


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    repo_root = Path(__file__).resolve().parents[1]
    parser.add_argument("--source-root", type=Path, default=repo_root)
    parser.add_argument("--output", type=Path, default=repo_root / "catalog")
    parser.add_argument(
        "--check",
        action="store_true",
        help="validate sources and report catalog drift without publishing",
    )
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
    path.write_bytes(content.encode("utf-8"))
    path.chmod(0o644)


def is_transient(path: Path) -> bool:
    return path.name in TRANSIENT_NAMES or path.suffix in TRANSIENT_SUFFIXES


def contains_source_material(root: Path) -> bool:
    children = list(root.iterdir())
    if not children:
        return True
    for child in children:
        if child.name.startswith(".") or is_transient(child):
            continue
        mode = child.lstat().st_mode
        if not stat.S_ISDIR(mode) or contains_source_material(child):
            return True
    return False


def discover_packages(parent: Path, source_kind: str) -> list[Path]:
    if not parent.exists() or not stat.S_ISDIR(parent.lstat().st_mode):
        raise CatalogError(f"missing required {source_kind} source directory: {parent}")
    packages: list[Path] = []
    for path in sorted(parent.iterdir(), key=lambda item: item.name):
        if path.name.startswith(".") or is_transient(path):
            continue
        mode = path.lstat().st_mode
        if stat.S_ISLNK(mode):
            raise CatalogError(f"symlink source entry is not allowed: {path}")
        if stat.S_ISDIR(mode) and contains_source_material(path):
            packages.append(path)
    return packages


def require_regular_file(path: Path) -> None:
    if not path.exists() or not stat.S_ISREG(path.lstat().st_mode):
        raise CatalogError(f"missing required catalog source: {path}")


def parse_scalar(raw: str, *, key: str, path: Path, block_lines: list[str]) -> str:
    value = raw.strip()
    if value.startswith((">", "|")):
        if key != "description" or value not in {">", ">-", ">+", "|", "|-", "|+"}:
            raise CatalogError(f"invalid {key} scalar in required frontmatter: {path}")
        value = " ".join(line.strip() for line in block_lines if line.strip())
    elif value.startswith('"'):
        try:
            parsed = json.loads(value)
        except (json.JSONDecodeError, TypeError) as error:
            raise CatalogError(
                f"invalid {key} scalar in required frontmatter: {path}"
            ) from error
        if not isinstance(parsed, str):
            raise CatalogError(f"invalid {key} scalar in required frontmatter: {path}")
        value = parsed
    elif value.startswith("'"):
        if len(value) < 2 or not value.endswith("'"):
            raise CatalogError(f"invalid {key} scalar in required frontmatter: {path}")
        value = value[1:-1].replace("''", "'")
    elif not value or value[0] in "[{!&*" or value in {"~", "null", "Null", "NULL"}:
        raise CatalogError(f"invalid {key} scalar in required frontmatter: {path}")
    if not value.strip():
        raise CatalogError(f"empty required frontmatter {key}: {path}")
    return value.strip()


def read_required_frontmatter(path: Path) -> dict[str, str]:
    with path.open("rb") as source:
        prefix = source.read(MAX_FRONTMATTER_BYTES + 1)
    try:
        lines = prefix.decode("utf-8").splitlines()
    except UnicodeDecodeError as error:
        raise CatalogError(f"required frontmatter is not UTF-8: {path}") from error
    if not lines or lines[0] != "---":
        raise CatalogError(f"missing required frontmatter: {path}")
    closing = next((index for index in range(1, len(lines)) if lines[index] == "---"), None)
    if closing is None:
        raise CatalogError(f"unterminated or oversized required frontmatter: {path}")

    values: dict[str, str] = {}
    index = 1
    while index < closing:
        line = lines[index]
        if line[:1].isspace() or not line.strip() or line.lstrip().startswith("#"):
            index += 1
            continue
        key, separator, raw = line.partition(":")
        if key in {"name", "description"}:
            if not separator:
                raise CatalogError(f"malformed required frontmatter {key}: {path}")
            if key in values:
                raise CatalogError(f"duplicate required frontmatter {key}: {path}")
            block_lines: list[str] = []
            if raw.strip().startswith((">", "|")):
                cursor = index + 1
                while cursor < closing and (
                    not lines[cursor] or lines[cursor][:1].isspace()
                ):
                    block_lines.append(lines[cursor])
                    cursor += 1
                index = cursor - 1
            values[key] = parse_scalar(raw, key=key, path=path, block_lines=block_lines)
        elif re.match(r"^(name|description)\b", line):
            malformed_key = line.split(maxsplit=1)[0]
            raise CatalogError(
                f"malformed required frontmatter {malformed_key}: {path}"
            )
        index += 1
    for key in ("name", "description"):
        if key not in values:
            raise CatalogError(f"missing required frontmatter {key}: {path}")
    return values


def validate_entry_point(path: Path, package_name: str) -> None:
    require_regular_file(path)
    frontmatter = read_required_frontmatter(path)
    declared_name = frontmatter["name"]
    if not SAFE_INSTALL_NAME.fullmatch(declared_name):
        raise CatalogError(f"unsafe install name in required frontmatter: {path}")
    if declared_name != package_name:
        raise CatalogError(
            f"frontmatter name '{declared_name}' must match directory '{package_name}': {path}"
        )


def read_attribution_rows(path: Path, package_names: set[str]) -> dict[str, str]:
    rows: dict[str, str] = {}
    row_pattern = re.compile(r"^\|\s*`([^`]+)`\s*\|[^|]*\|[^|]*\|\s*$")
    for line in path.read_text(encoding="utf-8").splitlines():
        match = row_pattern.match(line)
        if not match:
            continue
        name = match.group(1)
        if name in rows:
            raise CatalogError(
                f"duplicate attribution row for third-party package '{name}': {path}"
            )
        rows[name] = line
    missing = sorted(package_names - rows.keys())
    if missing:
        raise CatalogError(
            f"missing attribution row for third-party package '{missing[0]}': {path}"
        )
    orphaned = sorted(rows.keys() - package_names)
    if orphaned:
        raise CatalogError(
            f"orphan attribution row for unknown package '{orphaned[0]}': {path}"
        )
    return rows


def validate_sources(
    source_root: Path,
) -> tuple[list[Path], list[Path], dict[str, str]]:
    first_party = discover_packages(source_root / "skills", "first-party")
    third_party = discover_packages(source_root / "third-party", "third-party")
    attribution = source_root / "third-party" / "ATTRIBUTION.md"
    require_regular_file(attribution)

    for package in first_party:
        shared = package / "shared"
        if not shared.exists() or not stat.S_ISDIR(shared.lstat().st_mode):
            raise CatalogError(f"missing required catalog source: {shared}")
        validate_entry_point(package / "runtimes" / "codex" / "SKILL.md", package.name)
        validate_source_tree(shared)
        validate_source_tree(package / "runtimes" / "codex")
    for package in third_party:
        validate_entry_point(package / "SKILL.md", package.name)
        validate_source_tree(package)
        if (package / "ATTRIBUTION.md").exists():
            raise CatalogError(
                f"third-party source package reserves generated ATTRIBUTION.md: {package}"
            )

    seen: dict[str, Path] = {}
    for package in (*first_party, *third_party):
        if not SAFE_INSTALL_NAME.fullmatch(package.name):
            raise CatalogError(f"unsafe install name: {package}")
        normalized = package.name.casefold()
        if normalized in seen:
            raise CatalogError(
                f"duplicate install name '{package.name}': {seen[normalized]} and {package}"
            )
        seen[normalized] = package
    rows = read_attribution_rows(attribution, {package.name for package in third_party})
    return first_party, third_party, rows


def validate_source_tree(root: Path) -> None:
    for path in (root, *root.rglob("*")):
        if is_transient(path) or "__pycache__" in path.parts:
            continue
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


ManifestEntry = tuple[str, bytes | None, int]


def catalog_manifest(root: Path) -> dict[str, ManifestEntry]:
    entries: dict[str, ManifestEntry] = {
        ".": ("directory", None, stat.S_IMODE(root.lstat().st_mode))
    }

    def visit(directory: Path) -> None:
        for path in sorted(directory.iterdir(), key=lambda item: item.name):
            relative = path.relative_to(root).as_posix()
            mode = path.lstat().st_mode
            permissions = stat.S_IMODE(mode)
            if stat.S_ISLNK(mode):
                raise CatalogError(f"unsafe output symlink: {relative}")
            if stat.S_ISDIR(mode):
                entries[relative] = ("directory", None, permissions)
                visit(path)
            elif stat.S_ISREG(mode):
                entries[relative] = ("file", path.read_bytes(), permissions)
            else:
                raise CatalogError(f"unsafe output special entry: {relative}")

    visit(root)
    return entries


def check_catalog(staging: Path, output: Path) -> None:
    if not output.exists():
        raise CatalogError(f"catalog check failed: missing output: {output}")
    expected = catalog_manifest(staging)
    actual = catalog_manifest(output)
    diagnostics: list[str] = []
    for relative in sorted(expected.keys() - actual.keys()):
        diagnostics.append(f"missing: {relative}")
    for relative in sorted(actual.keys() - expected.keys()):
        diagnostics.append(f"extra: {relative}")
    for relative in sorted(expected.keys() & actual.keys()):
        expected_kind, expected_content, expected_mode = expected[relative]
        actual_kind, actual_content, actual_mode = actual[relative]
        if expected_kind != actual_kind:
            diagnostics.append(f"stale-type: {relative}")
            continue
        if expected_content != actual_content:
            diagnostics.append(f"stale-content: {relative}")
        if expected_mode != actual_mode:
            diagnostics.append(
                f"stale-mode: {relative} (expected {expected_mode:04o}, found {actual_mode:04o})"
            )
    if diagnostics:
        raise CatalogError("catalog check failed:\n" + "\n".join(diagnostics))


def generate_first_party(source: Path, catalog_root: Path) -> None:
    package = catalog_root / "skills" / source.name
    package.mkdir(parents=True)
    copy_tree(source / "shared", package)
    copy_tree(source / "runtimes" / "codex", package)


def generate_third_party(
    source: Path, provenance_row: str, catalog_root: Path
) -> None:
    package = catalog_root / "skills" / source.name
    copy_tree(source, package)
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
    check: bool = False,
) -> None:
    first_party, third_party, attribution_rows = validate_sources(source_root)
    validate_output(source_root, output)
    if not check:
        output.parent.mkdir(parents=True, exist_ok=True)
    staging_parent = output.parent if output.parent.is_dir() else None
    staging = Path(tempfile.mkdtemp(prefix=f".{output.name}.", dir=staging_parent))
    try:
        for source in first_party:
            generate_first_party(source, staging)
        for source in third_party:
            generate_third_party(source, attribution_rows[source.name], staging)
        normalize_directory_modes(staging)
        if check:
            check_catalog(staging, output)
            shutil.rmtree(staging)
        else:
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
        generate(source_root, output, check=args.check)
    except CatalogError as error:
        print(f"catalog generation failed: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
