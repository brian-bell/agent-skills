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
  echo "commit skill" > "$dir/skills/commit/SKILL.md"
  echo "tdd skill" > "$dir/skills/tdd/SKILL.md"
  mkdir -p "$dir/third-party/autoreview"
  echo "autoreview skill" > "$dir/third-party/autoreview/SKILL.md"
  echo "stub" > "$dir/third-party/ATTRIBUTION.md"
  mkdir -p "$dir/agent-teams/go-review-team"
  echo "lead" > "$dir/agent-teams/go-review-team/review-lead.md"
  echo "manifest" > "$dir/agent-teams/go-review-team/SKILL.md"
  mkdir -p "$dir/agent-teams/feature-review-team"
  echo "acc" > "$dir/agent-teams/feature-review-team/acceptance-lead.md"
  echo "manifest" > "$dir/agent-teams/feature-review-team/SKILL.md"
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
  local repo home src staged
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"
  staged="$home/.skill-symlinks/skills/commit"

  HOME="$home" install_skill first commit "$src"

  [ -f "$staged/SKILL.md" ] || fail "Expected staged skill copy at $staged"
  assert_symlink_target "$home/.agents/skills/commit" "$staged"
  assert_symlink_target "$home/.claude/skills/commit" "$staged"
}

test_install_team_links_skill_and_agents() {
  local repo home src staged
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/agent-teams/go-review-team"
  staged="$home/.skill-symlinks/agent-teams/go-review-team"

  HOME="$home" install_skill team go-review "$src"

  [ -f "$staged/review-lead.md" ] || fail "Expected staged team copy at $staged"
  assert_symlink_target "$home/.claude/skills/go-review" "$staged"
  assert_symlink_target "$home/.claude/agents/go-review-team/review-lead.md" "$staged/review-lead.md"
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

  HOME="$home" install_skill first commit "$src"
  rm -f "$home/.agents/skills/commit"

  assert_state partial "$(HOME="$home" skill_state first commit "$src")"
}

