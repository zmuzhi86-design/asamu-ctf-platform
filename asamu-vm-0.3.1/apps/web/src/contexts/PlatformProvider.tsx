import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { categories, navigationItems } from '../data/platform'
import { fetchPlatformBootstrap, type PlatformBootstrap } from '../services/platformConfigApi'
import { useAssetSystem } from './AssetProvider'

const fallback: PlatformBootstrap = {
  profile: {
    profileKey: 'default', platformName: 'asamu', shortName: 'ASAMU', slogan: '探索无界 · 攻防未来', description: '网络安全学习与竞赛平台',
    logoAssetKey: 'mascot.default', faviconAssetKey: '', footerMarkdown: '© 2026 asamu Platform · Keep Hacking, Keep Growing', contact: {},
    defaultLocale: 'zh-CN', timezone: 'Asia/Shanghai', homepageTitle: 'asamu 网络安全学习平台', defaultThemeKey: 'platform-default', defaultBackgroundKey: '', runtimeDefaults: {}, version: 1,
  },
  features: { registration: true, teams: true, writeups: true, learning: true, competitions: true, runtime: false },
  navigation: navigationItems.map((item, index) => ({ itemKey: item.to === '/' ? 'home' : item.to.slice(1), label: item.label, href: item.to, iconAssetKey: '', requiredFeature: '', requiredPermission: '', sortOrder: index * 10, enabled: true })),
  homepageBlocks: [],
  directions: categories.map((item, index) => {
    const slug = item.label === 'AI Security' ? 'ai-security' : item.label.toLowerCase()
    const active = new Set(['web', 'misc', 'reverse', 'mobile', 'pwn', 'iot', 'crypto']).has(slug)
    return { id: undefined, slug, name: item.label, subtitle: item.name, description: '', iconAssetKey: `category.${item.key}.icon`, cardAssetKey: `direction.${item.label === 'AI Security' ? 'ai_security' : item.label.toLowerCase()}.scene`, bannerAssetKey: '', backgroundAssetKey: '', sortOrder: index * 10, status: active ? 'active' as const : 'archived' as const, showOnHome: active, showOnLibraryHeader: active, showOnLibrarySidebar: active, featured: false }
  }),
  challengeLibrary: { pageTitle: '挑战题库', pageSubtitle: '从兴趣出发，选择方向、难度和环境类型，逐步点亮你的 CTF 技能树。', searchPlaceholder: '搜索题目名称、标签或方向…', showDirectionSection: true, showSidebar: true, filterGroups: [], defaultSort: 'direction', pageSize: 20, cardFields: ['difficulty', 'score', 'solves', 'tags'], emptyState: {}, errorState: {} },
  publishedVersion: 0,
}

type PlatformContextValue = { config: PlatformBootstrap; loading: boolean; reload: () => Promise<void> }
const PlatformContext = createContext<PlatformContextValue | null>(null)

export function PlatformProvider({ children }: { children: ReactNode }) {
  const assetSystem = useAssetSystem()
  const [config, setConfig] = useState(fallback)
  const [loading, setLoading] = useState(true)

  async function reload() {
    try {
      const value = await fetchPlatformBootstrap()
      setConfig(value)
    } catch {
      setConfig(fallback)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void reload() }, [])
  useEffect(() => {
    document.title = config.profile.homepageTitle || config.profile.platformName
    if (config.profile.faviconAssetKey) {
      let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
      if (!link) {
        link = document.createElement('link')
        link.rel = 'icon'
        document.head.appendChild(link)
      }
      link.href = assetSystem.resolve(config.profile.faviconAssetKey).url
    }
  }, [assetSystem, config.profile.faviconAssetKey, config.profile.homepageTitle, config.profile.platformName])

  const value = useMemo(() => ({ config, loading, reload }), [config, loading])
  return <PlatformContext.Provider value={value}>{children}</PlatformContext.Provider>
}

export function usePlatform() {
  const value = useContext(PlatformContext)
  if (!value) throw new Error('usePlatform must be used inside PlatformProvider')
  return value
}
