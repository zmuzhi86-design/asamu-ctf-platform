#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_PATH="${BASH_SOURCE[0]}"
SCRIPT_DIR="${SCRIPT_PATH%/*}"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd -P)"
cd "$PROJECT_DIR"

usage() {
  cat <<'USAGE'
用法：
  sudo ./scripts/deploy-ubuntu.sh <公网IP或域名> [网站端口] [--fresh]

示例：
  sudo ./scripts/deploy-ubuntu.sh 203.0.113.10
  sudo ./scripts/deploy-ubuntu.sh 203.0.113.10 8080
  sudo ./scripts/deploy-ubuntu.sh ctf.example.com 80 --fresh

说明：
  --fresh 会删除现有 asamu 容器和数据库卷，仅适合首次清理或确认不要旧数据时使用。
USAGE
}

if [[ "$(id -u)" -ne 0 ]]; then
  echo "错误：请使用 sudo 运行此脚本。" >&2
  usage
  exit 1
fi

PUBLIC_HOST="${1:-}"
HTTP_PORT="${2:-8080}"
MODE="${3:-}"
APP_VERSION_EXPLICIT="${APP_VERSION+x}"
API_IMAGE_EXPLICIT="${API_IMAGE+x}"
WEB_IMAGE_EXPLICIT="${WEB_IMAGE+x}"
WORKER_IMAGE_EXPLICIT="${WORKER_IMAGE+x}"
LOCAL_BUILD_EXPLICIT="${ASAMU_LOCAL_BUILD+x}"
LEGACY_LOCAL_BUILD_EXPLICIT="${CHAIN_MIRROR_LOCAL_BUILD+x}"
if [[ -z "$LOCAL_BUILD_EXPLICIT" && -n "$LEGACY_LOCAL_BUILD_EXPLICIT" ]]; then
  ASAMU_LOCAL_BUILD="$CHAIN_MIRROR_LOCAL_BUILD"
  LOCAL_BUILD_EXPLICIT=legacy
fi
if [[ -f .env.docker && "$MODE" != "--fresh" ]]; then
  if [[ -z "$APP_VERSION_EXPLICIT" ]]; then APP_VERSION="$(sed -n 's/^APP_VERSION=//p' .env.docker | tail -n 1)"; fi
  if [[ -z "$API_IMAGE_EXPLICIT" ]]; then API_IMAGE="$(sed -n 's/^API_IMAGE=//p' .env.docker | tail -n 1)"; fi
  if [[ -z "$WEB_IMAGE_EXPLICIT" ]]; then WEB_IMAGE="$(sed -n 's/^WEB_IMAGE=//p' .env.docker | tail -n 1)"; fi
  if [[ -z "$WORKER_IMAGE_EXPLICIT" ]]; then WORKER_IMAGE="$(sed -n 's/^WORKER_IMAGE=//p' .env.docker | tail -n 1)"; fi
  if [[ -z "$LOCAL_BUILD_EXPLICIT" ]]; then
    ASAMU_LOCAL_BUILD="$(sed -n 's/^ASAMU_LOCAL_BUILD=//p' .env.docker | tail -n 1)"
    if [[ -z "$ASAMU_LOCAL_BUILD" ]]; then
      ASAMU_LOCAL_BUILD="$(sed -n 's/^CHAIN_MIRROR_LOCAL_BUILD=//p' .env.docker | tail -n 1)"
    fi
  fi
