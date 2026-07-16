# Docker 动态题目部署指南

asamu 的动态题目采用“管理员配置镜像 + 每用户隔离容器 + 动态 Flag”模型。管理员可以在题目后台配置镜像、协议、端口、资源、时长、环境变量和附件。

## 1. 准备镜像

本地练习环境可直接构建一个带标签的镜像：

```bash
docker build -t ctf-upload:latest .
```

远程仓库的生产镜像建议使用不可漂移的 RepoDigest，例如：

```text
registry.example.com/ctf/web@sha256:0123456789abcdef...（共 64 位十六进制）
```

使用固定引用时，题目后台的镜像引用和 Digest 必须一致。

## 2. 加入 Worker allowlist

本地镜像模式不需要维护 allowlist。先在宿主机执行 `docker build -t ctf-upload:latest .`，然后启用 Worker：

```bash
sudo ./scripts/enable-runtime.sh
```

在题目管理中直接填写 `ctf-upload:latest` 即可。若要让 Worker 自动拉取远程镜像，再把允许拉取的引用加入逗号分隔的 allowlist：

```dotenv
RUNTIME_ALLOWED_IMAGES=registry.example.com/ctf/web@sha256:...,registry.example.com/ctf/pwn@sha256:...
```

镜像已通过 `docker load`/`docker pull` 放在虚拟机时，可保持：

```dotenv
RUNTIME_PULL_MISSING_IMAGES=false
```

需要 Worker 自动拉取时设置为 `true`。私有仓库应先在“管理后台 → 私有镜像仓库”保存凭据，再在题目中选择。修改 allowlist 后执行：

```bash
sudo docker compose --env-file .env.docker up -d --force-recreate worker
```

## 3. 创建动态题

进入“管理后台 → 题目管理 → 新建题目”，勾选“Docker 动态容器题”，然后配置：

- 本地镜像标签，或远程固定镜像引用与可选 Digest；
- HTTP/HTTPS/TCP/UDP 协议和容器内部端口；
- CPU、内存、进程数、磁盘预算；
- 默认与最大运行时长；
- 只读根文件系统（建议开启）；
- 可选环境变量，每行 `KEY=VALUE`。

平台会为每个实例注入：

- `ASAMU_FLAG`：当前用户/实例专属动态 Flag；
- `ASAMU_INSTANCE_ID`：实例 ID。

为兼容重命名前构建的题目镜像，Worker 暂时也会注入旧变量名；新题目请只使用 `ASAMU_FLAG` 和 `ASAMU_INSTANCE_ID`。

题目环境变量不能覆盖这两个保留名称。保存后使用“保存并发布”，再以普通用户启动环境验证访问地址和 Flag。

## 4. 默认隔离措施

Worker 创建容器时默认启用资源限制、PIDs 限制、`no-new-privileges`、丢弃全部 Linux capabilities、独立内部网络、日志轮转和 `/tmp` 临时文件系统。只读根文件系统可按题目需求关闭，但应只在确有写入需求时关闭。
