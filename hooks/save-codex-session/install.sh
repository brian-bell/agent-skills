#!/usr/bin/env bash
#
# install.sh - install/uninstall the Codex save-session Stop hook.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SRC="$SCRIPT_DIR/save-session.sh"

CODEX_DIR="$HOME/.codex"
HOOKS_DIR="$CODEX_DIR/hooks"
TARGET="$HOOKS_DIR/save-session.sh"
HOOKS_JSON="$CODEX_DIR/hooks.json"
# Command Codex runs. $HOME is expanded by the shell at hook time.
CMD='$HOME/.codex/hooks/save-session.sh'

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
    echo "ERROR: jq is required to edit $HOOKS_JSON" >&2
    exit 1
  }
}

backup_hooks_json() {
  [ -f "$HOOKS_JSON" ] || return 0
  local bak="$HOOKS_JSON.bak.$(date '+%Y%m%d%H%M%S')"
  cp "$HOOKS_JSON" "$bak"
  echo "  backed up hooks.json -> $bak"
}

if [ "$UNINSTALL" = true ]; then
  echo "Uninstalling Codex save-session hook..."
  if [ -L "$TARGET" ] && [ "$(readlink "$TARGET")" = "$SRC" ]; then
    rm "$TARGET"
    echo "  removed symlink $TARGET"
  elif [ -e "$TARGET" ]; then
    echo "  left $TARGET in place (not a repo-owned symlink)"
  fi

  if [ -f "$HOOKS_JSON" ]; then
    need_jq
    if jq -e --arg cmd "$CMD" 'any(.hooks.Stop[]?; any(.hooks[]?; .command == $cmd))' "$HOOKS_JSON" >/dev/null 2>&1; then
      backup_hooks_json
      tmp="$(mktemp)"
      jq --arg cmd "$CMD" '
        (.hooks.Stop) |= ( (. // [])
          | map(.hooks |= map(select(.command != $cmd)))
          | map(select((.hooks | length) > 0)) )
        | if (.hooks.Stop | length) == 0 then del(.hooks.Stop) else . end
        | if (.hooks | length) == 0 then del(.hooks) else . end
      ' "$HOOKS_JSON" >"$tmp" && mv "$tmp" "$HOOKS_JSON"
      echo "  removed Stop hook entry from hooks.json"
    else
      echo "  no matching hook entry in hooks.json"
    fi
  fi

  echo "Done. Archived sessions under ~/.agent-sessions/codex are left untouched."
  exit 0
fi

echo "Installing Codex save-session hook..."

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

need_jq
if [ ! -f "$HOOKS_JSON" ]; then
  echo '{}' >"$HOOKS_JSON"
  echo "  created $HOOKS_JSON"
fi

if jq -e --arg cmd "$CMD" 'any(.hooks.Stop[]?; any(.hooks[]?; .command == $cmd))' "$HOOKS_JSON" >/dev/null 2>&1; then
  echo "  hooks.json already has the Stop hook"
else
  backup_hooks_json
  tmp="$(mktemp)"
  jq --arg cmd "$CMD" '
    .hooks = (.hooks // {})
    | .hooks.Stop = (.hooks.Stop // [])
    | .hooks.Stop += [{
        hooks: [{
          type: "command",
          command: $cmd,
          timeout: 30,
          statusMessage: "Saving Codex session"
        }]
      }]
  ' "$HOOKS_JSON" >"$tmp" && mv "$tmp" "$HOOKS_JSON"
  echo "  added Stop hook to hooks.json"
fi

cat <<'EOF'

Done.

Codex requires command hooks to be reviewed and trusted before they run. Start a
fresh Codex session, run /hooks if prompted, trust this hook, then exit a session
to archive it. Verify with:

  find ~/.agent-sessions/codex -type f | sort
  cat ~/.agent-sessions/codex/save-session.log
EOF
