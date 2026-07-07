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

command -v rg >/dev/null 2>&1 || fail "ripgrep (rg) is required"

claude_only_tokens='Claude Code|Agent tool|subagent_type|TaskCreate|TaskUpdate|TaskList|TeamCreate|SendMessage|AskUserQuestion|Artifact|WebSearch|WebFetch'

forked_skills=()
for runtimes_dir in "$ROOT"/skills/*/runtimes; do
  [ -d "$runtimes_dir" ] || continue
  forked_skills+=("$(basename "$(dirname "$runtimes_dir")")")
done
[ "${#forked_skills[@]}" -gt 0 ] || fail "no runtime-forked skills found under skills/"

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

  while IFS= read -r rel; do
    for runtime in codex cursor; do
      if [ -e "$ROOT/skills/$skill/runtimes/$runtime/$rel" ]; then
        continue
      fi
      if [ -e "$ROOT/skills/$skill/shared/$rel" ]; then
        continue
      fi
      [ ! -e "$home_dir/.skill-symlinks/runtimes/$runtime/skills/$skill/$rel" ] \
        || fail "$skill $runtime staged tree must not include Claude-only overlay file $rel"
    done
  done < <(cd "$ROOT/skills/$skill/runtimes/claude" && find . -type f | sed 's|^\./||')

  for runtime in codex cursor; do
    staged="$home_dir/.skill-symlinks/runtimes/$runtime/skills/$skill"
    matches="$(rg -n -g '*.md' "$claude_only_tokens" "$staged" || true)"
    if [ -n "$matches" ]; then
      printf '%s\n' "$matches" >&2
      fail "$skill $runtime staged tree contains Claude-only tokens"
    fi
  done
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

echo "PASS: forked skills install"
