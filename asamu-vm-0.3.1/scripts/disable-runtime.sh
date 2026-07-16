#!/usr/bin/env bash
set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd -P)"
cd "$PROJECT_DIR"
if [[ "$(id -u)" -ne 0 ]]; then
  echo "错误：请使用 sudo 运行。" >&2
  exit 1
fi
[[ -f .env.docker ]] || { echo "错误：缺少 .env.docker。" >&2; exit 1; }
if grep -q '^RUNTIME_ENABLED=' .env.docker; then
  sed -i 's/^RUNTIME_ENABLED=.*/RUNTIME_ENABLED=false/' .env.docker
else
  printf 'RUNTIME_ENABLED=false\n' >> .env.docker
fi
COMPOSE=(docker compose --env-file .env.docker)
if grep -Eq '^(ASAMU_LOCAL_BUILD|CHAIN_MIRROR_LOCAL_BUILD)=true$' .env.docker; then
  COMPOSE+=(-f docker-compose.yml -f docker-compose.build.yml)
fi
"${COMPOSE[@]}" --profile runtime stop worker
"${COMPOSE[@]}" --profile runtime rm -f worker
"${COMPOSE[@]}" up -d --no-deps --force-recreate api
echo "Docker 靶场 Worker 已停止，API 已重新加载禁用配置，网站服务不受影响。"
