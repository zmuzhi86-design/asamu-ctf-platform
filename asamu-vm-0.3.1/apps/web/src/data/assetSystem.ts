export type AssetStatus = 'draft' | 'published' | 'archived'
export type AssetFit = 'contain' | 'cover'
export type BackgroundFit = AssetFit | 'repeat' | 'repeat-x'
export type AssetTheme = 'light' | 'dark'

export type AssetVersion = {
  id: string
  version: number
  url: string
  mobileUrl?: string
  darkUrl?: string
  darkMobileUrl?: string
  mimeType: string
  width: number
  height: number
  hasAlpha: boolean
  sha256: string
  createdAt: string
  note: string
}

export type AssetRecord = {
  id: string
  assetKey: string
  name: string
  category: string
  tags: string[]
  altText: string
  status: AssetStatus
  currentVersion: number
  versions: AssetVersion[]
  fit: AssetFit
  position: string
  focalPoint: { x: number; y: number }
  safeArea: { top: number; right: number; bottom: number; left: number }
  applicablePages: string[]
  fallbackAssetKey?: string
  updatedAt: string
}

export type AssetSlot = {
  id: string
  slotKey: string
  name: string
  page: string
  assetKey: string
  mobileAssetKey?: string
  darkAssetKey?: string
  darkMobileAssetKey?: string
  fit: AssetFit
  position: string
  enabled: boolean
  version: number
}

export type BackgroundConfig = {
  id: string
  pageKey: string
  label: string
  lightAssetKey: string
  darkAssetKey?: string
  mobileAssetKey?: string
  darkMobileAssetKey?: string
  fit: BackgroundFit
  position: string
  focalPoint: { x: number; y: number }
  overlayColor: string
  overlayOpacity: number
  assetOpacity: number
  blur: number
  enabled: boolean
  status: AssetStatus
  scheduledAt?: string
  version: number
}

type SeedAsset = {
  key: string
  name: string
  category: string
  path: string
  alt: string
  pages: string[]
  fit?: AssetFit
  position?: string
  width?: number
  height?: number
  alpha?: boolean
  tags?: string[]
}

const now = '2026-07-11T10:30:00+08:00'

function record(seed: SeedAsset, index: number): AssetRecord {
  return {
    id: `asset-${String(index + 1).padStart(3, '0')}`,
    assetKey: seed.key,
    name: seed.name,
    category: seed.category,
    tags: seed.tags ?? [seed.category, 'v3'],
    altText: seed.alt,
    status: 'published',
    currentVersion: 1,
    versions: [{ id: `asset-version-${String(index + 1).padStart(3, '0')}-1`, version: 1, url: seed.path, mimeType: seed.path.endsWith('.png') ? 'image/png' : 'image/webp', width: seed.width ?? 1254, height: seed.height ?? 1254, hasAlpha: seed.alpha ?? true, sha256: `v3-${String(index + 1).padStart(4, '0')}`, createdAt: now, note: 'v3.0 默认发布版本' }],
    fit: seed.fit ?? 'contain',
    position: seed.position ?? 'center',
    focalPoint: { x: 50, y: 50 },
    safeArea: { top: 8, right: 8, bottom: 8, left: 8 },
    applicablePages: seed.pages,
    fallbackAssetKey: seed.key === 'mascot.default' ? undefined : 'mascot.default',
    updatedAt: now,
  }
}

