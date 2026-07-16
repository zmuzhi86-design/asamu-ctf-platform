#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "${BASH_SOURCE[0]%/*}" && pwd -P)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd -P)"
cd "$PROJECT_DIR"

TARGET_VERSION="${1:-}"
PASSPHRASE_FILE="${2:-${BACKUP_PASSPHRASE_FILE:-}}"
BACKUP_DIR="${BACKUP_DIR:-$PROJECT_DIR/backups}"
fail() { echo "错误：$*" >&2; return 1; }
[[ -f .env.docker ]] || fail "缺少 .env.docker"
[[ "$TARGET_VERSION" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ && "$TARGET_VERSION" != "latest" ]] || fail "目标版本无效，禁止使用 latest"
[[ -n "$PASSPHRASE_FILE" && -f "$PASSPHRASE_FILE" ]] || fail "缺少备份口令文件"

if ! grep -q '^REGISTRY_CREDENTIAL_ENCRYPTION_KEY_BASE64=' .env.docker; then
  printf 'REGISTRY_CREDENTIAL_ENCRYPTION_KEY_BASE64=%s\n' "$(openssl rand -base64 32 | tr -d '\n')" >> .env.docker
fi
if ! grep -q '^RUNTIME_WORKER_API_TOKEN=' .env.docker; then
  printf 'RUNTIME_WORKER_API_TOKEN=%s\n' "$(openssl rand -hex 48)" >> .env.docker
fi
if ! grep -q '^RUNTIME_WORKER_INTERNAL_API_URL=' .env.docker; then
  printf 'RUNTIME_WORKER_INTERNAL_API_URL=http://api:8787/api/v1\n' >> .env.docker
fi
chmod 600 .env.docker

CURRENT_VERSION="$(sed -n 's/^APP_VERSION=//p' .env.docker | tail -n 1)"
[[ "$TARGET_VERSION" != "$CURRENT_VERSION" ]] || fail "目标版本与当前版本相同"
echo "升级前创建一致性备份…"
"$SCRIPT_DIR/backup.sh" "$BACKUP_DIR" "$PASSPHRASE_FILE"

COMPOSE=(docker compose --env-file .env.docker)
LOCAL_BUILD=false
if grep -Eq '^(ASAMU_LOCAL_BUILD|CHAIN_MIRROR_LOCAL_BUILD)=true$' .env.docker; then
  LOCAL_BUILD=true
  COMPOSE+=(-f docker-compose.yml -f docker-compose.build.yml)
fi
if grep -q '^RUNTIME_ENABLED=true$' .env.docker; then COMPOSE+=(--profile runtime); fi
START_SERVICES=(api web)
if grep -q '^RUNTIME_ENABLED=true$' .env.docker; then START_SERVICES+=(worker); fi
if [[ "$LOCAL_BUILD" == "true" ]]; then
  echo "本地构建版本 $TARGET_VERSION…"
  APP_VERSION="$TARGET_VERSION" "${COMPOSE[@]}" build api web worker
else
  echo "预拉取版本 $TARGET_VERSION…"
  APP_VERSION="$TARGET_VERSION" "${COMPOSE[@]}" pull
fi
cp .env.docker ".env.docker.pre-upgrade.${CURRENT_VERSION}"
sed -i "s/^APP_VERSION=.*/APP_VERSION=${TARGET_VERSION}/" .env.docker

rollback_on_error() {
  local code=$?
  echo "升级失败，尝试回到应用版本 $CURRENT_VERSION（数据库迁移为扩展式，不执行破坏性 down）…" >&2
  sed -i "s/^APP_VERSION=.*/APP_VERSION=${CURRENT_VERSION}/" .env.docker
  "$SCRIPT_DIR/run-init.sh" >/dev/null 2>&1 || true
  "${COMPOSE[@]}" up -d --no-deps --remove-orphans "${START_SERVICES[@]}" || true
  exit "$code"
}
trap rollback_on_error ERR
"$SCRIPT_DIR/run-init.sh"
"${COMPOSE[@]}" up -d --no-deps --remove-orphans "${START_SERVICES[@]}"
HTTP_PORT="$(sed -n 's/^HTTP_PORT=//p' .env.docker | tail -n 1)"
for attempt in $(seq 1 90); do
  if curl -fsS "http://127.0.0.1:${HTTP_PORT:-8080}/healthz" >/dev/null 2>&1 && curl -fsS "http://127.0.0.1:${HTTP_PORT:-8080}/health" >/dev/null 2>&1; then break; fi
  [[ "$attempt" -lt 90 ]] || fail "升级后健康检查未通过"
  sleep 2
done
if grep -q '^RUNTIME_ENABLED=true$' .env.docker; then
  doctor_log="$(mktemp)"
  runtime_ready=false
  for attempt in $(seq 1 30); do
    if bash "$SCRIPT_DIR/docker-doctor.sh" >"$doctor_log" 2>&1; then runtime_ready=true; break; fi
    [[ "$attempt" -lt 30 ]] || break
    sleep 2
  done
  if [[ "$runtime_ready" != "true" ]]; then
    cat "$doctor_log" >&2
    rm -f -- "$doctor_log"
    fail "升级后的 Worker 健康检查未通过"
  fi
  rm -f -- "$doctor_log"
fi
trap - ERR
echo "已升级到 $TARGET_VERSION。保留版本回滚文件：.env.docker.pre-upgrade.${CURRENT_VERSION}"
