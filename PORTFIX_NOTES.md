# asamu 0.3.1 正式修复版部署说明

## 处理的问题

- 修复虚拟机重启、重新安装或手工删除 Docker 容器后，数据库旧端口租约长期存活并导致 `PORT_EXHAUSTED`。
- Worker 启动时先核对实际 Docker 容器，再进行心跳和端口续租。
- 端口池看似耗尽时，Worker 自动执行一次状态核对、租约回收和重新分配。
- 只为真实有效、属于当前 Worker 的运行实例续租端口。
- 回收已过期且实例心跳失效的端口租约。
- 不再把数据库、迁移和 SQL 错误误报为 `PORT_EXHAUSTED`；真实分配器故障使用 `PORT_ALLOCATION_FAILED`。
- 端口池配置变更后停用旧范围，并允许 `RUNTIME_PORT_MIN` 与 `RUNTIME_PORT_MAX` 相等的单端口池。
- Docker 宿主机端口冲突时隔离冲突端口并自动尝试池中下一个端口。
- Docker 暂时不可访问时不回收尚未核实的租约，避免端口被重复分配。
- 长时间拉取镜像或创建容器时持续续期任务锁，避免任务被其他 Worker 重复领取。
- 前端持续轮询超过 60 秒的启动任务，避免页面永久卡在“启动中”。
- 为 PostgreSQL `generate_series` 的端口范围参数显式指定 `integer` 类型，修复 `SQLSTATE 42725`。
- 空 Outbox 查询不再每 5 秒输出一次无害的 `record not found` 日志。
- 靶场独立 bridge 网络允许 Docker 落实宿主机端口发布；不再出现 HostConfig 有绑定但运行时 `Ports=null`。
- 恢复既有容器时同时校验配置绑定与实际运行时映射，缺失映射会自动销毁并重建容器和网络。
- TCP/UDP 访问地址直接显示为 `IP:端口`，不再附加 `tcp://` 或 `udp://`。
- 题目管理可选择标准随机或 `flag{UUID}` 动态 Flag，新建动态题默认使用 TCP。
- 用户可以在个人资料中可视化选择头像，也可填写管理员上传的自定义素材 Key。
- 动态容器端口池扩大为 `20000-30000`。
- 首页和题库默认展示七个核心方向，其他方向默认归档。
- 全站使用 0.3 正式版背景，队长可以安全上传自定义战队头像。
- 修复本地部署构建了 Worker 镜像但未启动 Worker、后台始终显示 0 个在线 Worker 的问题。
- 部署完成前会强制验证 Worker 心跳和宿主机镜像发现，失败时直接输出诊断日志。
- 首页背景已淡化，容器镜像字段初始为空。
- 强化 `--fresh`：自动识别历史 asamu、chain-mirror 以及以旧解压目录命名的 Compose 项目、容器、网络和数据卷。
- 新增 `scripts/reset-runtime-state.sh`，可在保留平台业务数据的情况下重置动态靶场状态。

## 全新安装

```bash
cd /root/CTF
tar -xzf asamu-vm-0.3.1.tar.gz
cd asamu-vm-0.3.1
chmod 755 scripts/*.sh
sudo bash ./scripts/install-local.sh 192.168.1.36 8080 --fresh
```

## 应急重置动态靶场

```bash
cd /root/CTF/asamu-vm-0.3.1
sudo bash ./scripts/reset-runtime-state.sh --yes
```

应急重置不会删除用户、题目、比赛、提交记录、Writeup、素材和管理员账号。