fi
if [[ -z "$API_IMAGE_EXPLICIT" && "${API_IMAGE:-}" == "chainmirror/api" ]]; then API_IMAGE=asamu/api; fi
if [[ -z "$WEB_IMAGE_EXPLICIT" && "${WEB_IMAGE:-}" == "chainmirror/web" ]]; then WEB_IMAGE=asamu/web; fi
if [[ -z "$WORKER_IMAGE_EXPLICIT" && "${WORKER_IMAGE:-}" == "chainmirror/runtime-worker" ]]; then WORKER_IMAGE=asamu/runtime-worker; fi
APP_VERSION="${APP_VERSION:-0.3.1}"
API_IMAGE="${API_IMAGE:-asamu/api}"
WEB_IMAGE="${WEB_IMAGE:-asamu/web}"
WORKER_IMAGE="${WORKER_IMAGE:-asamu/runtime-worker}"
LOCAL_BUILD="${ASAMU_LOCAL_BUILD:-false}"
DEFAULT_RUNTIME_ENABLED=false
DEFAULT_RUNTIME_BIND_HOST=127.0.0.1
if [[ "$LOCAL_BUILD" == "true" ]]; then
  DEFAULT_RUNTIME_ENABLED=true
  DEFAULT_RUNTIME_BIND_HOST=0.0.0.0
fi

if [[ -z "$PUBLIC_HOST" || ! "$PUBLIC_HOST" =~ ^[A-Za-z0-9.-]+$ ]]; then
  echo "错误：请提供有效的公网 IPv4 地址或域名。" >&2
  usage
  exit 1
fi
if [[ ! "$HTTP_PORT" =~ ^[0-9]+$ ]] || (( HTTP_PORT < 1 || HTTP_PORT > 65535 )); then
  echo "错误：网站端口无效：$HTTP_PORT" >&2
  exit 1
fi
if [[ -n "$MODE" && "$MODE" != "--fresh" ]]; then
  echo "错误：未知参数：$MODE" >&2
  usage
  exit 1
fi
if [[ "$LOCAL_BUILD" != "true" && "$LOCAL_BUILD" != "false" ]]; then
  echo "错误：ASAMU_LOCAL_BUILD 仅支持 true 或 false。" >&2
  exit 1
fi

export DEBIAN_FRONTEND=noninteractive

install_docker() {
  if [[ ! -r /etc/os-release ]]; then
    echo "错误：无法识别操作系统。此脚本仅支持 Ubuntu。" >&2
    exit 1
  fi
  # shellcheck disable=SC1091
  . /etc/os-release
  if [[ "${ID:-}" != "ubuntu" ]]; then
    echo "错误：当前系统为 ${ID:-unknown}，此脚本仅支持 Ubuntu。" >&2
    exit 1
  fi

  apt-get update
  apt-get install -y ca-certificates curl openssl
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc

  cat > /etc/apt/sources.list.d/docker.sources <<APT
Types: deb
URIs: https://download.docker.com/linux/ubuntu
Suites: ${UBUNTU_CODENAME:-$VERSION_CODENAME}
Components: stable
Architectures: $(dpkg --print-architecture)
Signed-By: /etc/apt/keyrings/docker.asc
APT

  apt-get update
  apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
}

if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
  echo "正在从 Docker 官方软件源安装 Docker Engine 与 Compose 插件……"
  install_docker
else
  apt-get update
  apt-get install -y ca-certificates curl openssl
fi

systemctl enable --now docker
docker info >/dev/null
docker compose version

# Redis 的后台持久化和 fork 在内存紧张时依赖该内核参数。现场设置并持久化，
# 避免仅因宿主机默认值为 0 而在后续备份或 AOF 重写时失败。
printf '%s\n' 'vm.overcommit_memory = 1' > /etc/sysctl.d/99-asamu.conf
sysctl -w vm.overcommit_memory=1 >/dev/null

if [[ "$HTTP_PORT" == "80" ]]; then
  PUBLIC_URL="http://${PUBLIC_HOST}"
else
  PUBLIC_URL="http://${PUBLIC_HOST}:${HTTP_PORT}"
fi

upsert_env() {
  local key="$1"
  local value="$2"
  if grep -q "^${key}=" .env.docker 2>/dev/null; then
    sed -i "s|^${key}=.*$|${key}=${value}|" .env.docker
  else
    printf '%s=%s\n' "$key" "$value" >> .env.docker
  fi
}

