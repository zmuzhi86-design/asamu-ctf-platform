# Runtime 多 Worker 运维

## 节点身份与连接

单机 Compose 默认使用稳定 ID `asamu-runtime-worker`。多机部署时，每台 Worker 必须设置不同的：

```env
RUNTIME_WORKER_ID=worker-shanghai-01
RUNTIME_PUBLIC_HOST=lab-node-01.example.com
RUNTIME_WORKER_CPU_MILLI=16000
RUNTIME_WORKER_MEMORY_MB=32768
RUNTIME_WORKER_MAX_INSTANCES=120
RUNTIME_WORKER_PROTOCOLS=http,https,tcp,udp
```

各节点共享 PostgreSQL 和 Redis，但 Docker Socket、端口池及题目容器只属于本机。`RUNTIME_PUBLIC_HOST` 必须解析到对应 Worker，TCP/UDP 防火墙范围应与该节点的 `RUNTIME_PORT_MIN/MAX` 一致。

## 调度规则

- 新实例只会被在线、启用且未排空的 Worker 接收。
- 检查实例数、CPU、内存和协议能力后才会认领启动任务。
- 当另一健康节点已经缓存目标镜像时，未缓存节点会让出任务，减少拉取和冷启动。
- 重启、重置、停止和到期回收固定回到原 Worker，禁止跨主机误操作容器。
- 排空只阻止新实例；已有实例的停止、清理等任务继续执行。
- 心跳超过 90 秒或进程正常退出时，管理端显示节点离线。

## 管理与审计

管理后台“动态环境管理”展示节点容量、占用、协议、镜像缓存和心跳。排空与恢复要求操作理由和节点版本，使用数据库行锁防止并发覆盖，并写入不可变审计日志。

节点维护推荐顺序：

1. 在管理端排空节点。
2. 等待活动实例归零，或由管理员逐一停止。
3. 停止 Worker 并维护宿主机。
4. 启动 Worker，确认心跳和镜像缓存恢复。
5. 在管理端恢复接单。

当前 Compose 仍是单 Worker 模板；多机应在每台节点仅启动 `worker` profile，并通过受控网络连接中心 PostgreSQL/Redis。生产部署还应使用数据库 TLS、Redis TLS/VPN 和主机级访问控制。
