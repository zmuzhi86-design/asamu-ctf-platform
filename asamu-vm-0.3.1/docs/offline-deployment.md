# asamu 标准离线部署

## 在有 Docker 镜像的构建机导出

平台镜像必须使用固定 `APP_VERSION`，题目镜像必须使用固定标签或 digest。下面的额外参数会作为 Runtime 允许列表写入离线包：

```bash
bash ./scripts/export-offline-bundle.sh ./offline-dist \
  asamu/web-sqli:1.0 \
  registry.example.com/ctf/pwn@sha256:0123456789abcdef...
```

导出内容包括 API、Web、Worker、PostgreSQL、Redis、Traefik、可选题目镜像、生产 Compose、默认素材清单和运维脚本。包内不包含 `.env.docker` 或生产密钥。

将 `.tar.gz` 和同名 `.sha256` 一起复制到隔离服务器，先验证：

```bash
sha256sum -c asamu-offline-*.tar.gz.sha256
tar -xzf asamu-offline-*.tar.gz
cd asamu-offline-*/
chmod 755 scripts/*.sh
```

## 在离线 Ubuntu 服务器安装

服务器需要预先安装 Docker Engine、Compose 插件、OpenSSL、gzip、sha256sum 和 curl；这些操作系统软件包应由组织自己的离线软件源提供。

网站模式：

```bash
sudo ./scripts/offline-install.sh 192.0.2.10 8080
```

启用本地 Docker Runtime：

```bash
sudo ./scripts/offline-install.sh ctf.example.internal 8080 --runtime
```

安装器只执行本地校验、`docker load`、现场随机密钥生成、Compose 启动和健康检查，不访问镜像仓库。已有 `.env.docker` 时会拒绝覆盖；已有站点应使用 `upgrade.sh` 和备份恢复流程。

安装完成后立即执行：

```bash
sudo ./scripts/docker-doctor.sh
sudo cat deployment-credentials.txt
```

随后按 [恢复演练手册](recovery-runbook.md) 创建首份加密备份，并将凭据、备份口令分开保管。