env_value() {
  local key="$1"
  sed -n "s/^${key}=//p" .env.docker | tail -n 1
}

remove_env() {
  local key="$1"
  sed -i "/^${key}=/d" .env.docker
}

legacy_project_exists() {
  docker ps -aq --filter label=com.docker.compose.project=chain-mirror | grep -q .
}

if [[ "$MODE" == "--fresh" ]]; then
  bash "$PROJECT_DIR/scripts/purge-installation.sh" --project-dir "$PROJECT_DIR" --yes
fi

PREVIOUS_DEPLOY_VERSION=0
if [[ -f .env.docker ]]; then
  PREVIOUS_DEPLOY_VERSION="$(env_value ASAMU_DEPLOY_VERSION)"
fi
if [[ ! "$PREVIOUS_DEPLOY_VERSION" =~ ^[0-9]+$ ]]; then
  PREVIOUS_DEPLOY_VERSION=0
fi

POSTGRES_VOLUME="asamu_postgres-data"
LEGACY_POSTGRES_VOLUME="chain-mirror_postgres-data"
if [[ ! -f .env.docker ]] && { docker volume inspect "$POSTGRES_VOLUME" >/dev/null 2>&1 || docker volume inspect "$LEGACY_POSTGRES_VOLUME" >/dev/null 2>&1; }; then
  cat >&2 <<ERROR
错误：检测到已有 PostgreSQL 数据卷 ${POSTGRES_VOLUME}，但当前目录缺少 .env.docker。
数据库卷保存的是旧部署数据和旧密码；直接生成新密码会导致 PostgreSQL 认证失败。

请选择一种恢复方式：
  1. 保留数据：从原部署目录或备份恢复原 .env.docker 后重新运行本脚本。
  2. 首次安装且确认不要旧数据：在命令末尾显式添加 --fresh。
ERROR
  exit 1
fi

if [[ ! -f .env.docker ]]; then
  POSTGRES_PASSWORD="$(openssl rand -hex 24)"
  REDIS_PASSWORD="$(openssl rand -hex 24)"
  JWT_ACCESS_SECRET="$(openssl rand -hex 48)"
  FLAG_HMAC_SECRET="$(openssl rand -hex 48)"
  FLAG_ENCRYPTION_KEY_BASE64="$(openssl rand -base64 32 | tr -d '\n')"
  REGISTRY_CREDENTIAL_ENCRYPTION_KEY_BASE64="$(openssl rand -base64 32 | tr -d '\n')"
  RUNTIME_WORKER_API_TOKEN="$(openssl rand -hex 48)"
  CONFIRMATION_TOKEN_SECRET="$(openssl rand -hex 48)"
  ADMIN_PASSWORD="$(openssl rand -base64 24 | tr -d '\n' | tr '/+' '_-')"
  STUDENT_PASSWORD="$(openssl rand -base64 18 | tr -d '\n' | tr '/+' '_-')"

  umask 077
  cat > .env.docker <<ENV
