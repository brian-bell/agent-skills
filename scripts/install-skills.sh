#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
FIRST_PARTY_DIR="$REPO_DIR/skills"
THIRD_PARTY_DIR="$REPO_DIR/third-party"
AGENT_TEAMS_DIR="$REPO_DIR/agent-teams"
CLAUDE_DIR="$HOME/.claude"
AGENTS_DIR="$HOME/.agents"

first_party_skills=(
  chrome-reading-list
  commit
  docs
  merge-prs-review-loop
  planned-implementation-agent
  product-manager
  ship
  skill-parity-audit
  tdd
  tdd-with-review
  work-prs
)

third_party_skills=(
  autoreview
  grill-me
  improve-codebase-architecture
  prd-to-issues
  prd-to-plan
  review-loop
  write-a-prd
)

link_dir() {
  local source="$1"
  local target="$2"

  rm -rf "$target"
  ln -s "$source" "$target"
}

install_portable_skills() {
  local source_dir="$1"
  shift

  for skill in "$@"; do
    if [ ! -d "$source_dir/$skill" ]; then
      echo "Missing portable skill: $source_dir/$skill" >&2
      exit 1
    fi

    link_dir "$source_dir/$skill" "$AGENTS_DIR/skills/$skill"
    link_dir "$source_dir/$skill" "$CLAUDE_DIR/skills/$skill"
  done
}

mkdir -p "$CLAUDE_DIR/skills" "$CLAUDE_DIR/agents" "$AGENTS_DIR/skills"

# Portable skills are symlinked into both Claude and Codex/agents skill roots.
install_portable_skills "$FIRST_PARTY_DIR" "${first_party_skills[@]}"
install_portable_skills "$THIRD_PARTY_DIR" "${third_party_skills[@]}"

# Agent-team skills are Claude-native and stay under agent-teams/.
link_dir "$AGENT_TEAMS_DIR/go-review-team" "$CLAUDE_DIR/skills/go-review"
link_dir "$AGENT_TEAMS_DIR/feature-review-team" "$CLAUDE_DIR/skills/feature-review"

mkdir -p "$CLAUDE_DIR/agents/go-review-team"
for agent in review-lead security-reviewer style-reviewer error-reviewer structure-reviewer; do
  ln -sf "$AGENT_TEAMS_DIR/go-review-team/$agent.md" "$CLAUDE_DIR/agents/go-review-team/$agent.md"
done

mkdir -p "$CLAUDE_DIR/agents/feature-review-team"
for agent in acceptance-lead product-reviewer safety-reviewer quality-reviewer maintainability-reviewer documentation-reviewer; do
  ln -sf "$AGENT_TEAMS_DIR/feature-review-team/$agent.md" "$CLAUDE_DIR/agents/feature-review-team/$agent.md"
done

echo "Installed portable skills into ~/.agents/skills and ~/.claude/skills via symlinks:"
for skill in "${first_party_skills[@]}" "${third_party_skills[@]}"; do
  echo "  $skill"
done
echo "Installed agent-team skills:"
echo "  ~/.claude/skills/go-review -> agent-teams/go-review-team"
echo "  ~/.claude/skills/feature-review -> agent-teams/feature-review-team"
