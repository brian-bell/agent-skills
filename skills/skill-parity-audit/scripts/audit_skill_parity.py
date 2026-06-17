#!/usr/bin/env python3
"""Compare two AI skill directories for parity."""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
import re
from pathlib import Path
from typing import Any


PLATFORM_PATTERNS = {
    "Agent tool": re.compile(r"\b(?:Use|using|Invoke|invokes?) the Agent tool\b|\bAgent tool\b"),
    "subagent_type": re.compile(r"\bsubagent_type\b"),
    "TaskCreate": re.compile(r"\bTaskCreate\b"),
    "TaskUpdate": re.compile(r"\bTaskUpdate\b"),
    "TaskList": re.compile(r"\bTaskList\b"),
    "TeamCreate": re.compile(r"\bTeamCreate\b"),
    "SendMessage": re.compile(r"\bSendMessage\b"),
    "WebSearch": re.compile(r"\bWebSearch\b"),
    "WebFetch": re.compile(r"\bWebFetch\b"),
}


def sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def read_text(path: Path) -> str:
    try:
        return path.read_text(encoding="utf-8")
    except UnicodeDecodeError:
        return path.read_text(encoding="utf-8", errors="replace")


def skill_name_from_frontmatter(skill_md: Path) -> str | None:
    text = read_text(skill_md)
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        return None
    for line in lines[1:]:
        if line.strip() == "---":
            return None
        if line.startswith("name:"):
            return line.split(":", 1)[1].strip().strip("\"'")
    return None


def list_files(root: Path) -> dict[str, str]:
    files: dict[str, str] = {}
    for dirpath, dirnames, filenames in os.walk(root, followlinks=True):
        dirnames[:] = [d for d in dirnames if d not in {".git", "__pycache__"}]
        for filename in filenames:
            if filename in {".DS_Store"}:
                continue
            path = Path(dirpath) / filename
            rel = path.relative_to(root).as_posix()
            if path.is_file():
                files[rel] = sha256(path)
    return files


def platform_tokens(root: Path) -> list[str]:
    found: set[str] = set()
    for rel in list_files(root):
        path = root / rel
        if path.suffix.lower() not in {".md", ".txt", ".yaml", ".yml"}:
            continue
        text = read_text(path)
        for name, pattern in PLATFORM_PATTERNS.items():
            if pattern.search(text):
                found.add(name)
    return sorted(found)


def inspect_root(root: Path) -> dict[str, Any]:
    root = root.expanduser().resolve()
    skills: dict[str, Any] = {}
    non_skills: dict[str, Any] = {}
    if not root.exists():
        return {"root": str(root), "exists": False, "skills": {}, "non_skills": {}}

    for entry in sorted(root.iterdir(), key=lambda p: p.name):
        if entry.name.startswith("."):
            continue
        skill_md = entry / "SKILL.md"
        if skill_md.exists():
            declared = skill_name_from_frontmatter(skill_md)
            files = list_files(entry)
            skills[entry.name] = {
                "path": str(entry),
                "is_symlink": entry.is_symlink(),
                "target": os.readlink(entry) if entry.is_symlink() else None,
                "declared_name": declared,
                "files": files,
                "tree_hash": hashlib.sha256(
                    "\n".join(f"{k}:{v}" for k, v in sorted(files.items())).encode("utf-8")
                ).hexdigest(),
                "platform_tokens": platform_tokens(entry),
            }
        else:
            non_skills[entry.name] = {
                "path": str(entry),
                "is_symlink": entry.is_symlink(),
                "target": os.readlink(entry) if entry.is_symlink() else None,
                "exists": entry.exists(),
                "reason": "missing SKILL.md",
            }

    return {"root": str(root), "exists": True, "skills": skills, "non_skills": non_skills}


def compare(left: dict[str, Any], right: dict[str, Any]) -> dict[str, Any]:
    left_skills = left["skills"]
    right_skills = right["skills"]
    shared = sorted(set(left_skills) & set(right_skills))
    left_only = sorted(set(left_skills) - set(right_skills))
    right_only = sorted(set(right_skills) - set(left_skills))
    identical: list[str] = []
    drifted: dict[str, Any] = {}

    for name in shared:
        l_files = left_skills[name]["files"]
        r_files = right_skills[name]["files"]
        if l_files == r_files:
            identical.append(name)
            continue
        left_files = set(l_files)
        right_files = set(r_files)
        changed = sorted(
            rel for rel in (left_files & right_files) if l_files[rel] != r_files[rel]
        )
        drifted[name] = {
            "left_only_files": sorted(left_files - right_files),
            "right_only_files": sorted(right_files - left_files),
            "changed_files": changed,
        }

    return {
        "left_count": len(left_skills),
        "right_count": len(right_skills),
        "shared_count": len(shared),
        "identical": identical,
        "drifted": drifted,
        "left_only": left_only,
        "right_only": right_only,
    }


