#!/usr/bin/env bash
# Kiểm tra điều kiện trước khi dùng SpecTest.
#
# Cách dùng:  scripts/preflight.sh <init|update|run> [thư_mục_workspace]
#   init    kiểm tra trước `spectest generate --mode init`
#   update  kiểm tra trước `spectest generate --mode update`
#   run     kiểm tra trước `spectest run`
#
# Mặc định workspace là ./.spectest ; có thể đổi đường dẫn Compose bằng
# biến môi trường SPECTEST_COMPOSE.
#
# Exit code: 0 = không có lỗi chặn (có thể vẫn có cảnh báo), 1 = có lỗi chặn,
# 2 = dùng sai tham số.
#
# Lưu ý: phần quét Compose chỉ là heuristic phát hiện dấu hiệu bề mặt.
# Nó không chứng minh được môi trường đã cô lập.

set -u

MODE="${1:-}"
WS="${2:-./.spectest}"
COMPOSE="${SPECTEST_COMPOSE:-$WS/docker-compose.test.yml}"
SUITE="$WS/spectest.yaml"
LOG="$WS/spectest.log.yaml"
CFG="$WS/config.yaml"

case "$MODE" in
  init|update|run) ;;
  *) echo "Cách dùng: $0 <init|update|run> [thư_mục_workspace]" >&2; exit 2 ;;
esac

ERR=0
WARN=0
ok()   { echo "[OK]         $*"; }
info() { echo "[INFO]       $*"; }
warn() { echo "[CẢNH BÁO]   $*"; WARN=$((WARN + 1)); }
err()  { echo "[LỖI]        $*"; ERR=$((ERR + 1)); }

echo "== SpecTest preflight: $MODE (workspace: $WS) =="

# ---------------------------------------------------------------- 1. CLI
check_flag() {  # check_flag "<help text>" <cờ> [<cờ>...]
  local help="$1"; shift
  local flag
  for flag in "$@"; do
    if ! printf '%s\n' "$help" | grep -Eq -- "(^|[^[:alnum:]_-])${flag}([^[:alnum:]_-]|$)"; then
      warn "Cờ '$flag' không thấy trong --help của CLI đang cài. Tin vào --help, không tin skill."
    fi
  done
}

if ! command -v spectest >/dev/null 2>&1; then
  err "Không tìm thấy lệnh 'spectest' trong PATH. Hỏi người dùng cách cài đặt; không tự đoán."
else
  ok "Tìm thấy spectest: $(command -v spectest)"
  if [ "$MODE" = "run" ]; then
    HELP="$(spectest run --help 2>&1 || true)"
    if [ -z "$HELP" ]; then
      warn "Không đọc được 'spectest run --help'."
    else
      check_flag "$HELP" -c -f -t --only-changed --triage --log
    fi
  else
    HELP="$(spectest generate --help 2>&1 || true)"
    if [ -z "$HELP" ]; then
      warn "Không đọc được 'spectest generate --help'."
    else
      check_flag "$HELP" --mode --output --api --req --dry-run
    fi
  fi
fi

# ---------------------------------------------------------------- 2. File suite
case "$MODE" in
  init)
    if [ -f "$SUITE" ] || [ -f "$LOG" ]; then
      info "Đã có file suite/log trong $WS. Khi init, để CLI tự hỏi xác nhận overwrite; không tự xóa file."
    else
      ok "Chưa có suite cũ: init sẽ không ghi đè gì."
    fi
    ;;
  update)
    [ -f "$SUITE" ] && ok "Có $SUITE" || err "Thiếu $SUITE. Dừng update và yêu cầu người dùng chạy lại init."
    [ -f "$LOG" ]   && ok "Có $LOG"   || err "Thiếu $LOG. Dừng update và yêu cầu người dùng chạy lại init."
    ;;
  run)
    [ -f "$SUITE" ] && ok "Có $SUITE" || err "Thiếu $SUITE. Chuyển sang bước generate."
    if [ -f "$LOG" ]; then
      ok "Có $LOG (mặc định CLI tìm log cạnh suite)"
    else
      warn "Không thấy $LOG cạnh suite. Nếu log ở nơi khác thì phải truyền --log."
    fi
    if [ -f "$COMPOSE" ]; then
      ok "Có Compose test: $COMPOSE"
    else
      err "Thiếu Compose test: $COMPOSE. Chuyển sang bước prepare."
    fi
    ;;
esac

