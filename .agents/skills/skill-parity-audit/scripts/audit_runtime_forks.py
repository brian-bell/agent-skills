#!/usr/bin/env python3
"""Audit first-party Claude and Codex runtime forks for parity."""

from __future__ import annotations

import argparse
import ast
import hashlib
import json
import os
from pathlib import Path
from typing import Any


RUNTIMES = ("claude", "codex")


def block_scalar_header(value: str) -> tuple[str, int | None] | None:
    if not value or value[0] not in {"|", ">"}:
        return None

    indent = None
    chomp = None
    for indicator in value[1:]:
        if indicator in "123456789" and indent is None:
            indent = int(indicator)
        elif indicator in {"+", "-"} and chomp is None:
            chomp = indicator
        else:
            return None
    return value[0], indent


def fold_block_lines(lines: list[str]) -> str:
    parts = []
    previous_more_indented = False
    pending_blank_lines = 0
    saw_content = False

    for line in lines:
        if not line:
            pending_blank_lines += 1
            continue

        more_indented = line[0].isspace()
        if saw_content:
            if pending_blank_lines:
                line_breaks = pending_blank_lines
                if previous_more_indented or more_indented:
                    line_breaks += 1
                parts.append("\n" * line_breaks)
            elif previous_more_indented or more_indented:
                parts.append("\n")
            else:
                parts.append(" ")
        elif pending_blank_lines:
            parts.append("\n" * pending_blank_lines)

        parts.append(line)
        previous_more_indented = more_indented
        pending_blank_lines = 0
        saw_content = True

    return "".join(parts)


def frontmatter(text: str) -> dict[str, str]:
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        return {}

    values: dict[str, str] = {}
    index = 1
    while index < len(lines):
        line = lines[index]
        if line.strip() == "---":
            break
        key, separator, raw_value = line.partition(":")
        if not separator:
            index += 1
            continue
        value = raw_value.strip()
        block_header = block_scalar_header(value)
        if block_header is not None:
            style, explicit_indent = block_header
            block_lines = []
            index += 1
            while index < len(lines):
                block_line = lines[index]
                if block_line == "---":
                    break
                if block_line and not block_line[0].isspace():
                    break
                block_lines.append(block_line)
                index += 1

            indents = [
                len(block_line) - len(block_line.lstrip())
                for block_line in block_lines
                if block_line.strip()
            ]
            indent = explicit_indent or min(indents, default=0)
            normalized = [
                block_line[indent:] if block_line.strip() else ""
                for block_line in block_lines
            ]
            if style == ">":
                value = fold_block_lines(normalized)
            else:
                value = "\n".join(normalized)
            value = value.rstrip("\n")
            values[key.strip()] = value
            continue
        if value[:1] in {'"', "'"}:
            try:
                value = str(ast.literal_eval(value))
            except (SyntaxError, ValueError):
                pass
        values[key.strip()] = value
        index += 1
    return values


def file_entries(root: Path) -> dict[str, dict[str, str | bool]]:
    files: dict[str, dict[str, str | bool]] = {}
    for path in sorted(root.rglob("*")):
        if path.name == ".DS_Store" or "__pycache__" in path.parts:
            continue
        relative_path = path.relative_to(root).as_posix()
        if path.is_symlink():
            files[relative_path] = {
                "kind": "symlink",
                "target": os.readlink(path),
            }
        elif path.is_file():
            files[relative_path] = {
                "kind": "file",
                "sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
                "executable": bool(path.stat().st_mode & 0o111),
            }
    return files


