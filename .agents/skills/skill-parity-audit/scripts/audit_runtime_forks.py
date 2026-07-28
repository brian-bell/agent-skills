#!/usr/bin/env python3
"""Audit first-party Claude and Codex runtime forks for parity."""

from __future__ import annotations

import argparse
import ast
import hashlib
import json
from pathlib import Path
from typing import Any


RUNTIMES = ("claude", "codex")


def frontmatter(text: str) -> dict[str, str]:
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        return {}

    values: dict[str, str] = {}
    for line in lines[1:]:
        if line.strip() == "---":
            break
        key, separator, raw_value = line.partition(":")
        if not separator:
            continue
        value = raw_value.strip()
        if value[:1] in {'"', "'"}:
            try:
                value = str(ast.literal_eval(value))
            except (SyntaxError, ValueError):
                pass
        values[key.strip()] = value
    return values


def file_hashes(root: Path) -> dict[str, str]:
    files: dict[str, str] = {}
    for path in sorted(root.rglob("*")):
        if (
            not path.is_file()
            or path.name == ".DS_Store"
            or "__pycache__" in path.parts
        ):
            continue
        files[path.relative_to(root).as_posix()] = hashlib.sha256(
            path.read_bytes()
        ).hexdigest()
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
            runtime: file_hashes(skill_dir / "runtimes" / runtime)
            for runtime in RUNTIMES
        }
        claude_paths = set(overlay_files["claude"])
        codex_paths = set(overlay_files["codex"])
        changed_files = sorted(
            path
            for path in claude_paths & codex_paths
            if overlay_files["claude"][path] != overlay_files["codex"][path]
        )
        claude_only_files = sorted(claude_paths - codex_paths)
        codex_only_files = sorted(codex_paths - claude_paths)
        review_required = bool(
            metadata_differences
            or changed_files
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
            "| Skill | Trigger metadata | Changed in both | Claude-only | Codex-only |",
            "| --- | --- | --- | --- | --- |",
        ]
        for name, detail in reviews.items():
            lines.append(
                f"| `{name}` | {file_list(detail['metadata_differences'])} | "
                f"{file_list(detail['changed_files'])} | "
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
