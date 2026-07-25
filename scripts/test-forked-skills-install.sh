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

# Seed what a pre-as-77n install left behind, so the first apply exercises the
# UPGRADE path. A clean HOME cannot catch a missing migration prune.
seed_legacy_team() {
  local teamdir="$1"; shift
  local staged="$home_dir/.skill-symlinks/agent-teams/$teamdir"
  local agents="$home_dir/.claude/agents/$teamdir"
  mkdir -p "$staged" "$agents"
  for name in "$@"; do
    printf 'legacy agent\n' >"$staged/$name"
    ln -s "$staged/$name" "$agents/$name"
  done
}
seed_legacy_team go-review-team review-lead.md structure-reviewer.md
seed_legacy_team feature-review-team acceptance-lead.md product-reviewer.md
# A hand-written agent the installer does not own must survive the prune.
printf 'mine\n' >"$home_dir/.claude/agents/go-review-team/my-own.md"

HOME="$home_dir" "$ROOT/install.sh" --all >"$home_dir/stdout" 2>"$home_dir/stderr"

# Upgrade prune: installer-owned legacy registrations and staged team copies
# are gone; the user's own file (and therefore its directory) survives.
for teamdir in go-review-team feature-review-team; do
  [ ! -e "$home_dir/.skill-symlinks/agent-teams/$teamdir" ] \
    || fail "legacy staged $teamdir copy should be pruned on upgrade"
done
for legacy in go-review-team/review-lead.md go-review-team/structure-reviewer.md \
              feature-review-team/acceptance-lead.md feature-review-team/product-reviewer.md; do
  [ ! -e "$home_dir/.claude/agents/$legacy" ] \
    || fail "legacy agent registration $legacy should be pruned on upgrade"
done
[ ! -e "$home_dir/.claude/agents/feature-review-team" ] \
  || fail "emptied feature-review-team agent dir should be removed"
[ -f "$home_dir/.claude/agents/go-review-team/my-own.md" ] \
  || fail "a user's own agent file must survive the prune"

for skill in "${forked_skills[@]}"; do
  codex="$home_dir/.skill-symlinks/runtimes/codex/skills/$skill"
  claude="$home_dir/.skill-symlinks/runtimes/claude/skills/$skill"

  [ -f "$codex/SKILL.md" ] || fail "$skill missing Codex staged SKILL.md"
  [ -f "$claude/SKILL.md" ] || fail "$skill missing Claude staged SKILL.md"

  assert_symlink_target "$home_dir/.agents/skills/$skill" "$codex"
  assert_symlink_target "$home_dir/.claude/skills/$skill" "$claude"

  [ "$codex" != "$claude" ] || fail "$skill Codex and Claude staged paths must differ"

  while IFS= read -r rel; do
    for runtime in codex; do
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

  for runtime in codex; do
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
[ -f "$home_dir/.skill-symlinks/runtimes/codex/skills/skill-parity-audit/scripts/audit_skill_parity.py" ] \
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

[ -f "$home_dir/.skill-symlinks/runtimes/claude/skills/product-manager/research-agent.md" ] \
  || fail "product-manager Claude research prompt did not install"

# The review skills are runtime-forked first-party skills (as-77n): two
# runtime assemblies (codex → ~/.agents, claude → ~/.claude), never ~/.cursor,
# and no ~/.claude/agents registrations at all — the orchestrator runs inline
# and the role briefs are prompt source, not agent definitions.
for skill in go-review feature-review; do
  review_codex="$home_dir/.skill-symlinks/runtimes/codex/skills/$skill"
  review_claude="$home_dir/.skill-symlinks/runtimes/claude/skills/$skill"

  assert_symlink_target "$home_dir/.agents/skills/$skill" "$review_codex"
  assert_symlink_target "$home_dir/.claude/skills/$skill" "$review_claude"

  [ ! -e "$home_dir/.cursor/skills/$skill" ] \
    || fail "$skill must not install into ~/.cursor"
  [ ! -e "$home_dir/.claude/agents/$skill" ] \
    || fail "$skill must not register any Claude agents"

  for runtime_dir in "$review_codex" "$review_claude"; do
    [ -d "$runtime_dir/roles" ] || fail "$skill assembly missing shared roles/ ($runtime_dir)"
  done
  [ -f "$review_codex/agents/openai.yaml" ] || fail "$skill codex assembly missing openai.yaml"

  matches="$(rg -n "$claude_only_tokens" "$review_codex/SKILL.md" "$review_codex/roles" || true)"
  if [ -n "$matches" ]; then
    printf '%s\n' "$matches" >&2
    fail "$skill codex assembly contains Claude-only tokens"
  fi
done

# feature-review-team emptied completely, so its dir is gone; go-review-team
# survives only because of the user's own file seeded above.
[ ! -e "$home_dir/.claude/agents/feature-review-team" ] \
  || fail "no feature-review-team agent registrations should exist"
[ "$(ls "$home_dir/.claude/agents/go-review-team")" = "my-own.md" ] \
  || fail "go-review-team agent dir should contain only the user's own file"

# --none removes the installer-owned links again.
HOME="$home_dir" "$ROOT/install.sh" --none >"$home_dir/stdout-none" 2>"$home_dir/stderr-none"
for skill in go-review feature-review; do
  [ ! -e "$home_dir/.agents/skills/$skill" ] || fail "--none should remove the ~/.agents $skill link"
  [ ! -e "$home_dir/.claude/skills/$skill" ] || fail "--none should remove the ~/.claude $skill link"
done

echo "PASS: forked skills install"
