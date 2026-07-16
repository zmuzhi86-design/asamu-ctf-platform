#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "${BASH_SOURCE[0]%/*}" && pwd -P)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd -P)"
cd "$PROJECT_DIR"

PUBLIC_HOST="${1:-}"
HTTP_PORT="${2:-8080}"
MODE="${3:-}"
APP_VERSION="${APP_VERSION:-0.3.1}"

if [[ "$(id -u)" -ne 0 ]]; then
  echo "错误：请使用 sudo 运行。" >&2
  exit 1
fi
if [[ -z "$PUBLIC_HOST" ]]; then
  echo "用法：sudo bash ./scripts/install-local.sh <服务器IP或域名> [网站端口] [--fresh]" >&2
  exit 2
fi
if [[ -n "$MODE" && "$MODE" != "--fresh" ]]; then
  echo "错误：第三个参数仅支持 --fresh。" >&2
  exit 2
fi

env ASAMU_LOCAL_BUILD=true APP_VERSION="$APP_VERSION" bash "$SCRIPT_DIR/deploy-ubuntu.sh" "$PUBLIC_HOST" "$HTTP_PORT" "$MODE"

doctor_log="$(mktemp)"
# Invoked indirectly by the trap below.
# shellcheck disable=SC2329
cleanup() { rm -f -- "$doctor_log"; }
trap cleanup EXIT INT TERM
for attempt in $(seq 1 30); do
  if bash "$SCRIPT_DIR/docker-doctor.sh" >"$doctor_log" 2>&1; then
    cat "$doctor_log"
    echo
    echo "asamu 本地 Docker 靶场安装完成：http://${PUBLIC_HOST}:${HTTP_PORT}"
    exit 0
  fi
  [[ "$attempt" -lt 30 ]] || break
  sleep 2
done

cat "$doctor_log" >&2
echo "错误：安装后的健康检查未通过。" >&2
exit 1
