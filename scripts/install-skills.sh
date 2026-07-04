#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
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

# shellcheck source=scripts/skills-tui.sh
source "$REPO_DIR/scripts/skills-tui.sh"

install_one() {
  local kind="$1" name="$2" source="$3"

  if [ "$FORCE" = true ]; then
    install_skill "$kind" "$name" "$source" true true
    return
  fi

  apply_skill "$kind" "$name" "$source" 1 false >/dev/null
  case "$(skill_state "$kind" "$name" "$source")" in
    installed|skipped) ;;
    *)
      echo "Refusing to overwrite existing target for $name (use --force)" >&2
      exit 1
      ;;
  esac
}

echo "Note: install-skills.sh is deprecated; prefer ./install.sh (interactive) or ./install.sh --all." >&2

installed=()
while IFS=$'\t' read -r kind name source; do
  [ -n "$name" ] || continue
  install_one "$kind" "$name" "$source"
  installed+=("$name")
done <<EOF
$(discover_skills "$REPO_DIR")
EOF

echo "Installed skills into ~/.agents/skills, ~/.claude/skills, and ~/.cursor/skills via staged symlinks:"
for skill in "${installed[@]}"; do
  echo "  $skill"
done
echo "Staged copies live under ~/.skill-symlinks."
