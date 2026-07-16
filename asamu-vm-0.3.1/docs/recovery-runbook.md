# asamu 备份、恢复、升级与回滚手册

## 安全边界

- 备份包含数据库、Redis 持久化数据、本地素材及 `.env.docker` 中的加密密钥，必须按敏感凭据保管。
- 归档使用 OpenSSL AES-256-CBC + PBKDF2（600000 次）加密，并生成 SHA-256 校验文件；口令文件不得进入代码仓库。
- 备份默认进入短维护窗口，暂停 API、Worker 和 Web，保证数据库与素材的一致性；结束后自动恢复服务。
- 恢复会覆盖数据库、Redis 和素材卷，必须显式输入 `RESTORE`。默认先为当前环境生成应急备份。
- 数据库迁移采用 expand-only；应用版本可回滚，但不执行破坏性数据库 down。需要回到旧数据状态时使用完整恢复。

## 首次准备

```bash
sudo install -d -m 700 /etc/asamu
sudo sh -c 'openssl rand -base64 48 > /etc/asamu/backup.pass'
sudo chmod 600 /etc/asamu/backup.pass
sudo chmod 755 scripts/backup.sh scripts/restore.sh scripts/upgrade.sh scripts/rollback.sh
```

口令文件必须另存一份到受控的离线介质。丢失口令将无法恢复归档。

## 创建与校验备份

```bash
sudo ./scripts/backup.sh /srv/asamu-backups /etc/asamu/backup.pass
cd /srv/asamu-backups
sha256sum -c asamu-YYYYMMDDTHHMMSSZ.tar.gz.enc.sha256
```

建议至少保留“每日 7 份、每周 4 份、每月 6 份”，并复制到另一台主机或离线介质。定期在隔离环境做恢复演练，不能只验证校验和。

## 完整恢复

先确保归档对应版本的 API、Web、Worker、PostgreSQL 与 Redis 镜像已经导入本机，然后执行：

```bash
sudo ./scripts/restore.sh \
  /srv/asamu-backups/asamu-YYYYMMDDTHHMMSSZ.tar.gz.enc \
  /etc/asamu/backup.pass \
  RESTORE
sudo ./scripts/docker-doctor.sh
```

恢复脚本会验证外层校验和、拒绝路径穿越、解密归档、验证每个载荷，再恢复配置与三个持久化数据集。

## 版本升级

```bash
sudo ./scripts/upgrade.sh 0.2.0 /etc/asamu/backup.pass
sudo ./scripts/docker-doctor.sh
```

升级前会强制生成一致性备份并预拉取镜像。健康检查失败时脚本会尝试恢复原应用版本。

## 应用回滚

```bash
sudo ./scripts/rollback.sh 0.1.0 /etc/asamu/backup.pass ROLLBACK
sudo ./scripts/docker-doctor.sh
```

若旧应用不能兼容已经执行的扩展式迁移，使用对应时间点的完整备份执行 `restore.sh`，不要手工删除列或表。

## 演练验收

1. 在隔离服务器导入与备份一致的离线镜像。
2. 执行完整恢复，确认 Web、API、登录、附件下载和管理端可用。
3. 启用 Runtime 时，再验证 Worker、实例启动、Flag 提交、停止与端口回收。
4. 比对用户、题目、比赛、提交、素材数量，并记录恢复点目标（RPO）与恢复时间目标（RTO）。
5. 销毁演练环境前清理归档、口令副本及 `.env.docker`。
