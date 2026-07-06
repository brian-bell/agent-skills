#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

fail() {
  echo "FAIL: $*" >&2
  exit 1
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

for skill in "${forked_skills[@]}"; do
  dir="$ROOT/skills/$skill"
  [ -d "$dir/shared" ] || fail "$skill must have shared/"
  [ ! -e "$dir/SKILL.md" ] || fail "$skill must not keep a root SKILL.md"
  [ ! -d "$dir/agents" ] || fail "$skill must move agents/openai.yaml under runtimes/codex/agents/"

  for runtime in claude codex cursor; do
    [ -f "$dir/runtimes/$runtime/SKILL.md" ] \
      || fail "$skill must have runtimes/$runtime/SKILL.md"
  done

  if rg -n "Platform —" "$dir/runtimes" >/dev/null; then
    rg -n "Platform —" "$dir/runtimes" >&2
    fail "$skill must not contain Platform blocks in runtime overlays"
  fi

  for runtime in codex cursor; do
    overlay="$dir/runtimes/$runtime"
    if rg -n "Claude Code|Agent tool|subagent_type|TaskCreate|TaskUpdate|TaskList|TeamCreate|SendMessage|AskUserQuestion|Artifact|WebSearch|WebFetch" "$overlay" >/dev/null; then
      rg -n "Claude Code|Agent tool|subagent_type|TaskCreate|TaskUpdate|TaskList|TeamCreate|SendMessage|AskUserQuestion|Artifact|WebSearch|WebFetch" "$overlay" >&2
      fail "$skill $runtime overlay contains Claude-only tokens"
    fi
  done
done

[ -f "$ROOT/skills/product-manager/shared/product-brief-template.md" ] \
  || fail "product-manager must keep product-brief-template.md in shared/"
[ -f "$ROOT/skills/product-manager/runtimes/claude/research-agent.md" ] \
  || fail "product-manager must keep research-agent.md in the Claude overlay"
[ ! -e "$ROOT/skills/product-manager/shared/research-agent.md" ] \
  || fail "product-manager research-agent.md must not be shared"
[ ! -e "$ROOT/skills/product-manager/runtimes/codex/research-agent.md" ] \
  || fail "product-manager research-agent.md must not be in the Codex overlay"
[ ! -e "$ROOT/skills/product-manager/runtimes/cursor/research-agent.md" ] \
  || fail "product-manager research-agent.md must not be in the Cursor overlay"

echo "PASS: forked skills layout"
