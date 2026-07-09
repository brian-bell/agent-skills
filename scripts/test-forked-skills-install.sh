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
  has_cursor=false
  if [ -f "$ROOT/skills/$skill/runtimes/cursor/SKILL.md" ]; then
    has_cursor=true
  fi

  [ -f "$codex/SKILL.md" ] || fail "$skill missing Codex staged SKILL.md"
  [ -f "$claude/SKILL.md" ] || fail "$skill missing Claude staged SKILL.md"

  assert_symlink_target "$home_dir/.agents/skills/$skill" "$codex"
  assert_symlink_target "$home_dir/.claude/skills/$skill" "$claude"

  if $has_cursor; then
    [ -f "$cursor/SKILL.md" ] || fail "$skill missing Cursor staged SKILL.md"
    assert_symlink_target "$home_dir/.cursor/skills/$skill" "$cursor"
    [ "$codex" != "$cursor" ] || fail "$skill Codex and Cursor staged paths must differ"
    [ "$claude" != "$cursor" ] || fail "$skill Claude and Cursor staged paths must differ"
  else
    [ ! -e "$home_dir/.cursor/skills/$skill" ] \
      || fail "$skill must not link into ~/.cursor when cursor overlay is absent"
    [ ! -e "$cursor" ] \
      || fail "$skill must not stage a cursor tree when cursor overlay is absent"
  fi

  [ "$codex" != "$claude" ] || fail "$skill Codex and Claude staged paths must differ"

  while IFS= read -r rel; do
    for runtime in codex cursor; do
      [ -d "$ROOT/skills/$skill/runtimes/$runtime" ] || continue
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
    [ -d "$ROOT/skills/$skill/runtimes/$runtime" ] || continue
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

for runtime in codex claude; do
  [ -f "$home_dir/.skill-symlinks/runtimes/$runtime/skills/product-manager/product-brief-template.md" ] \
    || fail "product-manager shared brief template did not install for $runtime"
  [ -d "$home_dir/.skill-symlinks/runtimes/$runtime/skills/product-manager/roles" ] \
    || fail "product-manager shared roles/ did not install for $runtime"
done
[ ! -e "$home_dir/.cursor/skills/product-manager" ] \
  || fail "product-manager must not install a ~/.cursor link"
[ ! -e "$home_dir/.skill-symlinks/runtimes/cursor/skills/product-manager" ] \
  || fail "product-manager must not stage a cursor tree"

[ -f "$home_dir/.skill-symlinks/runtimes/claude/skills/product-manager/research-agent.md" ] \
  || fail "product-manager Claude research prompt did not install"

# Runtime-forked agent team: two runtime assemblies (codex → ~/.agents,
# claude → ~/.claude + agent links), never ~/.cursor. The shared reviewer
# files legitimately contain Claude-only tokens (they are the Claude agent
# definitions), so token hygiene is asserted on the codex overlay SKILL.md
# only — not the assembled tree.
team_codex="$home_dir/.skill-symlinks/runtimes/codex/agent-teams/feature-review-team"
team_claude="$home_dir/.skill-symlinks/runtimes/claude/agent-teams/feature-review-team"

assert_symlink_target "$home_dir/.agents/skills/feature-review" "$team_codex"
assert_symlink_target "$home_dir/.claude/skills/feature-review" "$team_claude"
for md in acceptance-lead product-reviewer safety-reviewer quality-reviewer maintainability-reviewer documentation-reviewer; do
  assert_symlink_target "$home_dir/.claude/agents/feature-review-team/$md.md" "$team_claude/$md.md"
done
[ ! -e "$home_dir/.claude/agents/feature-review-team/SKILL.md" ] \
  || fail "feature-review SKILL.md must not be linked as an agent"
[ ! -e "$home_dir/.cursor/skills/feature-review" ] \
  || fail "feature-review must not install into ~/.cursor"
[ ! -e "$home_dir/.skill-symlinks/runtimes/cursor/agent-teams/feature-review-team" ] \
  || fail "feature-review must not stage a cursor assembly"

[ -f "$team_codex/agents/openai.yaml" ] || fail "feature-review codex assembly missing openai.yaml"
[ -f "$team_codex/quality-reviewer.md" ] || fail "feature-review codex assembly missing shared reviewers"
[ ! -e "$team_codex/acceptance-lead.md" ] \
  || fail "feature-review acceptance-lead.md must not reach the codex assembly"
[ -f "$team_claude/acceptance-lead.md" ] || fail "feature-review claude assembly missing acceptance-lead.md"

matches="$(rg -n "$claude_only_tokens" "$team_codex/SKILL.md" || true)"
if [ -n "$matches" ]; then
  printf '%s\n' "$matches" >&2
  fail "feature-review codex SKILL.md contains Claude-only tokens"
fi

# --none removes the installer-owned team links again.
HOME="$home_dir" "$ROOT/install.sh" --none >"$home_dir/stdout-none" 2>"$home_dir/stderr-none"
[ ! -e "$home_dir/.agents/skills/feature-review" ] || fail "--none should remove the ~/.agents team link"
[ ! -e "$home_dir/.claude/skills/feature-review" ] || fail "--none should remove the ~/.claude team link"
[ ! -e "$home_dir/.claude/agents/feature-review-team" ] || fail "--none should prune the team agents dir"

echo "PASS: forked skills install"
