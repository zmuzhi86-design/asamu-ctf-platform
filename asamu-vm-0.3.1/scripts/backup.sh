#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "${BASH_SOURCE[0]%/*}" && pwd -P)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd -P)"
cd "$PROJECT_DIR"

DESTINATION="${1:-$PROJECT_DIR/backups}"
PASSPHRASE_FILE="${2:-${BACKUP_PASSPHRASE_FILE:-}}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
NAME="asamu-${STAMP}"

fail() { echo "错误：$*" >&2; exit 1; }
[[ -f .env.docker ]] || fail "缺少 .env.docker"
command -v docker >/dev/null 2>&1 || fail "未安装 Docker"
docker compose version >/dev/null 2>&1 || fail "Docker Compose 不可用"
command -v openssl >/dev/null 2>&1 || fail "未安装 openssl"
command -v sha256sum >/dev/null 2>&1 || fail "未安装 sha256sum"
[[ -n "$PASSPHRASE_FILE" && -f "$PASSPHRASE_FILE" && ! -L "$PASSPHRASE_FILE" ]] || fail "请提供非符号链接的备份口令文件：backup.sh [目录] [口令文件]"
[[ "$(tr -d '\r\n' < "$PASSPHRASE_FILE" | wc -c)" -ge 20 ]] || fail "备份口令至少 20 个字符"
if find "$PASSPHRASE_FILE" -perm /077 -print -quit | grep -q .; then
  fail "口令文件权限过宽，请执行 chmod 600"
fi

mkdir -p "$DESTINATION"
DESTINATION="$(cd "$DESTINATION" && pwd -P)"
STAGE="$(mktemp -d)"
PLAIN_ARCHIVE="$STAGE/${NAME}.tar.gz"
OUTPUT="$DESTINATION/${NAME}.tar.gz.enc"
COMPOSE=(docker compose --env-file .env.docker)
if grep -q '^RUNTIME_ENABLED=true$' .env.docker; then COMPOSE+=(--profile runtime); fi
# Redis is stopped while its volume is archived. Include it explicitly when
# resuming because --no-deps intentionally prevents Compose from starting it
# through the API/worker dependency graph.
APP_SERVICES=(redis api web)
if grep -q '^RUNTIME_ENABLED=true$' .env.docker; then APP_SERVICES+=(worker); fi
POSTGRES_USER="$(sed -n 's/^POSTGRES_USER=//p' .env.docker | tail -n 1)"
POSTGRES_DB="$(sed -n 's/^POSTGRES_DB=//p' .env.docker | tail -n 1)"
POSTGRES_USER="${POSTGRES_USER:-asamu}"
POSTGRES_DB="${POSTGRES_DB:-asamu}"

resume_services() {
  local code=$?
	trap - EXIT INT TERM
  "${COMPOSE[@]}" up -d --no-deps --remove-orphans "${APP_SERVICES[@]}" >/dev/null 2>&1 || true
  if [[ "$code" -ne 0 ]]; then rm -f "$OUTPUT" "$OUTPUT.sha256"; fi
  rm -rf "$STAGE"
  exit "$code"
}
trap resume_services EXIT INT TERM

"${COMPOSE[@]}" config --quiet
"${COMPOSE[@]}" up -d postgres redis
echo "进入短维护窗口，暂停 API、Worker 与 Web…"
"${COMPOSE[@]}" stop api worker web >/dev/null 2>&1 || true

echo "备份 PostgreSQL…"
"${COMPOSE[@]}" exec -T postgres pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --compress=6 --no-owner --no-acl > "$STAGE/database.dump"

echo "备份本地素材卷…"
"${COMPOSE[@]}" run --rm -T --no-deps --entrypoint tar api -C /app/var/storage -czf - . > "$STAGE/assets.tar.gz"

echo "备份 Redis 持久化卷…"
"${COMPOSE[@]}" exec -T redis redis-cli -a "$(sed -n 's/^REDIS_PASSWORD=//p' .env.docker | tail -n 1)" SAVE >/dev/null
"${COMPOSE[@]}" stop redis >/dev/null
"${COMPOSE[@]}" run --rm -T --no-deps --entrypoint tar redis -C /data -czf - . > "$STAGE/redis-data.tar.gz"

install -m 600 .env.docker "$STAGE/env.docker"
{
  echo "format=asamu-backup-v1"
  echo "created_at=${STAMP}"
  echo "app_version=$(sed -n 's/^APP_VERSION=//p' .env.docker | tail -n 1)"
  echo "database_format=postgres-custom"
  echo "storage_driver=local"
} > "$STAGE/manifest.txt"
(cd "$STAGE" && sha256sum database.dump assets.tar.gz redis-data.tar.gz env.docker manifest.txt > payload.sha256)
tar -C "$STAGE" -czf "$PLAIN_ARCHIVE" database.dump assets.tar.gz redis-data.tar.gz env.docker manifest.txt payload.sha256

echo "加密归档…"
openssl enc -aes-256-cbc -salt -pbkdf2 -iter 600000 -in "$PLAIN_ARCHIVE" -out "$OUTPUT" -pass "file:$PASSPHRASE_FILE"
(cd "$DESTINATION" && sha256sum "${NAME}.tar.gz.enc" > "${NAME}.tar.gz.enc.sha256")
chmod 600 "$OUTPUT" "$OUTPUT.sha256"

trap - EXIT INT TERM
"${COMPOSE[@]}" up -d --no-deps --remove-orphans "${APP_SERVICES[@]}" >/dev/null
rm -rf "$STAGE"
echo "备份完成：$OUTPUT"
echo "校验文件：$OUTPUT.sha256"
