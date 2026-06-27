#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
FIRST_PARTY_DIR="$REPO_DIR/skills"
THIRD_PARTY_DIR="$REPO_DIR/third-party"
AGENT_TEAMS_DIR="$REPO_DIR/agent-teams"
CLAUDE_DIR="$HOME/.claude"
AGENTS_DIR="$HOME/.agents"
FORCE=false

usage() {
  cat <<EOF
Usage: $(basename "$0") [--force]

Options:
  --force   Overwrite existing skill targets before linking.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --force)
      FORCE=true
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
  shift
done

first_party_skills=(
  autobuild
  autofix
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

link_path() {
  local source="$1"
  local target="$2"

  if [ -L "$target" ]; then
    if [ "$(readlink "$target")" = "$source" ]; then
      return
    fi

    if [ "$FORCE" != true ]; then
      echo "Refusing to overwrite existing target: $target (use --force)" >&2
      exit 1
    fi

    rm -f "$target"
  elif [ -e "$target" ]; then
    if [ "$FORCE" != true ]; then
      echo "Refusing to overwrite existing target: $target (use --force)" >&2
      exit 1
    fi

    rm -rf "$target"
  fi

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

    link_path "$source_dir/$skill" "$AGENTS_DIR/skills/$skill"
    link_path "$source_dir/$skill" "$CLAUDE_DIR/skills/$skill"
  done
}

echo "Note: install-skills.sh is deprecated; prefer ./install.sh (interactive) or ./install.sh --all." >&2

mkdir -p "$CLAUDE_DIR/skills" "$CLAUDE_DIR/agents" "$AGENTS_DIR/skills"

# Portable skills are symlinked into both Claude and Codex/agents skill roots.
install_portable_skills "$FIRST_PARTY_DIR" "${first_party_skills[@]}"
install_portable_skills "$THIRD_PARTY_DIR" "${third_party_skills[@]}"

# Agent-team skills are Claude-native and stay under agent-teams/.
link_path "$AGENT_TEAMS_DIR/go-review-team" "$CLAUDE_DIR/skills/go-review"
link_path "$AGENT_TEAMS_DIR/feature-review-team" "$CLAUDE_DIR/skills/feature-review"

mkdir -p "$CLAUDE_DIR/agents/go-review-team"
for agent in review-lead security-reviewer style-reviewer error-reviewer structure-reviewer; do
  link_path "$AGENT_TEAMS_DIR/go-review-team/$agent.md" "$CLAUDE_DIR/agents/go-review-team/$agent.md"
done

mkdir -p "$CLAUDE_DIR/agents/feature-review-team"
for agent in acceptance-lead product-reviewer safety-reviewer quality-reviewer maintainability-reviewer documentation-reviewer; do
  link_path "$AGENT_TEAMS_DIR/feature-review-team/$agent.md" "$CLAUDE_DIR/agents/feature-review-team/$agent.md"
done

echo "Installed portable skills into ~/.agents/skills and ~/.claude/skills via symlinks:"
for skill in "${first_party_skills[@]}" "${third_party_skills[@]}"; do
  echo "  $skill"
done
echo "Installed agent-team skills:"
echo "  ~/.claude/skills/go-review -> agent-teams/go-review-team"
echo "  ~/.claude/skills/feature-review -> agent-teams/feature-review-team"
