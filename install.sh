#!/bin/bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
CLAUDE_DIR="$HOME/.claude"
AGENTS_DIR="$HOME/.agents"

portable_skills=(
  chrome-reading-list
  commit
  docs
  grill-me
  improve-codebase-architecture
  prd-to-issues
  prd-to-plan
  product-manager
  review-loop
  ship
  skill-parity-audit
  tdd
  work-prs
  write-a-prd
)

link_dir() {
  local source="$1"
  local target="$2"

  rm -rf "$target"
  ln -s "$source" "$target"
}

mkdir -p "$CLAUDE_DIR/skills" "$CLAUDE_DIR/agents" "$AGENTS_DIR/skills"

# Retired surfaces. Keep them absent on reinstall.
rm -rf "$CLAUDE_DIR/commands"
rm -rf "$CLAUDE_DIR/skills/rebase" "$AGENTS_DIR/skills/rebase"

# Portable skills are top-level repo directories and are symlinked into both
# Claude and Codex/agents skill roots.
for skill in "${portable_skills[@]}"; do
  if [ ! -d "$REPO_DIR/$skill" ]; then
    echo "Missing portable skill: $REPO_DIR/$skill" >&2
    exit 1
  fi

  link_dir "$REPO_DIR/$skill" "$AGENTS_DIR/skills/$skill"
  link_dir "$REPO_DIR/$skill" "$CLAUDE_DIR/skills/$skill"
done

# Claude-native skills stay under claude-native/.
link_dir "$REPO_DIR/claude-native/go-review-team" "$CLAUDE_DIR/skills/go-review"
link_dir "$REPO_DIR/claude-native/feature-review-team" "$CLAUDE_DIR/skills/feature-review"

mkdir -p "$CLAUDE_DIR/agents/go-review-team"
for agent in review-lead security-reviewer style-reviewer error-reviewer structure-reviewer; do
  ln -sf "$REPO_DIR/claude-native/go-review-team/$agent.md" "$CLAUDE_DIR/agents/go-review-team/$agent.md"
done

mkdir -p "$CLAUDE_DIR/agents/feature-review-team"
for agent in acceptance-lead product-reviewer safety-reviewer quality-reviewer maintainability-reviewer documentation-reviewer; do
  ln -sf "$REPO_DIR/claude-native/feature-review-team/$agent.md" "$CLAUDE_DIR/agents/feature-review-team/$agent.md"
done

echo "Installed portable skills into ~/.agents/skills and ~/.claude/skills via symlinks:"
for skill in "${portable_skills[@]}"; do
  echo "  $skill"
done
echo "Installed Claude-native skills:"
echo "  ~/.claude/skills/go-review -> claude-native/go-review-team"
echo "  ~/.claude/skills/feature-review -> claude-native/feature-review-team"
echo "Removed retired command layer: ~/.claude/commands"