def audit_repo(repo_root: Path) -> dict[str, Any]:
    skills: dict[str, Any] = {}
    skills_root = repo_root.resolve() / "skills"

    for skill_dir in sorted(skills_root.iterdir()):
        if not skill_dir.is_dir():
            continue
        errors = []
        if not (skill_dir / "shared").is_dir():
            errors.append("missing shared/")
        errors.extend(
            [
                f"missing runtimes/{runtime}/SKILL.md"
                for runtime in RUNTIMES
                if not (skill_dir / "runtimes" / runtime / "SKILL.md").is_file()
            ]
        )
        if any(error.startswith("missing runtimes/") for error in errors):
            skills[skill_dir.name] = {
                "status": "error",
                "errors": errors,
                "review_required": False,
            }
            continue

        runtime_files = {}
        for runtime in RUNTIMES:
            skill_md = skill_dir / "runtimes" / runtime / "SKILL.md"
            runtime_files[runtime] = skill_md.read_text(encoding="utf-8")
        metadata = {
            runtime: frontmatter(runtime_files[runtime]) for runtime in RUNTIMES
        }
        for runtime in RUNTIMES:
            declared_name = metadata[runtime].get("name")
            if declared_name != skill_dir.name:
                errors.append(
                    f"runtimes/{runtime}/SKILL.md declares name "
                    f"{declared_name or '<missing>'}, expected {skill_dir.name}"
                )
            if not metadata[runtime].get("description"):
                errors.append(
                    f"runtimes/{runtime}/SKILL.md is missing description"
                )
        metadata_differences = []
        descriptions = [
            metadata[runtime].get("description") for runtime in RUNTIMES
        ]
        if all(descriptions) and descriptions[0] != descriptions[1]:
            metadata_differences.append("runtime descriptions differ")

        overlay_files = {
            runtime: file_entries(skill_dir / "runtimes" / runtime)
            for runtime in RUNTIMES
        }
        claude_paths = set(overlay_files["claude"])
        codex_paths = set(overlay_files["codex"])
        changed_files = sorted(
            path
            for path in claude_paths & codex_paths
            if overlay_files["claude"][path] != overlay_files["codex"][path]
        )
        shared_source_candidates = sorted(
            path
            for path in claude_paths & codex_paths
            if path != "SKILL.md"
            and overlay_files["claude"][path] == overlay_files["codex"][path]
        )
        claude_only_files = sorted(claude_paths - codex_paths)
        codex_only_files = sorted(codex_paths - claude_paths)
        review_required = bool(
            metadata_differences
            or changed_files
            or shared_source_candidates
            or claude_only_files
            or codex_only_files
        )
        skills[skill_dir.name] = {
            "status": (
                "error" if errors else ("review" if review_required else "pass")
            ),
            "errors": errors,
            "metadata": metadata,
            "metadata_differences": metadata_differences,
            "review_required": review_required,
            "changed_files": changed_files,
            "shared_source_candidates": shared_source_candidates,
            "claude_only_files": claude_only_files,
            "codex_only_files": codex_only_files,
        }

    error_count = sum(detail["status"] == "error" for detail in skills.values())
    review_count = sum(detail["status"] == "review" for detail in skills.values())
    pass_count = sum(detail["status"] == "pass" for detail in skills.values())
    return {
        "repo_root": str(repo_root.resolve()),
        "skills": skills,
        "summary": {
            "skill_count": len(skills),
            "error_count": error_count,
            "review_count": review_count,
            "pass_count": pass_count,
        },
    }


def file_list(paths: list[str]) -> str:
    return ", ".join(f"`{path}`" for path in paths) or "-"


def markdown_report(data: dict[str, Any]) -> str:
    summary = data["summary"]
    lines = [
        "# Runtime Fork Parity Audit",
        "",
        f"- First-party skills: {summary['skill_count']}",
        f"- Blocking parity errors: {summary['error_count']}",
        f"- Skills requiring semantic review: {summary['review_count']}",
        f"- Skills passing without review: {summary['pass_count']}",
        "",
    ]

    errors = {
        name: detail
        for name, detail in data["skills"].items()
        if detail["status"] == "error"
    }
    if errors:
        lines += ["## Blocking Parity Errors", ""]
        for name, detail in errors.items():
            lines += [
                f"### `{name}`",
                "",
                *[f"- {error}" for error in detail["errors"]],
                "",
            ]

    reviews = {
        name: detail
        for name, detail in data["skills"].items()
        if detail["status"] == "review"
    }
    if reviews:
        lines += [
            "## Semantic Review Queue",
            "",
            "| Skill | Trigger metadata | Changed in both | Shared candidates | Claude-only | Codex-only |",
            "| --- | --- | --- | --- | --- | --- |",
        ]
        for name, detail in reviews.items():
            lines.append(
                f"| `{name}` | {file_list(detail['metadata_differences'])} | "
                f"{file_list(detail['changed_files'])} | "
                f"{file_list(detail['shared_source_candidates'])} | "
                f"{file_list(detail['claude_only_files'])} | "
                f"{file_list(detail['codex_only_files'])} |"
            )
        lines.append("")

    return "\n".join(lines).rstrip() + "\n"


def main() -> int:
    default_root = Path(__file__).resolve().parents[4]
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("repo_root", nargs="?", default=str(default_root))
    parser.add_argument("--json-out")
    parser.add_argument("--markdown-out")
    args = parser.parse_args()

    data = audit_repo(Path(args.repo_root))
    report = markdown_report(data)

    if args.json_out:
        Path(args.json_out).write_text(
            json.dumps(data, indent=2, sort_keys=True) + "\n", encoding="utf-8"
        )
    if args.markdown_out:
        Path(args.markdown_out).write_text(report, encoding="utf-8")
    else:
        print(report, end="")

    return 1 if data["summary"]["error_count"] else 0


if __name__ == "__main__":
    raise SystemExit(main())
