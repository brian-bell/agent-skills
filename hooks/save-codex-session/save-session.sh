#!/usr/bin/env bash
#
# save-session.sh - Codex Stop hook
#
# Archives the current Codex transcript plus a metadata sidecar to:
#   ~/.agent-sessions/codex/<session-id>/
#       transcript.jsonl
#       metadata.json
#
# The script is designed to never disrupt Codex. It always exits 0 and logs
# problems to <archive-root>/save-session.log.

set -uo pipefail

CODEX_ROOT="${CODEX_HOME:-$HOME/.codex}"
ARCHIVE_ROOT="${CODEX_SESSION_ARCHIVE_DIR:-$HOME/.agent-sessions/codex}"
LOG_FILE="$ARCHIVE_ROOT/save-session.log"

mkdir -p "$ARCHIVE_ROOT" 2>/dev/null

log() {
  printf '%s %s\n' "$(date '+%Y-%m-%dT%H:%M:%S%z')" "$*" >>"$LOG_FILE" 2>/dev/null
}

json_get() {
  local expr="$1"
  [ -n "${INPUT:-}" ] || return 0
  printf '%s' "$INPUT" | jq -r "$expr // empty" 2>/dev/null
}

fallback_get() {
  local key="$1"
  printf '%s' "$INPUT" \
    | grep -o "\"$key\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" \
    | head -1 \
    | sed 's/.*:[[:space:]]*"\(.*\)"/\1/'
}

transcript_meta() {
  local path="$1" expr="$2"
  [ -f "$path" ] || return 0
  jq -r "select(.type == \"session_meta\") | .payload | $expr // empty" "$path" 2>/dev/null | head -1
}

transcript_timestamp() {
  local path="$1" which="$2"
  [ -f "$path" ] || return 0
  case "$which" in
    first) jq -r '.timestamp // empty' "$path" 2>/dev/null | grep . | head -1 ;;
    last) jq -r '.timestamp // empty' "$path" 2>/dev/null | grep . | tail -1 ;;
  esac
}

session_id_from_filename() {
  local path="$1"
  basename "$path" .jsonl | sed -n 's/.*-\([0-9a-fA-F]\{8\}-[0-9a-fA-F]\{4\}-[0-9a-fA-F]\{4\}-[0-9a-fA-F]\{4\}-[0-9a-fA-F]\{12\}\)$/\1/p'
}

find_transcript_by_session_id() {
  local sid="$1" candidate
  [ -n "$sid" ] || return 0
  while IFS= read -r candidate; do
    [ -f "$candidate" ] && printf '%s\n' "$candidate" && return 0
  done < <(
    find "$CODEX_ROOT/sessions" "$CODEX_ROOT/archived_sessions" \
      -type f -name "*$sid.jsonl" 2>/dev/null | sort -r
  )
}

find_transcript_by_cwd() {
  local want_cwd="$1" candidate meta_cwd
  [ -n "$want_cwd" ] || return 0
  while IFS= read -r candidate; do
    meta_cwd="$(transcript_meta "$candidate" '.cwd')"
    if [ "$meta_cwd" = "$want_cwd" ]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done < <(
    find "$CODEX_ROOT/sessions" "$CODEX_ROOT/archived_sessions" \
      -type f -name '*.jsonl' 2>/dev/null | sort -r
  )
}

write_metadata_with_jq() {
  local dest="$1" copied="$2" lines="$3" bytes="$4" payload_json="$5"
  local source_kind="$6" source_transcript="$7" sid="$8" event="$9" cwd="${10}"
  local archived_at="${11}" started_at="${12}" ended_at="${13}" cli_version="${14}"
  local originator="${15}" transcript_source="${16}"

  jq -n \
    --arg source "$source_kind" \
    --arg archived_at "$archived_at" \
    --arg dest "$dest" \
    --arg source_transcript "$source_transcript" \
    --arg sid "$sid" \
    --arg event "$event" \
    --arg cwd "$cwd" \
    --arg started_at "$started_at" \
    --arg ended_at "$ended_at" \
    --arg cli_version "$cli_version" \
    --arg originator "$originator" \
    --arg transcript_source "$transcript_source" \
    --argjson copied "$copied" \
    --argjson lines "${lines:-0}" \
    --argjson bytes "${bytes:-0}" \
    --argjson hook_payload "$payload_json" \
    '{
       source: $source,
       archived_at: $archived_at,
       archive_dir: $dest,
       transcript_copied: $copied,
       transcript_lines: $lines,
       transcript_bytes: $bytes,
       hook_payload: $hook_payload,
       session: {
         session_id: (if $sid == "" then null else $sid end),
         source_transcript: (if $source_transcript == "" then null else $source_transcript end),
         transcript_source: (if $transcript_source == "" then null else $transcript_source end),
         hook_event_name: (if $event == "" then null else $event end),
         cwd: (if $cwd == "" then null else $cwd end),
         started_at: (if $started_at == "" then null else $started_at end),
         ended_at: (if $ended_at == "" then null else $ended_at end),
         cli_version: (if $cli_version == "" then null else $cli_version end),
         originator: (if $originator == "" then null else $originator end)
       }
     }' >"$dest/metadata.json" 2>/dev/null
}

