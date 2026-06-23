#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
HOOK_DIR="$REPO_DIR/hooks/save-codex-session"
SAVE_SCRIPT="$HOOK_DIR/save-session.sh"
INSTALL_SCRIPT="$HOOK_DIR/install.sh"
BACKFILL_SCRIPT="$HOOK_DIR/backfill.sh"
VALIDATE_SCRIPT="$HOOK_DIR/validate-archives.sh"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_file() {
  local path="$1"
  [ -f "$path" ] || fail "Expected file: $path"
}

assert_eq() {
  local want="$1" got="$2"
  [ "$got" = "$want" ] || fail "Expected '$want', got '$got'"
}

assert_json_eq() {
  local path="$1" expr="$2" want="$3" got
  got="$(jq -r "$expr" "$path")"
  assert_eq "$want" "$got"
}

make_transcript() {
  local codex_home="$1" sid="$2" cwd="$3" line_count="${4:-3}"
  local dir="$codex_home/sessions/2026/06/19"
  local path="$dir/rollout-2026-06-19T00-00-00-$sid.jsonl"
  mkdir -p "$dir"
  printf '{"timestamp":"2026-06-19T00:00:00.000Z","type":"session_meta","payload":{"id":"%s","timestamp":"2026-06-19T00:00:00.000Z","cwd":"%s","originator":"codex-tui","cli_version":"0.140.0","source":"cli","thread_source":"user","model_provider":"openai"}}\n' "$sid" "$cwd" >"$path"
  local i=2
  while [ "$i" -le "$line_count" ]; do
    printf '{"timestamp":"2026-06-19T00:00:0%s.000Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[]}}\n' "$i" >>"$path"
    i=$((i + 1))
  done
  printf '%s\n' "$path"
}

run_save() {
  local home_dir="$1" codex_home="$2" archive_root="$3" payload="$4"
  HOME="$home_dir" CODEX_HOME="$codex_home" CODEX_SESSION_ARCHIVE_DIR="$archive_root" \
    "$SAVE_SCRIPT" <<<"$payload"
}

test_stop_hook_archives_transcript_and_metadata() {
  local home_dir codex_home archive_root sid cwd transcript dest metadata
  home_dir="$(mktemp -d)"; trap 'rm -rf "$home_dir"' RETURN
  codex_home="$home_dir/.codex"
  archive_root="$home_dir/.agent-sessions/codex"
  sid="00000000-0000-0000-0000-000000000001"
  cwd="/tmp/project one"
  transcript="$(make_transcript "$codex_home" "$sid" "$cwd" 3)"

  run_save "$home_dir" "$codex_home" "$archive_root" \
    "{\"hook_event_name\":\"Stop\",\"session_id\":\"$sid\",\"transcript_path\":\"$transcript\",\"cwd\":\"$cwd\"}"

  dest="$archive_root/$sid"
  metadata="$dest/metadata.json"
  assert_file "$dest/transcript.jsonl"
  assert_file "$metadata"
  cmp "$transcript" "$dest/transcript.jsonl" >/dev/null || fail "Archived transcript differs from source"
  assert_json_eq "$metadata" '.source' "hook"
  assert_json_eq "$metadata" '.transcript_copied' "true"
  assert_json_eq "$metadata" '.transcript_lines' "3"
  assert_json_eq "$metadata" '.hook_payload.hook_event_name' "Stop"
  assert_json_eq "$metadata" '.session.session_id' "$sid"
  assert_json_eq "$metadata" '.session.cwd' "$cwd"
  assert_json_eq "$metadata" '.session.cli_version' "0.140.0"
}

test_stop_hook_finds_transcript_by_session_id() {
  local home_dir codex_home archive_root sid cwd transcript dest
  home_dir="$(mktemp -d)"; trap 'rm -rf "$home_dir"' RETURN
  codex_home="$home_dir/.codex"
  archive_root="$home_dir/.agent-sessions/codex"
  sid="00000000-0000-0000-0000-000000000002"
  cwd="/tmp/project-two"
  transcript="$(make_transcript "$codex_home" "$sid" "$cwd" 2)"

  run_save "$home_dir" "$codex_home" "$archive_root" \
    "{\"hook_event_name\":\"Stop\",\"session_id\":\"$sid\",\"cwd\":\"$cwd\"}"

  dest="$archive_root/$sid"
  assert_file "$dest/transcript.jsonl"
  cmp "$transcript" "$dest/transcript.jsonl" >/dev/null || fail "Expected hook to find transcript by session id"
}