ASAMU_DEPLOY_VERSION=8
ASAMU_LOCAL_BUILD=${LOCAL_BUILD}
APP_VERSION=${APP_VERSION}
API_IMAGE=${API_IMAGE}
WEB_IMAGE=${WEB_IMAGE}
WORKER_IMAGE=${WORKER_IMAGE}
APP_ENV=production
HTTP_ADDR=:8787
HTTP_PORT=${HTTP_PORT}
PUBLIC_BASE_URL=${PUBLIC_URL}
WEB_ORIGINS=${PUBLIC_URL},https://${PUBLIC_HOST}
COOKIE_SECURE=false
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
POSTGRES_USER=asamu
POSTGRES_DB=asamu
POSTGRES_VOLUME_NAME=asamu_postgres-data
REDIS_VOLUME_NAME=asamu_redis-data
ASSET_VOLUME_NAME=asamu_asset-data
BUILDKIT_VOLUME_NAME=asamu_buildkit-state
DATABASE_URL=postgres://asamu:${POSTGRES_PASSWORD}@postgres:5432/asamu?sslmode=disable
DATABASE_MAX_OPEN=40
DATABASE_MAX_IDLE=10
REDIS_PASSWORD=${REDIS_PASSWORD}
REDIS_ADDR=redis:6379
REDIS_DB=0
REDIS_STREAM=asamu.runtime.jobs
REDIS_CONSUMER_GROUP=asamu-workers
STORAGE_DRIVER=local
LOCAL_STORAGE_ROOT=/app/var/storage
JWT_ISSUER=asamu-platform
JWT_ACCESS_SECRET=${JWT_ACCESS_SECRET}
JWT_ACCESS_TTL=15m
REFRESH_TOKEN_TTL=720h
FLAG_HMAC_SECRET=${FLAG_HMAC_SECRET}
FLAG_ENCRYPTION_KEY_BASE64=${FLAG_ENCRYPTION_KEY_BASE64}
REGISTRY_CREDENTIAL_ENCRYPTION_KEY_BASE64=${REGISTRY_CREDENTIAL_ENCRYPTION_KEY_BASE64}
CONFIRMATION_TOKEN_SECRET=${CONFIRMATION_TOKEN_SECRET}
MAIL_PUBLIC_BASE_URL=${PUBLIC_URL}
RUNTIME_PROVIDER=docker
RUNTIME_ENABLED=${DEFAULT_RUNTIME_ENABLED}
RUNTIME_PUBLIC_HOST=${PUBLIC_HOST}
RUNTIME_BIND_HOST=${DEFAULT_RUNTIME_BIND_HOST}
RUNTIME_PORT_MIN=20000
RUNTIME_PORT_MAX=30000
RUNTIME_ALLOWED_IMAGES=
RUNTIME_PULL_MISSING_IMAGES=false
RUNTIME_WORKER_ID=asamu-runtime-worker
RUNTIME_WORKER_INTERNAL_API_URL=http://api:8787/api/v1
RUNTIME_WORKER_API_TOKEN=${RUNTIME_WORKER_API_TOKEN}
RUNTIME_WORKER_CPU_MILLI=4000
RUNTIME_WORKER_MEMORY_MB=8192
RUNTIME_WORKER_MAX_INSTANCES=50
RUNTIME_WORKER_PROTOCOLS=http,tcp,udp
SEED_DEMO_CONTENT=false
RATE_LIMIT_RPS=15
RATE_LIMIT_BURST=30
DOCS_ENABLED=false
DEV_ADMIN_EMAIL=admin@asamu.local
DEV_ADMIN_USERNAME=admin
DEV_ADMIN_PASSWORD=${ADMIN_PASSWORD}
DEV_USER_EMAIL=student@asamu.local
DEV_USER_USERNAME=student
DEV_USER_PASSWORD=${STUDENT_PASSWORD}
ENV
else
  echo "检测到现有 .env.docker，将保留数据库密码和账号密码。"
  DATABASE_URL_CURRENT="$(env_value DATABASE_URL)"
  DATABASE_USER_CURRENT="$(printf '%s' "$DATABASE_URL_CURRENT" | sed -nE 's#^postgres://([^:]+):.*@[^/]+/([^?]+).*$#\1#p')"
  DATABASE_NAME_CURRENT="$(printf '%s' "$DATABASE_URL_CURRENT" | sed -nE 's#^postgres://([^:]+):.*@[^/]+/([^?]+).*$#\2#p')"
  if ! grep -q '^POSTGRES_USER=' .env.docker; then upsert_env POSTGRES_USER "${DATABASE_USER_CURRENT:-asamu}"; fi
  if ! grep -q '^POSTGRES_DB=' .env.docker; then upsert_env POSTGRES_DB "${DATABASE_NAME_CURRENT:-asamu}"; fi
  if ! grep -q '^POSTGRES_VOLUME_NAME=' .env.docker; then
    if docker volume inspect chain-mirror_postgres-data >/dev/null 2>&1 && ! docker volume inspect asamu_postgres-data >/dev/null 2>&1; then
      upsert_env POSTGRES_VOLUME_NAME chain-mirror_postgres-data
      upsert_env REDIS_VOLUME_NAME chain-mirror_redis-data
      upsert_env ASSET_VOLUME_NAME chain-mirror_asset-data
      upsert_env BUILDKIT_VOLUME_NAME chain-mirror_buildkit-state
    else
      upsert_env POSTGRES_VOLUME_NAME asamu_postgres-data
      upsert_env REDIS_VOLUME_NAME asamu_redis-data
      upsert_env ASSET_VOLUME_NAME asamu_asset-data
      upsert_env BUILDKIT_VOLUME_NAME asamu_buildkit-state
    fi
  fi
  VOLUME_PREFIX=asamu
  if [[ "$(env_value POSTGRES_VOLUME_NAME)" == chain-mirror_* ]]; then VOLUME_PREFIX=chain-mirror; fi
  if ! grep -q '^REDIS_VOLUME_NAME=' .env.docker; then upsert_env REDIS_VOLUME_NAME "${VOLUME_PREFIX}_redis-data"; fi
  if ! grep -q '^ASSET_VOLUME_NAME=' .env.docker; then upsert_env ASSET_VOLUME_NAME "${VOLUME_PREFIX}_asset-data"; fi
  if ! grep -q '^BUILDKIT_VOLUME_NAME=' .env.docker; then upsert_env BUILDKIT_VOLUME_NAME "${VOLUME_PREFIX}_buildkit-state"; fi
  if [[ "$LOCAL_BUILD" == "true" && "$PREVIOUS_DEPLOY_VERSION" -lt 8 && "$(env_value RUNTIME_ENABLED)" != "true" ]]; then
    echo "正在为本地 Docker 靶场启用运行时 Worker。"
    upsert_env RUNTIME_ENABLED true
    upsert_env RUNTIME_BIND_HOST 0.0.0.0
  fi
  upsert_env ASAMU_DEPLOY_VERSION 8
  upsert_env ASAMU_LOCAL_BUILD "$LOCAL_BUILD"
  upsert_env APP_VERSION "$APP_VERSION"
  upsert_env API_IMAGE "$API_IMAGE"
  upsert_env WEB_IMAGE "$WEB_IMAGE"
  upsert_env WORKER_IMAGE "$WORKER_IMAGE"
  upsert_env APP_ENV production
  upsert_env HTTP_ADDR :8787
  upsert_env HTTP_PORT "$HTTP_PORT"
  upsert_env PUBLIC_BASE_URL "$PUBLIC_URL"
  upsert_env WEB_ORIGINS "${PUBLIC_URL},https://${PUBLIC_HOST}"
  upsert_env COOKIE_SECURE false
  upsert_env MAIL_PUBLIC_BASE_URL "$PUBLIC_URL"
  upsert_env STORAGE_DRIVER local
  upsert_env LOCAL_STORAGE_ROOT /app/var/storage
  if ! grep -q '^RUNTIME_ENABLED=' .env.docker; then
    upsert_env RUNTIME_ENABLED "$DEFAULT_RUNTIME_ENABLED"
    upsert_env RUNTIME_BIND_HOST "$DEFAULT_RUNTIME_BIND_HOST"
  fi
  upsert_env RUNTIME_PORT_MIN 20000
  upsert_env RUNTIME_PORT_MAX 30000
  if [[ "$(env_value RUNTIME_WORKER_ID)" == "chainmirror-runtime-worker" || -z "$(env_value RUNTIME_WORKER_ID)" ]]; then upsert_env RUNTIME_WORKER_ID asamu-runtime-worker; fi
  if ! grep -q '^RUNTIME_WORKER_INTERNAL_API_URL=' .env.docker; then upsert_env RUNTIME_WORKER_INTERNAL_API_URL http://api:8787/api/v1; fi
  if ! grep -q '^RUNTIME_WORKER_API_TOKEN=' .env.docker; then upsert_env RUNTIME_WORKER_API_TOKEN "$(openssl rand -hex 48)"; fi
  if ! grep -q '^REGISTRY_CREDENTIAL_ENCRYPTION_KEY_BASE64=' .env.docker; then upsert_env REGISTRY_CREDENTIAL_ENCRYPTION_KEY_BASE64 "$(openssl rand -base64 32 | tr -d '\n')"; fi
  if ! grep -q '^RUNTIME_WORKER_CPU_MILLI=' .env.docker; then upsert_env RUNTIME_WORKER_CPU_MILLI 4000; fi
  if ! grep -q '^RUNTIME_WORKER_MEMORY_MB=' .env.docker; then upsert_env RUNTIME_WORKER_MEMORY_MB 8192; fi
  if ! grep -q '^RUNTIME_WORKER_MAX_INSTANCES=' .env.docker; then upsert_env RUNTIME_WORKER_MAX_INSTANCES 50; fi
  if ! grep -q '^RUNTIME_WORKER_PROTOCOLS=' .env.docker; then upsert_env RUNTIME_WORKER_PROTOCOLS http,tcp,udp; fi
  upsert_env RUNTIME_PUBLIC_HOST "$PUBLIC_HOST"
  if [[ "$(env_value RUNTIME_ENABLED)" != "true" ]]; then
    upsert_env RUNTIME_BIND_HOST 127.0.0.1
  fi
  if [[ "$(env_value REDIS_STREAM)" == "chainmirror.runtime.jobs" ]]; then upsert_env REDIS_STREAM asamu.runtime.jobs; fi
  if [[ "$(env_value REDIS_CONSUMER_GROUP)" == "chainmirror-workers" ]]; then upsert_env REDIS_CONSUMER_GROUP asamu-workers; fi
  if [[ "$(env_value JWT_ISSUER)" == "chain-mirror-platform" ]]; then upsert_env JWT_ISSUER asamu-platform; fi
  remove_env CHAIN_MIRROR_DEPLOY_VERSION
  remove_env CHAIN_MIRROR_LOCAL_BUILD
  upsert_env SEED_DEMO_CONTENT false
