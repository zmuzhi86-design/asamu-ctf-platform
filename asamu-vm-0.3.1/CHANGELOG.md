# Changelog

## 0.3.1

- 修复本地源码部署只构建 Worker 镜像却默认不启动 Worker 的问题；旧本地部署升级时会自动启用运行时。
- 部署、启用运行时和升级流程现在必须等到 Worker 注册健康心跳并成功读取宿主机镜像后才报告成功。
- Worker 首次租约维护失败不再阻止注册，Docker 镜像发现增加超时且不再占用数据库事务。
- 修复页面背景遮罩层级错误，并将首页背景调整为低透明度纹理，其他页面进一步降低干扰。
- 新建动态题的容器镜像输入框保持真正空白，不再显示示例占位文字。

## 0.3.0

- UUID 动态 Flag 改为标准包裹格式 `flag{UUID}`。
- 动态容器端口池扩大为 `20000-30000`，升级脚本同步更新既有 `.env.docker`。
- 首页和题库默认只启用 Web、Misc、Reverse、Mobile、Pwn、IoT、Crypto 七个核心方向。
- 全站启用 0.3 正式版背景素材。
- 新增队长专属战队头像上传，包含真实文件类型、大小、像素和权限校验。
- 修复素材回滚后添加新版本可能发生版本号冲突的问题。
- 修复素材超过 1000 个后旧素材详情无法读取、公开素材清单截断的问题。
- 修复战队详情和管理接口静默忽略关联查询错误的问题。
- 限制战队素材 Key、入队说明和邀请用户名长度。

## 0.2.2-r20260716-portfix5

- TCP/UDP 动态环境访问地址改为纯 `IP:端口`，HTTP/HTTPS 仍保留可点击 URL。
- 题目管理新增标准随机与 UUID 两种动态 Flag 格式，格式随已发布运行时版本冻结，启动和重置均保持一致。
- 新建动态题默认使用 TCP 协议，数据库缺省值同步调整为 TCP。
- 个人资料新增可视化头像选择，并修复资料页未读取头像字段的问题。

## 0.2.2-r20260716-portfix4

- 修复 Docker `Internal=true` 靶场网络导致 HostConfig 已配置但运行时端口映射未生效的问题。
- Worker 恢复容器时校验实际 `NetworkSettings.Ports`，端口映射缺失时自动重建。

## 0.2.2-r20260716-portfix3

- 修复 PostgreSQL 无法解析未定类型 `generate_series` 参数导致的 `PORT_ALLOCATION_FAILED`（SQLSTATE 42725）。
- 空运行任务 Outbox 不再反复输出 `record not found` 噪声。

## 0.2.2-r20260716-portfix2

- 修正端口分配错误分类，只有真实池耗尽才返回 `PORT_EXHAUSTED`。
- 同步运行时端口池范围并支持单端口配置；宿主端口冲突时自动换用下一端口。
- Docker 对账失败时停止破坏性租约回收，Docker 临时故障不再误判为容器漂移。
- 为长任务续期数据库锁，避免多 Worker 重复处理同一运行任务。
- 修复运行版本资源释放、旧实例端口释放和跨题目并发配额竞争。
- 修复前端超过 60 秒的环境启动状态停止刷新的问题。

## 0.2.2-r20260716-portfix1

### Changed

- Renamed the product, Go modules, CLI binaries, Docker images, Compose project, runtime labels, frontend design-token namespace, documentation and release archives to `asamu`.
- New installations use the `asamu` PostgreSQL identity and named volumes. The deployment path recognizes the previous Compose project, image defaults, database identity, volumes, backups, runtime labels and challenge variables so an in-place rename does not discard existing data or running-instance cleanup metadata.

### Fixed

- Fixed false `PORT_EXHAUSTED` after VM reboot, reinstall, or manual Docker cleanup: the Worker now reconciles Docker state before its first heartbeat, releases stale leases before renewing healthy ones, and performs one automatic recovery-and-retry when the database port pool appears full.
- Fixed active lease renewal so only active instances owned by the current Worker are renewed; expired leases with stale instance heartbeats are reclaimed automatically.
- Strengthened `--fresh` cleanup to discover historical Compose project names, containers, networks, and volumes whose names or labels contain `asamu` or `chain-mirror`.
- Added `scripts/reset-runtime-state.sh` for a schema-matched emergency reset of runtime tasks, port leases, Redis job streams, and orphaned dynamic containers without deleting users, challenges, competitions, submissions, or assets.

