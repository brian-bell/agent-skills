#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

command -v rg >/dev/null 2>&1 || fail "ripgrep (rg) is required"

claude_only_tokens='Claude Code|Agent tool|subagent_type|TaskCreate|TaskUpdate|TaskList|TeamCreate|SendMessage|AskUserQuestion|Artifact|WebSearch|WebFetch'

forked_skills=()
for runtimes_dir in "$ROOT"/skills/*/runtimes; do
  [ -d "$runtimes_dir" ] || continue
  forked_skills+=("$(basename "$(dirname "$runtimes_dir")")")
done
[ "${#forked_skills[@]}" -gt 0 ] || fail "no runtime-forked skills found under skills/"

for skill in "${forked_skills[@]}"; do
  dir="$ROOT/skills/$skill"
  [ -d "$dir/shared" ] || fail "$skill must have shared/"
  [ ! -e "$dir/SKILL.md" ] || fail "$skill must not keep a root SKILL.md"
  [ ! -d "$dir/agents" ] || fail "$skill must move agents/openai.yaml under runtimes/codex/agents/"

  # Claude + Codex overlays are required; Cursor is optional.
  for runtime in claude codex; do
    [ -f "$dir/runtimes/$runtime/SKILL.md" ] \
      || fail "$skill must have runtimes/$runtime/SKILL.md"
  done

  matches="$(rg -n "Platform —" "$dir/runtimes" || true)"
  if [ -n "$matches" ]; then
    printf '%s\n' "$matches" >&2
    fail "$skill must not contain Platform blocks in runtime overlays"
  fi

  for runtime in codex cursor; do
    [ -d "$dir/runtimes/$runtime" ] || continue
    matches="$(rg -n "$claude_only_tokens" "$dir/runtimes/$runtime" || true)"
    if [ -n "$matches" ]; then
      printf '%s\n' "$matches" >&2
      fail "$skill $runtime overlay contains Claude-only tokens"
    fi
  done
done

[ -f "$ROOT/skills/product-manager/shared/product-brief-template.md" ] \
  || fail "product-manager must keep product-brief-template.md in shared/"
[ -f "$ROOT/skills/product-manager/shared/roles/researcher.md" ] \
  || fail "product-manager must have shared/roles/researcher.md"
[ -f "$ROOT/skills/product-manager/shared/roles/codebase-surveyor.md" ] \
  || fail "product-manager must have shared/roles/codebase-surveyor.md"
[ -f "$ROOT/skills/product-manager/shared/roles/brief-critic.md" ] \
  || fail "product-manager must have shared/roles/brief-critic.md"

shared_matches="$(rg -n "$claude_only_tokens" "$ROOT/skills/product-manager/shared" || true)"
if [ -n "$shared_matches" ]; then
  printf '%s\n' "$shared_matches" >&2
  fail "product-manager shared/ contains Claude-only tokens"
fi

for runtime in claude codex; do
  rg -q 'roles/researcher\.md' "$ROOT/skills/product-manager/runtimes/$runtime/SKILL.md" \
    || fail "product-manager $runtime overlay must reference roles/researcher.md"
done

[ ! -e "$ROOT/skills/product-manager/runtimes/cursor" ] \
  || fail "product-manager must not have a cursor overlay (cursor-less by design)"

[ -f "$ROOT/skills/product-manager/runtimes/claude/research-agent.md" ] \
  || fail "product-manager must keep research-agent.md in the Claude overlay"
[ ! -e "$ROOT/skills/product-manager/shared/research-agent.md" ] \
  || fail "product-manager research-agent.md must not be shared"
[ ! -e "$ROOT/skills/product-manager/runtimes/codex/research-agent.md" ] \
  || fail "product-manager research-agent.md must not be in the Codex overlay"
[ ! -e "$ROOT/skills/product-manager/runtimes/cursor/research-agent.md" ] \
  || fail "product-manager research-agent.md must not be in the Cursor overlay"

echo "PASS: forked skills layout"