fi

ADMIN_USER="$(env_value DEV_ADMIN_USERNAME)"
ADMIN_PASSWORD="$(env_value DEV_ADMIN_PASSWORD)"
STUDENT_USER="$(env_value DEV_USER_USERNAME)"
STUDENT_PASSWORD="$(env_value DEV_USER_PASSWORD)"

umask 077
cat > deployment-credentials.txt <<CREDS
URL: ${PUBLIC_URL}
Admin: ${ADMIN_USER:-admin} / ${ADMIN_PASSWORD:-请查看 .env.docker}
Student: ${STUDENT_USER:-student} / ${STUDENT_PASSWORD:-请查看 .env.docker}
Generated: $(date --iso-8601=seconds)
CREDS
chmod 600 .env.docker deployment-credentials.txt
chmod 755 scripts/install-local.sh scripts/run-init.sh scripts/docker-doctor.sh scripts/enable-runtime.sh scripts/disable-runtime.sh scripts/purge-installation.sh scripts/backup.sh scripts/restore.sh scripts/upgrade.sh scripts/rollback.sh scripts/export-offline-bundle.sh scripts/offline-install.sh

COMPOSE=(docker compose --env-file .env.docker)
if [[ "$LOCAL_BUILD" == "true" ]]; then
  COMPOSE+=(-f docker-compose.yml -f docker-compose.build.yml)
