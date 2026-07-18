#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
AUTOREVIEW="$ROOT/third-party/autoreview/scripts/autoreview"
AUTOREVIEW_SKILL="$ROOT/third-party/autoreview"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_contains() {
  local output="$1"
  local expected="$2"
  local message="$3"

  grep -Fq "$expected" <<<"$output" || fail "$message"
}

assert_not_contains() {
  local output="$1"
  local unexpected="$2"
  local message="$3"

  if grep -Fq "$unexpected" <<<"$output"; then
    fail "$message"
  fi
}

codex_output="$(cd "$ROOT" && "$AUTOREVIEW" --mode commit --dry-run)"
assert_contains "$codex_output" "engine: codex" "Codex should remain the default engine"
assert_contains "$codex_output" "model: gpt-5.6-terra" "Codex should default to gpt-5.6-terra"
assert_contains "$codex_output" "thinking: high" "Codex should default to high effort"

panel_output="$(cd "$ROOT" && "$AUTOREVIEW" --mode commit --panel --dry-run)"
assert_contains "$panel_output" "codex model=gpt-5.6-terra thinking=high" "Codex panel reviews should use the defaults"

override_output="$(cd "$ROOT" && "$AUTOREVIEW" --mode commit --model codex=gpt-test --thinking codex=xhigh --dry-run)"
assert_contains "$override_output" "model: gpt-test" "An explicit Codex model should override the default"
assert_contains "$override_output" "thinking: xhigh" "Explicit Codex effort should override the default"

claude_output="$(cd "$ROOT" && "$AUTOREVIEW" --mode commit --engine claude --dry-run)"
assert_not_contains "$claude_output" "gpt-5.6-terra" "Codex defaults must not leak into Claude reviews"
assert_not_contains "$claude_output" "thinking: high" "Codex effort must not leak into Claude reviews"

docs_output="$(cat \
  "$AUTOREVIEW_SKILL/SKILL.md" \
  "$AUTOREVIEW_SKILL/autoreview-breakdown.html" \
  "$AUTOREVIEW_SKILL/autoreview-walkthrough.html")"
assert_contains "$docs_output" "gpt-5.6-terra" "Autoreview docs should name the default Codex model"
assert_not_contains "$docs_output" "gpt-5.6-sol" "Autoreview docs should not name the retired Codex model"
assert_not_contains "$docs_output" "/Users/" "Autoreview docs should not contain user-specific absolute paths"
assert_contains "$(cat "$AUTOREVIEW_SKILL/SKILL.md")" '<skill-dir>/scripts/autoreview' "The skill should document its portable helper path"

echo "autoreview default tests passed"
