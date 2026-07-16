#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "${BASH_SOURCE[0]%/*}" && pwd -P)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd -P)"
cd "$PROJECT_DIR"

fail() { echo "错误：$*" >&2; exit 1; }
[[ -f .env.docker ]] || fail "缺少 .env.docker"

COMPOSE=(docker compose --env-file .env.docker)
if grep -Eq '^(ASAMU_LOCAL_BUILD|CHAIN_MIRROR_LOCAL_BUILD)=true$' .env.docker; then
  COMPOSE+=(-f docker-compose.yml -f docker-compose.build.yml)
fi
if grep -q '^RUNTIME_ENABLED=true$' .env.docker; then
  COMPOSE+=(--profile runtime)
fi

"${COMPOSE[@]}" config --quiet
"${COMPOSE[@]}" up -d postgres redis

for service in postgres redis; do
  ready=false
  for _ in $(seq 1 90); do
    container_id="$("${COMPOSE[@]}" ps -q "$service" 2>/dev/null || true)"
    if [[ -n "$container_id" ]]; then
      state="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else if .State.Running}}healthy{{else}}{{.State.Status}}{{end}}' "$container_id" 2>/dev/null || true)"
      if [[ "$state" == "healthy" ]]; then ready=true; break; fi
      if [[ "$state" == "exited" || "$state" == "dead" ]]; then break; fi
    fi
    sleep 2
  done
  [[ "$ready" == "true" ]] || fail "$service 未通过健康检查"
done

"${COMPOSE[@]}" rm -sf init >/dev/null 2>&1 || true
"${COMPOSE[@]}" up -d --no-deps --force-recreate init
init_id="$("${COMPOSE[@]}" ps -aq init 2>/dev/null || true)"
[[ -n "$init_id" ]] || fail "无法创建 init 容器"

for _ in $(seq 1 180); do
  state="$(docker inspect -f '{{.State.Status}}' "$init_id" 2>/dev/null || true)"
  if [[ "$state" == "exited" ]]; then
    exit_code="$(docker inspect -f '{{.State.ExitCode}}' "$init_id" 2>/dev/null || echo 1)"
    if [[ "$exit_code" == "0" ]]; then
      echo "数据库迁移、种子与依赖检查已完成。"
      exit 0
    fi
    "${COMPOSE[@]}" logs --tail=200 init >&2 || true
    fail "init 执行失败（退出码 $exit_code）"
  fi
  if [[ "$state" == "dead" || "$state" == "removing" || -z "$state" ]]; then
    "${COMPOSE[@]}" logs --tail=200 init >&2 || true
    fail "init 容器状态异常：${state:-missing}"
  fi
  sleep 1
done

"${COMPOSE[@]}" logs --tail=200 init >&2 || true
fail "init 执行超时"
