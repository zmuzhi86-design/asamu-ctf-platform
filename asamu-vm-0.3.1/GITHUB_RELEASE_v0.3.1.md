# asamu v0.3.1 — 首个公开版本

这是 **asamu** 的首个 GitHub 公开版本。🎉

asamu 是一套面向学习、训练与比赛场景的 CTF 平台，使用 React、TypeScript、Go、PostgreSQL、Redis 与 Docker Compose 构建。

## 主要能力

- 题库、学习中心与方向管理
- 比赛、战队、排行榜和前三血
- Writeup、通知与个人资料
- 管理员题目、比赛、用户和素材管理
- Docker 用户独立动态靶场
- 标准随机与 UUID 动态 Flag
- 本地镜像发现与远程镜像拉取
- 部署、升级、备份、恢复、回滚与自检脚本

## v0.3.1 更新

- 本地源码部署默认构建并启动 Runtime Worker。
- 部署流程等待 Worker 心跳和宿主机镜像发现完成后才报告成功。
- 修复 Worker 首次维护失败时无法在线注册的问题。
- Docker 镜像发现增加超时，避免阻塞数据库事务。
- 修复页面背景遮罩层级和首页背景透明度。
- 动态容器镜像输入框默认保持空白。
- 动态 Flag 支持 `flag{UUID}` 格式。
- 动态宿主端口池扩大到 `20000-30000`。
- TCP/UDP 实例显示 `IP:端口`，HTTP/HTTPS 显示可点击 URL。
- 默认启用 Web、Misc、Reverse、Mobile、Pwn、IoT、Crypto 七个方向。
- 增加战队头像上传和用户头像选择。

## 快速部署

```bash
chmod 755 scripts/*.sh
sudo bash ./scripts/install-local.sh 你的服务器IP或域名 8080
```

安装后运行：

```bash
sudo ./scripts/docker-doctor.sh
```

## 注意事项

- 动态靶场默认端口范围为 TCP/UDP `20000-30000`。
- `--fresh` 会删除现有平台数据库和数据卷。
- 不要将 `.env.docker` 或 `deployment-credentials.txt` 上传到公开仓库。
- Docker Socket 仅应由受信任的 Runtime Worker 使用。

## 已知限制

- 当前尚未提供官方在线演示站点。
- 自动化测试和 CI 流程仍需继续补充。
- 不同 Linux 发行版与云环境的兼容性仍在验证中。

感谢每一位愿意体验、反馈和提出建议的人。🙏
