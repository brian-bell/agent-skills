#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILL_SCRIPT="$REPO_DIR/skills/fix-pr/scripts/gather_unresolved_pr_comments.py"
SKILL_DOC="$REPO_DIR/skills/fix-pr/SKILL.md"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

cat >"$tmp_dir/gh" <<'EOF'
#!/bin/bash
set -euo pipefail

printf '%s\n' "$*" >> "$GH_CALL_LOG"

case "$1 $2" in
  "repo view")
    cat <<'JSON'
{"owner":{"login":"octo"},"name":"demo"}
JSON
    ;;
  "pr view")
    cat <<'JSON'
{"number":42,"title":"Improve parser","url":"https://github.com/octo/demo/pull/42","headRefName":"feature","baseRefName":"main"}
JSON
    ;;
  "api graphql")
    if printf '%s\n' "$*" | grep -qi 'mutation'; then
      echo "unexpected mutation" >&2
      exit 9
    fi
    if printf '%s\n' "$*" | grep -q 'after=THREAD_CURSOR'; then
      cat <<'JSON'
{
  "data": {
    "repository": {
      "pullRequest": {
        "number": 42,
        "title": "Improve parser",
        "url": "https://github.com/octo/demo/pull/42",
        "headRefName": "feature",
        "baseRefName": "main",
        "reviewThreads": {
          "pageInfo": {"hasNextPage": false, "endCursor": null},
          "nodes": [
            {
              "id": "THREAD_unresolved_second_page",
              "isResolved": false,
              "isOutdated": true,
              "path": "lexer.go",
              "line": 31,
              "startLine": null,
              "originalLine": 31,
              "originalStartLine": null,
              "diffSide": "RIGHT",
              "startDiffSide": null,
              "comments": {
                "pageInfo": {"hasNextPage": false, "endCursor": null},
                "nodes": [
                  {
                    "id": "COMMENT_3",
                    "databaseId": 1003,
                    "author": {"login": "second-reviewer"},
                    "body": "This paginated thread should be included.",
                    "bodyText": "This paginated thread should be included.",
                    "createdAt": "2026-06-01T13:00:00Z",
                    "updatedAt": "2026-06-01T13:00:00Z",
                    "url": "https://github.com/octo/demo/pull/42#discussion_r1003",
                    "path": "lexer.go",
                    "line": 31,
                    "originalLine": 31,
                    "diffHunk": "@@ -28,6 +28,8 @@"
                  }
                ]
              }
            }
          ]
        }
      }
    }
  }
}
JSON
      exit 0
    fi
    cat <<'JSON'
{
  "data": {
    "repository": {
      "pullRequest": {
        "number": 42,
        "title": "Improve parser",
        "url": "https://github.com/octo/demo/pull/42",
        "headRefName": "feature",
        "baseRefName": "main",
        "reviewThreads": {
          "pageInfo": {"hasNextPage": true, "endCursor": "THREAD_CURSOR"},
          "nodes": [
            {
              "id": "THREAD_unresolved",
              "isResolved": false,
              "isOutdated": false,
              "path": "parser.go",
              "line": 17,
              "startLine": null,
              "originalLine": 17,
              "originalStartLine": null,
              "diffSide": "RIGHT",
              "startDiffSide": null,
              "comments": {
                "pageInfo": {"hasNextPage": false, "endCursor": null},
                "nodes": [
                  {
                    "id": "COMMENT_1",
                    "databaseId": 1001,
                    "author": {"login": "reviewer"},
                    "body": "Needs a bounds check before indexing.",
                    "bodyText": "Needs a bounds check before indexing.",
                    "createdAt": "2026-06-01T12:00:00Z",
                    "updatedAt": "2026-06-01T12:00:00Z",
                    "url": "https://github.com/octo/demo/pull/42#discussion_r1001",
                    "path": "parser.go",
                    "line": 17,
                    "originalLine": 17,
                    "diffHunk": "@@ -14,6 +14,8 @@"
                  }
                ]
              }
            },
            {
              "id": "THREAD_resolved",
              "isResolved": true,
              "isOutdated": false,
              "path": "parser.go",
              "line": 9,
              "startLine": null,
              "originalLine": 9,
              "originalStartLine": null,
              "diffSide": "RIGHT",
              "startDiffSide": null,
              "comments": {
                "pageInfo": {"hasNextPage": false, "endCursor": null},
                "nodes": [
                  {
                    "id": "COMMENT_2",
                    "databaseId": 1002,
                    "author": {"login": "reviewer"},
                    "body": "Resolved comment that should not appear.",
                    "bodyText": "Resolved comment that should not appear.",
                    "createdAt": "2026-06-01T12:30:00Z",
                    "updatedAt": "2026-06-01T12:30:00Z",
                    "url": "https://github.com/octo/demo/pull/42#discussion_r1002",
                    "path": "parser.go",
                    "line": 9,
                    "originalLine": 9,
                    "diffHunk": "@@ -8,6 +8,7 @@"
                  }
                ]
              }
            }
          ]
        }
      }
    }
  }
}
JSON
    ;;
  *)
    echo "unexpected gh call: $*" >&2
    exit 2
    ;;
