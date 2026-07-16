#!/usr/bin/env bash
set -u

SCRIPT_PATH="${BASH_SOURCE[0]}"
SCRIPT_DIR="${SCRIPT_PATH%/*}"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd -P)"
cd "$PROJECT_DIR" || exit 1

failures=0
check() {
  local label="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    printf '[OK]   %s\n' "$label"
  else
    printf '[FAIL] %s\n' "$label"
    failures=$((failures + 1))
  fi
}

if [[ ! -f .env.docker ]]; then
  echo "[FAIL] 缺少 .env.docker"
  exit 1
fi

HTTP_PORT="$(sed -n 's/^HTTP_PORT=//p' .env.docker | tail -n 1)"
HTTP_PORT="${HTTP_PORT:-8080}"
RUNTIME_ENABLED="$(sed -n 's/^RUNTIME_ENABLED=//p' .env.docker | tail -n 1)"
POSTGRES_USER="$(sed -n 's/^POSTGRES_USER=//p' .env.docker | tail -n 1)"
POSTGRES_DB="$(sed -n 's/^POSTGRES_DB=//p' .env.docker | tail -n 1)"
POSTGRES_USER="${POSTGRES_USER:-asamu}"
POSTGRES_DB="${POSTGRES_DB:-asamu}"
COMPOSE=(docker compose --env-file .env.docker)
if [[ "$RUNTIME_ENABLED" == "true" ]]; then
  COMPOSE+=(--profile runtime)
fi

check "Docker daemon" docker info
check "Docker Compose plugin" docker compose version
check "Compose configuration" "${COMPOSE[@]}" config --quiet

for service in postgres redis api web; do
  container_id="$("${COMPOSE[@]}" ps -q "$service" 2>/dev/null || true)"
  if [[ -n "$container_id" ]] && [[ "$(docker inspect -f '{{.State.Running}}' "$container_id" 2>/dev/null)" == "true" ]]; then
    echo "[OK]   service $service is running"
  else
    echo "[FAIL] service $service is not running"
    failures=$((failures + 1))
  fi
done

api_id="$("${COMPOSE[@]}" ps -q api 2>/dev/null || true)"
api_runtime_enabled=""
if [[ -n "$api_id" ]]; then
  api_runtime_enabled="$(docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$api_id" 2>/dev/null | sed -n 's/^RUNTIME_ENABLED=//p' | tail -n 1)"
fi
if [[ "$api_runtime_enabled" == "${RUNTIME_ENABLED:-false}" ]]; then
  echo "[OK]   API runtime configuration is current"
else
  echo "[FAIL] API runtime configuration is stale (env=${RUNTIME_ENABLED:-false}, container=${api_runtime_enabled:-missing})"
  failures=$((failures + 1))
fi

if [[ "$RUNTIME_ENABLED" == "true" ]]; then
  worker_id="$("${COMPOSE[@]}" ps -q worker 2>/dev/null || true)"
  if [[ -n "$worker_id" ]] && [[ "$(docker inspect -f '{{.State.Running}}' "$worker_id" 2>/dev/null)" == "true" ]]; then
    echo "[OK]   service worker is running"
  else
    echo "[FAIL] service worker is not running"
    failures=$((failures + 1))
  fi
  socket_source="$(docker inspect -f '{{range .Mounts}}{{if eq .Destination "/var/run/docker.sock"}}{{.Source}}{{end}}{{end}}' "$worker_id" 2>/dev/null || true)"
  if [[ "$socket_source" == "/var/run/docker.sock" && -S /var/run/docker.sock ]]; then
    echo "[OK]   worker Docker socket mount"
  else
    echo "[FAIL] worker Docker socket mount is missing"
    failures=$((failures + 1))
  fi
fi

init_id="$("${COMPOSE[@]}" ps -aq init 2>/dev/null || true)"
if [[ -n "$init_id" ]] && [[ "$(docker inspect -f '{{.State.ExitCode}}' "$init_id" 2>/dev/null)" == "0" ]]; then
  echo "[OK]   init completed successfully"
else
  echo "[FAIL] init did not complete successfully"
  failures=$((failures + 1))
fi

check "PostgreSQL readiness" "${COMPOSE[@]}" exec -T postgres pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"
if "${COMPOSE[@]}" exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "SELECT CASE WHEN to_regclass('public.learning_paths') IS NOT NULL AND to_regclass('public.learning_stages') IS NOT NULL AND to_regclass('public.learning_stage_challenges') IS NOT NULL THEN 'ok' ELSE 'missing' END" 2>/dev/null | grep -q '^ok$'; then
  echo "[OK]   learning schema"
else
  echo "[FAIL] learning schema is missing"
  failures=$((failures + 1))
fi
if [[ "$RUNTIME_ENABLED" == "true" ]]; then
  if "${COMPOSE[@]}" exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "SELECT CASE WHEN EXISTS (SELECT 1 FROM runtime_worker_nodes WHERE status='online' AND enabled=true AND last_heartbeat>now()-interval '90 seconds' AND last_error_code='') THEN 'ok' ELSE 'failed' END" 2>/dev/null | grep -q '^ok$'; then
    echo "[OK]   worker Docker heartbeat"
  else
    echo "[FAIL] worker is not reporting a healthy Docker heartbeat"
    failures=$((failures + 1))
  fi
fi
# The password is expanded by the shell inside the Redis container.
# shellcheck disable=SC2016
if "${COMPOSE[@]}" exec -T redis sh -ec 'redis-cli -a "$REDIS_PASSWORD" ping' 2>/dev/null | grep -q PONG; then
  echo "[OK]   Redis readiness"
else
  echo "[FAIL] Redis readiness"
  failures=$((failures + 1))
fi
check "Nginx health" curl -fsS "http://127.0.0.1:${HTTP_PORT}/healthz"
check "API liveness" curl -fsS "http://127.0.0.1:${HTTP_PORT}/api/v1/health"
check "API readiness" curl -fsS "http://127.0.0.1:${HTTP_PORT}/health"
check "Learning API" curl -fsS "http://127.0.0.1:${HTTP_PORT}/api/v1/learning/paths"
check "Website homepage" curl -fsS "http://127.0.0.1:${HTTP_PORT}/"

echo
"${COMPOSE[@]}" ps

if (( failures > 0 )); then
  echo
  echo "$failures 项检查失败，最近日志如下："
  "${COMPOSE[@]}" logs --tail=100 postgres redis init api web worker
  exit 1
fi

echo
echo "全部 Docker 检查通过。"