# ---------------------------------------------------------------- 3. config.yaml
if [ -f "$CFG" ]; then
  ok "Có $CFG"
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
        PARSE_ERROR) err "Không parse được $CFG: $alias" ;;
        NO_SERVERS)  warn "$CFG không có mục 'servers' hợp lệ." ;;
        SERVER)
          if [ -z "$path" ]; then
            err "Alias '$alias': chưa khai báo 'openapi'."
          elif [ "$exists" != "1" ]; then
            err "Alias '$alias': không thấy file OpenAPI '$path'."
          elif [ "$valid" != "1" ]; then
            warn "Alias '$alias': '$path' không có khóa 'openapi'/'swagger' ở cấp cao nhất. Kiểm tra lại file."
          else
            ok "Alias '$alias': OpenAPI '$path' tồn tại và có vẻ hợp lệ."
          fi
          if [ "$MODE" = "run" ]; then
            if [ -z "$url" ]; then
              warn "Alias '$alias': chưa có base_url. Hỏi người dùng URL runtime; đừng đoán từ tên alias."
            else
              ok "Alias '$alias': base_url = $url (nhớ xác nhận nếu chưa chắc nguồn)"
            fi
          fi
          [ "$exists" = "1" ] && API_FLAGS="$API_FLAGS --api $alias=$path"
          [ -n "$url" ]  && T_FLAGS="$T_FLAGS -t $alias=$url"
          ;;
      esac
    done <<< "$CFG_OUT"
    if [ "$MODE" = "run" ]; then
      [ -n "$T_FLAGS" ] && info "Gợi ý cờ target:$T_FLAGS"
    else
      [ -n "$API_FLAGS" ] && info "Gợi ý cờ API (thứ tự quyết định thứ tự server):$API_FLAGS"
    fi
  else
    warn "Không có python3 + PyYAML nên bỏ qua việc đọc $CFG. Hãy mở file và kiểm tra thủ công."
  fi
else
  case "$MODE" in
    run) warn "Chưa có $CFG. Phải lấy URL runtime từ người dùng cho từng alias trước khi chạy." ;;
    *)   warn "Chưa có $CFG. Thu thập alias, đường dẫn OpenAPI, URL runtime rồi lưu lại (xem references/prepare.md)." ;;
  esac
fi

# ---------------------------------------------------------------- 4. Quét Compose (heuristic)
if [ -f "$COMPOSE" ] && { [ "$MODE" = "run" ] || [ "$MODE" = "init" ]; }; then
  echo "-- Quét Compose test (heuristic, không thay thế việc đọc và xác nhận) --"
  C_WARN_BEFORE=$WARN

  if grep -Eiq 'prod(uction)?|staging' "$COMPOSE"; then
    warn "Compose có chữ prod/production/staging. Kiểm tra có trỏ tới tài nguyên thật không."
  fi
  if grep -Eq '^\s*external:\s*(true|\{|$)' "$COMPOSE" || grep -Eq '^\s*external:\s*$' "$COMPOSE"; then
    warn "Compose dùng volume/network 'external'. Có thể là tài nguyên dùng chung với môi trường khác."
  fi
  if grep -Eq '^\s*-\s*["'\'']?(/|~|\.\./|\$)[^:]*:' "$COMPOSE"; then
    warn "Compose có bind mount tới đường dẫn ngoài thư mục dự án (/, ~, .., biến). Volume này có thể bị ảnh hưởng khi teardown."
  fi
  if grep -Eq '^\s*-\s*["'\'']?\./[^:]*:[^ ]*(/var/lib/(postgresql|mysql|mongodb)|/data/db|/data)\b' "$COMPOSE"; then
    warn "Compose bind mount thư mục dự án vào thư mục dữ liệu của database. Đảm bảo đó không phải dữ liệu cần giữ."
  fi
  if grep -Eq '^\s*env_file:' "$COMPOSE"; then
    warn "Compose dùng env_file. Mở các file đó và xác nhận không chứa credentials dev/prod."
  fi
  if grep -Eiq '(HOST|URL|URI|ENDPOINT)[A-Z_]*\s*[:=]\s*["'\'']?[a-z0-9+.-]*(://)?[a-z0-9-]+\.[a-z0-9.-]*\.(com|net|org|io|cloud|internal|local)\b' "$COMPOSE"; then
    warn "Compose có biến HOST/URL trỏ tới tên miền ngoài. Xác nhận đó không phải database/dịch vụ thật."
  fi
  if grep -Eq '^volumes:\s*$' "$COMPOSE"; then
    info "Compose khai báo named volumes: chúng có thể bị xóa sau khi SpecTest teardown."
  fi
  if [ "$WARN" -eq "$C_WARN_BEFORE" ]; then
    ok "Không thấy dấu hiệu bất thường trong Compose. Vẫn cần bạn đọc file và xác nhận cô lập với người dùng."
  fi
fi

# ---------------------------------------------------------------- 5. Tổng kết
echo "== Kết quả: $ERR lỗi chặn, $WARN cảnh báo =="
if [ "$ERR" -gt 0 ]; then
  echo "Không tiếp tục bước '$MODE' cho tới khi xử lý các lỗi trên."
  exit 1
fi
if [ "$MODE" = "run" ]; then
  echo "Trước khi chạy: xác nhận Compose cô lập khỏi dữ liệu dev/prod và người dùng đã yêu cầu chạy."
fi
exit 0