esac
EOF
chmod +x "$tmp_dir/gh"

GH_CALL_LOG="$tmp_dir/gh-calls.log" \
  PATH="$tmp_dir:$PATH" \
  python3 "$SKILL_SCRIPT" --format json >"$tmp_dir/report.json"

python3 - "$tmp_dir/report.json" "$tmp_dir/gh-calls.log" <<'PY'
import json
import sys
from pathlib import Path

report = json.loads(Path(sys.argv[1]).read_text())
calls = Path(sys.argv[2]).read_text().splitlines()

assert report["repo"] == "octo/demo", report
assert report["pull_request"]["number"] == 42, report
threads = report["unresolved_threads"]
assert len(threads) == 2, threads
assert threads[0]["id"] == "THREAD_unresolved", threads
assert threads[0]["comments"][0]["body_text"] == "Needs a bounds check before indexing.", threads
assert threads[1]["id"] == "THREAD_unresolved_second_page", threads
assert threads[1]["comments"][0]["body_text"] == "This paginated thread should be included.", threads
assert "Resolved comment that should not appear." not in json.dumps(report), report
assert not any("mutation" in call.lower() for call in calls), calls
PY

GH_CALL_LOG="$tmp_dir/gh-calls-markdown.log" \
  PATH="$tmp_dir:$PATH" \
  python3 "$SKILL_SCRIPT" --repo octo/demo --pr 42 --format markdown >"$tmp_dir/report.md"

grep -q "Unresolved PR Comments: octo/demo#42" "$tmp_dir/report.md" || fail "missing markdown heading"
grep -q "| Decision | Location | Reviewer | Finding | Evidence | Action | URL |" "$tmp_dir/report.md" || fail "missing markdown table header"
grep -q "| pending | parser.go:17 | @reviewer | Needs a bounds check before indexing. |  |  | https://github.com/octo/demo/pull/42#discussion_r1001 |" "$tmp_dir/report.md" || fail "missing first table row"
grep -q "| pending | lexer.go:31 | @second-reviewer | This paginated thread should be included. |  |  | https://github.com/octo/demo/pull/42#discussion_r1003 |" "$tmp_dir/report.md" || fail "missing paginated table row"
if grep -q "^## 1\\. " "$tmp_dir/report.md"; then
  fail "markdown output should be table-first, not per-thread sections"
fi
grep -q "Needs a bounds check before indexing." "$tmp_dir/report.md" || fail "missing comment body"
grep -q "This paginated thread should be included." "$tmp_dir/report.md" || fail "missing paginated comment body"
grep -q '\$autofix --comment <thread URL>' "$SKILL_DOC" || fail "fix-pr skill should hand accepted findings to autofix"
grep -q 'Do not implement accepted findings directly in `fix-pr`' "$SKILL_DOC" || fail "fix-pr skill should forbid direct implementation of accepted findings"

echo "PASS: fix-pr"