const seeds: SeedAsset[] = [
  { key: 'home.hero', name: '首页主视觉', category: 'banner', path: '/assets/default/home/home-hero.webp', alt: 'asamu 浮空安全学院', pages: ['home'], fit: 'contain', position: 'right center', width: 1672, height: 941 },
  { key: 'home.quick.start_training', name: '快捷入口·开始训练', category: 'quick_action', path: '/assets/default/home/quick-start-training.webp', alt: '开始训练实验台', pages: ['home'] },
  { key: 'home.quick.join_competition', name: '快捷入口·加入比赛', category: 'quick_action', path: '/assets/default/home/quick-join-competition.webp', alt: '加入比赛奖杯台', pages: ['home'] },
  { key: 'home.quick.create_team', name: '快捷入口·创建战队', category: 'quick_action', path: '/assets/default/home/quick-create-team.webp', alt: '创建战队旗帜台', pages: ['home'] },
  { key: 'home.quick.read_writeup', name: '快捷入口·阅读 WriteUp', category: 'quick_action', path: '/assets/default/home/quick-read-writeup.webp', alt: '阅读题解分析台', pages: ['home'] },
  ...['web', 'pwn', 'reverse', 'crypto', 'misc', 'forensics', 'iot', 'mobile', 'cloud', 'ai_security'].map((name) => ({ key: `direction.${name}.scene`, name: `方向场景·${name}`, category: 'category_scene', path: `/assets/default/directions/${name.replace('_', '-')}.webp`, alt: `${name} 方向专题场景`, pages: ['home', 'challenges', 'learning'], fit: 'contain' as const })),
  { key: 'training.route.hero', name: '训练路线地图', category: 'training_map', path: '/assets/default/training/training-route-map.webp', alt: '完整的训练路线关卡地图', pages: ['learning'], fit: 'contain', width: 1672, height: 941 },
  { key: 'competition.hero', name: '比赛中心主视觉', category: 'competition_scene', path: '/assets/default/competitions/competition-hero.webp', alt: '蓝白 CTF 正式比赛舞台', pages: ['competitions'], fit: 'contain', position: 'right center', width: 1672, height: 941 },
  { key: 'team.profile.banner', name: '战队详情 Banner', category: 'banner', path: '/assets/default/teams/team-profile-banner.webp', alt: '战队校园基地横幅', pages: ['team_detail'], fit: 'contain', width: 1672, height: 941 },
  { key: 'team.base.hero', name: '战队基地推荐场景', category: 'team_scene', path: '/assets/default/teams/team-base-hero.webp', alt: '战队广场与学院建筑', pages: ['teams'], fit: 'contain', position: 'right center', width: 1916, height: 821 },
  { key: 'team.announcement', name: '战队公告板', category: 'team_scene', path: '/assets/default/teams/team-announcement.webp', alt: '蓝色像素战队公告板', pages: ['team_detail'] },
  ...['bronze', 'silver', 'gold', 'platinum', 'diamond', 'master', 'king', 'legend'].map((name) => ({ key: `rank.${name}.main`, name: `等级徽章·${name}`, category: 'rank_badge', path: `/assets/default/ranks/${name}.webp`, alt: `${name} 等级徽章`, pages: ['profile', 'admin_levels'] })),
  ...[['champion', 'champion-trophy'], ['gold', 'gold-medal'], ['silver', 'silver-medal'], ['bronze', 'bronze-medal'], ['elite', 'elite-three-star'], ['verified', 'verified-team']].map(([key, file]) => ({ key: `team.honor.${key}`, name: `战队荣誉·${key}`, category: 'medal', path: `/assets/default/honors/${file}.webp`, alt: `${key} 战队荣誉`, pages: ['team_detail', 'leaderboard'] })),
  { key: 'character.student.male.default', name: '默认男性角色', category: 'character', path: '/assets/default/characters/student-male-default.webp', alt: '男性 CTF 学员角色', pages: ['profile', 'home'] },
  { key: 'character.student.male.presenter', name: '男性讲解角色', category: 'character', path: '/assets/default/characters/student-male-presenter.webp', alt: '讲解中的男性 CTF 学员', pages: ['writeups', 'learning'] },
  { key: 'character.student.female.default', name: '默认女性角色', category: 'character', path: '/assets/default/characters/student-female-default.webp', alt: '女性 CTF 学员角色', pages: ['profile', 'teams'] },
  { key: 'character.student.female.analyst', name: '女性分析角色', category: 'character', path: '/assets/default/characters/student-female-analyst.webp', alt: '安全分析员角色', pages: ['writeups', 'learning'] },
  { key: 'mascot.default', name: '默认机器人助手', category: 'mascot', path: '/assets/processed/characters/helper.png', alt: 'asamu 像素机器人助手', pages: ['global'] },
  ...['idle', 'starting', 'running', 'resetting', 'failed', 'expired', 'maintenance'].map((status) => ({ key: `challenge.instance.${status}`, name: `环境状态·${status}`, category: 'environment', path: `/assets/processed/environment/${status}.png`, alt: `动态环境 ${status} 状态`, pages: ['challenge_detail', 'admin_instances'] })),
  { key: 'flag.feedback.success', name: 'Flag 正确', category: 'flag_feedback', path: '/assets/processed/flag-feedback/success.png', alt: 'Flag 验证成功', pages: ['challenge_detail'] },
  { key: 'flag.feedback.error', name: 'Flag 错误', category: 'flag_feedback', path: '/assets/processed/flag-feedback/error.png', alt: 'Flag 验证失败', pages: ['challenge_detail'] },
  { key: 'empty.search', name: '搜索空状态', category: 'empty_state', path: '/assets/processed/empty-states/search.png', alt: '没有找到内容', pages: ['global'] },
  { key: 'background.grid.light', name: '全站浅色网格背景', category: 'background', path: '/assets/processed/banners/grid-panel.png', alt: '', pages: ['global'], fit: 'cover' },
  { key: 'background.grid.dark', name: '全站夜间实验室背景', category: 'background', path: '/assets/processed/banners/night-lab.png', alt: '', pages: ['global'], fit: 'cover' },
  { key: 'background.platform.light', name: '0.3 平台背景', category: 'background', path: '/assets/processed/banners/platform-0.3-background.png', alt: '', pages: ['global'], fit: 'cover' },
]

