#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "${BASH_SOURCE[0]%/*}" && pwd -P)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd -P)"
cd "$PROJECT_DIR"

PUBLIC_HOST="${1:-}"
HTTP_PORT="${2:-8080}"
MODE="${3:-}"
fail() { echo "错误：$*" >&2; exit 1; }
[[ "$PUBLIC_HOST" =~ ^[A-Za-z0-9.-]+$ ]] || fail "请提供有效的公网 IP 或域名"
if [[ ! "$HTTP_PORT" =~ ^[0-9]+$ ]] || (( HTTP_PORT < 1 || HTTP_PORT > 65535 )); then
  fail "网站端口无效"
fi
[[ -z "$MODE" || "$MODE" == "--runtime" ]] || fail "第三个参数仅支持 --runtime"
[[ ! -e .env.docker ]] || fail ".env.docker 已存在；离线安装只用于全新环境，升级请使用 upgrade.sh"
for tool in docker openssl gzip sha256sum curl; do command -v "$tool" >/dev/null 2>&1 || fail "缺少命令：$tool"; done
docker compose version >/dev/null 2>&1 || fail "Docker Compose 不可用"
[[ -f bundle.env && -f images.tar.gz && -f payload.sha256 ]] || fail "离线包不完整"
sha256sum -c payload.sha256

bundle_value() { sed -n "s/^$1=//p" bundle.env | tail -n 1; }
for key in APP_VERSION API_IMAGE WEB_IMAGE WORKER_IMAGE POSTGRES_IMAGE REDIS_IMAGE TRAEFIK_IMAGE; do
  value="$(bundle_value "$key")"
  [[ "$value" =~ ^[A-Za-z0-9][A-Za-z0-9._/@:-]+$ ]] || fail "bundle.env 中的 $key 非法"
done

echo "从离线包导入镜像…"
gzip -dc images.tar.gz | docker load
cp .env.docker.example .env.docker
upsert_env() {
  local key="$1" value="$2"
  if grep -q "^${key}=" .env.docker; then sed -i "s|^${key}=.*$|${key}=${value}|" .env.docker; else printf '%s=%s\n' "$key" "$value" >> .env.docker; fi
}

if [[ "$HTTP_PORT" == "80" ]]; then PUBLIC_URL="http://${PUBLIC_HOST}"; else PUBLIC_URL="http://${PUBLIC_HOST}:${HTTP_PORT}"; fi
POSTGRES_PASSWORD="$(openssl rand -hex 24)"
REDIS_PASSWORD="$(openssl rand -hex 24)"
ADMIN_PASSWORD="$(openssl rand -base64 24 | tr -d '\n' | tr '/+' '_-')"
STUDENT_PASSWORD="$(openssl rand -base64 18 | tr -d '\n' | tr '/+' '_-')"
upsert_env APP_VERSION "$(bundle_value APP_VERSION)"
upsert_env API_IMAGE "$(bundle_value API_IMAGE)"
upsert_env WEB_IMAGE "$(bundle_value WEB_IMAGE)"
upsert_env WORKER_IMAGE "$(bundle_value WORKER_IMAGE)"
upsert_env POSTGRES_IMAGE "$(bundle_value POSTGRES_IMAGE)"
upsert_env REDIS_IMAGE "$(bundle_value REDIS_IMAGE)"
upsert_env TRAEFIK_IMAGE "$(bundle_value TRAEFIK_IMAGE)"
upsert_env APP_ENV production
upsert_env HTTP_PORT "$HTTP_PORT"
upsert_env PUBLIC_BASE_URL "$PUBLIC_URL"
upsert_env WEB_ORIGINS "$PUBLIC_URL"
upsert_env COOKIE_SECURE false
upsert_env POSTGRES_PASSWORD "$POSTGRES_PASSWORD"
upsert_env DATABASE_URL "postgres://asamu:${POSTGRES_PASSWORD}@postgres:5432/asamu?sslmode=disable"
upsert_env REDIS_PASSWORD "$REDIS_PASSWORD"
upsert_env JWT_ACCESS_SECRET "$(openssl rand -hex 48)"
upsert_env FLAG_HMAC_SECRET "$(openssl rand -hex 48)"
upsert_env FLAG_ENCRYPTION_KEY_BASE64 "$(openssl rand -base64 32 | tr -d '\n')"
upsert_env REGISTRY_CREDENTIAL_ENCRYPTION_KEY_BASE64 "$(openssl rand -base64 32 | tr -d '\n')"
upsert_env CONFIRMATION_TOKEN_SECRET "$(openssl rand -hex 48)"
upsert_env MAIL_PUBLIC_BASE_URL "$PUBLIC_URL"
upsert_env RUNTIME_PUBLIC_HOST "$PUBLIC_HOST"
upsert_env RUNTIME_WORKER_INTERNAL_API_URL http://api:8787/api/v1
upsert_env RUNTIME_WORKER_API_TOKEN "$(openssl rand -hex 48)"
upsert_env RUNTIME_ALLOWED_IMAGES "$(bundle_value RUNTIME_ALLOWED_IMAGES)"
upsert_env RUNTIME_ENABLED "$([[ "$MODE" == "--runtime" ]] && echo true || echo false)"
upsert_env RUNTIME_BIND_HOST "$([[ "$MODE" == "--runtime" ]] && echo 0.0.0.0 || echo 127.0.0.1)"
upsert_env RUNTIME_PULL_MISSING_IMAGES false
upsert_env SEED_DEMO_CONTENT false
upsert_env DEV_ADMIN_PASSWORD "$ADMIN_PASSWORD"
upsert_env DEV_USER_PASSWORD "$STUDENT_PASSWORD"
chmod 600 .env.docker
chmod 755 scripts/*.sh

umask 077
{
  echo "URL: $PUBLIC_URL"
  echo "Admin: admin / $ADMIN_PASSWORD"
  echo "Student: student / $STUDENT_PASSWORD"
  echo "Generated: $(date --iso-8601=seconds)"
} > deployment-credentials.txt

COMPOSE=(docker compose --env-file .env.docker)
if [[ "$MODE" == "--runtime" ]]; then COMPOSE+=(--profile runtime); fi
"${COMPOSE[@]}" config --quiet
START_SERVICES=(api web)
if [[ "$MODE" == "--runtime" ]]; then START_SERVICES+=(worker); fi
"$SCRIPT_DIR/run-init.sh"
"${COMPOSE[@]}" up -d --no-deps --remove-orphans "${START_SERVICES[@]}"
for attempt in $(seq 1 90); do
  if curl -fsS "http://127.0.0.1:${HTTP_PORT}/healthz" >/dev/null 2>&1 && curl -fsS "http://127.0.0.1:${HTTP_PORT}/health" >/dev/null 2>&1; then break; fi
  [[ "$attempt" -lt 90 ]] || fail "离线安装健康检查未通过"
  sleep 2
done
echo "离线安装完成：$PUBLIC_URL"
echo "凭据：$PROJECT_DIR/deployment-credentials.txt"
