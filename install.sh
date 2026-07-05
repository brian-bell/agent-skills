#!/bin/bash
set -euo pipefail

# Builds (when needed) and launches the Go skills manager (install/uninstall TUI).
# Non-interactive flags: --all, --none, --force. See tools/skills-tui --help.
REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
TUI_DIR="$REPO_DIR/tools/skills-tui"
BIN="$TUI_DIR/bin/skills-tui"

if ! command -v go >/dev/null 2>&1; then
  echo "install.sh requires the Go toolchain to build the installer: https://go.dev/dl" >&2
  exit 1
fi

if [[ ! -x "$BIN" ]] ||
  [[ -n "$(find "$TUI_DIR" \( -name '*.go' -o -name 'go.mod' \) -newer "$BIN" -print -quit)" ]]; then
  mkdir -p "$TUI_DIR/bin"
  (cd "$TUI_DIR" && go build -o bin/skills-tui .)
fi

exec "$BIN" --repo "$REPO_DIR" "$@"
