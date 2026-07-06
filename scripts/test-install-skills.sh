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

  mkdir -p "$home_dir/.agents/skills/autofix" "$home_dir/.claude/skills/autofix" "$home_dir/.cursor/skills/autofix"
  echo "keep me" > "$home_dir/.agents/skills/autofix/local.txt"
  echo "keep me" > "$home_dir/.claude/skills/autofix/local.txt"
  echo "keep me" > "$home_dir/.cursor/skills/autofix/local.txt"

  if HOME="$home_dir" "$REPO_DIR/scripts/install-skills.sh" >"$home_dir/stdout" 2>"$home_dir/stderr"; then
    fail "Expected install without --force to fail when a skill target exists"
  fi

  assert_exists "$home_dir/.agents/skills/autofix/local.txt"
  assert_exists "$home_dir/.claude/skills/autofix/local.txt"
  assert_exists "$home_dir/.cursor/skills/autofix/local.txt"
  assert_not_symlink "$home_dir/.agents/skills/autofix"
  assert_not_symlink "$home_dir/.claude/skills/autofix"
  assert_not_symlink "$home_dir/.cursor/skills/autofix"
}

test_force_overwrites_existing_targets() {
  local home_dir
  home_dir="$(mktemp -d)"
  trap 'rm -rf "$home_dir"' RETURN

  mkdir -p "$home_dir/.agents/skills/autofix" "$home_dir/.claude/skills/autofix" "$home_dir/.cursor/skills/autofix"
  echo "replace me" > "$home_dir/.agents/skills/autofix/local.txt"
  echo "replace me" > "$home_dir/.claude/skills/autofix/local.txt"
  echo "replace me" > "$home_dir/.cursor/skills/autofix/local.txt"

  HOME="$home_dir" "$REPO_DIR/install.sh" --force >"$home_dir/stdout" 2>"$home_dir/stderr"

  assert_exists "$home_dir/.skill-symlinks/skills/autofix/SKILL.md"
  assert_symlink_target "$home_dir/.agents/skills/autofix" "$home_dir/.skill-symlinks/skills/autofix"
  assert_symlink_target "$home_dir/.claude/skills/autofix" "$home_dir/.skill-symlinks/skills/autofix"
  assert_symlink_target "$home_dir/.cursor/skills/autofix" "$home_dir/.skill-symlinks/skills/autofix"
}

test_go_entrypoint_creates_missing_tui_bin_dir() {
  local home_dir bin_dir fake_bin
  home_dir="$(mktemp -d)"

  bin_dir="$REPO_DIR/tools/skills-tui/bin"
  fake_bin="$home_dir/fake-bin"
  trap 'rm -rf "$home_dir"' RETURN

  rm -rf "$bin_dir"
  mkdir -p "$fake_bin"

  cat >"$fake_bin/go" <<'FAKE_GO'
#!/bin/bash
set -euo pipefail

if [[ "$*" != "build -o bin/skills-tui ." ]]; then
  echo "unexpected go invocation: $*" >&2
  exit 2
fi

out=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o)
      out="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

if [[ ! -d "$(dirname "$out")" ]]; then
  echo "missing output parent directory: $(dirname "$out")" >&2
  exit 3
fi

cat >"$out" <<'FAKE_BINARY'
#!/bin/bash
exit 0
FAKE_BINARY
chmod +x "$out"
FAKE_GO
  chmod +x "$fake_bin/go"

  PATH="$fake_bin:$PATH" SKILL_INSTALL_TARGETS=agents HOME="$home_dir" \
    "$REPO_DIR/install.sh" --none >"$home_dir/stdout" 2>"$home_dir/stderr"

  [ -x "$bin_dir/skills-tui" ] \
    || fail "install.sh must build tools/skills-tui/bin/skills-tui when bin/ is missing"

  rm -rf "$bin_dir"
  (cd "$REPO_DIR/tools/skills-tui" && go build -o bin/skills-tui .)
}

test_legacy_installer_migrates_repo_symlink_targets() {
  local home_dir
  home_dir="$(mktemp -d)"
  trap 'rm -rf "$home_dir"' RETURN

  mkdir -p "$home_dir/.agents/skills" "$home_dir/.claude/skills" "$home_dir/.cursor/skills"
  ln -s "$REPO_DIR/skills/autofix" "$home_dir/.agents/skills/autofix"
  ln -s "$REPO_DIR/skills/autofix" "$home_dir/.claude/skills/autofix"
  ln -s "$REPO_DIR/skills/autofix" "$home_dir/.cursor/skills/autofix"

  HOME="$home_dir" "$REPO_DIR/scripts/install-skills.sh" >"$home_dir/stdout" 2>"$home_dir/stderr"

  assert_exists "$home_dir/.skill-symlinks/skills/autofix/SKILL.md"
  assert_symlink_target "$home_dir/.agents/skills/autofix" "$home_dir/.skill-symlinks/skills/autofix"
  assert_symlink_target "$home_dir/.claude/skills/autofix" "$home_dir/.skill-symlinks/skills/autofix"
  assert_symlink_target "$home_dir/.cursor/skills/autofix" "$home_dir/.skill-symlinks/skills/autofix"
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

test_legacy_installer_skips_team_skills_when_claude_excluded() {
  local home_dir
  home_dir="$(mktemp -d)"
  trap 'rm -rf "$home_dir"' RETURN

  SKILL_INSTALL_TARGETS=cursor HOME="$home_dir" "$REPO_DIR/scripts/install-skills.sh" \
    >"$home_dir/stdout" 2>"$home_dir/stderr"

  [ ! -e "$home_dir/.claude/skills/go-review" ] \
    || fail "team skills must not install when claude is excluded"
  assert_symlink_target "$home_dir/.cursor/skills/autofix" "$home_dir/.skill-symlinks/skills/autofix"
}

test_existing_targets_require_force
test_go_entrypoint_creates_missing_tui_bin_dir
test_force_overwrites_existing_targets
test_legacy_installer_migrates_repo_symlink_targets
test_legacy_installer_installs_autofix
test_legacy_installer_skips_team_skills_when_claude_excluded

echo "PASS: install-skills"