fi
if [[ "$(env_value RUNTIME_ENABLED)" == "true" ]]; then
  COMPOSE+=(--profile runtime)
fi

on_error() {
  local code=$?
  echo >&2
  echo "部署失败，退出码：$code" >&2
  "${COMPOSE[@]}" ps >&2 || true
  "${COMPOSE[@]}" logs --tail=150 postgres redis init api web worker >&2 || true
  if "${COMPOSE[@]}" logs --no-color init 2>&1 | grep -q "password authentication failed"; then
    cat >&2 <<'ERROR'

检测到 PostgreSQL 密码不匹配：当前 .env.docker 与已有数据库卷不是同一套部署凭据。
- 需要旧数据：恢复初始化该数据库卷时使用的原 .env.docker，然后重试。
- 不需要旧数据：确认后使用同一部署命令并在末尾添加 --fresh（会永久删除数据库卷）。
ERROR
  fi
  exit "$code"
}
trap on_error ERR

echo "正在校验 Compose 配置……"
"${COMPOSE[@]}" config --quiet

if [[ "$LOCAL_BUILD" == "true" ]]; then
  echo "正在 VM 内构建固定版本平台镜像……"
  BUILD_SERVICES=(api web worker)
  "${COMPOSE[@]}" build "${BUILD_SERVICES[@]}"
