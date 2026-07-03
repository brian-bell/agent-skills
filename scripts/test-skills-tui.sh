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
  mkdir -p "$dir/agent-teams/hybrid-review-team/agents"
  echo "lead" > "$dir/agent-teams/hybrid-review-team/hybrid-lead.md"
  echo "manifest" > "$dir/agent-teams/hybrid-review-team/SKILL.md"
  echo "interface:" > "$dir/agent-teams/hybrid-review-team/agents/openai.yaml"
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

test_discover_lists_hybrid_team_when_codex_metadata_exists() {
  local repo
  repo="$(make_repo)"
  trap 'rm -rf "$repo"' RETURN

  local out
  out="$(discover_skills "$repo")"

  echo "$out" | grep -q "^team-hybrid	hybrid-review	$repo/agent-teams/hybrid-review-team$" \
    || fail "Expected hybrid team hybrid-review, got: $out"
}

test_repo_go_review_is_hybrid_feature_review_stays_claude_only() {
  local out
  out="$(discover_skills "$REPO_DIR")"

  echo "$out" | grep -q "^team-hybrid	go-review	$REPO_DIR/agent-teams/go-review-team$" \
    || fail "Expected repo go-review team to be Codex-compatible, got: $out"
  echo "$out" | grep -q "^team	feature-review	$REPO_DIR/agent-teams/feature-review-team$" \
    || fail "Expected repo feature-review team to remain Claude-only, got: $out"
}

