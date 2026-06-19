#!/usr/bin/env bash
#
# install.sh — install/uninstall the save-session SessionEnd hook.
#
# Installing:
#   1. Symlinks save-session.sh -> ~/.claude/hooks/save-session.sh
#   2. Idempotently adds a SessionEnd hook entry to ~/.claude/settings.json
#
# Uninstalling (--uninstall) reverses both steps. settings.json is backed up
# before any edit. Requires jq for the settings.json merge.
#
# Usage:
#   ./install.sh [--force] [--uninstall]
#
#   --force      Replace an existing ~/.claude/hooks/save-session.sh that is
#                a real file or a foreign symlink. Repo-owned symlinks are
#                always relinked.
#   --uninstall  Remove the symlink and the settings.json hook entry.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SRC="$SCRIPT_DIR/save-session.sh"

CLAUDE_DIR="$HOME/.claude"
HOOKS_DIR="$CLAUDE_DIR/hooks"
TARGET="$HOOKS_DIR/save-session.sh"
SETTINGS="$CLAUDE_DIR/settings.json"
# Command Claude Code runs. $HOME is expanded by the shell at hook time.
CMD='$HOME/.claude/hooks/save-session.sh'

FORCE=false
UNINSTALL=false
while [ "$#" -gt 0 ]; do
  case "$1" in
    --force) FORCE=true ;;
    --uninstall) UNINSTALL=true ;;
    -h|--help) sed -n '2,24p' "$0"; exit 0 ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
  shift
done

need_jq() {
  command -v jq >/dev/null 2>&1 || {
    echo "ERROR: jq is required to edit $SETTINGS" >&2
    exit 1
  }
}

backup_settings() {
  [ -f "$SETTINGS" ] || return 0
  local bak="$SETTINGS.bak.$(date '+%Y%m%d%H%M%S')"
  cp "$SETTINGS" "$bak"
  echo "  backed up settings.json -> $bak"
}

if [ "$UNINSTALL" = true ]; then
  echo "Uninstalling save-session hook..."
  # Remove the symlink only if it points at our repo source.
  if [ -L "$TARGET" ] && [ "$(readlink "$TARGET")" = "$SRC" ]; then
    rm "$TARGET"
    echo "  removed symlink $TARGET"
  elif [ -e "$TARGET" ]; then
    echo "  left $TARGET in place (not a repo-owned symlink)"
  fi
  # Remove the settings.json entry referencing our command.
  if [ -f "$SETTINGS" ]; then
    need_jq
    if jq -e --arg cmd "$CMD" 'any(.hooks.SessionEnd[]?; any(.hooks[]?; .command == $cmd))' "$SETTINGS" >/dev/null 2>&1; then
      backup_settings
      tmp="$(mktemp)"
      jq --arg cmd "$CMD" '
        (.hooks.SessionEnd) |= ( (. // [])
          | map(.hooks |= map(select(.command != $cmd)))
          | map(select((.hooks | length) > 0)) )
        | if (.hooks.SessionEnd | length) == 0 then del(.hooks.SessionEnd) else . end
        | if (.hooks | length) == 0 then del(.hooks) else . end
      ' "$SETTINGS" >"$tmp" && mv "$tmp" "$SETTINGS"
      echo "  removed SessionEnd hook entry from settings.json"
    else
      echo "  no matching hook entry in settings.json"
    fi
  fi
  echo "Done. Archived sessions under ~/.agent-sessions/claude are left untouched."
  exit 0
fi

echo "Installing save-session hook..."

# 1. Symlink the script.
mkdir -p "$HOOKS_DIR"
if [ -L "$TARGET" ]; then
  if [ "$(readlink "$TARGET")" = "$SRC" ]; then
    echo "  symlink already correct: $TARGET"
  else
    ln -sfn "$SRC" "$TARGET"
    echo "  relinked symlink: $TARGET -> $SRC"
  fi
elif [ -e "$TARGET" ]; then
  if [ "$FORCE" = true ]; then
    rm -rf "$TARGET"
    ln -s "$SRC" "$TARGET"
    echo "  replaced existing file with symlink: $TARGET (--force)"
  else
    echo "ERROR: $TARGET exists and is not a repo-owned symlink. Re-run with --force." >&2
    exit 1
  fi
else
  ln -s "$SRC" "$TARGET"
  echo "  created symlink: $TARGET -> $SRC"
fi

# 2. Merge the SessionEnd hook entry into settings.json.
need_jq
if [ ! -f "$SETTINGS" ]; then
  echo '{}' >"$SETTINGS"
  echo "  created $SETTINGS"
fi

if jq -e --arg cmd "$CMD" 'any(.hooks.SessionEnd[]?; any(.hooks[]?; .command == $cmd))' "$SETTINGS" >/dev/null 2>&1; then
  echo "  settings.json already has the SessionEnd hook"
else
  backup_settings
  tmp="$(mktemp)"
  jq --arg cmd "$CMD" '
    .hooks = (.hooks // {})
    | .hooks.SessionEnd = (.hooks.SessionEnd // [])
    | .hooks.SessionEnd += [{matcher: "", hooks: [{type: "command", command: $cmd}]}]
  ' "$SETTINGS" >"$tmp" && mv "$tmp" "$SETTINGS"
  echo "  added SessionEnd hook to settings.json"
fi

cat <<'EOF'

Done.

NOTE: Hooks are snapshotted at session start, so this won't fire for the
current Claude Code session. Start a fresh session (or run /hooks and reload),
then end a session to archive it. Verify with:

  find ~/.agent-sessions/claude -type f | sort
  cat ~/.agent-sessions/claude/save-session.log
EOF
