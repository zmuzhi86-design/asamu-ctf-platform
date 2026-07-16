# 私有镜像仓库凭据

## 安全边界

仓库 token 使用 `REGISTRY_CREDENTIAL_ENCRYPTION_KEY_BASE64` 指定的独立 32 字节 AES-GCM 密钥加密，不复用 Flag、JWT 或邮件密钥。管理 API 只返回 `tokenConfigured`，不会返回密文、指纹或明文；创建后只能轮换，不能查看。

题目草稿按镜像引用中的仓库主机自动绑定唯一启用凭据，发布时把凭据 ID 固化到不可变 Runtime revision。比赛快照继续引用该 revision，因此后来编辑题目草稿不会改变正在运行的比赛版本。

## Worker 租约

Worker 使用以下独立配置连接 API：

```env
RUNTIME_WORKER_INTERNAL_API_URL=http://api:8787/api/v1
RUNTIME_WORKER_API_TOKEN=<至少 32 字节随机值>
```

多主机部署必须把内部 URL 改为受控网络中的 HTTPS 地址，并限制只有 Worker 节点可以访问。API 使用常量时间 token 校验，同时要求：

- Worker 已注册、启用、在线且最近 90 秒内有心跳；
- 实例已经由该 Worker 认领；
- 实例的不可变 Runtime revision 确实绑定所请求的凭据；
- 凭据仍处于启用状态。

通过后 API 解密凭据，返回带 60 秒有效提示和 `Cache-Control: no-store` 的响应，并在同一数据库事务内更新最近使用时间和写入 `registry.credential.lease` 审计。Worker 仅把认证内容编码进 Docker `ImagePull` 的 `RegistryAuth`，不会写入容器环境变量、任务载荷或运行日志。

## 运维

新装和离线安装脚本会生成独立密钥与 Worker token；升级脚本会为旧部署补齐并在升级前备份。`.env.docker` 必须保持 `0600` 权限并纳入加密备份。密钥丢失后现有凭据无法恢复，只能由管理员轮换；不要在无重加密迁移的情况下直接替换密钥。

推荐轮换顺序：新 token 在仓库侧生效、管理端轮换凭据、启动测试实例确认拉取、最后撤销旧 token。停用凭据会阻止新的镜像拉取租约，不会把 secret 泄露给前端，也不会修改历史 revision。
