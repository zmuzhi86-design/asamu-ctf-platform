#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "${BASH_SOURCE[0]%/*}" && pwd -P)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd -P)"
cd "$PROJECT_DIR"

TARGET_VERSION="${1:-}"
PASSPHRASE_FILE="${2:-${BACKUP_PASSPHRASE_FILE:-}}"
CONFIRMATION="${3:-}"
fail() { echo "错误：$*" >&2; exit 1; }
[[ -f .env.docker ]] || fail "缺少 .env.docker"
[[ "$TARGET_VERSION" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ && "$TARGET_VERSION" != "latest" ]] || fail "回滚版本无效"
[[ "$CONFIRMATION" == "ROLLBACK" ]] || fail "请将第三个参数设为 ROLLBACK"
[[ -n "$PASSPHRASE_FILE" && -f "$PASSPHRASE_FILE" ]] || fail "缺少备份口令文件"

CURRENT_VERSION="$(sed -n 's/^APP_VERSION=//p' .env.docker | tail -n 1)"
echo "回滚前创建一致性备份…"
"$SCRIPT_DIR/backup.sh" "${BACKUP_DIR:-$PROJECT_DIR/backups}" "$PASSPHRASE_FILE"
COMPOSE=(docker compose --env-file .env.docker)
LOCAL_BUILD=false
if grep -Eq '^(ASAMU_LOCAL_BUILD|CHAIN_MIRROR_LOCAL_BUILD)=true$' .env.docker; then
  LOCAL_BUILD=true
  COMPOSE+=(-f docker-compose.yml -f docker-compose.build.yml)
fi
if grep -q '^RUNTIME_ENABLED=true$' .env.docker; then COMPOSE+=(--profile runtime); fi
if [[ "$LOCAL_BUILD" == "true" ]]; then
  API_IMAGE="$(sed -n 's/^API_IMAGE=//p' .env.docker | tail -n 1)"
  WEB_IMAGE="$(sed -n 's/^WEB_IMAGE=//p' .env.docker | tail -n 1)"
  WORKER_IMAGE="$(sed -n 's/^WORKER_IMAGE=//p' .env.docker | tail -n 1)"
  for image in "${API_IMAGE:-asamu/api}:$TARGET_VERSION" "${WEB_IMAGE:-asamu/web}:$TARGET_VERSION" "${WORKER_IMAGE:-asamu/runtime-worker}:$TARGET_VERSION"; do
    docker image inspect "$image" >/dev/null 2>&1 || fail "本地回滚镜像不存在：$image"
  done
else
  APP_VERSION="$TARGET_VERSION" "${COMPOSE[@]}" pull
fi
cp .env.docker ".env.docker.pre-rollback.${CURRENT_VERSION}"
sed -i "s/^APP_VERSION=.*/APP_VERSION=${TARGET_VERSION}/" .env.docker
"$SCRIPT_DIR/run-init.sh"
START_SERVICES=(api web)
if grep -q '^RUNTIME_ENABLED=true$' .env.docker; then START_SERVICES+=(worker); fi
"${COMPOSE[@]}" up -d --no-deps --remove-orphans "${START_SERVICES[@]}"
HTTP_PORT="$(sed -n 's/^HTTP_PORT=//p' .env.docker | tail -n 1)"
for attempt in $(seq 1 90); do
  if curl -fsS "http://127.0.0.1:${HTTP_PORT:-8080}/health" >/dev/null 2>&1; then break; fi
  [[ "$attempt" -lt 90 ]] || fail "回滚后 API 就绪检查未通过"
  sleep 2
done
echo "应用已切换到 $TARGET_VERSION。数据库仅支持扩展式兼容回滚；如需恢复数据，请使用 restore.sh。"
