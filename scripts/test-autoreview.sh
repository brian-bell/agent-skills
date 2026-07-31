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
assert_contains "$codex_output" "model: gpt-5.6-luna" "Codex should default to gpt-5.6-luna"
assert_contains "$codex_output" "thinking: high" "Codex should default to high effort"

panel_output="$(cd "$ROOT" && "$AUTOREVIEW" --mode commit --panel --dry-run)"
assert_contains "$panel_output" "codex model=gpt-5.6-luna thinking=high" "Codex panel reviews should use the defaults"

override_output="$(cd "$ROOT" && "$AUTOREVIEW" --mode commit --model codex=gpt-test --thinking codex=xhigh --dry-run)"
assert_contains "$override_output" "model: gpt-test" "An explicit Codex model should override the default"
assert_contains "$override_output" "thinking: xhigh" "Explicit Codex effort should override the default"

claude_output="$(cd "$ROOT" && "$AUTOREVIEW" --mode commit --engine claude --dry-run)"
assert_not_contains "$claude_output" "gpt-5.6-luna" "Codex defaults must not leak into Claude reviews"
assert_not_contains "$claude_output" "thinking: high" "Codex effort must not leak into Claude reviews"

docs_output="$(cat \
  "$AUTOREVIEW_SKILL/SKILL.md" \
  "$AUTOREVIEW_SKILL/autoreview-breakdown.html" \
  "$AUTOREVIEW_SKILL/autoreview-walkthrough.html")"
assert_not_contains "$docs_output" "gpt-5.6-sol" "Autoreview docs should not name the retired Codex model"
assert_not_contains "$docs_output" "/Users/" "Autoreview docs should not contain user-specific absolute paths"
for doc in \
  "$AUTOREVIEW_SKILL/SKILL.md" \
  "$AUTOREVIEW_SKILL/autoreview-breakdown.html" \
  "$AUTOREVIEW_SKILL/autoreview-walkthrough.html"; do
  doc_output="$(cat "$doc")"
  assert_contains "$doc_output" "gpt-5.6-luna" "$doc should name the preferred Codex model"
  assert_contains "$doc_output" "gpt-5.6-terra" "$doc should name the Codex fallback model"
  assert_contains "$doc_output" "fallback" "$doc should explain the Codex model fallback"
done
assert_contains "$(cat "$AUTOREVIEW_SKILL/SKILL.md")" '<skill-dir>/scripts/autoreview' "The skill should document its portable helper path"

fake_codex_dir="$(mktemp -d)"
trap 'rm -rf "$fake_codex_dir"' EXIT
fake_codex="$fake_codex_dir/codex"
fake_codex_calls="$fake_codex_dir/calls"
cat >"$fake_codex" <<'EOF'
#!/bin/bash
set -euo pipefail

model=""
output_path=""
while (($#)); do
  case "$1" in
    --model)
      model="$2"
      shift 2
      ;;
    --output-last-message)
      output_path="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

printf '%s\n' "$model" >>"$FAKE_CODEX_CALLS"
if [[ "$model" == "gpt-5.6-luna" ]]; then
  echo "${FAKE_CODEX_LUNA_ERROR:-The 'gpt-5.6-luna' model is not available with your ChatGPT account}" >&2
  exit 1
fi

printf '%s\n' '{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"No findings.","overall_confidence":1}' >"$output_path"
EOF
chmod +x "$fake_codex"

set +e
fallback_output="$(
  cd "$ROOT"
  FAKE_CODEX_CALLS="$fake_codex_calls" CODEX_BIN="$fake_codex" \
    "$AUTOREVIEW" --mode commit --no-web-search 2>&1
)"
fallback_status=$?
set -e
[[ "$fallback_status" -eq 0 ]] || fail "Unavailable default Luna should retry with Terra"
assert_contains "$fallback_output" "retrying with gpt-5.6-terra" "Unavailable default Luna should fall back to Terra"
assert_contains "$fallback_output" "autoreview clean" "The Terra fallback should complete the review"
[[ "$(wc -l <"$fake_codex_calls")" -eq 2 ]] || fail "Default-model fallback should invoke Codex exactly twice"

: >"$fake_codex_calls"
set +e
quota_output="$(
  cd "$ROOT"
  FAKE_CODEX_CALLS="$fake_codex_calls" \
    FAKE_CODEX_LUNA_ERROR="You've hit your usage limit for gpt-5.6-luna. Switch to another model now" \
    CODEX_BIN="$fake_codex" "$AUTOREVIEW" --mode commit --no-web-search 2>&1
)"
quota_status=$?
set -e
[[ "$quota_status" -eq 0 ]] || fail "A model-specific Luna quota limit should retry with Terra"
assert_contains "$quota_output" "retrying with gpt-5.6-terra" "A model-specific Luna quota limit should fall back to Terra"
[[ "$(wc -l <"$fake_codex_calls")" -eq 2 ]] || fail "Quota-limit fallback should invoke Codex exactly twice"

: >"$fake_codex_calls"
set +e
explicit_output="$(
  cd "$ROOT"
  FAKE_CODEX_CALLS="$fake_codex_calls" CODEX_BIN="$fake_codex" \
    "$AUTOREVIEW" --mode commit --model gpt-5.6-luna --no-web-search 2>&1
)"
explicit_status=$?
set -e
[[ "$explicit_status" -ne 0 ]] || fail "An unavailable explicit model should fail"
assert_not_contains "$explicit_output" "retrying with gpt-5.6-terra" "Explicit model selections must not fall back"
[[ "$(wc -l <"$fake_codex_calls")" -eq 1 ]] || fail "An unavailable explicit model should invoke Codex exactly once"

fake_claude="$fake_codex_dir/claude"
cat >"$fake_claude" <<'EOF'
#!/bin/bash
printf '%s\n' '{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"No findings.","overall_confidence":1}'
EOF
chmod +x "$fake_claude"

: >"$fake_codex_calls"
panel_fallback_output="$(
  cd "$ROOT"
  FAKE_CODEX_CALLS="$fake_codex_calls" CODEX_BIN="$fake_codex" CLAUDE_BIN="$fake_claude" \
    "$AUTOREVIEW" --mode commit --panel --no-web-search 2>&1
)"
assert_contains "$panel_fallback_output" "codex model=gpt-5.6-terra thinking=high: 0 finding(s)" \
  "Panel summaries should identify the effective Terra fallback model"

echo "autoreview default tests passed"