test_stop_hook_keeps_existing_longer_archive() {
  local home_dir codex_home archive_root sid cwd short_transcript dest metadata lines
  home_dir="$(mktemp -d)"; trap 'rm -rf "$home_dir"' RETURN
  codex_home="$home_dir/.codex"
  archive_root="$home_dir/.agent-sessions/codex"
  sid="00000000-0000-0000-0000-000000000003"
  cwd="/tmp/project-three"
  short_transcript="$(make_transcript "$codex_home" "$sid" "$cwd" 1)"
  dest="$archive_root/$sid"
  mkdir -p "$dest"
  printf 'old-1\nold-2\nold-3\n' >"$dest/transcript.jsonl"

  run_save "$home_dir" "$codex_home" "$archive_root" \
    "{\"hook_event_name\":\"Stop\",\"session_id\":\"$sid\",\"transcript_path\":\"$short_transcript\",\"cwd\":\"$cwd\"}"

  lines="$(wc -l <"$dest/transcript.jsonl" | tr -d ' ')"
  assert_eq "3" "$lines"
  metadata="$dest/metadata.json"
  assert_json_eq "$metadata" '.transcript_copied' "false"
  assert_json_eq "$metadata" '.transcript_lines' "3"
}

test_stop_hook_adopts_transcript_id_when_payload_id_mismatches() {
  local home_dir codex_home archive_root payload_sid real_sid cwd transcript dest metadata tid
  home_dir="$(mktemp -d)"; trap 'rm -rf "$home_dir"' RETURN
  codex_home="$home_dir/.codex"
  archive_root="$home_dir/.agent-sessions/codex"
  payload_sid="00000000-0000-0000-0000-0000000000aa"
  real_sid="00000000-0000-0000-0000-0000000000bb"
  cwd="/tmp/project-mismatch"
  transcript="$(make_transcript "$codex_home" "$real_sid" "$cwd" 3)"

  # Payload claims session A with no transcript_path; only a transcript for
  # session B exists, discoverable via the cwd fallback. The archive must
  # follow the transcript's own identity, not the payload's claimed id.
  run_save "$home_dir" "$codex_home" "$archive_root" \
    "{\"hook_event_name\":\"Stop\",\"session_id\":\"$payload_sid\",\"cwd\":\"$cwd\"}"

  [ ! -f "$archive_root/$payload_sid/transcript.jsonl" ] \
    || fail "Created misleading archive at payload id containing fallback transcript"

  dest="$archive_root/$real_sid"
  metadata="$dest/metadata.json"
  assert_file "$dest/transcript.jsonl"
  cmp "$transcript" "$dest/transcript.jsonl" >/dev/null || fail "Archived transcript differs from source"
  assert_json_eq "$metadata" '.session.session_id' "$real_sid"
  tid="$(jq -r 'select(.type=="session_meta")|.payload.id' "$dest/transcript.jsonl" | head -1)"
  assert_eq "$real_sid" "$tid"
}