test_go_review_skill_declares_codex_workflow() {
  local file codex_block reviewer
  file="$REPO_DIR/agent-teams/go-review-team/SKILL.md"

  grep -q "Platform — Claude Code" "$file" \
    || fail "go-review SKILL.md should keep a Claude Code platform block"
  grep -q "Platform — Codex" "$file" \
    || fail "go-review SKILL.md should include a Codex platform block"

  codex_block="$(awk '
    /^## Platform — Codex/ { in_block = 1 }
    /^## Platform — / && in_block && $0 !~ /^## Platform — Codex/ { in_block = 0 }
    in_block { print }
  ' "$file")"

  [ -n "$codex_block" ] || fail "Codex platform block should not be empty"
  for reviewer in structure-reviewer error-reviewer style-reviewer security-reviewer; do
    printf '%s\n' "$codex_block" | grep -q "$reviewer.md" \
      || fail "Codex platform block should reference $reviewer.md"
  done

  if printf '%s\n' "$codex_block" | grep -Eq "TeamCreate|TaskCreate|SendMessage|subagent_type"; then
    fail "Codex platform block should not depend on Claude-only team/subagent primitives"
  fi
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

test_install_hybrid_team_links_agents_skill_and_claude_agents() {
  local repo home src staged
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/agent-teams/hybrid-review-team"
  staged="$home/.skill-symlinks/agent-teams/hybrid-review-team"

  HOME="$home" install_skill team-hybrid hybrid-review "$src"

  [ -f "$staged/hybrid-lead.md" ] || fail "Expected staged hybrid team copy at $staged"
  [ -f "$staged/agents/openai.yaml" ] || fail "Expected Codex metadata in staged hybrid team copy"
  assert_symlink_target "$home/.agents/skills/hybrid-review" "$staged"
  assert_symlink_target "$home/.claude/skills/hybrid-review" "$staged"
  assert_symlink_target "$home/.claude/agents/hybrid-review-team/hybrid-lead.md" "$staged/hybrid-lead.md"
  [ ! -e "$home/.claude/agents/hybrid-review-team/SKILL.md" ] \
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

test_path_mode_handles_gnu_stat_without_filesystem_output() {
  local tmp bin file out
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' RETURN
  bin="$tmp/bin"
  file="$tmp/file"
  mkdir -p "$bin"
  echo "data" > "$file"
  chmod 755 "$file"
  cat > "$bin/stat" <<'EOF'
#!/bin/sh
if [ "$1" = "-c" ] && [ "$2" = "%a" ]; then
  printf '755\n'
  exit 0
fi
if [ "$1" = "-f" ]; then
  printf '  File: "%s"\n' "$3"
  exit 1
fi
exit 1
EOF
  chmod +x "$bin/stat"

  out="$(PATH="$bin:$PATH" path_mode "$file")"

  [ "$out" = "755" ] || fail "GNU stat fallback should print only the mode, got: $out"
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
test_discover_lists_hybrid_team_when_codex_metadata_exists
test_repo_go_review_is_hybrid_feature_review_stays_claude_only
test_go_review_skill_declares_codex_workflow
test_install_first_party_links_both_roots
test_install_team_links_skill_and_agents
test_install_hybrid_team_links_agents_skill_and_claude_agents
test_uninstall_removes_owned_links
test_uninstall_leaves_real_dir_untouched
test_uninstall_leaves_foreign_symlink_untouched
test_path_mode_handles_gnu_stat_without_filesystem_output
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
  got="$(plan_action upgrade -)";       assert_state none "$got"
  got="$(plan_action partial 1)";       assert_state install "$got"
  got="$(plan_action installed 1)";     assert_state none "$got"
  got="$(plan_action installed 0)";     assert_state remove "$got"
  got="$(plan_action upgrade 0)";       assert_state remove "$got"
  got="$(plan_action partial 0)";       assert_state remove "$got"
  got="$(plan_action not-installed 0)"; assert_state none "$got"
}

test_upgrade_rows_default_to_update_and_can_be_held() {
  local out
  TNAME=(commit)
  TKIND=(first)
  TDESIRED=(1)
  TSTATE=(upgrade)

  out="$(TERM_ROWS=8 render 0)"
  printf '%s' "$out" | grep -q "\\[x\\].*will be updated" \
    || fail "selected upgrade should render as checked and will be updated"

  toggle_desired 0

  [ "${TDESIRED[0]}" = - ] || fail "space toggle should hold an upgradeable skill, got ${TDESIRED[0]}"
  [ "$(plan_action upgrade "${TDESIRED[0]}")" = none ] \
    || fail "held upgrade should not apply any action"

  out="$(TERM_ROWS=8 render 0)"
  printf '%s' "$out" | grep -q "\\[-\\].*upgrade available" \
    || fail "held upgrade should render with '-' and upgrade available"
}

test_installed_rows_selected_for_uninstall_show_removed_label() {
  local out
  TNAME=(commit)
  TKIND=(first)
  TDESIRED=(0)
  TSTATE=(installed)

  out="$(TERM_ROWS=8 render 0)"

  printf '%s' "$out" | grep -q "\\[ \\].*will be removed" \
    || fail "installed skill selected for uninstall should render as will be removed"
}

test_refresh_states_selects_upgrades_by_default() {
  local repo home src
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"

  HOME="$home" install_skill first commit "$src"
  echo "updated skill" > "$src/SKILL.md"

  TNAME=(commit)
  TKIND=(first)
  TSRC=("$src")
  TDESIRED=(0)
  TSTATE=("")

  HOME="$home" refresh_states

  [ "${TSTATE[0]}" = upgrade ] || fail "expected refresh to mark changed staged copy as upgrade"
  [ "${TDESIRED[0]}" = 1 ] || fail "upgrade should be selected by default"
}

test_upgrade_rows_default_to_update_and_can_be_held
test_installed_rows_selected_for_uninstall_show_removed_label
test_refresh_states_selects_upgrades_by_default
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

test_render_oversized_list_uses_viewport_without_full_clear() {
  local out lines
  TNAME=(one two three four five six seven eight nine ten)
  TKIND=(first first first first first first first first first first)
  TDESIRED=(0 0 0 0 0 0 0 0 0 0)
  TSTATE=(not-installed not-installed not-installed not-installed not-installed not-installed not-installed not-installed not-installed not-installed)

  out="$(TERM_ROWS=8 render 6)"

  case "$out" in
    *"$ESC[2J"*) fail "oversized render should not full-clear on cursor movement" ;;
  esac

  lines="$(printf '%s' "$out" | awk 'END { print NR }')"
  [ "$lines" -le 8 ] || fail "expected oversized render to fit in 8 rows, got $lines"
  printf '%s' "$out" | grep -q "seven" || fail "selected item should stay visible"
}

test_render_oversized_list_uses_viewport_without_full_clear
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

test_uninstall_removes_existing_repo_symlinks() {
  local repo home src
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"

  mkdir -p "$home/.agents/skills" "$home/.claude/skills"
  ln -s "$src" "$home/.agents/skills/commit"
  ln -s "$src" "$home/.claude/skills/commit"

  assert_state upgrade "$(HOME="$home" skill_state first commit "$src")"
  HOME="$home" uninstall_skill first commit "$src"

  [ ! -L "$home/.agents/skills/commit" ] \
    || fail "uninstall left legacy repo symlink in ~/.agents"
  [ ! -L "$home/.claude/skills/commit" ] \
    || fail "uninstall left legacy repo symlink in ~/.claude"
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
  local repo home src staged backup
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
  backup="$(find "$home/.skill-symlinks/backups/skills/commit" -mindepth 1 -maxdepth 1 -type d -print 2>/dev/null | head -1 || true)"
  [ -n "$backup" ] || fail "upgrade did not create a staged skill backup"
  grep -q "commit skill" "$backup/SKILL.md" \
    || fail "backup did not preserve the previous staged skill"
  if grep -q "updated skill" "$backup/SKILL.md"; then
    fail "backup contains upgraded content instead of previous staged content"
  fi
}

test_chmod_only_repo_update_marks_staged_copy_upgrade() {
  local repo home src staged
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"
  staged="$home/.skill-symlinks/skills/commit"

  echo 'echo helper' > "$src/helper.sh"
  chmod 644 "$src/helper.sh"
  HOME="$home" install_skill first commit "$src"

  chmod 755 "$src/helper.sh"

  assert_state upgrade "$(HOME="$home" skill_state first commit "$src")"
  HOME="$home" apply_skill first commit "$src" 1 false >/dev/null
  [ -x "$staged/helper.sh" ] || fail "upgrade did not refresh helper executable bit"
}

test_cp_fallback_preserves_staged_root_permissions() {
  local repo home src staged bin cmd
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"
  staged="$home/.skill-symlinks/skills/commit"
  bin="$home/bin"
  mkdir -p "$bin"
  for cmd in chmod cp dirname ln mkdir mv rm stat; do
    ln -s "$(command -v "$cmd")" "$bin/$cmd"
  done
  chmod 700 "$src"

  (
    PATH="$bin"
    hash -r
    HOME="$home" install_skill first commit "$src"
  )

  [ "$(path_mode "$staged")" = "$(path_mode "$src")" ] \
    || fail "staged root mode should match source root mode"
}

test_staged_root_permission_drift_marks_upgrade() {
  local repo home src staged
  repo="$(make_repo)"; home="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home"' RETURN
  src="$repo/skills/commit"
  staged="$home/.skill-symlinks/skills/commit"
  chmod 700 "$src"
  HOME="$home" install_skill first commit "$src"
  chmod 755 "$staged"

  assert_state upgrade "$(HOME="$home" skill_state first commit "$src")"
}

test_refresh_replaces_staged_symlink_without_touching_target() {
  local repo home src staged elsewhere
  repo="$(make_repo)"; home="$(mktemp -d)"; elsewhere="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home" "$elsewhere"' RETURN
  src="$repo/skills/commit"
  staged="$home/.skill-symlinks/skills/commit"

  mkdir -p "$(dirname "$staged")"
  echo "external" > "$elsewhere/keep.txt"
  ln -s "$elsewhere" "$staged"

  HOME="$home" install_skill first commit "$src"

  [ -f "$elsewhere/keep.txt" ] || fail "refresh followed staged symlink and mutated its target"
  [ ! -L "$staged" ] || fail "refresh left staged path as a symlink"
  [ -f "$staged/SKILL.md" ] || fail "refresh did not create a real staged skill copy"
}

test_refresh_of_staged_symlink_does_not_backup_external_target() {
  local repo home src staged elsewhere backup_root copied
  repo="$(make_repo)"; home="$(mktemp -d)"; elsewhere="$(mktemp -d)"
  trap 'rm -rf "$repo" "$home" "$elsewhere"' RETURN
  src="$repo/skills/commit"
  staged="$home/.skill-symlinks/skills/commit"
  backup_root="$home/.skill-symlinks/backups/skills/commit"

  mkdir -p "$(dirname "$staged")" "$elsewhere/private"
  echo "private" > "$elsewhere/private/secret.txt"
  ln -s "$elsewhere" "$staged"

  HOME="$home" install_skill first commit "$src"

  copied="$(find "$backup_root" -name secret.txt -print 2>/dev/null | head -1 || true)"
  [ -z "$copied" ] || fail "refresh copied staged symlink target into backup: $copied"
}

test_apply_partial_links_missing_keeps_real_dir
test_existing_repo_symlinks_migrate_to_staged_symlinks
test_uninstall_removes_existing_repo_symlinks
test_installed_skill_survives_repo_source_removal
test_apply_upgrade_refreshes_staged_copy
test_chmod_only_repo_update_marks_staged_copy_upgrade
test_cp_fallback_preserves_staged_root_permissions
test_staged_root_permission_drift_marks_upgrade
test_refresh_replaces_staged_symlink_without_touching_target
test_refresh_of_staged_symlink_does_not_backup_external_target
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