else
  echo "正在拉取版本化服务镜像……"
  "${COMPOSE[@]}" pull
fi

if legacy_project_exists; then
  echo "检测到旧项目容器，正在停止旧容器并复用原数据卷……"
  docker compose --project-name chain-mirror --env-file .env.docker down --remove-orphans
fi

echo "正在强制执行本版本数据库迁移与种子校验……"
"$SCRIPT_DIR/run-init.sh"

echo "正在启动网站服务……"
START_SERVICES=(api web)
if [[ "$(env_value RUNTIME_ENABLED)" == "true" ]]; then
  START_SERVICES+=(worker)
fi
"${COMPOSE[@]}" up -d --no-deps --remove-orphans "${START_SERVICES[@]}"

for attempt in $(seq 1 90); do
  if curl -fsS "http://127.0.0.1:${HTTP_PORT}/healthz" >/dev/null 2>&1 \
    && curl -fsS "http://127.0.0.1:${HTTP_PORT}/health" >/dev/null 2>&1 \
    && curl -fsS "http://127.0.0.1:${HTTP_PORT}/" >/dev/null 2>&1; then
    break
  fi
  if [[ "$attempt" -eq 90 ]]; then
    echo "错误：网站或 API 未通过启动检查。" >&2
    exit 1
  fi
  sleep 2
done

if [[ "$(env_value RUNTIME_ENABLED)" == "true" ]]; then
  DOCTOR_LOG="$(mktemp)"
  RUNTIME_READY=false
  for attempt in $(seq 1 30); do
    if bash "$SCRIPT_DIR/docker-doctor.sh" >"$DOCTOR_LOG" 2>&1; then
      RUNTIME_READY=true
      break
    fi
    [[ "$attempt" -lt 30 ]] || break
    sleep 2
  done
  if [[ "$RUNTIME_READY" != "true" ]]; then
    cat "$DOCTOR_LOG" >&2
    rm -f -- "$DOCTOR_LOG"
    echo "错误：Worker 未能注册健康心跳或读取宿主机 Docker 镜像。" >&2
    exit 1
  fi
  rm -f -- "$DOCTOR_LOG"
fi

trap - ERR

echo
echo "asamu 网站已启动：${PUBLIC_URL}"
echo "账号信息：${PROJECT_DIR}/deployment-credentials.txt"
if [[ "$(env_value RUNTIME_ENABLED)" == "true" ]]; then
  echo "云服务器安全组需放行 TCP ${HTTP_PORT}，以及动态靶场 TCP/UDP 20000-30000。"
else
  echo "云服务器安全组只需放行 TCP ${HTTP_PORT}。"
fi
echo "运行状态检查：sudo ./scripts/docker-doctor.sh"