def table(rows: list[list[str]]) -> str:
    if not rows:
        return ""
    header = rows[0]
    out = ["| " + " | ".join(header) + " |"]
    out.append("| " + " | ".join(["---"] * len(header)) + " |")
    for row in rows[1:]:
        out.append("| " + " | ".join(row) + " |")
    return "\n".join(out)


def markdown_report(data: dict[str, Any], left_label: str, right_label: str) -> str:
    cmp = data["comparison"]
    left = data["left"]
    right = data["right"]
    lines = [
        "# Skill Parity Audit",
        "",
        f"Generated: {data['generated_at']}",
        "",
        f"- {left_label}: `{left['root']}`",
        f"- {right_label}: `{right['root']}`",
        "",
        "## Summary",
        "",
        f"- {left_label} usable skills: {cmp['left_count']}",
        f"- {right_label} usable skills: {cmp['right_count']}",
        f"- Shared skills: {cmp['shared_count']}",
        f"- Identical shared skills: {len(cmp['identical'])}",
        f"- Drifted shared skills: {len(cmp['drifted'])}",
        f"- {left_label}-only skills: {len(cmp['left_only'])}",
        f"- {right_label}-only skills: {len(cmp['right_only'])}",
        "",
    ]

    if cmp["left_only"]:
        lines += [f"## Missing From {right_label}", "", "\n".join(f"- `{s}`" for s in cmp["left_only"]), ""]
    if cmp["right_only"]:
        lines += [f"## Missing From {left_label}", "", "\n".join(f"- `{s}`" for s in cmp["right_only"]), ""]
    if cmp["identical"]:
        lines += ["## Identical Shared Skills", "", "\n".join(f"- `{s}`" for s in cmp["identical"]), ""]
    if cmp["drifted"]:
        rows = [["Skill", f"{left_label}-only files", f"{right_label}-only files", "Changed files"]]
        for name, detail in cmp["drifted"].items():
            rows.append([
                f"`{name}`",
                ", ".join(f"`{x}`" for x in detail["left_only_files"]) or "-",
                ", ".join(f"`{x}`" for x in detail["right_only_files"]) or "-",
                ", ".join(f"`{x}`" for x in detail["changed_files"]) or "-",
            ])
        lines += ["## Drifted Shared Skills", "", table(rows), ""]

    for label, root in [(left_label, left), (right_label, right)]:
        if root["non_skills"]:
            rows = [["Entry", "Reason", "Symlink Target"]]
            for name, detail in root["non_skills"].items():
                rows.append([
                    f"`{name}`",
                    detail["reason"],
                    f"`{detail['target']}`" if detail["target"] else "-",
                ])
            lines += [f"## Non-Usable Entries In {label}", "", table(rows), ""]

    token_rows = [["Skill", "Root", "Platform-specific tokens"]]
    for label, root in [(left_label, left), (right_label, right)]:
        for name, detail in sorted(root["skills"].items()):
            tokens = detail.get("platform_tokens") or []
            if tokens:
                token_rows.append([f"`{name}`", label, ", ".join(f"`{t}`" for t in tokens)])
    if len(token_rows) > 1:
        lines += ["## Platform-Specific Token Hints", "", table(token_rows), ""]

    return "\n".join(lines).rstrip() + "\n"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("left_root", nargs="?", default="~/.claude/skills")
    parser.add_argument("right_root", nargs="?", default="~/.agents/skills")
    parser.add_argument("--left-label", default="Left")
    parser.add_argument("--right-label", default="Right")
    parser.add_argument("--markdown-out")
    parser.add_argument("--json-out")
    args = parser.parse_args()

    left = inspect_root(Path(args.left_root))
    right = inspect_root(Path(args.right_root))
    data = {
        "generated_at": dt.datetime.now().astimezone().isoformat(timespec="seconds"),
        "left": left,
        "right": right,
        "comparison": compare(left, right),
    }

    markdown = markdown_report(data, args.left_label, args.right_label)
    if args.markdown_out:
        Path(args.markdown_out).expanduser().write_text(markdown, encoding="utf-8")
    else:
        print(markdown)

    if args.json_out:
        Path(args.json_out).expanduser().write_text(
            json.dumps(data, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
