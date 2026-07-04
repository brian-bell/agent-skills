#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_exists() {
  local path="$1"

  [ -e "$path" ] || fail "Expected $path to exist"
}

assert_not_symlink() {
  local path="$1"

  [ ! -L "$path" ] || fail "Expected $path not to be a symlink"
}

assert_symlink_target() {
  local path="$1"
  local target="$2"

  [ -L "$path" ] || fail "Expected $path to be a symlink"
  [ "$(readlink "$path")" = "$target" ] || fail "Expected $path to link to $target"
}

test_existing_targets_require_force() {
  local home_dir
  home_dir="$(mktemp -d)"
  trap 'rm -rf "$home_dir"' RETURN

  mkdir -p "$home_dir/.agents/skills/tdd" "$home_dir/.claude/skills/tdd" "$home_dir/.cursor/skills/tdd"
  echo "keep me" > "$home_dir/.agents/skills/tdd/local.txt"
  echo "keep me" > "$home_dir/.claude/skills/tdd/local.txt"
  echo "keep me" > "$home_dir/.cursor/skills/tdd/local.txt"

  if HOME="$home_dir" "$REPO_DIR/scripts/install-skills.sh" >"$home_dir/stdout" 2>"$home_dir/stderr"; then
    fail "Expected install without --force to fail when a skill target exists"
  fi

  assert_exists "$home_dir/.agents/skills/tdd/local.txt"
  assert_exists "$home_dir/.claude/skills/tdd/local.txt"
  assert_exists "$home_dir/.cursor/skills/tdd/local.txt"
  assert_not_symlink "$home_dir/.agents/skills/tdd"
  assert_not_symlink "$home_dir/.claude/skills/tdd"
  assert_not_symlink "$home_dir/.cursor/skills/tdd"
}

test_force_overwrites_existing_targets() {
  local home_dir
  home_dir="$(mktemp -d)"
  trap 'rm -rf "$home_dir"' RETURN

  mkdir -p "$home_dir/.agents/skills/tdd" "$home_dir/.claude/skills/tdd" "$home_dir/.cursor/skills/tdd"
  echo "replace me" > "$home_dir/.agents/skills/tdd/local.txt"
  echo "replace me" > "$home_dir/.claude/skills/tdd/local.txt"
  echo "replace me" > "$home_dir/.cursor/skills/tdd/local.txt"

  HOME="$home_dir" "$REPO_DIR/install.sh" --force >"$home_dir/stdout" 2>"$home_dir/stderr"

  assert_exists "$home_dir/.skill-symlinks/skills/tdd/SKILL.md"
  assert_symlink_target "$home_dir/.agents/skills/tdd" "$home_dir/.skill-symlinks/skills/tdd"
  assert_symlink_target "$home_dir/.claude/skills/tdd" "$home_dir/.skill-symlinks/skills/tdd"
  assert_symlink_target "$home_dir/.cursor/skills/tdd" "$home_dir/.skill-symlinks/skills/tdd"
}

test_legacy_installer_migrates_repo_symlink_targets() {
  local home_dir
  home_dir="$(mktemp -d)"
  trap 'rm -rf "$home_dir"' RETURN

  mkdir -p "$home_dir/.agents/skills" "$home_dir/.claude/skills" "$home_dir/.cursor/skills"
  ln -s "$REPO_DIR/skills/tdd" "$home_dir/.agents/skills/tdd"
  ln -s "$REPO_DIR/skills/tdd" "$home_dir/.claude/skills/tdd"
  ln -s "$REPO_DIR/skills/tdd" "$home_dir/.cursor/skills/tdd"

  HOME="$home_dir" "$REPO_DIR/scripts/install-skills.sh" >"$home_dir/stdout" 2>"$home_dir/stderr"

  assert_exists "$home_dir/.skill-symlinks/skills/tdd/SKILL.md"
  assert_symlink_target "$home_dir/.agents/skills/tdd" "$home_dir/.skill-symlinks/skills/tdd"
  assert_symlink_target "$home_dir/.claude/skills/tdd" "$home_dir/.skill-symlinks/skills/tdd"
  assert_symlink_target "$home_dir/.cursor/skills/tdd" "$home_dir/.skill-symlinks/skills/tdd"
}

test_legacy_installer_installs_autofix() {
  local home_dir
  home_dir="$(mktemp -d)"
  trap 'rm -rf "$home_dir"' RETURN

  HOME="$home_dir" "$REPO_DIR/scripts/install-skills.sh" >"$home_dir/stdout" 2>"$home_dir/stderr"

  assert_exists "$home_dir/.skill-symlinks/skills/autofix/SKILL.md"
  assert_symlink_target "$home_dir/.agents/skills/autofix" "$home_dir/.skill-symlinks/skills/autofix"
  assert_symlink_target "$home_dir/.claude/skills/autofix" "$home_dir/.skill-symlinks/skills/autofix"
  assert_symlink_target "$home_dir/.cursor/skills/autofix" "$home_dir/.skill-symlinks/skills/autofix"
}

test_existing_targets_require_force
test_force_overwrites_existing_targets
test_legacy_installer_migrates_repo_symlink_targets
test_legacy_installer_installs_autofix

echo "PASS: install-skills"
