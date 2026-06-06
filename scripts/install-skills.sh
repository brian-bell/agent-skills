#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
PORTABLE_SKILLS_DIR="$REPO_DIR/catalog/portable"
CLAUDE_NATIVE_DIR="$REPO_DIR/catalog/claude-native"
CLAUDE_DIR="$HOME/.claude"
AGENTS_DIR="$HOME/.agents"

portable_skills=(
  chrome-reading-list
  commit
  docs
  grill-me
  improve-codebase-architecture
  planned-implementation-agent
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

# Portable skills live in the canonical catalog and are symlinked into both
# Claude and Codex/agents skill roots.
for skill in "${portable_skills[@]}"; do
  if [ ! -d "$PORTABLE_SKILLS_DIR/$skill" ]; then
    echo "Missing portable skill: $PORTABLE_SKILLS_DIR/$skill" >&2
    exit 1
  fi

  link_dir "$PORTABLE_SKILLS_DIR/$skill" "$AGENTS_DIR/skills/$skill"
  link_dir "$PORTABLE_SKILLS_DIR/$skill" "$CLAUDE_DIR/skills/$skill"
done

# Claude-native skills stay in the Claude-native catalog.
link_dir "$CLAUDE_NATIVE_DIR/go-review-team" "$CLAUDE_DIR/skills/go-review"
link_dir "$CLAUDE_NATIVE_DIR/feature-review-team" "$CLAUDE_DIR/skills/feature-review"

mkdir -p "$CLAUDE_DIR/agents/go-review-team"
for agent in review-lead security-reviewer style-reviewer error-reviewer structure-reviewer; do
  ln -sf "$CLAUDE_NATIVE_DIR/go-review-team/$agent.md" "$CLAUDE_DIR/agents/go-review-team/$agent.md"
done

mkdir -p "$CLAUDE_DIR/agents/feature-review-team"
for agent in acceptance-lead product-reviewer safety-reviewer quality-reviewer maintainability-reviewer documentation-reviewer; do
  ln -sf "$CLAUDE_NATIVE_DIR/feature-review-team/$agent.md" "$CLAUDE_DIR/agents/feature-review-team/$agent.md"
done

echo "Installed portable skills into ~/.agents/skills and ~/.claude/skills via symlinks:"
for skill in "${portable_skills[@]}"; do
  echo "  $skill"
done
echo "Installed Claude-native skills:"
echo "  ~/.claude/skills/go-review -> catalog/claude-native/go-review-team"
echo "  ~/.claude/skills/feature-review -> catalog/claude-native/feature-review-team"