INPUT="$(cat)"

if [ -z "$INPUT" ]; then
  log "WARN: empty stdin payload; nothing to do"
  exit 0
fi

if command -v jq >/dev/null 2>&1; then
  payload_json="$(printf '%s' "$INPUT" | jq -c '.' 2>/dev/null || printf '{}')"
  session_id="$(json_get '.session_id // .conversation_id // .thread_id // .threadId // .codex_session_id // .session.id // .thread.id')"
  transcript="$(json_get '.transcript_path // .transcriptPath // .transcript // .session.transcript_path // .session.transcriptPath')"
  cwd="$(json_get '.cwd // .working_directory // .workingDirectory // .workspace // .session.cwd')"
  event="$(json_get '.hook_event_name // .hookEventName // .event_name // .event')"
else
  payload_json='{}'
  session_id="$(fallback_get session_id)"
  transcript="$(fallback_get transcript_path)"
  cwd="$(fallback_get cwd)"
  event="$(fallback_get hook_event_name)"
fi

transcript_source="payload"
if [ -z "$transcript" ] || [ ! -f "$transcript" ]; then
  found="$(find_transcript_by_session_id "$session_id")"
  if [ -n "$found" ]; then
    transcript="$found"
    transcript_source="session_id"
  else
    found="$(find_transcript_by_cwd "$cwd")"
    if [ -n "$found" ]; then
      transcript="$found"
      transcript_source="cwd"
    fi
  fi
fi

if [ -n "$transcript" ] && [ -f "$transcript" ]; then
  meta_sid="$(transcript_meta "$transcript" '.id')"
  [ -n "$session_id" ] || session_id="$meta_sid"
  [ -n "$cwd" ] || cwd="$(transcript_meta "$transcript" '.cwd')"
else
  log "WARN: transcript missing or not found for session_id='$session_id' cwd='$cwd'"
fi

[ -n "$session_id" ] || session_id="$(session_id_from_filename "$transcript")"
session_dir="${session_id:-unknown}"
short_id="${session_id:0:8}"; [ -n "$short_id" ] || short_id="unknown"
archived_at="$(date '+%Y-%m-%dT%H:%M:%S%z')"

DEST="$ARCHIVE_ROOT/$session_dir"
mkdir -p "$DEST" 2>/dev/null || { log "ERROR: cannot create $DEST"; exit 0; }

transcript_copied=false
transcript_lines=0
transcript_bytes=0
ARCHIVED="$DEST/transcript.jsonl"
if [ -n "$transcript" ] && [ -f "$transcript" ]; then
  tmp="$DEST/.transcript.$$.tmp"
  if cp "$transcript" "$tmp" 2>/dev/null && [ -s "$tmp" ]; then
    new_lines="$(wc -l <"$tmp" 2>/dev/null | tr -d ' ')"
    old_lines=0
    [ -f "$ARCHIVED" ] && old_lines="$(wc -l <"$ARCHIVED" 2>/dev/null | tr -d ' ')"
    if [ "${new_lines:-0}" -ge "${old_lines:-0}" ]; then
      mv "$tmp" "$ARCHIVED" && transcript_copied=true
    else
      rm -f "$tmp"
      log "WARN: source ($new_lines lines) shorter than archive ($old_lines); kept existing for $short_id"
    fi
  else
    rm -f "$tmp" 2>/dev/null
    log "ERROR: empty or failed copy from '$transcript'; kept any existing archive"
  fi
fi

if [ -f "$ARCHIVED" ]; then
  transcript_lines="$(wc -l <"$ARCHIVED" 2>/dev/null | tr -d ' ')"
  transcript_bytes="$(wc -c <"$ARCHIVED" 2>/dev/null | tr -d ' ')"
fi

started_at=""
ended_at=""
cli_version=""
originator=""
if [ -n "$transcript" ] && [ -f "$transcript" ]; then
  started_at="$(transcript_timestamp "$transcript" first)"
  ended_at="$(transcript_timestamp "$transcript" last)"
  cli_version="$(transcript_meta "$transcript" '.cli_version')"
  originator="$(transcript_meta "$transcript" '.originator')"
fi

if command -v jq >/dev/null 2>&1; then
  write_metadata_with_jq "$DEST" "$transcript_copied" "$transcript_lines" "$transcript_bytes" \
    "$payload_json" "hook" "${transcript:-}" "$session_id" "$event" "$cwd" "$archived_at" \
    "$started_at" "$ended_at" "$cli_version" "$originator" "$transcript_source" \
    || log "ERROR: failed to write metadata.json for $short_id"
else
  cat >"$DEST/metadata.json" <<EOF
{
  "source": "hook",
  "archived_at": "$archived_at",
  "archive_dir": "$DEST",
  "transcript_copied": $transcript_copied,
  "transcript_lines": ${transcript_lines:-0},
  "transcript_bytes": ${transcript_bytes:-0},
  "session": {
    "session_id": "$session_id",
    "source_transcript": "$transcript",
    "hook_event_name": "$event",
    "cwd": "$cwd"
  }
}
EOF
fi

log "OK: archived session $short_id (event=${event:-unknown} copied=$transcript_copied lines=$transcript_lines) -> $DEST"
exit 0