- Fixed repeat initialization of challenge categories and demo challenges: conflict candidates are no longer reused for GORM lookups, preventing an unintended `key/slug + newly generated id` query and `record not found` on an existing database.
- Seed runs are now serialized by a PostgreSQL transaction advisory lock and committed atomically; failures identify the initialization stage, and defaults no longer overwrite archived or administrator-edited categories, directions, progression tiers, asset slots, demo content or learning paths.
- Demo seeding is opt-in, fills the missing challenge direction, preserves existing runtime/Hint/competition configuration, and never republishes archived content or restarts an existing competition.
- Repeated init runs are now safe: permission, role and challenge-tag seeding uses atomic conflict handling, while deployment and maintenance scripts start application services with `--no-deps` after the explicit init gate so Compose cannot immediately execute the one-shot init service a second time.
- Fixed the Learning Center 500 caused by GORM treating response-only nested stages and challenges as database relations; published public challenges now reconcile into system-managed learning paths transactionally.
- Host-local Docker images are always discovered and accepted even when a remote-pull allowlist is configured; Docker inspection/list failures now keep the Worker offline with an operator-visible error code instead of reporting a false zero-image healthy state.
- Runtime enable/disable scripts now recreate the API so its startup-loaded configuration matches `.env.docker`; the frontend understands every runtime lifecycle state and continues polling from `pending`, `pulling`, and `creating`.
- Deploy, upgrade, restore and offline-install flows now force a fresh migration/seed/doctor init run and wait for success before starting the application; readiness and the Docker doctor verify the complete learning schema.
- Offline runtime deployments bind challenge ports for remote access, and local-build upgrades no longer fall back to pulling old remote images.

- Docker challenge administration now accepts host-local `image:tag` references without a registry or digest; an empty allowlist means local-only mode, while automatic pulls still require an explicit allowlist.
- Runtime workers report all locally cached tags/digests for admin image suggestions and prevent both reserved instance environment variables from being overridden.
- Seeded Learning Center paths now reconcile newly published challenges, keep empty generated paths out of the public page, and publish demo content backed by a local tagged image.
- Learning Center errors can be retried in place, and duplicate route slugs now return a conflict instead of an internal error.
- Challenge create/update APIs now enforce title, category, score-range, visibility and score-mode validation consistently.
- JSON and multipart request bodies are bounded before parsing; the Nginx upload limit now matches the documented 64 MiB challenge attachment limit.
- Default asset and background seeding no longer ignores database failures that could leave the Learning Center or other pages partially initialized.
- Challenge attachments now download through the authenticated API client, including access-token refresh, instead of unauthenticated browser navigation.
- Administrator accounts now receive a visible management-console entry; management routes and navigation are role/permission guarded.
- Platform publishing now saves the currently edited draft first and refreshes the live platform bootstrap immediately.
- Public clients now load published page-background configuration; background draft, publish and rollback use their dedicated versioned endpoints.
- Asset administration reloads after authentication and no longer exposes cached draft assets after logout.
- Default asset metadata is included in the API image and seeding now fails loudly if the manifest is missing or malformed.
- Page-slot asset keys are persisted as version bindings instead of being silently discarded.
- Challenge directions and legacy challenge categories are synchronized so newly configured directions can be used by challenge creation.
- Challenge-file downloads verify that the requested file belongs to the challenge in the URL.
- Archiving a challenge direction now publishes and reloads the public platform snapshot immediately, so archived directions disappear from the home page without a second manual publish.
- Static challenge detail pages now replace the runtime-instance console with a prominent attachment panel and no longer request or expose dynamic-environment controls.
- Duplicate uploads of the same active challenge attachment are idempotent, while legacy duplicate rows are collapsed in the public challenge view.
- Learning Center routes, stages and challenge assignments are now database-backed, expose real per-user solve progress, and have a permission-gated administration console.
- Challenge administration now supports safe delete/unpublish through archival, preserving immutable revisions, submissions and competition history while removing the challenge from public APIs.
- Re-running the deployment seed no longer re-enables challenge categories that an administrator previously archived.

### Added

