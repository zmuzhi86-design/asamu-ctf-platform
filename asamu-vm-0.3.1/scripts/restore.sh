#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "${BASH_SOURCE[0]%/*}" && pwd -P)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd -P)"
cd "$PROJECT_DIR"

ARCHIVE="${1:-}"
PASSPHRASE_FILE="${2:-${BACKUP_PASSPHRASE_FILE:-}}"
CONFIRMATION="${3:-}"
fail() { echo "错误：$*" >&2; exit 1; }

[[ -f "$ARCHIVE" ]] || fail "备份归档不存在"
ARCHIVE="$(cd "$(dirname "$ARCHIVE")" && pwd -P)/$(basename "$ARCHIVE")"
[[ -f "$ARCHIVE.sha256" ]] || fail "缺少归档校验文件：$ARCHIVE.sha256"
[[ -n "$PASSPHRASE_FILE" && -f "$PASSPHRASE_FILE" ]] || fail "缺少备份口令文件"
[[ "$CONFIRMATION" == "RESTORE" ]] || fail "恢复会覆盖数据库和素材，请将第三个参数设为 RESTORE"
command -v openssl >/dev/null 2>&1 || fail "未安装 openssl"
command -v docker >/dev/null 2>&1 || fail "未安装 Docker"

(cd "$(dirname "$ARCHIVE")" && sha256sum -c "$(basename "$ARCHIVE").sha256")
STAGE="$(mktemp -d)"
cleanup() { rm -rf "$STAGE"; }
trap cleanup EXIT INT TERM
openssl enc -d -aes-256-cbc -pbkdf2 -iter 600000 -in "$ARCHIVE" -out "$STAGE/backup.tar.gz" -pass "file:$PASSPHRASE_FILE"
if tar -tzf "$STAGE/backup.tar.gz" | grep -Eq '(^/|(^|/)\.\.(/|$))'; then fail "归档包含不安全路径"; fi
tar -C "$STAGE" -xzf "$STAGE/backup.tar.gz"
BACKUP_FORMAT="$(sed -n 's/^format=//p' "$STAGE/manifest.txt")"
[[ "$BACKUP_FORMAT" == "asamu-backup-v1" || "$BACKUP_FORMAT" == "chain-mirror-backup-v1" ]] || fail "不支持的备份格式"
(cd "$STAGE" && sha256sum -c payload.sha256)

if [[ -f .env.docker && "${SKIP_PRE_RESTORE_BACKUP:-0}" != "1" ]]; then
  echo "恢复前创建应急备份…"
  "$SCRIPT_DIR/backup.sh" "${PRE_RESTORE_BACKUP_DIR:-$PROJECT_DIR/backups}" "$PASSPHRASE_FILE"
fi
TARGET_ENV=""
if [[ -f .env.docker ]]; then
  TARGET_ENV=".env.docker.pre-restore.$(date -u +%Y%m%dT%H%M%SZ)"
  install -m 600 .env.docker "$TARGET_ENV"
fi
install -m 600 "$STAGE/env.docker" .env.docker
if [[ -n "$TARGET_ENV" ]]; then
  preserve_target_value() {
    local key="$1" value
    value="$(sed -n "s/^${key}=//p" "$TARGET_ENV" | tail -n 1)"
    [[ -n "$value" ]] || return 0
    if grep -q "^${key}=" .env.docker; then
      sed -i "s|^${key}=.*$|${key}=${value}|" .env.docker
    else
      printf '%s=%s\n' "$key" "$value" >> .env.docker
    fi
  }
  for key in ASAMU_LOCAL_BUILD CHAIN_MIRROR_LOCAL_BUILD APP_VERSION API_IMAGE WEB_IMAGE WORKER_IMAGE POSTGRES_IMAGE REDIS_IMAGE TRAEFIK_IMAGE HTTP_PORT PUBLIC_BASE_URL WEB_ORIGINS COOKIE_SECURE POSTGRES_PASSWORD POSTGRES_USER POSTGRES_DB POSTGRES_VOLUME_NAME REDIS_VOLUME_NAME ASSET_VOLUME_NAME BUILDKIT_VOLUME_NAME DATABASE_URL RUNTIME_ENABLED RUNTIME_PUBLIC_HOST RUNTIME_BIND_HOST RUNTIME_PORT_MIN RUNTIME_PORT_MAX; do
    preserve_target_value "$key"
  done
fi

env_value() { sed -n "s/^$1=//p" .env.docker | tail -n 1; }
upsert_env() {
  local key="$1" value="$2"
  if grep -q "^${key}=" .env.docker; then sed -i "s|^${key}=.*$|${key}=${value}|" .env.docker; else printf '%s=%s\n' "$key" "$value" >> .env.docker; fi
}
if [[ -z "$(env_value ASAMU_LOCAL_BUILD)" && -n "$(env_value CHAIN_MIRROR_LOCAL_BUILD)" ]]; then
  upsert_env ASAMU_LOCAL_BUILD "$(env_value CHAIN_MIRROR_LOCAL_BUILD)"
