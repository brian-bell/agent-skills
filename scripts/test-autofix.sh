#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILL_SCRIPT="$REPO_DIR/skills/autofix/shared/scripts/gather_unresolved_pr_comments.py"
CODEX_SKILL="$REPO_DIR/skills/autofix/runtimes/codex/SKILL.md"
CLAUDE_SKILL="$REPO_DIR/skills/autofix/runtimes/claude/SKILL.md"
CODEX_METADATA="$REPO_DIR/skills/autofix/runtimes/codex/agents/openai.yaml"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

grep -Eq '^\| P2 \|.*\| yes \|$' "$CODEX_SKILL" \
  || fail "Codex autofix severity matrix should auto-fix P2 findings"
grep -Fq 'Fix every `accepted` P0, P1, and P2 finding' "$CODEX_SKILL" \
  || fail "Codex autofix workflow should implement accepted P2 findings"
grep -Fq 'Auto-fix P0, P1, and P2 findings by default.' "$CLAUDE_SKILL" \
  || fail "Claude autofix workflow should implement accepted P2 findings"
grep -Fq 'auto-fix P0, P1, and P2 findings' "$CODEX_METADATA" \
  || fail "Codex autofix launcher should advertise P2 findings"
grep -Fq 'auto-fix P0, P1, and P2 unresolved feedback' "$REPO_DIR/README.md" \
  || fail "README should advertise P2 autofixes"
if grep -En 'P0/P1|P0 or P1|P0 and P1|P2/P3|more severe than P2' \
  "$CODEX_SKILL" "$CLAUDE_SKILL" "$CODEX_METADATA" "$REPO_DIR/README.md"; then
  fail "autofix instructions still advertise the old P0/P1 severity threshold"
fi

cat >"$tmp_dir/gh" <<'EOF'
#!/bin/bash
set -euo pipefail

printf '%s\n' "$*" >>"$GH_CALL_LOG"

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
                  },
                  {
                    "id": "COMMENT_1_REPLY",
                    "databaseId": 2001,
                    "author": {"login": "pr-author"},
                    "body": "I can handle this in a follow-up.",
                    "bodyText": "I can handle this in a follow-up.",
                    "createdAt": "2026-06-01T12:05:00Z",
                    "updatedAt": "2026-06-01T12:05:00Z",
                    "url": "https://github.com/octo/demo/pull/42#discussion_r2001",
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

staged_home="$tmp_dir/home"
SKILL_INSTALL_TARGETS=agents HOME="$staged_home" "$REPO_DIR/install.sh" --all \
  >"$tmp_dir/install-stdout" 2>"$tmp_dir/install-stderr"
staged_script="$staged_home/.skill-symlinks/runtimes/codex/skills/autofix/scripts/gather_unresolved_pr_comments.py"
[ -f "$staged_script" ] || fail "missing staged autofix collector: $staged_script"

GH_CALL_LOG="$tmp_dir/staged-gh-calls.log" \
  PATH="$tmp_dir:$PATH" \
  python3 "$staged_script" --repo octo/demo --pr 42 --format markdown >"$tmp_dir/staged-report.md"
grep -q "Unresolved PR Comments: octo/demo#42" "$tmp_dir/staged-report.md" \
  || fail "staged collector missing markdown heading"
grep -q "This paginated thread should be included." "$tmp_dir/staged-report.md" \
  || fail "staged collector missing paginated comment"

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
assert threads[0]["comments"][1]["body_text"] == "I can handle this in a follow-up.", threads
assert threads[1]["id"] == "THREAD_unresolved_second_page", threads
assert threads[1]["comments"][0]["body_text"] == "This paginated thread should be included.", threads
assert "Resolved comment that should not appear." not in json.dumps(report), report
assert not any("mutation" in call.lower() for call in calls), calls
PY

GH_CALL_LOG="$tmp_dir/gh-calls-markdown.log" \
  PATH="$tmp_dir:$PATH" \
  python3 "$SKILL_SCRIPT" --repo octo/demo --pr 42 --format markdown >"$tmp_dir/report.md"

grep -q "Unresolved PR Comments: octo/demo#42" "$tmp_dir/report.md" || fail "missing markdown heading"
grep -q "| Decision | Location | Reviewer | Finding | Evidence | Action | URL |" "$tmp_dir/report.md" \
  || fail "missing markdown table header"
grep -q "| pending | parser.go:17 | @reviewer | Needs a bounds check before indexing. |  |  | https://github.com/octo/demo/pull/42#discussion_r1001 |" "$tmp_dir/report.md" \
  || fail "missing first table row"
grep -q "| pending | lexer.go:31 | @second-reviewer | This paginated thread should be included. |  |  | https://github.com/octo/demo/pull/42#discussion_r1003 |" "$tmp_dir/report.md" \
  || fail "missing paginated table row"
if grep -q "I can handle this in a follow-up." "$tmp_dir/report.md"; then
  fail "markdown output should summarize the root review comment, not the latest reply"
fi
if grep -q "^## 1\\. " "$tmp_dir/report.md"; then
  fail "markdown output should be table-first, not per-thread sections"
fi

for runtime in claude codex; do
  skill_file="$REPO_DIR/skills/autofix/runtimes/$runtime/SKILL.md"
  grep -q "numbered, vertically stacked decision list" "$skill_file" \
    || fail "$runtime autofix must require a narrow-screen decision list"
  grep -q "## Auto-fix queue" "$skill_file" \
    || fail "$runtime autofix must group auto-fixable decisions"
  grep -q "## No change required" "$skill_file" \
    || fail "$runtime autofix must group already-fixed and rejected decisions"
  grep -q "## Report only" "$skill_file" \
    || fail "$runtime autofix must group non-auto-fixable accepted decisions"
  if grep -q "| Decision | Severity | Location | Reviewer | Finding | Evidence | Action | URL |" "$skill_file"; then
    fail "$runtime autofix must not require the wide classification table"
  fi
done

echo "PASS: autofix"
