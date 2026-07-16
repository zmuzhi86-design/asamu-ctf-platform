#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "${BASH_SOURCE[0]%/*}" && pwd -P)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd -P)"
CONFIRMED=false

usage() {
  cat <<'USAGE'
用法：
  sudo ./scripts/reset-runtime-state.sh --yes

说明：
  仅重置动态靶场运行状态、端口租约、运行任务和 Redis 任务流。
  不删除用户、题目、比赛、提交记录、素材和管理员账号。
  所有正在运行的动态靶场容器会被强制关闭。
USAGE
}

while (($#)); do
  case "$1" in
    --yes)
      CONFIRMED=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "错误：未知参数 $1" >&2
      usage
      exit 2
      ;;
  esac
done

[[ "$(id -u)" -eq 0 ]] || { echo "错误：请使用 sudo 运行。" >&2; exit 1; }
[[ "$CONFIRMED" == true ]] || { echo "错误：请显式添加 --yes 确认重置。" >&2; usage; exit 2; }
[[ -f "$PROJECT_DIR/docker-compose.yml" && -f "$PROJECT_DIR/.env.docker" ]] || {
  echo "错误：没有找到 docker-compose.yml 或 .env.docker：$PROJECT_DIR" >&2
  exit 1
}
command -v docker >/dev/null 2>&1 || { echo "错误：未安装 Docker。" >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo "错误：Docker 守护进程不可用。" >&2; exit 1; }

cd "$PROJECT_DIR"

COMPOSE=(docker compose --env-file .env.docker)
if grep -Eq '^(ASAMU_LOCAL_BUILD|CHAIN_MIRROR_LOCAL_BUILD)=true$' .env.docker; then
  COMPOSE+=(-f docker-compose.yml -f docker-compose.build.yml)
fi
COMPOSE+=(--profile runtime)

"${COMPOSE[@]}" config --quiet

echo "【1/7】停止 API 和 Worker，冻结新的运行任务……"
"${COMPOSE[@]}" stop worker api >/dev/null 2>&1 || true

echo "【2/7】确保 PostgreSQL 和 Redis 可用……"
"${COMPOSE[@]}" up -d postgres redis
for _ in $(seq 1 60); do
  postgres_id="$("${COMPOSE[@]}" ps -q postgres)"
  redis_id="$("${COMPOSE[@]}" ps -q redis)"
  postgres_health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$postgres_id" 2>/dev/null || true)"
  redis_health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$redis_id" 2>/dev/null || true)"
  if [[ "$postgres_health" == healthy && "$redis_health" == healthy ]]; then
    break
  fi
  sleep 2
done
[[ "${postgres_health:-}" == healthy ]] || { echo "错误：PostgreSQL 未进入 healthy。" >&2; exit 1; }
[[ "${redis_health:-}" == healthy ]] || { echo "错误：Redis 未进入 healthy。" >&2; exit 1; }

echo "【3/7】删除动态靶场容器……"
runtime_container_ids="$(
  {
    docker ps -aq --filter label=asamu.managed=true
    docker ps -aq --filter label=chainmirror.managed=true
  } | sort -u
)"
if [[ -n "$runtime_container_ids" ]]; then
  mapfile -t ids <<<"$runtime_container_ids"
  docker rm -f "${ids[@]}" >/dev/null
fi

echo "【4/7】删除动态靶场隔离网络……"
runtime_network_ids="$(
  {
    docker network ls -q --filter label=asamu.kind=range-network
    docker network ls -q --filter label=chainmirror.kind=range-network
  } | sort -u
)"
if [[ -n "$runtime_network_ids" ]]; then
  mapfile -t ids <<<"$runtime_network_ids"
  docker network rm "${ids[@]}" >/dev/null 2>&1 || true
fi

echo "【5/7】重置数据库运行状态和端口租约……"
cat <<'SQL' | "${COMPOSE[@]}" exec -T postgres sh -lc 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
BEGIN;

UPDATE instance_runtime_jobs
SET status='failed',
    locked_at=NULL,
    finished_at=NOW(),
    last_error='RUNTIME_STATE_RESET'
WHERE status IN ('queued','running');

UPDATE runtime_operations
SET status='failed',
    error_code='RUNTIME_STATE_RESET',
    error_message='管理员执行动态靶场状态重置',
    completed_at=NOW()
WHERE status IN ('pending','running','retrying');

UPDATE instance_operations
SET result='failed',
    error_code='RUNTIME_STATE_RESET',
    finished_at=NOW()
WHERE result='pending';

UPDATE runtime_port_leases
SET status='released',
    released_at=NOW(),
    expires_at=NOW(),
    last_error_code='RUNTIME_STATE_RESET'
