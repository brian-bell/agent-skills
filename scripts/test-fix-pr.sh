#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILL_SCRIPT="$REPO_DIR/skills/fix-pr/scripts/gather_unresolved_pr_comments.py"
SKILL_DOC="$REPO_DIR/skills/fix-pr/SKILL.md"
AUTOFIX_DOC="$REPO_DIR/skills/autofix/SKILL.md"
SHIP_DOC="$REPO_DIR/skills/ship/SKILL.md"
OPENAI_METADATA="$REPO_DIR/skills/fix-pr/agents/openai.yaml"
README_DOC="$REPO_DIR/README.md"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_contains() {
  local file="$1"
  local expected="$2"
  local message="$3"

  grep -Fq "$expected" "$file" || fail "$message"
}

assert_not_contains() {
  local file="$1"
  local unexpected="$2"
  local message="$3"

  if grep -Fq "$unexpected" "$file"; then
    fail "$message"
  fi
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
grep -q "| Decision | Location | Reviewer | Finding | Evidence | Action | URL |" "$tmp_dir/report.md" || fail "missing markdown table header"
grep -q "| pending | parser.go:17 | @reviewer | Needs a bounds check before indexing. |  |  | https://github.com/octo/demo/pull/42#discussion_r1001 |" "$tmp_dir/report.md" || fail "missing first table row"
grep -q "| pending | lexer.go:31 | @second-reviewer | This paginated thread should be included. |  |  | https://github.com/octo/demo/pull/42#discussion_r1003 |" "$tmp_dir/report.md" || fail "missing paginated table row"
if grep -q "I can handle this in a follow-up." "$tmp_dir/report.md"; then
  fail "markdown output should summarize the root review comment, not the latest reply"
fi
if grep -q "^## 1\\. " "$tmp_dir/report.md"; then
  fail "markdown output should be table-first, not per-thread sections"
fi
grep -q "Needs a bounds check before indexing." "$tmp_dir/report.md" || fail "missing comment body"
grep -q "This paginated thread should be included." "$tmp_dir/report.md" || fail "missing paginated comment body"
assert_contains "$SKILL_DOC" "run the *autofix* skill in deferred mode for fix-pr orchestration with --comment <comment-or-thread URL>" "fix-pr skill should hand accepted findings to deferred autofix"
assert_contains "$SKILL_DOC" "ask whether to run autofix for all or selected accepted findings, then run aggregate autoreview and ship to the existing PR after approval" "fix-pr frontmatter should describe consent, aggregate review, and ship"
assert_contains "$SKILL_DOC" "Ask the user whether to run the *autofix* skill for all accepted findings or a selected subset before mutating the PR." "fix-pr should ask for post-classification autofix consent"
assert_contains "$SKILL_DOC" "If the user declines or does not explicitly approve, do not run autofix; report the pending autofix commands/actions and stop." "fix-pr should stay read-only without explicit approval"
assert_not_contains "$SKILL_DOC" "Run the autofix handoff only when the user explicitly asks to fix, implement, apply, or otherwise mutate the PR." "fix-pr should not keep stale original-request-only autofix gating"
grep -q 'Do not implement accepted findings directly in `fix-pr`' "$SKILL_DOC" || fail "fix-pr skill should forbid direct implementation of accepted findings"
assert_contains "$SKILL_DOC" "Pass the accepted row's finding, evidence, and local action as context" "fix-pr should pass accepted finding context to autofix"
assert_contains "$SKILL_DOC" "If a later approved finding blocks after earlier deferred fixes succeeded, report completed fixes and the blocker, then ask before continuing to aggregate autoreview/ship the partial set." "fix-pr should ask before shipping partial completed fixes"
assert_contains "$SKILL_DOC" "If the user declines partial review/ship, leave local uncommitted changes intact, do not revert, do not ship, and report exact changed files/proof/blockers." "fix-pr should preserve partial local fixes when partial ship is declined"
assert_contains "$SKILL_DOC" "After all approved deferred autofix runs complete, run the *autoreview* skill in local/dirty mode on the aggregate resulting uncommitted diff." "fix-pr should run aggregate local autoreview"
assert_contains "$SKILL_DOC" "Do not run the *ship* skill until autoreview exits clean with no accepted/actionable findings." "fix-pr should gate ship on clean autoreview"
assert_contains "$SKILL_DOC" "Accepted/actionable aggregate autoreview findings are handled as ordinary local code fixes within the active orchestrated workflow, not as direct GitHub review-comment mutation by fix-pr." "fix-pr should keep aggregate autoreview fixes separate from GitHub comment mutation"
assert_contains "$SKILL_DOC" "Before invoking the *ship* skill, verify the current checkout still targets the original PR: same repo, PR number, PR URL, and head branch/ref." "fix-pr should verify PR identity before shipping"
assert_contains "$SKILL_DOC" "Stop before calling the *ship* skill if no existing PR is found, the current branch maps to a different PR, or PR identity is ambiguous." "fix-pr should stop before ship when PR identity is unsafe"
assert_contains "$SKILL_DOC" "existing PR only; stop rather than create" "fix-pr should constrain ship to an existing PR"
assert_contains "$SKILL_DOC" "Run the *ship* skill to push the reviewed fixes to the existing PR." "fix-pr should ship reviewed fixes to the existing PR"
assert_contains "$SKILL_DOC" "The deferred fix-pr path does not reply to original review threads or resolve them." "fix-pr should document no thread replies or resolution"
assert_contains "$AUTOFIX_DOC" "deferred fix-pr orchestration mode" "autofix skill should document deferred fix-pr orchestration mode"
assert_contains "$AUTOFIX_DOC" "run the *autofix* skill in deferred mode for fix-pr orchestration with --comment <comment-or-thread URL>" "autofix skill should document the stable deferred handoff phrase"
assert_contains "$AUTOFIX_DOC" "stop before commit, autoreview, ship, GitHub replies, or thread resolution" "autofix deferred mode should stop before commit/review/ship/reply"
assert_contains "$AUTOFIX_DOC" "return changed files, tests/proof run, the original comment/thread URL, dirty-worktree status, and any blockers" "autofix deferred mode should report local fix evidence and blockers"
assert_contains "$AUTOFIX_DOC" "Normal standalone autofix --comment <url> behavior remains unchanged: run autoreview, ship, and reply." "autofix standalone behavior should remain unchanged"
assert_contains "$AUTOFIX_DOC" "deferred mode may continue on a dirty checkout only when those dirty changes are from the same active fix-pr orchestration and target the same PR" "autofix deferred mode should constrain dirty worktrees"
assert_contains "$AUTOFIX_DOC" "For deferred fix-pr orchestration, the worktree may contain prior deferred fixes from the same active fix-pr orchestration targeting the same PR." "autofix checkout prep should allow prior deferred fixes from the same orchestration"
assert_contains "$AUTOFIX_DOC" "stop before editing if the worktree contains unrelated changes, changes from another PR, or an ambiguous dirty state" "autofix deferred mode should stop on unrelated dirty state"
assert_contains "$AUTOFIX_DOC" "If test-first is not practical, return why and what proof was run instead." "autofix deferred mode should report skipped test-first rationale"
assert_contains "$AUTOFIX_DOC" "must not commit, revert, or overwrite existing changes without explicit instruction" "autofix deferred mode should not commit or overwrite work"
assert_contains "$SHIP_DOC" "existing PR only; stop rather than create" "ship should support existing-PR-only handoff"
assert_contains "$SHIP_DOC" "When that handoff is present, check for the existing PR before committing, pushing, or creating a PR." "ship should verify existing PR before mutating under existing-PR-only handoff"
assert_contains "$OPENAI_METADATA" 'ask whether to run $autofix' "fix-pr Codex metadata should mention asking to run autofix"
assert_contains "$OPENAI_METADATA" "run autoreview on the aggregate result before shipping" "fix-pr Codex metadata should mention aggregate autoreview before shipping"
assert_contains "$README_DOC" "fix-pr asks whether to use autofix and ships reviewed fixes to the PR" "README should summarize updated fix-pr behavior"

echo "PASS: fix-pr"
