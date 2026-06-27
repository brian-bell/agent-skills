#!/bin/bash
set -euo pipefail

# Print discovered skills as "kind<TAB>name<TAB>source" lines.
#   kind: first | third | team
discover_skills() {
  local repo="$1"
  local d name

  for d in "$repo"/skills/*; do
    [ -d "$d" ] || continue
    name="$(basename "$d")"
    printf 'first\t%s\t%s\n' "$name" "$d"
  done

  for d in "$repo"/third-party/*; do
    [ -d "$d" ] || continue
    name="$(basename "$d")"
    printf 'third\t%s\t%s\n' "$name" "$d"
  done

  for d in "$repo"/agent-teams/*-team; do
    [ -d "$d" ] || continue
    name="$(basename "$d")"
    name="${name%-team}"
    printf 'team\t%s\t%s\n' "$name" "$d"
  done
}

# Return the staged copy path for a skill source, honoring $HOME.
stage_root() {
  printf '%s\n' "${SKILL_SYMLINKS_DIR:-$HOME/.skill-symlinks}"
}

staged_source() {
  local kind="$1" name="$2" source="$3"

  case "$kind" in
    first|third)
      printf '%s/skills/%s\n' "$(stage_root)" "$name"
      ;;
    team)
      printf '%s/agent-teams/%s\n' "$(stage_root)" "$(basename "$source")"
      ;;
  esac
}

# Refresh the staged copy that installed symlinks point at.
sync_staged_source() {
  local source="$1" staged="$2" tmp

  if [ ! -d "$source" ]; then
    echo "Missing skill source: $source" >&2
    return 1
  fi

  mkdir -p "$(dirname "$staged")"
  if command -v rsync >/dev/null 2>&1; then
    mkdir -p "$staged"
    rsync -a --delete "$source"/ "$staged"/
    return
  fi

  tmp="$staged.tmp.$$"
  rm -rf "$tmp"
  mkdir -p "$tmp"
  cp -R "$source"/. "$tmp"/
  rm -rf "$staged"
  mv "$tmp" "$staged"
}

# Print "target<TAB>link-source<TAB>comparison-source" symlink pairs for a
# skill, honoring $HOME. Installed targets link to the staged source, while
# state checks compare that staged copy to the current repo source.
skill_links() {
  local kind="$1" name="$2" source="$3"
  local claude="$HOME/.claude" agents="$HOME/.agents"
  local staged
  staged="$(staged_source "$kind" "$name" "$source")"

  case "$kind" in
    first|third)
      printf '%s\t%s\t%s\n' "$agents/skills/$name" "$staged" "$source"
      printf '%s\t%s\t%s\n' "$claude/skills/$name" "$staged" "$source"
      ;;
    team)
      local teamdir md
      teamdir="$(basename "$source")"
      printf '%s\t%s\t%s\n' "$claude/skills/$name" "$staged" "$source"
      for md in "$source"/*.md; do
        [ -f "$md" ] || continue
        case "$(basename "$md")" in
          SKILL.md|README.md) continue ;;  # manifest/docs, not agent definitions
        esac
        printf '%s\t%s\t%s\n' "$claude/agents/$teamdir/$(basename "$md")" "$staged/$(basename "$md")" "$md"
      done
      ;;
  esac
}

# Create one symlink, creating parent dirs.
#   force:   replace an existing symlink (foreign or stale). Non-destructive:
#            removing a symlink never deletes the data it points at.
#   destroy: additionally allow rm -rf of a real file/directory at the target.
#            This is the only path that can lose user data, so it is gated
#            behind the explicit --force flag.
link_path() {
  local source="$1" target="$2" force="${3:-false}" destroy="${4:-false}"

  if [ -L "$target" ] && [ "$(readlink "$target")" = "$source" ]; then
    return 0
  fi

  if [ -L "$target" ]; then
    # Existing symlink (foreign/dangling/stale). Safe to replace under force.
    if [ "$force" != true ]; then
      echo "Refusing to replace existing symlink: $target (use --force)" >&2
      return 1
    fi
    rm -f "$target"
  elif [ -e "$target" ]; then
    # Real file/directory — replacing it destroys data. Require --force.
    if [ "$destroy" != true ]; then
      echo "Refusing to overwrite real path: $target (use --force)" >&2
      return 1
    fi
    rm -rf "$target"
  fi

  mkdir -p "$(dirname "$target")"
  ln -s "$source" "$target"
}

install_skill() {
  local kind="$1" name="$2" source="$3" force="${4:-false}" destroy="${5:-false}"
  local target linksrc comparesrc rc=0 staged

  staged="$(staged_source "$kind" "$name" "$source")"
  sync_staged_source "$source" "$staged" || return 1

  while IFS=$'\t' read -r target linksrc comparesrc; do
    [ -n "$target" ] || continue
    link_path "$linksrc" "$target" "$force" "$destroy" || rc=1
  done <<EOF
$(skill_links "$kind" "$name" "$source")
EOF

  return "$rc"
}

# Remove a target only if it is a symlink we created (points at the expected
# source). Real dirs and foreign symlinks are left untouched.
unlink_owned() {
  local target="$1" linksrc="$2"

  if [ -L "$target" ] && [ "$(readlink "$target")" = "$linksrc" ]; then
    rm -f "$target"
    return 0
  fi
  return 1
}

uninstall_skill() {
  local kind="$1" name="$2" source="$3"
  local target linksrc comparesrc

  while IFS=$'\t' read -r target linksrc comparesrc; do
    [ -n "$target" ] || continue
    unlink_owned "$target" "$linksrc" || true
  done <<EOF
$(skill_links "$kind" "$name" "$source")
EOF

  # Prune the now-empty per-team agent dir only. Never the shared skills roots
  # (~/.claude/skills, ~/.agents/skills) — rmdir on a non-empty dir is a no-op,
  # but targeting only the team dir avoids ever removing a shared root.
  if [ "$kind" = team ]; then
    rmdir "$HOME/.claude/agents/$(basename "$source")" 2>/dev/null || true
  fi
}

# Classify a single target relative to its expected source:
#   linked  - our symlink, pointing at the repo source (current)
#   missing - nothing there
#   foreign - a symlink pointing elsewhere (incl. dangling); replacing it is
#             non-destructive (the data it points at survives)
#   copy    - a real path whose content matches the source
#   stale   - a real path whose content differs (replacing it destroys data)
target_state() {
  local target="$1" linksrc="$2"
  local comparesrc="${3:-$2}"

  if [ -L "$target" ]; then
    if [ "$(readlink "$target")" = "$linksrc" ]; then
      if [ -e "$linksrc" ] && diff -rq --exclude=.DS_Store "$linksrc" "$comparesrc" >/dev/null 2>&1; then
        echo linked
      else
        echo stale
      fi
    else
      echo foreign
    fi
  elif [ ! -e "$target" ]; then
    echo missing
  elif diff -rq --exclude=.DS_Store "$target" "$comparesrc" >/dev/null 2>&1; then
    echo copy
  else
    echo stale
  fi
}

# Aggregate target states into one skill state:
#   not-installed | installed | upgrade | partial
skill_state() {
  local kind="$1" name="$2" source="$3"
  local target linksrc comparesrc s
  local n=0 linked=0 missing=0 differ=0 copy=0

  while IFS=$'\t' read -r target linksrc comparesrc; do
    [ -n "$target" ] || continue
    n=$((n + 1))
    s="$(target_state "$target" "$linksrc" "$comparesrc")"
    case "$s" in
      linked) linked=$((linked + 1)) ;;
      missing) missing=$((missing + 1)) ;;
      stale|foreign) differ=$((differ + 1)) ;;  # differs from repo → upgradeable
      copy) copy=$((copy + 1)) ;;
    esac
  done <<EOF
$(skill_links "$kind" "$name" "$source")
EOF

  if [ "$differ" -gt 0 ]; then
    echo upgrade
  elif [ "$missing" -eq "$n" ]; then
    echo not-installed
  elif [ $((linked + copy)) -eq "$n" ]; then
    echo installed
  else
    echo partial
  fi
}

# Decide what to do given current state and desired (1=install, 0=remove):
#   install | upgrade | remove | none
plan_action() {
  local current="$1" desired="$2"

  if [ "$desired" = 1 ]; then
    case "$current" in
      not-installed|partial) echo install ;;
      upgrade) echo upgrade ;;
      *) echo none ;;
    esac
  else
    case "$current" in
      not-installed) echo none ;;
      *) echo remove ;;
    esac
  fi
}

# Execute a planned action for one skill. Echoes a status line.
#   destroy: allow rm -rf of real dirs during an upgrade (set by --force).
# The status line is derived from the RESULTING state, so partial outcomes and
# unexpected failures are reported accurately rather than always blaming --force.
apply_skill() {
  local kind="$1" name="$2" source="$3" desired="$4" destroy="${5:-false}"
  local current action force=false result

  current="$(skill_state "$kind" "$name" "$source")"
  action="$(plan_action "$current" "$desired")"

  case "$action" in
    install|upgrade)
      [ "$action" = upgrade ] && force=true
      # --force (destroy) implies overwriting symlinks too.
      [ "$destroy" = true ] && force=true
      # The known "Refusing to overwrite" stderr is expected; suppress it but
      # judge success from the resulting state, not the exit code.
      install_skill "$kind" "$name" "$source" "$force" "$destroy" 2>/dev/null || true
      result="$(skill_state "$kind" "$name" "$source")"
      case "$result" in
        installed) if [ "$action" = upgrade ]; then echo "^ upgraded $name"; else echo "+ installed $name"; fi ;;
        partial)   echo "~ $name partially applied (some targets need --force)" ;;
        *)         echo "! $name blocked: $result (use --force to overwrite)" ;;
      esac
      ;;
    remove) uninstall_skill "$kind" "$name" "$source" && echo "- removed $name" ;;
    none) : ;;
  esac
}

# ---------------------------------------------------------------------------
# Interactive layer
# ---------------------------------------------------------------------------

ESC=$'\033'
C_RESET="$ESC[0m"; C_BOLD="$ESC[1m"; C_DIM="$ESC[2m"
C_GREEN="$ESC[32m"; C_YELLOW="$ESC[33m"; C_CYAN="$ESC[36m"

# Parallel arrays describing every discovered skill.
TNAME=(); TKIND=(); TSRC=(); TDESIRED=(); TSTATE=()

load_skills() {
  local repo="$1"
  local kind name source
  TNAME=(); TKIND=(); TSRC=(); TDESIRED=(); TSTATE=()
  while IFS=$'\t' read -r kind name source; do
    [ -n "$name" ] || continue
    TKIND+=("$kind"); TNAME+=("$name"); TSRC+=("$source")
    TDESIRED+=(0); TSTATE+=("")
  done <<EOF
$(discover_skills "$repo")
EOF
  refresh_states
}

# Recompute on-disk state for each skill; seed desired from it.
refresh_states() {
  local i st
  for i in "${!TNAME[@]}"; do
    st="$(skill_state "${TKIND[$i]}" "${TNAME[$i]}" "${TSRC[$i]}")"
    TSTATE[$i]="$st"
    case "$st" in
      installed|partial|upgrade) TDESIRED[$i]=1 ;;
      *) TDESIRED[$i]=0 ;;
    esac
  done
}

state_label() {
  case "$1" in
    installed)     printf '%sinstalled%s' "$C_GREEN" "$C_RESET" ;;
    not-installed) printf '%snot installed%s' "$C_DIM" "$C_RESET" ;;
    upgrade)       printf '%s⬆ upgrade available%s' "$C_YELLOW" "$C_RESET" ;;
    partial)       printf '%s~ partial%s' "$C_CYAN" "$C_RESET" ;;
  esac
}

kind_header() {
  case "$1" in
    first) echo "first-party" ;;
    third) echo "third-party" ;;
    team)  echo "agent-teams (claude only)" ;;
  esac
}

# Draw the whole screen in a single write to avoid flicker. Home the cursor
# (no full clear), erase each line to its end (ESC[K), and erase anything below
# the frame at the end (ESC[J). Building one string and emitting it once means
# the terminal repaints in a single pass instead of line-by-line.
#
# Home-and-overwrite only stays correct while the frame fits the terminal; if
# the list is taller than the window the viewport scrolls and stale rows can
# survive. When the frame would not fit (TERM_ROWS, measured in run_tui), fall
# back to a full clear for that frame — correct, at the cost of flicker only in
# the doesn't-fit case.
render() {
  local cur="$1" msg="${2:-}"
  local i box mark prevkind="" eol="$ESC[K" nl out="" lead nls
  nl="$eol"$'\n'

  out+="$C_BOLD  agent-skills · manage skills$C_RESET$nl"
  out+="$C_DIM  ↑↓ move · space toggle · a all · n none · enter apply · q quit$C_RESET$nl"
  for i in "${!TNAME[@]}"; do
    if [ "${TKIND[$i]}" != "$prevkind" ]; then
      out+="$nl  $C_BOLD$(kind_header "${TKIND[$i]}")$C_RESET$nl"
      prevkind="${TKIND[$i]}"
    fi
    if [ "${TDESIRED[$i]}" = 1 ]; then box="[x]"; else box="[ ]"; fi
    if [ "$i" = "$cur" ]; then mark="$C_BOLD>$C_RESET"; else mark=" "; fi
    out+="$(printf '  %s %s %-32s %s' "$mark" "$box" "${TNAME[$i]}" "$(state_label "${TSTATE[$i]}")")$nl"
  done
  if [ -n "$msg" ]; then out+="$nl  $msg$nl"; fi
  out+="$ESC[J"

  # Full-clear this frame only if it would overflow the terminal height.
  lead="$ESC[H"
  nls="${out//[!$'\n']/}"
  if [ "${#nls}" -ge "${TERM_ROWS:-24}" ]; then lead="$ESC[2J$ESC[H"; fi
  printf '%s%s' "$lead" "$out"
}

# Apply all pending changes; print a summary.
apply_changes() {
  local i action current changed=0 out
  for i in "${!TNAME[@]}"; do
    current="${TSTATE[$i]}"
    action="$(plan_action "$current" "${TDESIRED[$i]}")"
    [ "$action" = none ] && continue
    out="$(apply_skill "${TKIND[$i]}" "${TNAME[$i]}" "${TSRC[$i]}" "${TDESIRED[$i]}" false)"
    if [ -n "$out" ]; then printf '  %s\n' "$out"; fi
    changed=1
  done
  if [ "$changed" = 0 ]; then echo "  nothing to do"; fi
  refresh_states
}

# Read one keypress, expanding arrow escape sequences.
read_key() {
  local k rest
  IFS= read -rsn1 k || return 1
  if [ "$k" = "$ESC" ]; then
    # Grab the rest of an escape sequence (e.g. arrow keys send ESC [ A/B).
    # Integer timeout for bash 3.2 compatibility; the trailing bytes arrive
    # together with ESC, so this returns immediately for real arrow presses.
    IFS= read -rsn2 -t 1 rest 2>/dev/null || true
    k="$k$rest"
  fi
  printf '%s' "$k"
}

run_tui() {
  local repo="$1" cur=0 key n msg=""
  load_skills "$repo"
  n="${#TNAME[@]}"
  if [ "$n" -eq 0 ]; then echo "No skills found in $repo" >&2; return 1; fi

  # Terminal height drives render's full-clear fallback for oversized frames.
  TERM_ROWS="$( (stty size 2>/dev/null || echo) | awk '{print $1}')"
  [ -n "$TERM_ROWS" ] || TERM_ROWS=24

  local saved_stty
  saved_stty="$(stty -g 2>/dev/null || true)"
  printf '%s[?25l%s[2J' "$ESC" "$ESC"  # hide cursor, clear once on entry
  # shellcheck disable=SC2064
  trap "stty '$saved_stty' 2>/dev/null; printf '%s[?25h\n' '$ESC'" EXIT INT TERM

  while true; do
    render "$cur" "$msg"; msg=""
    key="$(read_key)" || break
    case "$key" in
      "$ESC[A"|k) cur=$(((cur - 1 + n) % n)) ;;
      "$ESC[B"|j) cur=$(((cur + 1) % n)) ;;
      " ") if [ "${TDESIRED[$cur]}" = 1 ]; then TDESIRED[$cur]=0; else TDESIRED[$cur]=1; fi ;;
      a) for i in "${!TNAME[@]}"; do TDESIRED[$i]=1; done ;;
      n) for i in "${!TNAME[@]}"; do TDESIRED[$i]=0; done ;;
      "") # Enter
        printf '%s[2J%s[H\n' "$ESC" "$ESC"
        echo "  Applying…"; echo
        apply_changes
        echo; echo "  Done. Press any key to continue, q to quit."
        key="$(read_key)"; if [ "$key" = q ]; then break; fi ;;
      q|"$ESC") break ;;
    esac
  done
  return 0
}

apply_noninteractive() {
  local repo="$1" want="$2" force="$3" i
  load_skills "$repo"
  for i in "${!TNAME[@]}"; do TDESIRED[$i]="$want"; done
  if [ "$force" = true ] && [ "$want" = 1 ]; then
    # Force-relink everything, overwriting foreign symlinks AND real dirs.
    for i in "${!TNAME[@]}"; do
      install_skill "${TKIND[$i]}" "${TNAME[$i]}" "${TSRC[$i]}" true true \
        && echo "+ ${TNAME[$i]}" || echo "! ${TNAME[$i]}"
    done
    return
  fi
  apply_changes
}

usage() {
  cat <<EOF
Usage: $(basename "$0") [options]

Interactive skill installer/uninstaller. With no options, launches the TUI.

Options:
  --all      Install every skill (non-interactive)
  --none     Uninstall every skill (non-interactive)
  --force    Force-install everything, overwriting foreign symlinks AND real
             directories at the targets (destructive; the only path that can
             delete non-repo data)
  -h, --help Show this help
EOF
}

main() {
  local repo mode=tui force=false
  repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --all) mode=all ;;
      --none) mode=none ;;
      --force) force=true ;;
      -h|--help) usage; return 0 ;;
      *) echo "Unknown option: $1" >&2; usage >&2; return 1 ;;
    esac
    shift
  done

  # `--force` on its own means "force-install everything" (non-interactive).
  if [ "$mode" = tui ] && [ "$force" = true ]; then
    mode=all
  fi

  case "$mode" in
    all) apply_noninteractive "$repo" 1 "$force" ;;
    none) apply_noninteractive "$repo" 0 false ;;
    tui)
      if [ ! -t 0 ] || [ ! -t 1 ]; then
        echo "Not a terminal. Use --all or --none for non-interactive mode." >&2
        return 1
      fi
      run_tui "$repo"
      ;;
  esac
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
  main "$@"
fi
