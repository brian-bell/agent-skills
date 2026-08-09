#!/usr/bin/env python3
"""Monitor a GitHub pull request's reactions for the fix-pr skill."""

import argparse
import json
import re
import subprocess
import sys
import time
from urllib.parse import urlparse


PR_URL_RE = re.compile(r"^/([^/]+/[^/]+)/pull/(\d+)(?:/.*)?$")


def parse_target(target: str, repo: str | None) -> tuple[str, int]:
    if repo:
        if not re.fullmatch(r"[^/\s]+/[^/\s]+", repo):
            raise ValueError("repo must be in OWNER/REPOSITORY format")
        if not target.isdigit() or int(target) < 1:
            raise ValueError("pull request number must be a positive integer")
        return repo, int(target)

    parsed = urlparse(target)
    match = PR_URL_RE.fullmatch(parsed.path) if parsed.scheme and parsed.netloc else None
    if not match or parsed.netloc.lower() != "github.com":
        raise ValueError("target must be a GitHub pull request URL or use --repo OWNER/REPOSITORY NUMBER")
    return match.group(1), int(match.group(2))


def is_merged(repo: str, number: int) -> bool:
    command = ["gh", "api", f"repos/{repo}/pulls/{number}", "--jq", ".merged"]
    try:
        result = subprocess.run(command, check=True, capture_output=True, text=True)
    except FileNotFoundError as exc:
        raise RuntimeError("gh is required and must be installed") from exc
    except subprocess.CalledProcessError as exc:
        message = exc.stderr.strip() or "GitHub API request failed"
        raise RuntimeError(message) from exc
    return result.stdout.strip() == "true"


def reaction_contents(repo: str, number: int) -> set[str]:
    command = [
        "gh",
        "api",
        f"repos/{repo}/issues/{number}/reactions",
        "--paginate",
        "--jq",
        ".[] | .content",
    ]
    try:
        result = subprocess.run(command, check=True, capture_output=True, text=True)
    except FileNotFoundError as exc:
        raise RuntimeError("gh is required and must be installed") from exc
    except subprocess.CalledProcessError as exc:
        message = exc.stderr.strip() or "GitHub API request failed"
        raise RuntimeError(message) from exc
    return {line.strip() for line in result.stdout.splitlines() if line.strip()}


def status_for(reactions: set[str]) -> str:
    if "+1" in reactions:
        return "ready to merge"
    if "eyes" in reactions:
        return "under review"
    return "needs fix"


def review_comment_summary(repo: str, number: int) -> tuple[list[tuple[str, str]], bool]:
    command = [
        "gh",
        "api",
        f"repos/{repo}/pulls/{number}/comments",
        "--paginate",
        "--slurp",
    ]
    try:
        result = subprocess.run(command, check=True, capture_output=True, text=True)
    except FileNotFoundError as exc:
        raise RuntimeError("gh is required and must be installed") from exc
    except subprocess.CalledProcessError as exc:
        message = exc.stderr.strip() or "GitHub API request failed"
        raise RuntimeError(message) from exc

    pages = json.loads(result.stdout or "[]")
    comments = [comment for page in pages for comment in page]
    has_ai_review = any(
        "codex" in comment.get("user", {}).get("login", "").lower()
        or "claude" in comment.get("user", {}).get("login", "").lower()
        for comment in comments
    )
    links = []
    for comment in comments:
        body = comment.get("body", "")
        match = re.search(r"(?<![A-Za-z0-9])P([01])(?![A-Za-z0-9])", body, re.IGNORECASE)
        if match:
            link = comment.get("html_url")
            severity = f"P{match.group(1)}"
            if link and (severity, link) not in links:
                links.append((severity, link))
    return links, has_ai_review


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Monitor a GitHub pull request's reactions")
    parser.add_argument("target", help="GitHub pull request URL, or PR number with --repo")
    parser.add_argument("--repo", help="Repository in OWNER/REPOSITORY format")
    parser.add_argument("--interval", type=float, default=30.0, help="Seconds between checks (default: 30)")
    return parser


def main() -> int:
    args = build_parser().parse_args()
    if args.interval <= 0:
        print("error: interval must be positive", file=sys.stderr)
        return 2
    try:
        repo, number = parse_target(args.target, args.repo)
        started_at = time.monotonic()
        while True:
            elapsed = int(time.monotonic() - started_at)
            status = "merged" if is_merged(repo, number) else status_for(reaction_contents(repo, number))
            links = []
            if status == "needs fix":
                links, has_ai_review = review_comment_summary(repo, number)
                if not has_ai_review:
                    status = "needs review"
                elif not links:
                    status = "ready to merge"
            print(f"{repo}#{number}: heartbeat {elapsed}s: {status}", flush=True)
            if status != "under review":
                print("Final verdict:", flush=True)
                print(f"Repo: {repo}", flush=True)
                print(f"PR: {number}", flush=True)
                if status == "merged":
                    print("Status: MERGED", flush=True)
                elif status == "ready to merge":
                    print("Status: READY TO MERGE", flush=True)
                elif status == "needs review":
                    print("Status: NEEDS REVIEW", flush=True)
                else:
                    print("Status: NEEDS FIX", flush=True)
                    for severity, link in links:
                        print(f"{severity}: {link}", flush=True)
                return 0
            time.sleep(args.interval)
    except (ValueError, RuntimeError, KeyboardInterrupt) as exc:
        if isinstance(exc, KeyboardInterrupt):
            print("", file=sys.stderr)
            return 130
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
