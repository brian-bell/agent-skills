#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
TUI="$REPO_DIR/scripts/skills-tui.sh"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

# Build a throwaway repo fixture and echo its path.
make_repo() {
  local dir
  dir="$(mktemp -d)"
  mkdir -p "$dir/skills/commit" "$dir/skills/tdd"
  mkdir -p "$dir/third-party/autoreview"
  echo "stub" > "$dir/third-party/ATTRIBUTION.md"
  mkdir -p "$dir/agent-teams/go-review-team"
  echo "lead" > "$dir/agent-teams/go-review-team/review-lead.md"
  echo "manifest" > "$dir/agent-teams/go-review-team/SKILL.md"
  echo "$dir"
}

# shellcheck source=/dev/null
source "$TUI"

test_discover_lists_first_party() {
  local repo
  repo="$(make_repo)"
  trap 'rm -rf "$repo"' RETURN

  local out
  out="$(discover_skills "$repo")"

  echo "$out" | grep -q "^first	commit	$repo/skills/commit$" \
    || fail "Expected first-party commit in discovery, got: $out"
}

test_discover_lists_third_party_skipping_files() {
  local repo
  repo="$(make_repo)"
  trap 'rm -rf "$repo"' RETURN

  local out
  out="$(discover_skills "$repo")"

  echo "$out" | grep -q "^third	autoreview	$repo/third-party/autoreview$" \
    || fail "Expected third-party autoreview, got: $out"
  if echo "$out" | grep -q "ATTRIBUTION"; then
    fail "Discovery should skip ATTRIBUTION.md, got: $out"
  fi
}

test_discover_lists_team_with_short_name() {
  local repo
  repo="$(make_repo)"
  trap 'rm -rf "$repo"' RETURN

  local out
  out="$(discover_skills "$repo")"

  echo "$out" | grep -q "^team	go-review	$repo/agent-teams/go-review-team$" \
    || fail "Expected team go-review, got: $out"
}

assert_symlink_target() {
  local path="$1" target="$2"
  [ -L "$path" ] || fail "Expected $path to be a symlink"
  [ "$(readlink "$path")" = "$target" ] || fail "Expected $path -> $target, got $(readlink "$path")"
}

test_install_first_party_links_both_roots() {
  local repo home src
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"

  HOME="$home" install_skill first commit "$src"

  assert_symlink_target "$home/.agents/skills/commit" "$src"
  assert_symlink_target "$home/.claude/skills/commit" "$src"
}

test_install_team_links_skill_and_agents() {
  local repo home src
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/agent-teams/go-review-team"

  HOME="$home" install_skill team go-review "$src"

  assert_symlink_target "$home/.claude/skills/go-review" "$src"
  assert_symlink_target "$home/.claude/agents/go-review-team/review-lead.md" "$src/review-lead.md"
  [ ! -e "$home/.agents/skills/go-review" ] || fail "Team skills must not link into ~/.agents"
  [ ! -e "$home/.claude/agents/go-review-team/SKILL.md" ] \
    || fail "SKILL.md is the manifest, not an agent; must not be linked"
}

test_uninstall_removes_owned_links() {
  local repo home src
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/agent-teams/go-review-team"

  HOME="$home" install_skill team go-review "$src"
  HOME="$home" uninstall_skill team go-review "$src"

  [ ! -L "$home/.claude/skills/go-review" ] || fail "Expected go-review link removed"
  [ ! -e "$home/.claude/agents/go-review-team" ] || fail "Expected empty team agent dir pruned"
}

test_uninstall_leaves_real_dir_untouched() {
  local repo home src
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"

  mkdir -p "$home/.claude/skills/commit" "$home/.agents/skills/commit"
  echo "precious" > "$home/.claude/skills/commit/local.txt"

  HOME="$home" uninstall_skill first commit "$src"

  [ -f "$home/.claude/skills/commit/local.txt" ] \
    || fail "Uninstall must not delete a real directory"
}

test_uninstall_leaves_foreign_symlink_untouched() {
  local repo home src elsewhere
  repo="$(make_repo)"; home="$(mktemp -d)"; elsewhere="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home" "$elsewhere"' RETURN
  src="$repo/skills/commit"

  mkdir -p "$home/.claude/skills"
  ln -s "$elsewhere" "$home/.claude/skills/commit"

  HOME="$home" uninstall_skill first commit "$src"

  [ -L "$home/.claude/skills/commit" ] \
    || fail "Uninstall must not remove a symlink pointing elsewhere"
  [ "$(readlink "$home/.claude/skills/commit")" = "$elsewhere" ] \
    || fail "Foreign symlink target changed"
}

assert_state() {
  local want="$1" got="$2"
  [ "$got" = "$want" ] || fail "Expected state '$want', got '$got'"
}

test_state_not_installed() {
  local repo home
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN

  assert_state not-installed \
    "$(HOME="$home" skill_state first commit "$repo/skills/commit")"
}