test_stop_hook_adopts_transcript_id_for_explicit_path_mismatch() {
  local home_dir codex_home archive_root payload_sid real_sid cwd transcript dest metadata tid
  home_dir="$(mktemp -d)"; trap 'rm -rf "$home_dir"' RETURN
  codex_home="$home_dir/.codex"
  archive_root="$home_dir/.agent-sessions/codex"
  payload_sid="00000000-0000-0000-0000-0000000000cc"
  real_sid="00000000-0000-0000-0000-0000000000dd"
  cwd="/tmp/project-explicit"
  transcript="$(make_transcript "$codex_home" "$real_sid" "$cwd" 3)"

  # Payload points transcript_path at a file whose internal id is B while
  # claiming session A. The transcript's own id must win.
  run_save "$home_dir" "$codex_home" "$archive_root" \
    "{\"hook_event_name\":\"Stop\",\"session_id\":\"$payload_sid\",\"transcript_path\":\"$transcript\",\"cwd\":\"$cwd\"}"

  [ ! -f "$archive_root/$payload_sid/transcript.jsonl" ] \
    || fail "Created misleading archive at payload id for explicit transcript_path"

  dest="$archive_root/$real_sid"
  metadata="$dest/metadata.json"
  assert_file "$dest/transcript.jsonl"
  assert_json_eq "$metadata" '.session.session_id' "$real_sid"
  tid="$(jq -r 'select(.type=="session_meta")|.payload.id' "$dest/transcript.jsonl" | head -1)"
  assert_eq "$real_sid" "$tid"
}

test_installer_manages_hooks_json_idempotently() {
  local home_dir target hooks_json
  home_dir="$(mktemp -d)"; trap 'rm -rf "$home_dir"' RETURN
  target="$home_dir/.codex/hooks/save-session.sh"
  hooks_json="$home_dir/.codex/hooks.json"

  HOME="$home_dir" "$INSTALL_SCRIPT" >/dev/null
  HOME="$home_dir" "$INSTALL_SCRIPT" >/dev/null

  [ -L "$target" ] || fail "Expected hook script symlink"
  assert_eq "$HOOK_DIR/save-session.sh" "$(readlink "$target")"
  assert_json_eq "$hooks_json" '[.hooks.Stop[]?.hooks[]? | select(.command == "$HOME/.codex/hooks/save-session.sh")] | length' "1"

  HOME="$home_dir" "$INSTALL_SCRIPT" --uninstall >/dev/null
  [ ! -L "$target" ] || fail "Expected uninstall to remove owned symlink"
  assert_json_eq "$hooks_json" '[.hooks.Stop[]?.hooks[]? | select(.command == "$HOME/.codex/hooks/save-session.sh")] | length' "0"
}

test_backfill_imports_existing_sessions() {
  local home_dir codex_home archive_root sid cwd transcript metadata
  home_dir="$(mktemp -d)"; trap 'rm -rf "$home_dir"' RETURN
  codex_home="$home_dir/.codex"
  archive_root="$home_dir/.agent-sessions/codex"
  sid="00000000-0000-0000-0000-000000000004"
  cwd="/tmp/project-four"
  transcript="$(make_transcript "$codex_home" "$sid" "$cwd" 4)"

  HOME="$home_dir" CODEX_HOME="$codex_home" CODEX_SESSION_ARCHIVE_DIR="$archive_root" \
    "$BACKFILL_SCRIPT" --quiet >/dev/null

  assert_file "$archive_root/$sid/transcript.jsonl"
  cmp "$transcript" "$archive_root/$sid/transcript.jsonl" >/dev/null || fail "Backfilled transcript differs from source"
  metadata="$archive_root/$sid/metadata.json"
  assert_json_eq "$metadata" '.source' "backfill"
  assert_json_eq "$metadata" '.transcript_lines' "4"
  assert_json_eq "$metadata" '.session.session_id' "$sid"
}

make_archive() {
  # make_archive <archive_root> <dir_id> <metadata_id> <transcript_id>
  local archive_root="$1" dir_id="$2" metadata_id="$3" transcript_id="$4"
  local dest="$archive_root/$dir_id"
  mkdir -p "$dest"
  printf '{"timestamp":"2026-06-19T00:00:00.000Z","type":"session_meta","payload":{"id":"%s","cwd":"/tmp/x"}}\n' \
    "$transcript_id" >"$dest/transcript.jsonl"
  jq -n --arg sid "$metadata_id" '{session: {session_id: $sid}}' >"$dest/metadata.json"
}

