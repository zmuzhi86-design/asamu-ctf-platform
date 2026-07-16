#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd -P)"
cd "$PROJECT_DIR"

usage() {
  cat <<'USAGE'
用法：
  sudo ./scripts/enable-runtime.sh [允许的镜像列表] [--pull]

示例（推荐：直接使用服务器本地 docker build/docker load 的镜像）：
  sudo ./scripts/enable-runtime.sh

示例（本地不存在时允许从远程仓库拉取）：
  sudo ./scripts/enable-runtime.sh 'registry.cn-hangzhou.aliyuncs.com/demo/web-sqli:1.0' --pull

说明：
  不传镜像列表时，后台可直接选择或填写宿主机已经存在的本地镜像；Worker 不会自动拉取缺失镜像。
  使用 --pull 时必须提供允许拉取的镜像列表；多镜像使用英文逗号分隔，生产环境建议使用 @sha256 固定引用。
  启用后需要在云安全组中放行 .env.docker 配置的 RUNTIME_PORT_MIN-RUNTIME_PORT_MAX。
USAGE
}

if [[ "$(id -u)" -ne 0 ]]; then
  echo "错误：请使用 sudo 运行。" >&2
  exit 1
fi

IMAGES="${1:-}"
MODE="${2:-}"
if [[ "$IMAGES" == "--pull" ]]; then
  MODE="--pull"
  IMAGES=""
fi
if [[ -n "$MODE" && "$MODE" != "--pull" ]]; then
  echo "错误：未知参数 $MODE" >&2
  usage
  exit 1
fi
if [[ "$MODE" == "--pull" && -z "$IMAGES" ]]; then
  echo "错误：启用自动拉取时必须提供允许的镜像列表。" >&2
  usage
  exit 1
fi
if [[ ! -f .env.docker ]]; then
  echo "错误：没有 .env.docker，请先运行 deploy-ubuntu.sh 部署网站。" >&2
  exit 1
fi

upsert_env() {
  local key="$1" value="$2"
  if grep -q "^${key}=" .env.docker; then
    sed -i "s|^${key}=.*$|${key}=${value}|" .env.docker
  else
    printf '%s=%s\n' "$key" "$value" >> .env.docker
  fi
}

PULL=false
if [[ "$MODE" == "--pull" ]]; then
  PULL=true
fi

upsert_env RUNTIME_PROVIDER docker
upsert_env RUNTIME_ENABLED true
upsert_env RUNTIME_BIND_HOST 0.0.0.0
upsert_env RUNTIME_PORT_MIN 20000
upsert_env RUNTIME_PORT_MAX 30000
upsert_env RUNTIME_ALLOWED_IMAGES "$IMAGES"
upsert_env RUNTIME_PULL_MISSING_IMAGES "$PULL"
chmod 600 .env.docker

COMPOSE=(docker compose --env-file .env.docker)
if grep -Eq '^(ASAMU_LOCAL_BUILD|CHAIN_MIRROR_LOCAL_BUILD)=true$' .env.docker; then
  COMPOSE+=(-f docker-compose.yml -f docker-compose.build.yml)
fi
COMPOSE+=(--profile runtime)
"${COMPOSE[@]}" config --quiet
if grep -Eq '^(ASAMU_LOCAL_BUILD|CHAIN_MIRROR_LOCAL_BUILD)=true$' .env.docker; then
  "${COMPOSE[@]}" build api worker
else
  "${COMPOSE[@]}" pull api worker
fi
"$SCRIPT_DIR/run-init.sh"
"${COMPOSE[@]}" up -d --no-deps --force-recreate api worker

doctor_log="$(mktemp)"
runtime_ready=false
for attempt in $(seq 1 30); do
  if bash "$SCRIPT_DIR/docker-doctor.sh" >"$doctor_log" 2>&1; then
    runtime_ready=true
    break
  fi
  [[ "$attempt" -lt 30 ]] || break
  sleep 2
done
if [[ "$runtime_ready" != "true" ]]; then
  cat "$doctor_log" >&2
  rm -f -- "$doctor_log"
  echo "错误：Worker 未能注册健康心跳或读取宿主机 Docker 镜像。" >&2
  exit 1
fi
rm -f -- "$doctor_log"

echo
echo "Docker 靶场 Worker 已启用。"
if [[ -n "$IMAGES" ]]; then
  echo "允许镜像：$IMAGES"
else
  echo "镜像模式：宿主机本地镜像（后台直接填写镜像名）"
fi
echo "自动拉取：$PULL"
echo "API 已重新加载运行时配置。"
echo "查看日志：docker compose --env-file .env.docker --profile runtime logs -f worker"
