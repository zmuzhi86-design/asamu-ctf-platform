# asamu Platform 前端视觉验收报告

## 验收范围

已检查以下视口：

- `1440 × 900`
- `1920 × 1080`
- `390 × 844`

已检查主要页面：首页、题库、题目详情、比赛中心、比赛答题、训练路线、战队、排行榜、WriteUp、个人资料、管理后台。

## 验收结果

- 所有主要路由可访问，标题正常显示。
- 所有已引用素材均能加载，浏览器检查结果为 0 张破图。
- 桌面端内容统一限制在 1440px 内并居中，没有只占左半边或右侧大片无意义空白。
- 题目详情桌面端使用 `240px minmax(0, 1fr) 360px` 三栏 Grid，中间作战台保持视觉核心。
- 题目详情移动端顺序为：摘要 → 动态环境 → Flag → 题目描述 → 附件 → Hint → 最近提交 → 其他辅助信息。
- 排行榜宽表被限制在自身横向滚动容器内，不再撑开移动页面。
- 比赛答题页移动端不再出现 2px 横向溢出。
- 当前导航在用户端使用黄色高亮；管理端使用蓝色左侧激活条。
- 页面没有使用暗黑 SOC、3D 地球、代码雨、毛玻璃或霓虹赛博朋克元素。
- 主卡片使用深色 2px 描边和像素硬阴影；次级卡片使用浅蓝细边，减少满屏亮蓝框。
- 中文正文使用清晰系统字体，像素风仅用于插画、徽章、局部阴影和装饰。

## 浏览器交互验证

- 点击“启动环境”后：`未启动 → 启动中 → 运行中`。
- 运行中显示访问地址、端口与剩余时间。
- 复制地址按钮在环境运行后启用。
- 输入 `flag{test}` 并提交：显示成功 Toast，最近提交和个人解题状态同步更新。
- 输入其他 Flag：显示失败反馈并记录错误提交。
- 题库搜索、方向、难度、状态和排序均为可交互 Mock。
- 管理后台新增/发布按钮可打开 Mock 表单 Modal。

## 路由清单

### 用户端

- `/`
- `/challenges`
- `/challenges/:id`
- `/competitions`
- `/competitions/:id`
- `/competitions/:id/play`
- `/learning`
- `/teams`
- `/teams/:id`
- `/leaderboard`
- `/writeups`
- `/writeups/:id`
- `/profile`

### 管理端

- `/admin`
- `/admin/challenges`
- `/admin/competitions`
- `/admin/instances`
- `/admin/users`
- `/admin/submissions`
- `/admin/anti-cheat`
- `/admin/writeups`
- `/admin/announcements`
- `/admin/settings`

## 仍需后端接入

- 登录、权限、用户与战队真实数据。
- 题目、附件、Hint、收藏和讨论 API。
- Docker/Kubernetes 环境启动、重置、TTL 与动态 Flag。
- Flag 判题、计分、一血和排行榜实时更新。
- 比赛报名、封榜、公告和实时解题动态。
- WriteUp 发布、Markdown、审核、点赞和收藏。
- 管理端 CRUD、资源监控、反作弊规则和审计日志。
- 文件上传、对象存储、通知、邮件与 WebSocket/SSE。
