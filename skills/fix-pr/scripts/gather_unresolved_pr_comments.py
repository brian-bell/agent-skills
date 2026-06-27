#!/usr/bin/env python3
"""Gather unresolved GitHub PR review threads without mutating GitHub state."""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
import textwrap
from typing import Any


THREAD_QUERY = """
query($owner: String!, $name: String!, $number: Int!, $after: String) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $number) {
      number
      title
      url
      headRefName
      baseRefName
      reviewThreads(first: 100, after: $after) {
        pageInfo {
          hasNextPage
          endCursor
        }
        nodes {
          id
          isResolved
          isOutdated
          path
          line
          startLine
          originalLine
          originalStartLine
          diffSide
          startDiffSide
          comments(first: 100) {
            pageInfo {
              hasNextPage
              endCursor
            }
            nodes {
              id
              databaseId
              author {
                login
              }
              body
              bodyText
              createdAt
              updatedAt
              url
              path
              line
              originalLine
              diffHunk
            }
          }
        }
      }
    }
  }
}
""".strip()


THREAD_COMMENTS_QUERY = """
query($threadId: ID!, $after: String) {
  node(id: $threadId) {
    ... on PullRequestReviewThread {
      comments(first: 100, after: $after) {
        pageInfo {
          hasNextPage
          endCursor
        }
        nodes {
          id
          databaseId
          author {
            login
          }
          body
          bodyText
          createdAt
          updatedAt
          url
          path
          line
          originalLine
          diffHunk
        }
      }
    }
  }
}
""".strip()


