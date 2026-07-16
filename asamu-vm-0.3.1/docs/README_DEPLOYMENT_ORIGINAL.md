# asamu — 精简后端 Docker 稳定版

这版以“网站先稳定运行，Docker 靶场按需启用”为目标。

## 默认服务

- PostgreSQL：业务数据
- Redis：限流、通知、提交保护和运行任务队列
- Init：迁移与初始化
- Go API：网站后端
- Nginx Web：前端和反向代理

默认不启动 Worker、不挂载 Docker Socket、不拉取题目镜像，也不开放靶场端口。

## 后端精简结果

已删除：

- Kubernetes Provider 与 Kubernetes SDK
- MinIO/S3 SDK（改为本地 Volume 存储）
- Prometheus SDK 与指标端点
- 旧 Node.js 模拟后端
- 缓存目录、Kubernetes 部署清单和无关生成文件

保留：

- PostgreSQL、Redis、Gin、GORM、JWT、数据库迁移
- 题库、比赛、战队、排名、Writeup、素材、通知、提交
- Docker 本地镜像启动和远程镜像拉取能力

## 部署网站

从精简 VM 源码包部署（在 Ubuntu VM 内构建镜像）：

```bash
chmod 755 scripts/*.sh
sudo env ASAMU_LOCAL_BUILD=true APP_VERSION=0.3.1 ./scripts/deploy-ubuntu.sh 你的公网IP或域名 8080
```

默认 Go 依赖下载会依次尝试 `goproxy.cn`、`proxy.golang.org` 和源码直连。也可按网络环境覆盖：

```bash
sudo env ASAMU_LOCAL_BUILD=true \
  APP_VERSION=0.3.1 \
  GOPROXY='https://goproxy.cn|https://proxy.golang.org|direct' \
  GOSUMDB=sum.golang.google.cn \
  ./scripts/deploy-ubuntu.sh 你的公网IP或域名 8080
```

更新源码包时请直接覆盖解压到原目录，并保留 `.env.docker` 和 `deployment-credentials.txt`。运行新包的部署脚本时显式传入包内标注的 `APP_VERSION`；脚本会强制重跑迁移、种子和依赖检查，成功后才重建 API。不要只删除项目目录而保留 Docker 数据卷，否则新生成的数据库密码无法登录旧数据卷。若原 `.env.docker` 已丢失，必须从备份恢复；只有确认不需要旧数据时才使用下方的 `--fresh`。

确认完全不要旧数据时，可先用新版包中的清理器删除另一个旧项目目录（包含 asamu/chain-mirror 容器、动态题容器、网络、数据卷、生成文件和旧项目文件）：

```bash
sudo bash ./scripts/purge-installation.sh --project-dir '/旧项目绝对路径' --delete-project-files --yes
```

若就在当前新版目录原地全新安装，不必手动清理；部署命令末尾添加 `--fresh` 即可清空全部旧 Docker 数据并重新构建：

```bash
sudo bash ./scripts/install-local.sh 你的公网IP或域名 8080 --fresh
```

该命令会构建固定版本平台镜像、执行迁移与幂等初始化、启用 Worker 读取宿主机本地 Docker 镜像，并等待完整健康检查通过。

从重命名前的旧部署升级时也使用上面的 `deploy-ubuntu.sh` 命令，不要先运行 `upgrade.sh`。部署脚本会将镜像、Worker 和 Compose 项目标识切换到 `asamu`，停止旧项目容器，并通过兼容配置继续挂载原 PostgreSQL、Redis、素材和 BuildKit 数据卷；数据库迁移只改动未被管理员自定义的默认品牌文案。

如果已经把固定版本平台镜像推送到镜像仓库，则使用原有拉取模式：

```bash
chmod 755 scripts/*.sh
sudo ./scripts/deploy-ubuntu.sh 你的公网IP或域名 8080
```

访问：

```text
http://你的公网IP:8080
```

账号：

```bash
sudo cat deployment-credentials.txt
```

## 启用 Docker 动态靶场

先准备服务器本地镜像：

```bash
docker build -t asamu/web-sqli:1.0 ./你的题目目录
```

启用本地镜像模式：

```bash
sudo ./scripts/enable-runtime.sh
```

之后在“管理后台 → 题目管理 → Docker 运行环境”的“容器镜像”中直接填写 `asamu/web-sqli:1.0`。Worker 会读取宿主机本地镜像，不需要仓库、Digest 或逐个修改白名单。

启用远程公开仓库拉取：

```bash
sudo ./scripts/enable-runtime.sh 'registry.cn-hangzhou.aliyuncs.com/你的空间/web-sqli:1.0' --pull
```

停止 Worker：

```bash
sudo ./scripts/disable-runtime.sh
```

运行时使用 Compose Profile：

```bash
docker compose --env-file .env.docker --profile runtime ps
docker compose --env-file .env.docker --profile runtime logs -f worker
```

> Docker Socket 具有宿主机高权限。只允许受信任管理员配置题目镜像，禁止用户提交任意镜像名。

## 国内镜像地址覆盖

不需要修改 Dockerfile，可在 `.env.docker` 增加：

```env
GO_BUILD_IMAGE=m.daocloud.io/docker.io/library/golang:1.25.12-alpine3.24
ALPINE_BASE_IMAGE=m.daocloud.io/docker.io/library/alpine:3.24
```

PostgreSQL 和 Redis 也可通过 Compose 环境变量覆盖（见 `docker-compose.yml`）。

## 自检

```bash
sudo ./scripts/docker-doctor.sh
docker compose --env-file .env.docker ps
```

## 全新重装

会删除数据库和已有数据：

```bash
sudo ./scripts/deploy-ubuntu.sh 你的公网IP或域名 8080 --fresh
```

## `PORT_EXHAUSTED` 自动修复与应急重置

本版本会在 Worker 启动时先核对 Docker 容器，再续租端口；端口池看似耗尽时会自动回收失效租约并重试一次。只有端口范围内确实没有可分配端口时才返回 `PORT_EXHAUSTED`；数据库、迁移或分配器异常会返回 `PORT_ALLOCATION_FAILED`，避免把所有内部错误误报成端口耗尽。

若宿主机已有其他进程占用端口池中的某个端口，Worker 会暂时隔离该端口并自动改用池中的下一个端口，不会在同一个冲突端口上重复失败。

如果旧版本遗留了大量错误运行状态，可执行：

```bash
sudo ./scripts/reset-runtime-state.sh --yes
```

该命令只重置动态靶场实例状态、端口租约、运行任务和 Redis 运行队列，不删除用户、题目、比赛、提交、Writeup、素材或管理员账号。重置后重新进入题目页面启动环境即可。

动态靶场对外放行的是 `.env.docker` 中的端口范围，默认是：

```env
RUNTIME_PORT_MIN=20000
RUNTIME_PORT_MAX=30000
```

题目配置里的 `9999` 等端口是容器内部端口，不需要直接映射为宿主机固定端口。
