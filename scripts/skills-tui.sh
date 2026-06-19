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

# Print "target<TAB>source" symlink pairs for a skill, honoring $HOME.
skill_links() {
  local kind="$1" name="$2" source="$3"
  local claude="$HOME/.claude" agents="$HOME/.agents"

  case "$kind" in
    first|third)
      printf '%s\t%s\n' "$agents/skills/$name" "$source"
      printf '%s\t%s\n' "$claude/skills/$name" "$source"
      ;;
    team)
      local teamdir md
      teamdir="$(basename "$source")"
      printf '%s\t%s\n' "$claude/skills/$name" "$source"
      for md in "$source"/*.md; do
        [ -f "$md" ] || continue
        case "$(basename "$md")" in
          SKILL.md|README.md) continue ;;  # manifest/docs, not agent definitions
        esac
        printf '%s\t%s\n' "$claude/agents/$teamdir/$(basename "$md")" "$md"
      done
      ;;
  esac
}

# Create one symlink, creating parent dirs. With force, replace a target we own
# or that the caller has deemed replaceable.
link_path() {
  local source="$1" target="$2" force="${3:-false}"

  if [ -L "$target" ] && [ "$(readlink "$target")" = "$source" ]; then
    return 0
  fi

  if [ -e "$target" ] || [ -L "$target" ]; then
    if [ "$force" != true ]; then
      echo "Refusing to overwrite existing target: $target" >&2
      return 1
    fi
    rm -rf "$target"
  fi

  mkdir -p "$(dirname "$target")"
  ln -s "$source" "$target"
}

install_skill() {
  local kind="$1" name="$2" source="$3" force="${4:-false}"
  local target linksrc rc=0

  while IFS=$'\t' read -r target linksrc; do
    [ -n "$target" ] || continue
    link_path "$linksrc" "$target" "$force" || rc=1
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
  local target linksrc

  while IFS=$'\t' read -r target linksrc; do
    [ -n "$target" ] || continue
    unlink_owned "$target" "$linksrc" || true
    # Prune now-empty parent dir for team agent files.
    rmdir "$(dirname "$target")" 2>/dev/null || true
  done <<EOF
$(skill_links "$kind" "$name" "$source")
EOF
}

# Classify a single target relative to its expected source:
#   linked | missing | stale | copy
target_state() {
  local target="$1" linksrc="$2"

  if [ -L "$target" ] && [ "$(readlink "$target")" = "$linksrc" ]; then
    echo linked
  elif [ ! -e "$target" ] && [ ! -L "$target" ]; then
    echo missing
  elif diff -rq --exclude=.DS_Store "$target" "$linksrc" >/dev/null 2>&1; then
    echo copy
  else
    echo stale
  fi
}

# Aggregate target states into one skill state:
#   not-installed | installed | upgrade | partial
skill_state() {
  local kind="$1" name="$2" source="$3"
  local target linksrc s
  local n=0 linked=0 missing=0 stale=0 copy=0

  while IFS=$'\t' read -r target linksrc; do
    [ -n "$target" ] || continue
    n=$((n + 1))
    s="$(target_state "$target" "$linksrc")"
    case "$s" in
      linked) linked=$((linked + 1)) ;;
      missing) missing=$((missing + 1)) ;;
      stale) stale=$((stale + 1)) ;;
      copy) copy=$((copy + 1)) ;;
    esac
  done <<EOF
$(skill_links "$kind" "$name" "$source")
EOF

  if [ "$stale" -gt 0 ]; then
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
apply_skill() {
  local kind="$1" name="$2" source="$3" desired="$4"
  local current action

  current="$(skill_state "$kind" "$name" "$source")"
  action="$(plan_action "$current" "$desired")"

  case "$action" in
    install) install_skill "$kind" "$name" "$source" false && echo "+ installed $name" || echo "! failed $name" ;;
    upgrade) install_skill "$kind" "$name" "$source" true && echo "^ upgraded $name" || echo "! failed $name" ;;
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

render() {
  local cur="$1" msg="${2:-}"
  local i box mark prevkind="" line
  printf '%s[2J%s[H' "$ESC" "$ESC"
  printf '%s  agent-skills · manage skills%s\n' "$C_BOLD" "$C_RESET"
  printf '%s  ↑↓ move · space toggle · a all · n none · enter apply · q quit%s\n' "$C_DIM" "$C_RESET"
  for i in "${!TNAME[@]}"; do
    if [ "${TKIND[$i]}" != "$prevkind" ]; then
      printf '\n  %s%s%s\n' "$C_BOLD" "$(kind_header "${TKIND[$i]}")" "$C_RESET"
      prevkind="${TKIND[$i]}"
    fi
    if [ "${TDESIRED[$i]}" = 1 ]; then box="[x]"; else box="[ ]"; fi
    if [ "$i" = "$cur" ]; then mark="${C_BOLD}>${C_RESET}"; else mark=" "; fi
    line="$(printf '%-32s %s' "${TNAME[$i]}" "$(state_label "${TSTATE[$i]}")")"
    printf '  %s %s %s\n' "$mark" "$box" "$line"
  done
  if [ -n "$msg" ]; then printf '\n  %s\n' "$msg"; fi
}

# Apply all pending changes; print a summary.
apply_changes() {
  local i action current changed=0 out
  for i in "${!TNAME[@]}"; do
    current="${TSTATE[$i]}"
    action="$(plan_action "$current" "${TDESIRED[$i]}")"
    [ "$action" = none ] && continue
    out="$(apply_skill "${TKIND[$i]}" "${TNAME[$i]}" "${TSRC[$i]}" "${TDESIRED[$i]}")"
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
    IFS= read -rsn2 -t 0.001 rest 2>/dev/null || true
    k="$k$rest"
  fi
  printf '%s' "$k"
}

run_tui() {
  local repo="$1" cur=0 key n msg=""
  load_skills "$repo"
  n="${#TNAME[@]}"
  if [ "$n" -eq 0 ]; then echo "No skills found in $repo" >&2; return 1; fi

  local saved_stty
  saved_stty="$(stty -g 2>/dev/null || true)"
  printf '%s[?25l' "$ESC"  # hide cursor
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
    # Force-relink everything, overwriting foreign targets too.
    for i in "${!TNAME[@]}"; do
      install_skill "${TKIND[$i]}" "${TNAME[$i]}" "${TSRC[$i]}" true \
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
  --force    With --all, overwrite foreign targets / relink copies
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
