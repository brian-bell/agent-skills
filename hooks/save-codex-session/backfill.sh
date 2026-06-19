#!/usr/bin/env bash
#
# backfill.sh - import existing Codex transcripts into the session archive.

set -uo pipefail

CODEX_ROOT="${CODEX_HOME:-$HOME/.codex}"
ARCHIVE_ROOT="${CODEX_SESSION_ARCHIVE_DIR:-$HOME/.agent-sessions/codex}"

MODE=skip
DRY_RUN=false
QUIET=false
while [ "$#" -gt 0 ]; do
  case "$1" in
    --update) MODE=update ;;
    --force) MODE=force ;;
    --dry-run) DRY_RUN=true ;;
    --quiet) QUIET=true ;;
    -h|--help) sed -n '2,30p' "$0"; exit 0 ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
  shift
done

command -v jq >/dev/null 2>&1 || { echo "ERROR: jq is required" >&2; exit 1; }

say() { [ "$QUIET" = true ] || printf '%s\n' "$*"; }

session_id_from_filename() {
  local path="$1"
  basename "$path" .jsonl | sed -n 's/.*-\([0-9a-fA-F]\{8\}-[0-9a-fA-F]\{4\}-[0-9a-fA-F]\{4\}-[0-9a-fA-F]\{4\}-[0-9a-fA-F]\{12\}\)$/\1/p'
}

meta() {
  local path="$1" expr="$2"
  jq -r "select(.type == \"session_meta\") | .payload | $expr // empty" "$path" 2>/dev/null | head -1
}

timestamp() {
  local path="$1" which="$2"
  case "$which" in
    first) jq -r '.timestamp // empty' "$path" 2>/dev/null | grep . | head -1 ;;
    last) jq -r '.timestamp // empty' "$path" 2>/dev/null | grep . | tail -1 ;;
  esac
}

write_metadata() {
  local dest="$1" src="$2" sid="$3" lines="$4" bytes="$5" archived_at="$6"
  local cwd cli_version originator started_at ended_at
  cwd="$(meta "$src" '.cwd')"
  cli_version="$(meta "$src" '.cli_version')"
  originator="$(meta "$src" '.originator')"
  started_at="$(timestamp "$src" first)"
  ended_at="$(timestamp "$src" last)"

  jq -n \
    --arg archived_at "$archived_at" \
    --arg dest "$dest" \
    --arg sid "$sid" \
    --arg src "$src" \
    --arg cwd "$cwd" \
    --arg cli_version "$cli_version" \
    --arg originator "$originator" \
    --arg started_at "$started_at" \
    --arg ended_at "$ended_at" \
    --argjson lines "${lines:-0}" \
    --argjson bytes "${bytes:-0}" \
    '{
       source: "backfill",
       archived_at: $archived_at,
       archive_dir: $dest,
       transcript_copied: true,
       transcript_lines: $lines,
       transcript_bytes: $bytes,
       session: {
         session_id: $sid,
         source_transcript: $src,
         cwd: (if $cwd == "" then null else $cwd end),
         cli_version: (if $cli_version == "" then null else $cli_version end),
         originator: (if $originator == "" then null else $originator end),
         started_at: (if $started_at == "" then null else $started_at end),
         ended_at: (if $ended_at == "" then null else $ended_at end)
       }
     }' >"$dest/metadata.json"
}

scanned=0 backfilled=0 updated=0 skipped=0 errors=0
archived_at="$(date '+%Y-%m-%dT%H:%M:%S%z')"

tmp_list="$(mktemp)"
trap 'rm -f "$tmp_list"' EXIT

find "$CODEX_ROOT/sessions" "$CODEX_ROOT/archived_sessions" \
  -type f -name '*.jsonl' 2>/dev/null | sort >"$tmp_list"

if [ ! -s "$tmp_list" ]; then
  echo "No Codex transcripts found under $CODEX_ROOT." >&2
  exit 0
fi

while IFS= read -r src; do
  scanned=$((scanned + 1))
  sid="$(meta "$src" '.id')"
  [ -n "$sid" ] || sid="$(session_id_from_filename "$src")"
  if [ -z "$sid" ]; then
    say "ERROR  $(basename "$src") (cannot determine session id)"
    errors=$((errors + 1))
    continue
  fi

  dest="$ARCHIVE_ROOT/$sid"
  src_lines="$(wc -l <"$src" 2>/dev/null | tr -d ' ')"

  if [ -d "$dest" ]; then
    case "$MODE" in
      skip)
        say "skip   $sid (already in store)"
        skipped=$((skipped + 1))
        continue
        ;;
      update)
        old_lines="$(wc -l <"$dest/transcript.jsonl" 2>/dev/null | tr -d ' ')"
        old_lines="${old_lines:-0}"
        if [ "${src_lines:-0}" -le "$old_lines" ]; then
          say "skip   $sid (store copy >= source: $old_lines >= ${src_lines:-0})"
          skipped=$((skipped + 1))
          continue
        fi
        action=update
        ;;
      force) action=update ;;
    esac
  else
    action=create
  fi

  if [ "$DRY_RUN" = true ]; then
    say "would $action $sid (${src_lines:-0} lines) -> $dest"
    [ "$action" = update ] && updated=$((updated + 1)) || backfilled=$((backfilled + 1))
    continue
  fi

  if ! mkdir -p "$dest" 2>/dev/null; then
    say "ERROR  $sid (cannot create $dest)"
    errors=$((errors + 1))
    continue
  fi
  if ! cp "$src" "$dest/transcript.jsonl" 2>/dev/null; then
    say "ERROR  $sid (copy failed)"
    errors=$((errors + 1))
    continue
  fi

  bytes="$(wc -c <"$dest/transcript.jsonl" 2>/dev/null | tr -d ' ')"
  if ! write_metadata "$dest" "$src" "$sid" "${src_lines:-0}" "${bytes:-0}" "$archived_at" 2>/dev/null; then
    say "ERROR  $sid (metadata write failed)"
    errors=$((errors + 1))
    continue
  fi

  say "$action $sid (${src_lines:-0} lines) -> $dest"
  [ "$action" = update ] && updated=$((updated + 1)) || backfilled=$((backfilled + 1))
done <"$tmp_list"

prefix=""
[ "$DRY_RUN" = true ] && prefix="[dry-run] "
say ""
echo "${prefix}Backfill complete: scanned=$scanned backfilled=$backfilled updated=$updated skipped=$skipped errors=$errors"
echo "${prefix}Store: $ARCHIVE_ROOT"