WHERE status IN ('reserved','active','releasing');

UPDATE instance_ports
SET released_at=NOW()
WHERE released_at IS NULL;

UPDATE challenge_instances
SET status='interrupted',
    runtime_id='',
    runtime_network_id='',
    access_url='',
    host_port=0,
    operation_id=NULL,
    expires_at=NULL,
    heartbeat_at=NULL,
    error_code='RUNTIME_STATE_RESET',
    last_error_code='RUNTIME_STATE_RESET',
    last_error_message='管理员执行动态靶场状态重置',
    updated_at=NOW(),
    status_version=status_version+1,
    version=version+1
WHERE status IN (
  'pending','pulling','creating','starting','running',
  'restarting','resetting','stopping'
)
OR COALESCE(host_port,0)<>0
OR runtime_id<>''
OR runtime_network_id<>'';

DELETE FROM runtime_port_leases;
DELETE FROM runtime_port_pool;

UPDATE runtime_usage_counters
SET active_instances=0,
    reserved_cpu_milli=0,
    reserved_memory_mb=0,
    updated_at=NOW()
WHERE active_instances<>0
   OR reserved_cpu_milli<>0
   OR reserved_memory_mb<>0;

UPDATE runtime_worker_nodes
SET status='offline',
    active_instances=0,
    reserved_cpu_milli=0,
    reserved_memory_mb=0,
    updated_at=NOW(),
    version=version+1;

COMMIT;
SQL

echo "【6/7】清空 Redis 运行任务流……"
REDIS_STREAM_VALUE="$(sed -n 's/^REDIS_STREAM=//p' .env.docker | tail -n 1)"
REDIS_STREAM_VALUE="${REDIS_STREAM_VALUE:-asamu.runtime.jobs}"
"${COMPOSE[@]}" exec -T redis sh -lc '
  stream="$1"
  redis-cli -a "$REDIS_PASSWORD" --no-auth-warning DEL "$stream" "$stream.dead" >/dev/null
' sh "$REDIS_STREAM_VALUE"

echo "【7/7】重新启动 API 和 Worker并执行自检……"
"${COMPOSE[@]}" up -d --no-deps --force-recreate api worker

for _ in $(seq 1 60); do
  worker_id="$("${COMPOSE[@]}" ps -q worker)"
  worker_status="$(docker inspect -f '{{.State.Status}}' "$worker_id" 2>/dev/null || true)"
  [[ "$worker_status" == running ]] && break
  sleep 2
done
[[ "${worker_status:-}" == running ]] || {
  echo "错误：Worker 未正常启动。" >&2
  "${COMPOSE[@]}" logs --tail=150 worker >&2 || true
  exit 1
}

read -r active_leases active_instances pending_jobs <<EOF
$(
  cat <<'SQL' | "${COMPOSE[@]}" exec -T postgres sh -lc 'psql -At -F " " -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
SELECT
  (SELECT COUNT(*) FROM runtime_port_leases WHERE status IN ('reserved','active','releasing')),
  (SELECT COUNT(*) FROM challenge_instances WHERE status IN ('pending','pulling','creating','starting','running','restarting','resetting','stopping')),
  (SELECT COUNT(*) FROM instance_runtime_jobs WHERE status IN ('queued','running'));
SQL
)
EOF

runtime_containers="$(
  {
    docker ps -aq --filter label=asamu.managed=true
    docker ps -aq --filter label=chainmirror.managed=true
  } | sed '/^$/d' | wc -l
)"
runtime_networks="$(
  {
    docker network ls -q --filter label=asamu.kind=range-network
    docker network ls -q --filter label=chainmirror.kind=range-network
  } | sed '/^$/d' | wc -l
)"

printf '\n%-28s %s\n' "活跃端口租约" "$active_leases"
printf '%-28s %s\n' "活跃/过渡中实例" "$active_instances"
printf '%-28s %s\n' "待处理运行任务" "$pending_jobs"
printf '%-28s %s\n' "遗留靶场容器" "$runtime_containers"
printf '%-28s %s\n' "遗留靶场网络" "$runtime_networks"

if [[ "$active_leases" != 0 || "$active_instances" != 0 || "$pending_jobs" != 0 || "$runtime_containers" != 0 || "$runtime_networks" != 0 ]]; then
  echo "错误：动态靶场状态仍有残留，请查看 Worker 日志。" >&2
  "${COMPOSE[@]}" logs --tail=150 worker >&2 || true
  exit 1
fi

echo
echo "动态靶场运行状态已重置。重新进入题目页面即可创建新环境。"