- Added a single-command local-runtime installer that performs a fixed-version local build, guarded fresh reset, migrations/seeding, Worker enablement and a complete post-install health gate.
- Added a guarded purge utility that removes current and legacy Compose services, host-created dynamic challenge containers and networks, named data volumes, generated deployment state and, only with an explicit flag, the old project directory.
- Encrypted database email outbox with SMTP/log transports, `SKIP LOCKED` dispatch, stale-lease recovery, bounded retry and dead-letter state.
- Email verification, password recovery, confirmed email change and mailed team invitation flows.
- Account action, account security and team invitation acceptance pages in the React application.
- Notification center with transactional team/WriteUp events and authenticated polling.
- WriteUp author workspace with revision-backed drafts, visibility enforcement, safe preview, review submission, comments, likes and favorites.
- Team workspace for creation, profile/recruiting changes, invitations, join review, announcements, member removal, captain transfer and leaving.
- Editable user profiles with server-enforced public privacy controls and stable slugs for non-ASCII team names.
- Home page now uses live challenge, competition, leaderboard, WriteUp and progression APIs instead of demo business arrays.
- Dedicated admin user/RBAC, WriteUp review and anti-cheat pages, plus typed/audited announcement creation and strict case-state validation.
- Dedicated runtime-instance operations console with safe DTOs, version-checked stop/reset, reason capture, idempotency conflict protection, atomic audit records and recursively redacted runtime events.
- Encrypted, checksummed maintenance-window backup and guarded full-restore scripts for PostgreSQL, Redis, local assets and deployment secrets.
- Backup-gated application upgrade/rollback scripts and an operator recovery rehearsal runbook.
- Standard offline exporter/installer with fixed-version platform, infrastructure and challenge images, payload checksums and on-site secret generation.
- Runtime Worker node registry with stable identities, heartbeats, capacity/protocol/cache reporting, drain/resume controls and audited optimistic concurrency.
- Multi-Worker-safe job routing: existing instances remain pinned to their owning node, while new starts honor drain, capacity, protocol and warm-image preference.
- AES-GCM-encrypted private-registry credentials, safe administration UI, immutable runtime-revision bindings, Worker-scoped 60-second leases and per-lease audit records.
- Independent `apps/worker` Go module containing all Docker SDK integration.
- Unified `asamu` CLI with serve, migrate, seed, doctor and administrator recovery commands.
- Production-image Compose and separate development build override.
- Optional Rootless BuildKit and Traefik profiles.
- V4 expand-only platform, identity, direction and runtime-state migrations.
- Redis Stream stale-pending reclaim and dead-letter delivery.
- Transactional runtime port leases, quota reservation and worker reconciliation.
- Immutable challenge/runtime/flag/file revisions and competition snapshots.
- Public platform bootstrap, draft/publish snapshots and challenge-library configuration.
- Runtime `PlatformProvider` and platform/direction administration page.
- Append-only score corrections, per-event rule snapshots and derived-score rebuild operations.
- Version-bound Hint unlocks with user/team scope and ledger-backed deductions.
- Integrity-checked challenge attachment upload/download and immutable attachment archival.
- Operational challenge manager for draft, Flag, Hint, runtime, attachment and publication workflows.
- Competition lifecycle console and validated append-only manual score adjustments.
- Competition schedule/challenge-pool editor and auditable score-event reversal console.

### Changed

- API production image now contains one non-root application binary.
- Online deployment pulls versioned images instead of compiling on the server.
- Runtime lifecycle now uses an explicit transition whitelist beginning at `pending`.
- Branding, navigation, feature visibility, home directions and challenge-library presentation now come from the published platform snapshot.
- Scoreboard totals no longer multiply score events through solve/blood joins; dynamic scoring is competition-scoped and snapshot-bound.
- Public challenge details no longer expose locked Hint content; frozen scoreboards never fall back to live standings.
- Challenge mutations accept write-only static/multi-stage/regex flags; plaintext static flags are HMAC-only and publication requires valid judging configuration.
- Demo seed publishes dynamic content only when a digest-pinned challenge image is configured.

### Security

- Docker SDK dependencies were removed from the API module.
- Docker socket remains mounted only by the optional runtime worker.
- Proxy routing uses the Traefik file provider and does not mount Docker socket.
- TLS keys and generated deployment credentials are ignored by source control.
- Backup archives, temporary deployment environments and local build caches are ignored by source control.
- JWT token versions and persistent sessions invalidate access after password, role or account-state changes.
- Refresh replay, last-super-admin protection, strict cookie policy, origin validation and security headers are enforced.