def run_gh(args: list[str]) -> dict[str, Any]:
    proc = subprocess.run(
        ["gh", *args],
        check=False,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if proc.returncode != 0:
        sys.stderr.write(proc.stderr)
        raise SystemExit(proc.returncode)
    try:
        return json.loads(proc.stdout)
    except json.JSONDecodeError as exc:
        raise SystemExit(f"gh returned invalid JSON for {' '.join(args)}: {exc}") from exc


def parse_repo(repo: str | None) -> tuple[str, str, str]:
    if repo:
        parts = repo.split("/", 1)
        if len(parts) != 2 or not all(parts):
            raise SystemExit("--repo must use owner/name format")
        return parts[0], parts[1], repo

    data = run_gh(["repo", "view", "--json", "owner,name"])
    owner = data.get("owner", {}).get("login")
    name = data.get("name")
    if not owner or not name:
        raise SystemExit("Could not infer repository from gh repo view")
    return owner, name, f"{owner}/{name}"


def read_pr(repo: str, number: int | None) -> dict[str, Any]:
    args = ["pr", "view"]
    if number is not None:
        args.append(str(number))
    args.extend(["--json", "number,title,url,headRefName,baseRefName", "--repo", repo])
    return run_gh(args)


def graphql(query: str, fields: dict[str, Any]) -> dict[str, Any]:
    args = ["api", "graphql", "-f", f"query={query}"]
    for key, value in fields.items():
        if value is None:
            continue
        args.extend(["-F", f"{key}={value}"])
    return run_gh(args)


def normalize_comment(comment: dict[str, Any]) -> dict[str, Any]:
    author = comment.get("author") or {}
    return {
        "id": comment.get("id"),
        "database_id": comment.get("databaseId"),
        "author": author.get("login"),
        "body": comment.get("body") or "",
        "body_text": comment.get("bodyText") or comment.get("body") or "",
        "created_at": comment.get("createdAt"),
        "updated_at": comment.get("updatedAt"),
        "url": comment.get("url"),
        "path": comment.get("path"),
        "line": comment.get("line"),
        "original_line": comment.get("originalLine"),
        "diff_hunk": comment.get("diffHunk") or "",
    }


def fetch_remaining_comments(thread_id: str, first_page: dict[str, Any]) -> list[dict[str, Any]]:
    comments = [normalize_comment(item) for item in first_page.get("nodes", [])]
    page_info = first_page.get("pageInfo") or {}
    cursor = page_info.get("endCursor")

    while page_info.get("hasNextPage"):
        data = graphql(THREAD_COMMENTS_QUERY, {"threadId": thread_id, "after": cursor})
        node = data.get("data", {}).get("node") or {}
        page = node.get("comments") or {}
        comments.extend(normalize_comment(item) for item in page.get("nodes", []))
        page_info = page.get("pageInfo") or {}
        cursor = page_info.get("endCursor")

    return comments


def normalize_thread(thread: dict[str, Any]) -> dict[str, Any]:
    return {
        "id": thread.get("id"),
        "is_resolved": bool(thread.get("isResolved")),
        "is_outdated": bool(thread.get("isOutdated")),
        "path": thread.get("path"),
        "line": thread.get("line"),
        "start_line": thread.get("startLine"),
        "original_line": thread.get("originalLine"),
        "original_start_line": thread.get("originalStartLine"),
        "diff_side": thread.get("diffSide"),
        "start_diff_side": thread.get("startDiffSide"),
        "comments": fetch_remaining_comments(thread.get("id"), thread.get("comments") or {}),
        "decision": "pending",
        "reason": "",
        "action": "",
    }


def gather_report(repo_arg: str | None, pr_arg: int | None) -> dict[str, Any]:
    owner, name, repo = parse_repo(repo_arg)
    pr = read_pr(repo, pr_arg)
    number = int(pr["number"])
    unresolved_threads: list[dict[str, Any]] = []
    cursor = None

    while True:
        data = graphql(
            THREAD_QUERY,
            {"owner": owner, "name": name, "number": number, "after": cursor},
        )
        pull_request = data.get("data", {}).get("repository", {}).get("pullRequest")
        if not pull_request:
            raise SystemExit(f"Could not read pull request {repo}#{number}")

        review_threads = pull_request.get("reviewThreads") or {}
        for thread in review_threads.get("nodes", []):
            if not thread.get("isResolved"):
                unresolved_threads.append(normalize_thread(thread))

        page_info = review_threads.get("pageInfo") or {}
        if not page_info.get("hasNextPage"):
            break
        cursor = page_info.get("endCursor")

    return {
        "repo": repo,
        "pull_request": {
            "number": number,
            "title": pr.get("title"),
            "url": pr.get("url"),
            "head_ref": pr.get("headRefName"),
            "base_ref": pr.get("baseRefName"),
        },
        "unresolved_threads": unresolved_threads,
    }


def markdown_quote(text: str) -> str:
    stripped = text.strip()
    if not stripped:
        return "> (empty comment)"
    return "\n".join(f"> {line}" if line else ">" for line in stripped.splitlines())


def render_markdown(report: dict[str, Any]) -> str:
    pr = report["pull_request"]
    lines = [
        f"# Unresolved PR Comments: {report['repo']}#{pr['number']}",
        "",
        f"- PR: [{pr['title']}]({pr['url']})",
        f"- Base: `{pr['base_ref']}`",
        f"- Head: `{pr['head_ref']}`",
        f"- Unresolved threads: {len(report['unresolved_threads'])}",
        "",
    ]

    if not report["unresolved_threads"]:
        lines.append("No unresolved review threads found.")
        return "\n".join(lines).rstrip() + "\n"

    for index, thread in enumerate(report["unresolved_threads"], start=1):
        location = thread.get("path") or "(unknown path)"
        line = thread.get("line") or thread.get("original_line")
        if line:
            location = f"{location}:{line}"
        comments = thread.get("comments") or []
        last_comment = comments[-1] if comments else {}

        lines.extend(
            [
                f"## {index}. {location}",
                "",
                f"- Thread ID: `{thread['id']}`",
                f"- Outdated: `{str(thread['is_outdated']).lower()}`",
                f"- URL: {last_comment.get('url') or '(no comment URL)'}",
                "- Decision: pending",
                "- Reason:",
                "- Action:",
                "",
            ]
        )

        for comment_index, comment in enumerate(comments, start=1):
            author = comment.get("author") or "unknown"
            created = comment.get("created_at") or "unknown time"
            lines.extend(
                [
                    f"### Comment {comment_index} by @{author} at {created}",
                    "",
                    markdown_quote(comment.get("body_text") or comment.get("body") or ""),
                    "",
                ]
            )

            diff_hunk = comment.get("diff_hunk")
            if diff_hunk:
                lines.extend(["```diff", textwrap.dedent(diff_hunk).strip(), "```", ""])

    return "\n".join(lines).rstrip() + "\n"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Gather unresolved GitHub PR review threads without replying or resolving comments."
    )
    parser.add_argument("--repo", help="Repository in owner/name format. Defaults to current repo.")
    parser.add_argument("--pr", type=int, help="Pull request number. Defaults to current branch PR.")
    parser.add_argument(
        "--format",
        choices=("markdown", "json"),
        default="markdown",
        help="Output format. Defaults to markdown.",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    report = gather_report(args.repo, args.pr)
    if args.format == "json":
        print(json.dumps(report, indent=2, sort_keys=True))
    else:
        print(render_markdown(report), end="")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
