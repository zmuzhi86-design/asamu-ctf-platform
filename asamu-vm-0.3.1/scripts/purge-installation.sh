#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "${BASH_SOURCE[0]%/*}" && pwd -P)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd -P)"
DELETE_PROJECT_FILES=false
CONFIRMED=false

usage() {
  cat <<'USAGE'
用法：
  sudo ./scripts/purge-installation.sh --yes [--delete-project-files]
  sudo ./scripts/purge-installation.sh --project-dir /旧/asamu/目录 --delete-project-files --yes

说明：
  --yes                   确认永久删除 asamu/chain-mirror 容器、动态题容器和数据卷
  --delete-project-files  清理完成后连同指定项目目录一起永久删除
  --project-dir PATH      从新版安装包清理另一个旧项目目录
USAGE
}

while (($#)); do
  case "$1" in
    --yes)
      CONFIRMED=true
      shift
      ;;
    --delete-project-files)
      DELETE_PROJECT_FILES=true
      shift
      ;;
    --project-dir)
      [[ $# -ge 2 && -n "$2" ]] || { echo "错误：--project-dir 缺少路径。" >&2; exit 2; }
      PROJECT_DIR="$(cd -- "$2" && pwd -P)"
      shift 2
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
[[ "$CONFIRMED" == true ]] || { echo "错误：这是永久删除操作，确认后请显式添加 --yes。" >&2; usage; exit 2; }
[[ -f "$PROJECT_DIR/docker-compose.yml" ]] || { echo "错误：目标不是 asamu 项目目录：$PROJECT_DIR" >&2; exit 1; }
grep -Eq '^name:[[:space:]]+(asamu|chain-mirror|chainmirror)[[:space:]]*$' "$PROJECT_DIR/docker-compose.yml" || {
  echo "错误：目标 compose 文件缺少 asamu/chain-mirror 项目标记，拒绝清理：$PROJECT_DIR" >&2
  exit 1
}
case "$PROJECT_DIR" in
  /|/bin|/boot|/dev|/etc|/home|/lib|/lib64|/opt|/proc|/root|/run|/sbin|/srv|/sys|/tmp|/usr|/var)
    echo "错误：拒绝清理系统目录：$PROJECT_DIR" >&2
    exit 1
    ;;
esac

if [[ "$DELETE_PROJECT_FILES" == true ]]; then
  if command -v getent >/dev/null 2>&1 && getent passwd | cut -d: -f6 | grep -Fxq "$PROJECT_DIR"; then
    echo "错误：拒绝删除用户 HOME 目录：$PROJECT_DIR" >&2
    exit 1
  fi
  if command -v mountpoint >/dev/null 2>&1 && mountpoint -q "$PROJECT_DIR"; then
    echo "错误：目标目录本身是挂载点，请先卸载后再删除：$PROJECT_DIR" >&2
    exit 1
  fi
fi

command -v docker >/dev/null 2>&1 || { echo "错误：未安装 Docker。" >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo "错误：Docker 守护进程不可用，未删除任何文件。" >&2; exit 1; }

ENV_FILE="$PROJECT_DIR/.env.docker"
custom_volumes=()
if [[ -f "$ENV_FILE" ]]; then
  for key in POSTGRES_VOLUME_NAME REDIS_VOLUME_NAME ASSET_VOLUME_NAME BUILDKIT_VOLUME_NAME; do
    value="$(sed -n "s/^${key}=//p" "$ENV_FILE" | tail -n 1)"
    if [[ "$value" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]*$ ]]; then custom_volumes+=("$value"); fi
  done
fi

echo "正在永久清理 $PROJECT_DIR 的容器、动态题实例、网络和数据卷……"
cd -- "$PROJECT_DIR"
if [[ -f .env.docker ]]; then
  docker compose --env-file .env.docker down -v --remove-orphans || echo "警告：Compose 清理未完全成功，将继续按标签清理并在删除文件前复核。" >&2
fi

compose_projects=(asamu chain-mirror chainmirror)
container_labels=(asamu.managed=true chainmirror.managed=true)
network_labels=(asamu.kind=range-network chainmirror.kind=range-network)

is_related_text() {
  local value="${1:-}"
  shopt -s nocasematch
  [[ "$value" =~ (^|[^[:alnum:]])asamu([^[:alnum:]]|$) || "$value" =~ chain[-_.]?[[:space:]]*mirror ]]
}

append_unique_project() {
  local candidate="${1:-}" existing
  [[ -n "$candidate" ]] || return 0
  is_related_text "$candidate" || return 0
  for existing in "${compose_projects[@]}"; do
    [[ "$existing" == "$candidate" ]] && return 0
  done
  compose_projects+=("$candidate")
}

# Old bundles sometimes used the extracted directory name as the Compose
# project. Discover those names before deleting containers so --fresh also
# removes volumes such as asamu-vm-0.2.1-r20260715_postgres-data.
while IFS= read -r project; do
  append_unique_project "$project"
done < <(docker ps -a --format '{{.Label "com.docker.compose.project"}}' | sort -u)

remove_containers_by_label() {
  local label="$1" output
  output="$(docker ps -aq --filter "label=$label")"
  [[ -z "$output" ]] && return 0
  mapfile -t ids <<<"$output"
  docker rm -f "${ids[@]}" >/dev/null || true
}