fi
DATABASE_URL_CURRENT="$(env_value DATABASE_URL)"
DATABASE_USER_CURRENT="$(printf '%s' "$DATABASE_URL_CURRENT" | sed -nE 's#^postgres://([^:]+):.*@[^/]+/([^?]+).*$#\1#p')"
DATABASE_NAME_CURRENT="$(printf '%s' "$DATABASE_URL_CURRENT" | sed -nE 's#^postgres://([^:]+):.*@[^/]+/([^?]+).*$#\2#p')"
if [[ -z "$(env_value POSTGRES_USER)" ]]; then upsert_env POSTGRES_USER "${DATABASE_USER_CURRENT:-asamu}"; fi
if [[ -z "$(env_value POSTGRES_DB)" ]]; then upsert_env POSTGRES_DB "${DATABASE_NAME_CURRENT:-asamu}"; fi
if [[ -z "$(env_value POSTGRES_VOLUME_NAME)" ]]; then upsert_env POSTGRES_VOLUME_NAME asamu_postgres-data; fi
if [[ -z "$(env_value REDIS_VOLUME_NAME)" ]]; then upsert_env REDIS_VOLUME_NAME asamu_redis-data; fi
if [[ -z "$(env_value ASSET_VOLUME_NAME)" ]]; then upsert_env ASSET_VOLUME_NAME asamu_asset-data; fi
if [[ -z "$(env_value BUILDKIT_VOLUME_NAME)" ]]; then upsert_env BUILDKIT_VOLUME_NAME asamu_buildkit-state; fi
if [[ "$(env_value API_IMAGE)" == "chainmirror/api" ]]; then upsert_env API_IMAGE asamu/api; fi
if [[ "$(env_value WEB_IMAGE)" == "chainmirror/web" ]]; then upsert_env WEB_IMAGE asamu/web; fi
if [[ "$(env_value WORKER_IMAGE)" == "chainmirror/runtime-worker" ]]; then upsert_env WORKER_IMAGE asamu/runtime-worker; fi
if [[ "$(env_value RUNTIME_WORKER_ID)" == "chainmirror-runtime-worker" ]]; then upsert_env RUNTIME_WORKER_ID asamu-runtime-worker; fi

COMPOSE=(docker compose --env-file .env.docker)
if grep -Eq '^(ASAMU_LOCAL_BUILD|CHAIN_MIRROR_LOCAL_BUILD)=true$' .env.docker; then COMPOSE+=(-f docker-compose.yml -f docker-compose.build.yml); fi
if grep -q '^RUNTIME_ENABLED=true$' .env.docker; then COMPOSE+=(--profile runtime); fi
POSTGRES_USER="$(sed -n 's/^POSTGRES_USER=//p' .env.docker | tail -n 1)"
POSTGRES_DB="$(sed -n 's/^POSTGRES_DB=//p' .env.docker | tail -n 1)"
POSTGRES_USER="${POSTGRES_USER:-asamu}"
POSTGRES_DB="${POSTGRES_DB:-asamu}"
"${COMPOSE[@]}" config --quiet
echo "停止应用并恢复持久化数据…"
"${COMPOSE[@]}" down --remove-orphans
"${COMPOSE[@]}" up -d postgres
for _ in $(seq 1 60); do
  if "${COMPOSE[@]}" exec -T postgres pg_isready -U "$POSTGRES_USER" -d postgres >/dev/null 2>&1; then break; fi
  sleep 1
done
"${COMPOSE[@]}" exec -T postgres dropdb -U "$POSTGRES_USER" --if-exists --force "$POSTGRES_DB"
"${COMPOSE[@]}" exec -T postgres createdb -U "$POSTGRES_USER" "$POSTGRES_DB"
"${COMPOSE[@]}" exec -T postgres pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --exit-on-error --no-owner --no-acl < "$STAGE/database.dump"

"${COMPOSE[@]}" run --rm -T --no-deps --entrypoint sh api -ec 'find /app/var/storage -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +; tar -xzf - -C /app/var/storage' < "$STAGE/assets.tar.gz"
"${COMPOSE[@]}" run --rm -T --no-deps --entrypoint sh redis -ec 'find /data -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +; tar -xzf - -C /data' < "$STAGE/redis-data.tar.gz"

"$SCRIPT_DIR/run-init.sh"
START_SERVICES=(api web)
if grep -q '^RUNTIME_ENABLED=true$' .env.docker; then START_SERVICES+=(worker); fi
"${COMPOSE[@]}" up -d --no-deps --remove-orphans "${START_SERVICES[@]}"
HTTP_PORT="$(sed -n 's/^HTTP_PORT=//p' .env.docker | tail -n 1)"
for attempt in $(seq 1 90); do
  if curl -fsS "http://127.0.0.1:${HTTP_PORT:-8080}/health" >/dev/null 2>&1; then break; fi
  [[ "$attempt" -lt 90 ]] || fail "恢复后 API 就绪检查未通过"
  sleep 2
done
echo "恢复完成。请执行：sudo ./scripts/docker-doctor.sh"
