# asamu API — 精简版

## 默认进程

- `cmd/asamu`：统一生产 CLI，提供 HTTP API、迁移、初始化、诊断和管理员恢复命令。
- `cmd/api`：兼容开发入口，运行 HTTP API、鉴权、题库、比赛、提交、管理端和 SSE。
- `cmd/migrate`：数据库迁移。
- `cmd/seed`：初始化权限、方向、等级和基础账号。

## 可选进程

- `apps/worker/cmd/asamu-worker`：独立 Docker 动态靶场 Worker，只在 Compose `runtime` Profile 中构建和启动。

## 精简内容

- 删除 Kubernetes Provider 和全部 Kubernetes SDK。
- 删除 MinIO/S3 SDK，素材默认使用 Docker Volume 本地存储。
- 删除 Prometheus SDK 与 `/metrics`，保留结构化请求日志和健康检查。
- 删除旧 Node.js 模拟后端，只保留 Go API。
- API Dockerfile 不再预下载整个 `go.mod` 的所有模块，只构建 API 实际导入的依赖。

## 仍然保留

PostgreSQL、Redis、Gin、GORM、JWT、Goose、素材处理、Writeup Markdown、题目提交、比赛、战队、排名和 Docker 动态靶场代码均保留。

API 与 Web 永远不挂载 Docker Socket；只有可选 Worker 挂载。
