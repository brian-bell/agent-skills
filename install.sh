#!/bin/bash
set -euo pipefail

# Launches the interactive skills manager (install/uninstall TUI).
# Non-interactive flags: --all, --none, --force. See scripts/skills-tui.sh --help.
REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
exec "$REPO_DIR/scripts/skills-tui.sh" "$@"
