#!/usr/bin/env bash
# Preflight validation script for SpecTest CLI & MCP workflows.
#
# Usage: scripts/preflight.sh <init|update|run> [workspace_dir]
#   init    Checks prerequisites before `spectest generate --mode init`
#   update  Checks prerequisites before `spectest generate --mode update`
#   run     Checks prerequisites before `spectest run`
#
# Default workspace is ./.spectest ; can override Compose file path via
# the SPECTEST_COMPOSE environment variable.
#
# Exit code: 0 = no blocking errors (warnings allowed), 1 = blocking error,
# 2 = invalid usage.
#
# Note: The Docker Compose scan is a surface heuristic.
# It does not guarantee that the environment is completely isolated.

set -u

MODE="${1:-}"
WS="${2:-./.spectest}"
COMPOSE="${SPECTEST_COMPOSE:-$WS/docker-compose.test.yml}"
SUITE="$WS/spectest.yaml"
LOG="$WS/spectest.log.yaml"
CFG="$WS/config.yaml"

case "$MODE" in
  init|update|run) ;;
  *) echo "Usage: $0 <init|update|run> [workspace_dir]" >&2; exit 2 ;;
esac

ERR=0
WARN=0
ok()   { echo "[OK]      $*"; }
info() { echo "[INFO]    $*"; }
warn() { echo "[WARN]    $*"; WARN=$((WARN + 1)); }
err()  { echo "[ERROR]   $*"; ERR=$((ERR + 1)); }

echo "== SpecTest preflight: $MODE (workspace: $WS) =="

# ---------------------------------------------------------------- 1. CLI Check
check_flag() {  # check_flag "<help text>" <flag> [<flag>...]
  local help="$1"; shift
  local flag
  for flag in "$@"; do
    if ! printf '%s\n' "$help" | grep -Eq -- "(^|[^[:alnum:]_-])${flag}([^[:alnum:]_-]|$)"; then
      warn "Flag '$flag' not found in --help of installed CLI. Trust CLI --help over documentation."
    fi
  done
}

if ! command -v spectest >/dev/null 2>&1; then
  err "Command 'spectest' not found in PATH. Verify installation or ask the user; do not guess."
else
  ok "Found spectest: $(command -v spectest)"
  if [ "$MODE" = "run" ]; then
    HELP="$(spectest run --help 2>&1 || true)"
    if [ -z "$HELP" ]; then
      warn "Unable to read 'spectest run --help'."
    else
      check_flag "$HELP" -c -f -t --only-changed --triage --log
    fi
  else
    HELP="$(spectest generate --help 2>&1 || true)"
    if [ -z "$HELP" ]; then
      warn "Unable to read 'spectest generate --help'."
    else
      check_flag "$HELP" --mode --output --api --req --dry-run
    fi
  fi
fi

# ---------------------------------------------------------------- 2. Suite & Compose Files
case "$MODE" in
  init)
    if [ -f "$SUITE" ] || [ -f "$LOG" ]; then
      info "Existing suite/log files found in $WS. During init, allow CLI to prompt for overwrite confirmation; do not delete files manually."
    else
      ok "No existing suite found: init will not overwrite anything."
    fi
    ;;
  update)
    [ -f "$SUITE" ] && ok "Found $SUITE" || err "Missing $SUITE. Abort update and ask user to run init first."
    [ -f "$LOG" ]   && ok "Found $LOG"   || err "Missing $LOG. Abort update and ask user to run init first."
    ;;
  run)
    [ -f "$SUITE" ] && ok "Found $SUITE" || err "Missing $SUITE. Transition to generate step first."
    if [ -f "$LOG" ]; then
      ok "Found $LOG (default location beside suite)"
    else
      warn "Log $LOG not found beside suite. If stored elsewhere, specify --log explicitly."
    fi
    if [ -f "$COMPOSE" ]; then
      ok "Found test Compose file: $COMPOSE"
    else
      err "Missing test Compose file: $COMPOSE. Transition to prepare step."
    fi
    ;;
esac

# ---------------------------------------------------------------- 3. config.yaml
if [ -f "$CFG" ]; then
  ok "Found $CFG"
  if command -v python3 >/dev/null 2>&1 && python3 -c 'import yaml' 2>/dev/null; then
    CFG_OUT="$(python3 - "$CFG" <<'PY'
import os, sys, yaml
try:
    with open(sys.argv[1], encoding="utf-8") as fh:
        data = yaml.safe_load(fh) or {}
except Exception as exc:
    print("PARSE_ERROR|%s" % exc)
    sys.exit(0)
servers = data.get("servers") if isinstance(data, dict) else None
if not isinstance(servers, dict) or not servers:
    print("NO_SERVERS")
    sys.exit(0)
for alias, info in servers.items():
    info = info if isinstance(info, dict) else {}
    path = str(info.get("openapi") or "")
    url = str(info.get("base_url") or "")
    exists = "1" if path and os.path.isfile(path) else "0"
    valid = "0"
    if exists == "1":
        try:
            with open(path, encoding="utf-8") as fh:
                doc = yaml.safe_load(fh)
            if isinstance(doc, dict) and ("openapi" in doc or "swagger" in doc):
                valid = "1"
        except Exception:
            valid = "0"
    print("SERVER|%s|%s|%s|%s|%s" % (alias, path, url, exists, valid))
