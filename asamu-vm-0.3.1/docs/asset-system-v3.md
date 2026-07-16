# asamu 素材系统 v3

## 运行结构

- `apps/web/public/assets/default/_source_v3/`：最新版原始素材，只用于追溯。
- `apps/web/public/assets/default/<category>/`：语义化 WebP、缩略图与元数据。
- `apps/web/public/assets/default/manifest.v3.json`：45 个默认素材的稳定 `assetKey` 清单。
- `apps/web/src/data/assetSystem.ts`：前端默认记录、槽位、背景配置与兜底规则。
- `apps/web/src/contexts/AssetProvider.tsx`：统一解析、版本、发布、回滚、归档与审计状态。
- `apps/api/src/services/asset-service.mjs`：素材、分类、标签、槽位、背景及审计 API。
- `apps/api/migrations/001_asset_system_and_instances.up.sql`：生产数据库结构。

## 核心映射

| 页面/用途 | assetKey 前缀 | 目录 |
| --- | --- | --- |
| 首页主视觉 | `home.hero` | `default/home/` |
| 首页四入口 | `home.quick.*` | `default/home/` |
| 十个探索方向 | `direction.*.scene` | `default/directions/` |
| 训练路线 | `training.route.hero` | `default/training/` |
| 比赛中心 | `competition.hero` | `default/competitions/` |
| 战队 Banner/基地/公告 | `team.profile.banner`、`team.base.hero`、`team.announcement` | `default/teams/` |
| 人物角色 | `character.student.*` | `default/characters/` |
| 等级徽章 | `rank.*.main` | `default/ranks/` |
| 荣誉墙 | `team.honor.*` | `default/honors/` |
| 动态环境 | `challenge.instance.*` | `processed/environment/` |

完整文件、哈希、尺寸、安全区域、焦点与原始文件名见 `manifest.v3.json`。

## 解析与兜底

组件只接收 `assetKey`。解析顺序为：夜间移动版 → 夜间版 → 移动版 → 当前发布版 → `fallbackAssetKey` → `mascot.default`。每个版本可独立记录 URL、尺寸、格式、透明通道、哈希、说明与创建时间。

## 管理端

- `/admin/assets`：单个/批量上传、分类、标签、版本、预览、发布、回滚、归档、使用位置。
- `/admin/appearance/slots`：新增页面槽位，设置 `slotKey`、页面、桌面/移动、白天/夜间素材、适配和对齐。
- `/admin/appearance/backgrounds`：12 类页面背景，四端素材、适配、对齐、焦点、遮罩、透明度、模糊、定时发布、回滚。
- `/admin/assets/audit`：操作审计与使用记录。

当前 API 使用内存仓库便于本地直接运行；迁移文件提供 PostgreSQL 持久化结构。生产接入对象存储、鉴权和任务队列时保持现有 API 与 `assetKey` 不变。