remove_networks_by_label() {
  local label="$1" output
  output="$(docker network ls -q --filter "label=$label")"
  [[ -z "$output" ]] && return 0
  mapfile -t ids <<<"$output"
  docker network rm "${ids[@]}" >/dev/null || true
}

for project in "${compose_projects[@]}"; do
  remove_containers_by_label "com.docker.compose.project=$project"
done
for label in "${container_labels[@]}"; do
  remove_containers_by_label "$label"
done

# Last-resort discovery for containers left by older bundle names or manually
# renamed Compose projects. The match uses container name, image and labels.
related_container_ids=()
while IFS=$'\t' read -r id name image labels; do
  if is_related_text "$name $image $labels"; then
    related_container_ids+=("$id")
  fi
done < <(docker ps -a --format '{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Labels}}')
if ((${#related_container_ids[@]})); then
  docker rm -f "${related_container_ids[@]}" >/dev/null || true
fi

for label in "${network_labels[@]}"; do
  remove_networks_by_label "$label"
done
for project in "${compose_projects[@]}"; do
  remove_networks_by_label "com.docker.compose.project=$project"
done

related_network_ids=()
while IFS=$'\t' read -r id name labels; do
  if is_related_text "$name $labels"; then
    related_network_ids+=("$id")
  fi
done < <(docker network ls --format '{{.ID}}\t{{.Name}}\t{{.Labels}}')
if ((${#related_network_ids[@]})); then
  docker network rm "${related_network_ids[@]}" >/dev/null 2>&1 || true
fi

declare -A volume_set=()
volumes=(
  asamu_postgres-data asamu_redis-data asamu_asset-data asamu_buildkit-state
  chain-mirror_postgres-data chain-mirror_redis-data chain-mirror_asset-data chain-mirror_buildkit-state
  chainmirror_postgres-data chainmirror_redis-data chainmirror_asset-data chainmirror_buildkit-state
  "${custom_volumes[@]}"
)
for name in "${volumes[@]}"; do [[ -z "$name" ]] || volume_set["$name"]=1; done
for project in "${compose_projects[@]}"; do
  output="$(docker volume ls -q --filter "label=com.docker.compose.project=$project")"
  if [[ -n "$output" ]]; then
    while IFS= read -r name; do [[ -z "$name" ]] || volume_set["$name"]=1; done <<<"$output"
  fi
done
while IFS=$'\t' read -r name labels; do
  if is_related_text "$name $labels"; then
    volume_set["$name"]=1
  fi
done < <(docker volume ls --format '{{.Name}}\t{{.Labels}}')

all_volumes="$(docker volume ls -q)"
for name in "${!volume_set[@]}"; do
  if grep -Fxq "$name" <<<"$all_volumes"; then docker volume rm -f "$name" >/dev/null 2>&1 || true; fi
done

# Destructive file deletion is committed only after Docker state is proven clean.
docker info >/dev/null 2>&1 || { echo "错误：清理后无法连接 Docker，保留凭据和项目文件。" >&2; exit 1; }
remaining=()
for project in "${compose_projects[@]}"; do
  output="$(docker ps -aq --filter "label=com.docker.compose.project=$project")"
  [[ -z "$output" ]] || remaining+=("compose containers ($project): $output")
done
for label in "${container_labels[@]}"; do
  output="$(docker ps -aq --filter "label=$label")"
  [[ -z "$output" ]] || remaining+=("runtime containers ($label): $output")
done
for project in "${compose_projects[@]}"; do
  output="$(docker network ls -q --filter "label=com.docker.compose.project=$project")"
  [[ -z "$output" ]] || remaining+=("compose networks ($project): $output")
done
for label in "${network_labels[@]}"; do
  output="$(docker network ls -q --filter "label=$label")"
  [[ -z "$output" ]] || remaining+=("runtime networks ($label): $output")
done
all_volumes="$(docker volume ls -q)"
for name in "${!volume_set[@]}"; do
  if grep -Fxq "$name" <<<"$all_volumes"; then
    users="$(docker ps -a --filter "volume=$name" --format '{{.ID}}:{{.Names}}' | paste -sd, -)"
    remaining+=("volume $name${users:+ (used by $users)}")
  fi
done
while IFS=$'\t' read -r id name image labels; do
  if is_related_text "$name $image $labels"; then
    remaining+=("related container $id:$name ($image)")
  fi
done < <(docker ps -a --format '{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Labels}}')
while IFS=$'\t' read -r id name labels; do
  if is_related_text "$name $labels"; then
    remaining+=("related network $id:$name")
  fi
done < <(docker network ls --format '{{.ID}}\t{{.Name}}\t{{.Labels}}')
if ((${#remaining[@]})); then
  echo "错误：以下 Docker 资源未能删除；凭据、备份和项目文件均已保留：" >&2
  printf '  - %s\n' "${remaining[@]}" >&2
  exit 1
fi

rm -f -- "$PROJECT_DIR/.env.docker" "$PROJECT_DIR/deployment-credentials.txt"
rm -rf -- "$PROJECT_DIR/backups" "$PROJECT_DIR/offline-dist"

if [[ "$DELETE_PROJECT_FILES" == true ]]; then
  parent="$(dirname "$PROJECT_DIR")"
  cd -- "$parent"
  rm -rf --one-file-system -- "$PROJECT_DIR"
  echo "旧项目目录已删除：$PROJECT_DIR"
else
  echo "旧数据已清空；项目源文件保留在：$PROJECT_DIR"
fi
