#!/usr/bin/env bash
#
# validate-archives.sh - audit saved Codex session archives for identity drift.
#
# For every archive under <archive-root>/<dir>/ that has a copied transcript,
# the archive directory name, metadata.json .session.session_id, and the
# transcript.jsonl session_meta.payload.id must all agree. This script reports
# any archive where they disagree and exits non-zero when at least one mismatch
# is found, so it can be used as a CI/check gate.
#
# Usage:
#   validate-archives.sh [--quiet]
#
# Honors CODEX_SESSION_ARCHIVE_DIR (default: ~/.agent-sessions/codex).

set -uo pipefail

ARCHIVE_ROOT="${CODEX_SESSION_ARCHIVE_DIR:-$HOME/.agent-sessions/codex}"
QUIET=0
[ "${1:-}" = "--quiet" ] && QUIET=1

if ! command -v jq >/dev/null 2>&1; then
  echo "validate-archives: jq is required" >&2
  exit 2
fi

if [ ! -d "$ARCHIVE_ROOT" ]; then
  echo "validate-archives: no archive directory at $ARCHIVE_ROOT" >&2
  exit 0
fi

transcript_id() {
  local path="$1"
  [ -f "$path" ] || return 0
  jq -r 'select(.type == "session_meta") | .payload.id // empty' "$path" 2>/dev/null | head -1
}

metadata_id() {
  local path="$1"
  [ -f "$path" ] || return 0
  jq -r '.session.session_id // empty' "$path" 2>/dev/null
}

checked=0
mismatches=0
incomplete=0

for dir in "$ARCHIVE_ROOT"/*/; do
  [ -d "$dir" ] || continue
  transcript="$dir/transcript.jsonl"
  metadata="$dir/metadata.json"
  # Only archives that actually copied a transcript can drift in identity.
  [ -f "$transcript" ] || continue

  dir_id="$(basename "$dir")"
  meta_id="$(metadata_id "$metadata")"
  tid="$(transcript_id "$transcript")"
  checked=$((checked + 1))

  # Only a present id that disagrees with the directory is real drift. An
  # absent metadata or transcript id can't be compared (e.g. a transcript
  # with no session_meta line) and is reported as incomplete, not a failure.
  bad=0
  [ -n "$meta_id" ] && [ "$meta_id" != "$dir_id" ] && bad=1
  [ -n "$tid" ] && [ "$tid" != "$dir_id" ] && bad=1

  if [ "$bad" -eq 1 ]; then
    mismatches=$((mismatches + 1))
    [ "$QUIET" -eq 1 ] || \
      printf 'MISMATCH %s metadata=%s transcript=%s\n' \
        "$dir_id" "${meta_id:-<none>}" "${tid:-<none>}"
  elif [ -z "$meta_id" ] || [ -z "$tid" ]; then
    incomplete=$((incomplete + 1))
    [ "$QUIET" -eq 1 ] || \
      printf 'INCOMPLETE %s metadata=%s transcript=%s\n' \
        "$dir_id" "${meta_id:-<none>}" "${tid:-<none>}"
  fi
done

if [ "$QUIET" -eq 0 ]; then
  printf '%s mismatches (%s incomplete) out of %s archives with a transcript\n' \
    "$mismatches" "$incomplete" "$checked"
fi

[ "$mismatches" -eq 0 ]