PY
)"
    API_FLAGS=""
    T_FLAGS=""
    while IFS='|' read -r kind alias path url exists valid; do
      case "$kind" in
        PARSE_ERROR) err "Unable to parse $CFG: $alias" ;;
        NO_SERVERS)  warn "$CFG has no valid 'servers' map." ;;
        SERVER)
          if [ -z "$path" ]; then
            err "Server '$alias': missing 'openapi' path."
          elif [ "$exists" != "1" ]; then
            err "Server '$alias': OpenAPI file not found at '$path'."
          elif [ "$valid" != "1" ]; then
            warn "Server '$alias': '$path' lacks top-level 'openapi' or 'swagger' key. Verify file."
          else
            ok "Server '$alias': OpenAPI '$path' exists and appears valid."
          fi
          if [ "$MODE" = "run" ]; then
            if [ -z "$url" ]; then
              warn "Server '$alias': missing base_url. Ask user for runtime URL; do not guess from alias."
            else
              ok "Server '$alias': base_url = $url (verify origin if unconfirmed)"
            fi
          fi
          [ "$exists" = "1" ] && API_FLAGS="$API_FLAGS --api $alias=$path"
          [ -n "$url" ]  && T_FLAGS="$T_FLAGS -t $alias=$url"
          ;;
      esac
    done <<< "$CFG_OUT"
    if [ "$MODE" = "run" ]; then
      [ -n "$T_FLAGS" ] && info "Suggested target flags:$T_FLAGS"
    else
      [ -n "$API_FLAGS" ] && info "Suggested API flags (order defines server order):$API_FLAGS"
    fi
  else
    warn "python3 + PyYAML not available; skipping detailed $CFG validation. Inspect manually."
  fi
else
  case "$MODE" in
    run) warn "Missing $CFG. Must retrieve runtime URL from user for each alias before running." ;;
    *)   warn "Missing $CFG. Collect aliases, OpenAPI paths, and runtime URLs then save to $CFG (see references/prepare.md)." ;;
  esac
fi

# ---------------------------------------------------------------- 4. Compose Scan (Heuristics)
if [ -f "$COMPOSE" ] && { [ "$MODE" = "run" ] || [ "$MODE" = "init" ]; }; then
  echo "-- Scanning test Compose file (heuristics; does not replace manual verification) --"
  C_WARN_BEFORE=$WARN

  if grep -Eiq 'prod(uction)?|staging' "$COMPOSE"; then
    warn "Compose contains 'prod', 'production', or 'staging'. Ensure it does not point to live resources."
  fi
  if grep -Eq '^\s*external:\s*(true|\{|$)' "$COMPOSE" || grep -Eq '^\s*external:\s*$' "$COMPOSE"; then
    warn "Compose uses 'external' volume/network. It may share resources with other environments."
  fi
  if grep -Eq '^\s*-\s*["'\'']?(/|~|\.\./|\$)[^:]*:' "$COMPOSE"; then
    warn "Compose has bind mounts escaping project directory (/, ~, .., vars). This mount may be mutated during teardown."
  fi
  if grep -Eq '^\s*-\s*["'\'']?\./[^:]*:[^ ]*(/var/lib/(postgresql|mysql|mongodb)|/data/db|/data)\b' "$COMPOSE"; then
    warn "Compose bind mounts project directory directly to database data directory. Ensure this is not persistent data."
  fi
  if grep -Eq '^\s*env_file:' "$COMPOSE"; then
    warn "Compose specifies env_file. Inspect those files and verify they contain no dev/prod credentials."
  fi
  if grep -Eiq '(HOST|URL|URI|ENDPOINT)[A-Z_]*\s*[:=]\s*["'\'']?[a-z0-9+.-]*(://)?[a-z0-9-]+\.[a-z0-9.-]*\.(com|net|org|io|cloud|internal|local)\b' "$COMPOSE"; then
    warn "Compose sets HOST/URL pointing to external domain. Verify it is not a real database or live service."
  fi
  if grep -Eq '^volumes:\s*$' "$COMPOSE"; then
    info "Compose declares named volumes: they may be purged when SpecTest tears down."
  fi
  if [ "$WARN" -eq "$C_WARN_BEFORE" ]; then
    ok "No high-risk patterns detected in Compose. Confirm isolation with the user before running."
  fi
fi

# ---------------------------------------------------------------- 5. Summary
echo "== Result: $ERR blocking error(s), $WARN warning(s) =="
if [ "$ERR" -gt 0 ]; then
  echo "Do NOT proceed with '$MODE' until the blocking errors above are resolved."
  exit 1
fi
if [ "$MODE" = "run" ]; then
  echo "Before running: confirm Compose is strictly isolated from dev/prod and user explicitly requested test execution."
fi
exit 0