test_force_install_relinks_stale_copy() {
  local repo home src staged
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"
  staged="$home/.skill-symlinks/skills/commit"

  mkdir -p "$home/.agents/skills/commit" "$home/.claude/skills/commit"
  echo "old" > "$home/.agents/skills/commit/SKILL.md"
  echo "old" > "$home/.claude/skills/commit/SKILL.md"

  # force + destroy required to overwrite a real directory.
  HOME="$home" install_skill first commit "$src" true true

  assert_symlink_target "$home/.agents/skills/commit" "$staged"
  assert_symlink_target "$home/.claude/skills/commit" "$staged"
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
  assert_symlink_target "$home/.claude/skills/commit" "$home/.skill-symlinks/skills/commit"
  assert_symlink_target "$home/.agents/skills/commit" "$home/.skill-symlinks/skills/commit"
  assert_symlink_target "$home/.claude/skills/go-review" "$home/.skill-symlinks/agent-teams/go-review-team"

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
# C1: uninstalling the last skill must not delete the shared skills roots.
test_uninstall_last_skill_keeps_shared_roots() {
  local repo home src
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"

  HOME="$home" install_skill first commit "$src"
  HOME="$home" uninstall_skill first commit "$src"

  [ -d "$home/.claude/skills" ] || fail "uninstall removed shared ~/.claude/skills root"
  [ -d "$home/.agents/skills" ] || fail "uninstall removed shared ~/.agents/skills root"
  [ ! -L "$home/.claude/skills/commit" ] || fail "commit link not removed"
}

# C2: an interactive apply (no --force/destroy) must NOT rm -rf a real dir.
test_apply_upgrade_keeps_real_dir_without_force() {
  local repo home src
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"
  echo "v2" > "$src/SKILL.md"

  mkdir -p "$home/.agents/skills/commit" "$home/.claude/skills/commit"
  echo "v1" > "$home/.agents/skills/commit/SKILL.md"
  echo "private" > "$home/.claude/skills/commit/NOTES.md"

  assert_state upgrade "$(HOME="$home" skill_state first commit "$src")"
  # desired=1, destroy=false (interactive apply): must preserve the real dir.
  HOME="$home" apply_skill first commit "$src" 1 false >/dev/null

  [ -f "$home/.claude/skills/commit/NOTES.md" ] \
    || fail "interactive apply destroyed a real user directory (data loss)"

  # With destroy=true (--force) it relinks.
  HOME="$home" apply_skill first commit "$src" 1 true >/dev/null
  assert_state installed "$(HOME="$home" skill_state first commit "$src")"
}

# I1: a foreign symlink differs from the repo -> upgrade; relinking it under
# force is non-destructive (the data it pointed at survives).
test_foreign_symlink_upgrade_is_nondestructive() {
  local repo home src staged elsewhere
  repo="$(make_repo)"; home="$(mktemp -d)"; elsewhere="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home" "$elsewhere"' RETURN
  src="$repo/skills/commit"
  staged="$home/.skill-symlinks/skills/commit"
  echo "keep" > "$elsewhere/data.txt"

  mkdir -p "$home/.agents/skills" "$home/.claude/skills"
  ln -s "$elsewhere" "$home/.agents/skills/commit"
  ln -s "$elsewhere" "$home/.claude/skills/commit"

  assert_state upgrade "$(HOME="$home" skill_state first commit "$src")"
  # Interactive apply (destroy=false) may relink a symlink (non-destructive).
  HOME="$home" apply_skill first commit "$src" 1 false >/dev/null

  assert_symlink_target "$home/.claude/skills/commit" "$staged"
  [ -f "$elsewhere/data.txt" ] || fail "relinking a foreign symlink destroyed its data"
}

# I2: feature-review-team must be discovered, installed, and SKILL.md excluded.
test_feature_review_team_discovered_and_installed() {
  local repo home src staged
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/agent-teams/feature-review-team"
  staged="$home/.skill-symlinks/agent-teams/feature-review-team"

  echo "$(discover_skills "$repo")" \
    | grep -q "^team	feature-review	$src$" \
    || fail "feature-review not discovered"

  HOME="$home" install_skill team feature-review "$src"
  assert_symlink_target "$home/.claude/skills/feature-review" "$staged"
  assert_symlink_target "$home/.claude/agents/feature-review-team/acceptance-lead.md" "$staged/acceptance-lead.md"
  [ ! -e "$home/.claude/agents/feature-review-team/SKILL.md" ] \
    || fail "feature-review SKILL.md must not be linked as an agent"
}

# Partial install: a real matching dir on one root, missing on the other.
# Apply must link the missing root but never destroy the real dir.
test_apply_partial_links_missing_keeps_real_dir() {
  local repo home src staged
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"
  staged="$home/.skill-symlinks/skills/commit"
  echo "same" > "$src/SKILL.md"

  # claude root: real dir with matching content + a private file; agents: missing
  mkdir -p "$home/.claude/skills/commit"
  echo "same" > "$home/.claude/skills/commit/SKILL.md"
  echo "private" > "$home/.claude/skills/commit/NOTES.md"

  HOME="$home" apply_skill first commit "$src" 1 false >/dev/null

  assert_symlink_target "$home/.agents/skills/commit" "$staged"
  [ -f "$home/.claude/skills/commit/NOTES.md" ] \
    || fail "partial install destroyed the real dir on the other root"
  [ ! -L "$home/.claude/skills/commit" ] \
    || fail "partial install overwrote a real dir without --force"
}

test_existing_repo_symlinks_migrate_to_staged_symlinks() {
  local repo home src staged
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"
  staged="$home/.skill-symlinks/skills/commit"

  mkdir -p "$home/.agents/skills" "$home/.claude/skills"
  ln -s "$src" "$home/.agents/skills/commit"
  ln -s "$src" "$home/.claude/skills/commit"

  assert_state upgrade "$(HOME="$home" skill_state first commit "$src")"
  HOME="$home" apply_skill first commit "$src" 1 false >/dev/null

  assert_symlink_target "$home/.agents/skills/commit" "$staged"
  assert_symlink_target "$home/.claude/skills/commit" "$staged"
  [ -f "$staged/SKILL.md" ] || fail "migration did not create staged copy"
}

test_installed_skill_survives_repo_source_removal() {
  local repo home src staged
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"
  staged="$home/.skill-symlinks/skills/commit"

  HOME="$home" install_skill first commit "$src"
  rm -rf "$src"

  assert_symlink_target "$home/.claude/skills/commit" "$staged"
  [ -f "$home/.claude/skills/commit/SKILL.md" ] \
    || fail "installed skill should still resolve through staged copy"
}

test_apply_upgrade_refreshes_staged_copy() {
  local repo home src staged
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"
  staged="$home/.skill-symlinks/skills/commit"

  HOME="$home" install_skill first commit "$src"
  echo "updated skill" > "$src/SKILL.md"

  assert_state upgrade "$(HOME="$home" skill_state first commit "$src")"
  HOME="$home" apply_skill first commit "$src" 1 false >/dev/null

  assert_state installed "$(HOME="$home" skill_state first commit "$src")"
  grep -q "updated skill" "$staged/SKILL.md" \
    || fail "upgrade did not refresh staged copy"
}

test_apply_partial_links_missing_keeps_real_dir
test_existing_repo_symlinks_migrate_to_staged_symlinks
test_installed_skill_survives_repo_source_removal
test_apply_upgrade_refreshes_staged_copy
test_state_not_installed
test_state_installed_when_linked
test_state_upgrade_when_copy_differs
test_state_installed_when_copy_identical
test_state_partial_when_one_root_missing
test_force_install_relinks_stale_copy
test_install_without_force_keeps_foreign_target
test_uninstall_last_skill_keeps_shared_roots
test_apply_upgrade_keeps_real_dir_without_force
test_foreign_symlink_upgrade_is_nondestructive
test_feature_review_team_discovered_and_installed

echo "PASS: skills-tui"
