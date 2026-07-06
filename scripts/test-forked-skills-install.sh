#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_symlink_target() {
  local path="$1" target="$2"
  [ -L "$path" ] || fail "Expected $path to be a symlink"
  [ "$(readlink "$path")" = "$target" ] || fail "Expected $path -> $target, got $(readlink "$path")"
}

forked_skills=(
  commit
  chrome-reading-list
  tdd
  docs
  tdd-with-review
  skill-parity-audit
  slice-issues
  ship
  fix-pr
  autofix
  work-prs
  merge-prs-review-loop
  plan-with-review
  planned-implementation-agent
  product-manager
)

home_dir="$(mktemp -d)"
trap 'chmod -R u+w "$home_dir" 2>/dev/null || true; rm -rf "$home_dir"' EXIT

HOME="$home_dir" "$ROOT/install.sh" --all >"$home_dir/stdout" 2>"$home_dir/stderr"

for skill in "${forked_skills[@]}"; do
  codex="$home_dir/.skill-symlinks/runtimes/codex/skills/$skill"
  claude="$home_dir/.skill-symlinks/runtimes/claude/skills/$skill"
  cursor="$home_dir/.skill-symlinks/runtimes/cursor/skills/$skill"

  [ -f "$codex/SKILL.md" ] || fail "$skill missing Codex staged SKILL.md"
  [ -f "$claude/SKILL.md" ] || fail "$skill missing Claude staged SKILL.md"
  [ -f "$cursor/SKILL.md" ] || fail "$skill missing Cursor staged SKILL.md"

  assert_symlink_target "$home_dir/.agents/skills/$skill" "$codex"
  assert_symlink_target "$home_dir/.claude/skills/$skill" "$claude"
  assert_symlink_target "$home_dir/.cursor/skills/$skill" "$cursor"

  [ "$codex" != "$claude" ] || fail "$skill Codex and Claude staged paths must differ"
  [ "$codex" != "$cursor" ] || fail "$skill Codex and Cursor staged paths must differ"
  [ "$claude" != "$cursor" ] || fail "$skill Claude and Cursor staged paths must differ"
done

[ -f "$home_dir/.skill-symlinks/runtimes/codex/skills/chrome-reading-list/extract.py" ] \
  || fail "chrome-reading-list shared extractor did not install"
[ -f "$home_dir/.skill-symlinks/runtimes/claude/skills/tdd/tests.md" ] \
  || fail "tdd shared reference docs did not install"
[ -f "$home_dir/.skill-symlinks/runtimes/cursor/skills/skill-parity-audit/scripts/audit_skill_parity.py" ] \
  || fail "skill-parity-audit shared script did not install"
[ -f "$home_dir/.skill-symlinks/runtimes/codex/skills/fix-pr/scripts/gather_unresolved_pr_comments.py" ] \
  || fail "fix-pr shared collector did not install"
[ -f "$home_dir/.skill-symlinks/runtimes/claude/skills/autofix/scripts/gather_unresolved_pr_comments.py" ] \
  || fail "autofix shared collector did not install"

for runtime in codex claude cursor; do
  [ -f "$home_dir/.skill-symlinks/runtimes/$runtime/skills/product-manager/product-brief-template.md" ] \
    || fail "product-manager shared brief template did not install for $runtime"
done

[ -f "$home_dir/.skill-symlinks/runtimes/claude/skills/product-manager/research-agent.md" ] \
  || fail "product-manager Claude research prompt did not install"
[ ! -e "$home_dir/.skill-symlinks/runtimes/codex/skills/product-manager/research-agent.md" ] \
  || fail "product-manager Codex staged tree must not include Claude research prompt"
[ ! -e "$home_dir/.skill-symlinks/runtimes/cursor/skills/product-manager/research-agent.md" ] \
  || fail "product-manager Cursor staged tree must not include Claude research prompt"

for runtime in codex cursor; do
  staged="$home_dir/.skill-symlinks/runtimes/$runtime/skills/product-manager"
  if rg -n "Claude Code|Agent tool|subagent_type|TaskCreate|TaskUpdate|TaskList|TeamCreate|SendMessage|AskUserQuestion|Artifact|WebSearch|WebFetch" "$staged" >/dev/null; then
    rg -n "Claude Code|Agent tool|subagent_type|TaskCreate|TaskUpdate|TaskList|TeamCreate|SendMessage|AskUserQuestion|Artifact|WebSearch|WebFetch" "$staged" >&2
    fail "product-manager $runtime staged tree contains Claude-only tokens"
  fi
done

echo "PASS: forked skills install"