test_validate_archives_flags_id_mismatch() {
  local home_dir archive_root output status
  home_dir="$(mktemp -d)"; trap 'rm -rf "$home_dir"' RETURN
  archive_root="$home_dir/.agent-sessions/codex"
  make_archive "$archive_root" "aaaaaaaa-0000-0000-0000-000000000001" \
    "aaaaaaaa-0000-0000-0000-000000000001" "aaaaaaaa-0000-0000-0000-000000000001"
  make_archive "$archive_root" "bbbbbbbb-0000-0000-0000-000000000002" \
    "bbbbbbbb-0000-0000-0000-000000000002" "cccccccc-0000-0000-0000-000000000003"

  set +e
  output="$(CODEX_SESSION_ARCHIVE_DIR="$archive_root" "$VALIDATE_SCRIPT" 2>&1)"
  status=$?
  set -e

  [ "$status" -ne 0 ] || fail "Expected non-zero exit when a mismatch exists"
  printf '%s' "$output" | grep -q "bbbbbbbb-0000-0000-0000-000000000002" \
    || fail "Expected mismatch report to name the bad archive dir"
  printf '%s' "$output" | grep -q "cccccccc-0000-0000-0000-000000000003" \
    || fail "Expected mismatch report to include the divergent transcript id"
}

test_validate_archives_passes_when_clean() {
  local home_dir archive_root status
  home_dir="$(mktemp -d)"; trap 'rm -rf "$home_dir"' RETURN
  archive_root="$home_dir/.agent-sessions/codex"
  make_archive "$archive_root" "aaaaaaaa-0000-0000-0000-000000000001" \
    "aaaaaaaa-0000-0000-0000-000000000001" "aaaaaaaa-0000-0000-0000-000000000001"

  set +e
  CODEX_SESSION_ARCHIVE_DIR="$archive_root" "$VALIDATE_SCRIPT" >/dev/null 2>&1
  status=$?
  set -e
  [ "$status" -eq 0 ] || fail "Expected zero exit when all archives agree"
}

test_validate_archives_skips_incomplete_archives() {
  local home_dir archive_root status output dir
  home_dir="$(mktemp -d)"; trap 'rm -rf "$home_dir"' RETURN
  archive_root="$home_dir/.agent-sessions/codex"

  # Transcript has no session_meta line (empty transcript id), but the dir
  # and metadata agree. Legitimate archive, not drift.
  dir="$archive_root/eeeeeeee-0000-0000-0000-000000000005"
  mkdir -p "$dir"
  printf '{"timestamp":"2026-06-19T00:00:00.000Z","type":"response_item","payload":{}}\n' \
    >"$dir/transcript.jsonl"
  jq -n '{session:{session_id:"eeeeeeee-0000-0000-0000-000000000005"}}' >"$dir/metadata.json"

  # Archive missing metadata.json entirely; dir matches the transcript id.
  dir="$archive_root/ffffffff-0000-0000-0000-000000000006"
  mkdir -p "$dir"
  printf '{"timestamp":"2026-06-19T00:00:00.000Z","type":"session_meta","payload":{"id":"ffffffff-0000-0000-0000-000000000006","cwd":"/tmp/x"}}\n' \
    >"$dir/transcript.jsonl"

  set +e
  output="$(CODEX_SESSION_ARCHIVE_DIR="$archive_root" "$VALIDATE_SCRIPT" 2>&1)"
  status=$?
  set -e

  [ "$status" -eq 0 ] || fail "Expected zero exit for incomplete-but-consistent archives"
  if printf '%s' "$output" | grep -q "MISMATCH"; then
    fail "Incomplete archives must not be reported as MISMATCH"
  fi
}

command -v jq >/dev/null 2>&1 || fail "jq is required for these tests"

test_stop_hook_archives_transcript_and_metadata
test_stop_hook_finds_transcript_by_session_id
test_stop_hook_keeps_existing_longer_archive
test_stop_hook_adopts_transcript_id_when_payload_id_mismatches
test_stop_hook_adopts_transcript_id_for_explicit_path_mismatch
test_installer_manages_hooks_json_idempotently
test_backfill_imports_existing_sessions
test_validate_archives_flags_id_mismatch
test_validate_archives_passes_when_clean
test_validate_archives_skips_incomplete_archives

echo "PASS: save-codex-session"