export const defaultAssets: AssetRecord[] = seeds.map(record)

export const defaultSlots: AssetSlot[] = [
  ['home.hero', '首页 Hero', 'home', 'home.hero', 'cover', 'right center'],
  ['home.quick.start_training', '开始训练快捷入口', 'home', 'home.quick.start_training', 'contain', 'center'],
  ['home.quick.join_competition', '加入比赛快捷入口', 'home', 'home.quick.join_competition', 'contain', 'center'],
  ['home.quick.create_team', '创建战队快捷入口', 'home', 'home.quick.create_team', 'contain', 'center'],
  ['home.quick.read_writeup', '阅读 WriteUp 快捷入口', 'home', 'home.quick.read_writeup', 'contain', 'center'],
  ['training.route.hero', '训练路线地图', 'learning', 'training.route.hero', 'contain', 'center'],
  ['competition.hero', '比赛中心主视觉', 'competitions', 'competition.hero', 'contain', 'right center'],
  ['team.profile.banner', '战队详情横幅', 'team_detail', 'team.profile.banner', 'contain', 'center'],
  ['team.base.hero', '战队基地推荐卡', 'teams', 'team.base.hero', 'contain', 'right center'],
  ['team.announcement', '战队公告板', 'team_detail', 'team.announcement', 'contain', 'center'],
].map(([slotKey, name, page, assetKey, fit, position], index) => ({ id: `slot-${index + 1}`, slotKey, name, page, assetKey, fit: fit as AssetFit, position, enabled: true, version: 1 }))

export const defaultBackgrounds: BackgroundConfig[] = [
  ['global', '全站背景'],
  ['home', '首页背景'],
  ['challenges', '题库背景'],
  ['challenge_detail', '题目详情背景'],
  ['learning', '训练路线背景'],
  ['competitions', '比赛中心背景'],
  ['team_list', '战队列表背景'],
  ['team_detail', '战队详情背景'],
  ['leaderboard', '排行榜背景'],
  ['writeups', 'WriteUp 背景'],
  ['profile', '个人主页背景'],
  ['login', '登录页背景'],
  ['admin', '管理后台背景'],
].map(([pageKey, label], index) => {
  const isHome = pageKey === 'home'
  return { id: `background-${index + 1}`, pageKey: String(pageKey), label: String(label), lightAssetKey: 'background.platform.light', darkAssetKey: 'background.platform.light', fit: 'cover', position: 'center', focalPoint: { x: 50, y: 50 }, overlayColor: '#ffffff', overlayOpacity: isHome ? 0.45 : 0.55, assetOpacity: isHome ? 0.18 : 0.12, blur: 0, enabled: true, status: 'published', version: 1 }
})

export const assetCategories = ['brand', 'character', 'mascot', 'quick_action', 'category_icon', 'category_scene', 'banner', 'background', 'training_map', 'competition_scene', 'team_scene', 'rank_badge', 'medal', 'trophy', 'environment', 'flag_feedback', 'empty_state', 'ui_decoration']

export function currentAssetUrl(asset: AssetRecord | undefined, theme: AssetTheme, mobile: boolean) {
  const version = asset?.versions.find((item) => item.version === asset.currentVersion) ?? asset?.versions[0]
  if (!version) return ''
  if (theme === 'dark' && mobile && version.darkMobileUrl) return version.darkMobileUrl
  if (theme === 'dark' && version.darkUrl) return version.darkUrl
  if (mobile && version.mobileUrl) return version.mobileUrl
  return version.url
}
