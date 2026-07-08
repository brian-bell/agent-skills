#!/bin/bash
#
# test-hooks-install.sh — round-trip the session hooks through the TUI
# installer's non-interactive modes against a temp HOME, using the REAL hook
# install scripts. This is the drift guard between hooks/*/hook.json (which
# drives the Go engine's read-only state detection) and hooks/*/install.sh
# (which owns all writes): any drift makes a hook re-read as upgrade/partial
# and re-act on the second --all.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

if ! command -v jq >/dev/null 2>&1; then
  echo "SKIP: hooks-install (jq not available)"
  exit 0
fi

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_symlink_target() {
  local path="$1" target="$2"
  [ -L "$path" ] || fail "Expected $path to be a symlink"
  [ "$(readlink "$path")" = "$target" ] || fail "Expected $path -> $target, got $(readlink "$path")"
}

HOME_DIR="$(mktemp -d)"
# install.sh may rebuild the Go binary with HOME set to the temp dir, which
# leaves a read-only Go module cache under $HOME_DIR/go/pkg/mod.
trap 'chmod -R u+w "$HOME_DIR" 2>/dev/null || true; rm -rf "$HOME_DIR"' EXIT

CLAUDE_CMD='$HOME/.claude/hooks/save-session.sh'
CODEX_CMD='$HOME/.codex/hooks/save-session.sh'
FOREIGN_CMD='/usr/local/bin/other-hook.sh'

# Plant a foreign hook entry: --none must never touch entries that are not ours.
mkdir -p "$HOME_DIR/.claude"
cat >"$HOME_DIR/.claude/settings.json" <<EOF
{"hooks":{"SessionEnd":[{"matcher":"","hooks":[{"type":"command","command":"$FOREIGN_CMD"}]}]}}
EOF

# 1. --all installs both hooks: staged-pointing symlinks + settings entries.
HOME="$HOME_DIR" "$REPO_DIR/install.sh" --all >"$HOME_DIR/stdout1" 2>"$HOME_DIR/stderr1"

STAGE="$HOME_DIR/.skill-symlinks"
assert_symlink_target "$HOME_DIR/.claude/hooks/save-session.sh" "$STAGE/hooks/save-claude-session/save-session.sh"
assert_symlink_target "$HOME_DIR/.codex/hooks/save-session.sh" "$STAGE/hooks/save-codex-session/save-session.sh"

jq -e --arg cmd "$CLAUDE_CMD" 'any(.hooks.SessionEnd[]?; any(.hooks[]?; .command == $cmd))' \
  "$HOME_DIR/.claude/settings.json" >/dev/null \
  || fail "settings.json missing the SessionEnd entry with literal \$HOME command"
jq -e --arg cmd "$CODEX_CMD" 'any(.hooks.Stop[]?; any(.hooks[]?; .command == $cmd))' \
  "$HOME_DIR/.codex/hooks.json" >/dev/null \
  || fail "hooks.json missing the Stop entry with literal \$HOME command"

# 2. Second --all: both hooks must round-trip as installed — no hook-named
# action line may appear (the manifest-vs-script drift guard).
HOME="$HOME_DIR" "$REPO_DIR/install.sh" --all >"$HOME_DIR/stdout2" 2>"$HOME_DIR/stderr2"
for hook in save-claude-session save-codex-session; do
  if grep -E "(installed|upgraded|removed|blocked|partially) .*$hook|$hook (blocked|partially)" "$HOME_DIR/stdout2" >/dev/null; then
    fail "second --all re-acted on $hook (hook.json drifted from install.sh?): $(cat "$HOME_DIR/stdout2")"
  fi
done

# 3. Legacy migration: a repo-pointing symlink (pre-TUI install) upgrades to
# the staged copy.
ln -sfn "$REPO_DIR/hooks/save-claude-session/save-session.sh" "$HOME_DIR/.claude/hooks/save-session.sh"
HOME="$HOME_DIR" "$REPO_DIR/install.sh" --all >"$HOME_DIR/stdout3" 2>"$HOME_DIR/stderr3"
assert_symlink_target "$HOME_DIR/.claude/hooks/save-session.sh" "$STAGE/hooks/save-claude-session/save-session.sh"

# 4. --none removes our symlinks and entries; the foreign entry survives.
HOME="$HOME_DIR" "$REPO_DIR/install.sh" --none >"$HOME_DIR/stdout4" 2>"$HOME_DIR/stderr4"
[ ! -e "$HOME_DIR/.claude/hooks/save-session.sh" ] || fail "--none left the claude hook symlink"
[ ! -e "$HOME_DIR/.codex/hooks/save-session.sh" ] || fail "--none left the codex hook symlink"
if jq -e --arg cmd "$CLAUDE_CMD" 'any(.hooks.SessionEnd[]?; any(.hooks[]?; .command == $cmd))' \
  "$HOME_DIR/.claude/settings.json" >/dev/null 2>&1; then
  fail "--none left our SessionEnd entry in settings.json"
fi
jq -e --arg cmd "$FOREIGN_CMD" 'any(.hooks.SessionEnd[]?; any(.hooks[]?; .command == $cmd))' \
  "$HOME_DIR/.claude/settings.json" >/dev/null \
  || fail "--none removed a foreign hook entry from settings.json"

echo "PASS: hooks-install"
