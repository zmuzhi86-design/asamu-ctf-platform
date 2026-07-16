#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "${BASH_SOURCE[0]%/*}" && pwd -P)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd -P)"
cd "$PROJECT_DIR"

OUTPUT_DIR="${1:-$PROJECT_DIR/offline-dist}"
shift || true
ENV_FILE="${OFFLINE_ENV_FILE:-.env.docker}"
fail() { echo "错误：$*" >&2; exit 1; }
command -v docker >/dev/null 2>&1 || fail "未安装 Docker"
command -v gzip >/dev/null 2>&1 || fail "未安装 gzip"
command -v sha256sum >/dev/null 2>&1 || fail "未安装 sha256sum"

env_value() {
  local key="$1" fallback="$2" value=""
  if [[ -f "$ENV_FILE" ]]; then value="$(sed -n "s/^${key}=//p" "$ENV_FILE" | tail -n 1)"; fi
  printf '%s' "${value:-$fallback}"
}

APP_VERSION="$(env_value APP_VERSION 0.3.1)"
API_IMAGE="$(env_value API_IMAGE asamu/api)"
WEB_IMAGE="$(env_value WEB_IMAGE asamu/web)"
WORKER_IMAGE="$(env_value WORKER_IMAGE asamu/runtime-worker)"
POSTGRES_IMAGE="$(env_value POSTGRES_IMAGE postgres:16-alpine)"
REDIS_IMAGE="$(env_value REDIS_IMAGE redis:7.2-alpine)"
TRAEFIK_IMAGE="$(env_value TRAEFIK_IMAGE traefik:v3.5.0)"
[[ "$APP_VERSION" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ && "$APP_VERSION" != "latest" ]] || fail "APP_VERSION 无效或使用了 latest"

IMAGES=("${API_IMAGE}:${APP_VERSION}" "${WEB_IMAGE}:${APP_VERSION}" "${WORKER_IMAGE}:${APP_VERSION}" "$POSTGRES_IMAGE" "$REDIS_IMAGE" "$TRAEFIK_IMAGE")
RUNTIME_IMAGES=()
for image in "$@"; do
  [[ "$image" =~ ^[A-Za-z0-9][A-Za-z0-9._/@:-]+$ \
    && "${image##*/}" == *:* \
    && "$image" != *:latest ]] || fail "题目镜像必须是固定标签或 digest：$image"
  IMAGES+=("$image")
  RUNTIME_IMAGES+=("$image")
done
for image in "${IMAGES[@]}"; do docker image inspect "$image" >/dev/null 2>&1 || fail "本机缺少镜像：$image"; done

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
NAME="asamu-offline-${APP_VERSION}-${STAMP}"
STAGE="$(mktemp -d)"
ROOT="$STAGE/$NAME"
cleanup() { rm -rf "$STAGE"; }
trap cleanup EXIT INT TERM
mkdir -p "$ROOT/scripts" "$ROOT/deploy" "$ROOT/apps/web/public/assets/default" "$OUTPUT_DIR"

cp docker-compose.yml .env.docker.example "$ROOT/"
cp -R deploy/. "$ROOT/deploy/"
cp apps/web/public/assets/default/manifest.v3.json "$ROOT/apps/web/public/assets/default/"
cp scripts/offline-install.sh scripts/run-init.sh scripts/docker-doctor.sh scripts/enable-runtime.sh scripts/disable-runtime.sh scripts/purge-installation.sh scripts/backup.sh scripts/restore.sh scripts/upgrade.sh scripts/rollback.sh "$ROOT/scripts/"

RUNTIME_CSV="$(IFS=,; echo "${RUNTIME_IMAGES[*]}")"
{
  echo "APP_VERSION=$APP_VERSION"
  echo "API_IMAGE=$API_IMAGE"
  echo "WEB_IMAGE=$WEB_IMAGE"
  echo "WORKER_IMAGE=$WORKER_IMAGE"
  echo "POSTGRES_IMAGE=$POSTGRES_IMAGE"
  echo "REDIS_IMAGE=$REDIS_IMAGE"
  echo "TRAEFIK_IMAGE=$TRAEFIK_IMAGE"
  echo "RUNTIME_ALLOWED_IMAGES=$RUNTIME_CSV"
} > "$ROOT/bundle.env"
printf '%s\n' "${IMAGES[@]}" > "$ROOT/images.list"

echo "导出 ${#IMAGES[@]} 个固定版本镜像…"
docker save "${IMAGES[@]}" | gzip -1 > "$ROOT/images.tar.gz"
(
  cd "$ROOT"
  find scripts deploy -type f -print0 | sort -z | xargs -0 sha256sum
  sha256sum images.tar.gz bundle.env images.list docker-compose.yml .env.docker.example apps/web/public/assets/default/manifest.v3.json
) > "$ROOT/payload.sha256"
tar -C "$STAGE" -czf "$OUTPUT_DIR/${NAME}.tar.gz" "$NAME"
(cd "$OUTPUT_DIR" && sha256sum "${NAME}.tar.gz" > "${NAME}.tar.gz.sha256")
chmod 600 "$OUTPUT_DIR/${NAME}.tar.gz" "$OUTPUT_DIR/${NAME}.tar.gz.sha256"
echo "离线包：$OUTPUT_DIR/${NAME}.tar.gz"
echo "校验：$OUTPUT_DIR/${NAME}.tar.gz.sha256"