test_discover_lists_first_party
test_discover_lists_third_party_skipping_files
test_discover_lists_team_with_short_name
test_install_first_party_links_both_roots
test_install_team_links_skill_and_agents
test_uninstall_removes_owned_links
test_uninstall_leaves_real_dir_untouched
test_uninstall_leaves_foreign_symlink_untouched
test_state_installed_when_linked() {
  local repo home src
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"

  HOME="$home" install_skill first commit "$src"
  assert_state installed "$(HOME="$home" skill_state first commit "$src")"
}

test_state_upgrade_when_copy_differs() {
  local repo home src
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"
  echo "v2" > "$src/SKILL.md"

  mkdir -p "$home/.agents/skills/commit" "$home/.claude/skills/commit"
  echo "v1" > "$home/.agents/skills/commit/SKILL.md"
  echo "v1" > "$home/.claude/skills/commit/SKILL.md"

  assert_state upgrade "$(HOME="$home" skill_state first commit "$src")"
}

test_state_installed_when_copy_identical() {
  local repo home src
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"
  echo "same" > "$src/SKILL.md"

  mkdir -p "$home/.agents/skills/commit" "$home/.claude/skills/commit"
  echo "same" > "$home/.agents/skills/commit/SKILL.md"
  echo "same" > "$home/.claude/skills/commit/SKILL.md"

  assert_state installed "$(HOME="$home" skill_state first commit "$src")"
}

test_state_partial_when_one_root_missing() {
  local repo home src
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"

  mkdir -p "$home/.claude/skills"
  ln -s "$src" "$home/.claude/skills/commit"

  assert_state partial "$(HOME="$home" skill_state first commit "$src")"
}

test_force_install_relinks_stale_copy() {
  local repo home src
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"

  mkdir -p "$home/.agents/skills/commit" "$home/.claude/skills/commit"
  echo "old" > "$home/.agents/skills/commit/SKILL.md"
  echo "old" > "$home/.claude/skills/commit/SKILL.md"

  HOME="$home" install_skill first commit "$src" true

  assert_symlink_target "$home/.agents/skills/commit" "$src"
  assert_symlink_target "$home/.claude/skills/commit" "$src"
  assert_state installed "$(HOME="$home" skill_state first commit "$src")"
}

test_install_without_force_keeps_foreign_target() {
  local repo home src
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"

  mkdir -p "$home/.claude/skills/commit"
  echo "mine" > "$home/.claude/skills/commit/SKILL.md"

  if HOME="$home" install_skill first commit "$src" 2>/dev/null; then
    fail "install without force should report failure over a real dir"
  fi
  [ -f "$home/.claude/skills/commit/SKILL.md" ] || fail "real dir clobbered without force"
}

test_plan_action_matrix() {
  local got
  got="$(plan_action not-installed 1)"; assert_state install "$got"
  got="$(plan_action upgrade 1)";       assert_state upgrade "$got"
  got="$(plan_action partial 1)";       assert_state install "$got"
  got="$(plan_action installed 1)";     assert_state none "$got"
  got="$(plan_action installed 0)";     assert_state remove "$got"
  got="$(plan_action upgrade 0)";       assert_state remove "$got"
  got="$(plan_action partial 0)";       assert_state remove "$got"
  got="$(plan_action not-installed 0)"; assert_state none "$got"
}

test_cli_all_then_none_roundtrip() {
  local home
  home="$(mktemp -d)"
  trap 'rm -rf "$home"' RETURN

  HOME="$home" "$TUI" --all >/dev/null 2>&1
  assert_symlink_target "$home/.claude/skills/commit" "$REPO_DIR/skills/commit"
  assert_symlink_target "$home/.agents/skills/commit" "$REPO_DIR/skills/commit"
  assert_symlink_target "$home/.claude/skills/go-review" "$REPO_DIR/agent-teams/go-review-team"

  HOME="$home" "$TUI" --none >/dev/null 2>&1
  [ ! -L "$home/.claude/skills/commit" ] || fail "--none should remove commit link"
  [ ! -L "$home/.claude/skills/go-review" ] || fail "--none should remove go-review link"
}

test_read_key_parses_arrow_sequences() {
  local k
  k="$(printf '\033[A' | read_key)"
  [ "$k" = $'\033[A' ] || fail "up arrow not parsed, got: $(printf '%q' "$k")"
  k="$(printf '\033[B' | read_key)"
  [ "$k" = $'\033[B' ] || fail "down arrow not parsed, got: $(printf '%q' "$k")"
  k="$(printf 'q' | read_key)"
  [ "$k" = q ] || fail "plain key not parsed, got: $(printf '%q' "$k")"
}

test_read_key_parses_arrow_sequences
test_cli_all_then_none_roundtrip
test_plan_action_matrix
test_state_not_installed
test_state_installed_when_linked
test_state_upgrade_when_copy_differs
test_state_installed_when_copy_identical
test_state_partial_when_one_root_missing
test_force_install_relinks_stale_copy
test_install_without_force_keeps_foreign_target

echo "PASS: skills-tui"
