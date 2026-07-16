import { apiRequest } from './apiClient'

export type PlatformProfile = {
  id?: string
  profileKey: string
  platformName: string
  shortName: string
  slogan: string
  description: string
  logoAssetKey: string
  faviconAssetKey: string
  footerMarkdown: string
  contact: Record<string, unknown>
  defaultLocale: string
  timezone: string
  homepageTitle: string
  defaultThemeKey: string
  defaultBackgroundKey: string
  runtimeDefaults: Record<string, unknown>
  version: number
}

export type NavigationItem = {
  id?: string
  itemKey: string
  label: string
  href: string
  iconAssetKey: string
  requiredFeature: string
  requiredPermission: string
  sortOrder: number
  enabled: boolean
}

export type HomepageBlock = {
  id?: string
  blockKey: string
  blockType: string
  title: string
  config: Record<string, unknown>
  sortOrder: number
  enabled: boolean
}

export type ChallengeDirection = {
  id?: string
  slug: string
  name: string
  subtitle: string
  description: string
  iconAssetKey: string
  cardAssetKey: string
  bannerAssetKey: string
  backgroundAssetKey: string
  sortOrder: number
  status: 'active' | 'disabled' | 'archived'
  showOnHome: boolean
  showOnLibraryHeader: boolean
  showOnLibrarySidebar: boolean
  featured: boolean
}

export type ChallengeLibraryConfig = {
  pageTitle: string
  pageSubtitle: string
  searchPlaceholder: string
  showDirectionSection: boolean
  showSidebar: boolean
  filterGroups: unknown[]
  defaultSort: string
  pageSize: number
  cardFields: string[]
  emptyState: Record<string, unknown>
  errorState: Record<string, unknown>
}

export type PlatformBootstrap = {
  profile: PlatformProfile
  features: Record<string, boolean>
  navigation: NavigationItem[]
  homepageBlocks: HomepageBlock[]
  directions: ChallengeDirection[]
  challengeLibrary: ChallengeLibraryConfig
  publishedVersion: number
}

export const fetchPlatformBootstrap = () => apiRequest<PlatformBootstrap>('/public/bootstrap')
export const fetchPlatformDraft = () => apiRequest<PlatformBootstrap>('/admin/platform/draft')
export const savePlatformDraft = (draft: PlatformBootstrap) => apiRequest<PlatformBootstrap>('/admin/platform/draft', { method: 'PUT', body: JSON.stringify(draft) })
export const publishPlatformDraft = () => apiRequest<PlatformBootstrap>('/admin/platform/publish', { method: 'POST' })
export const fetchAdminDirections = () => apiRequest<ChallengeDirection[]>('/admin/directions')
export const saveDirection = (direction: ChallengeDirection) => apiRequest<ChallengeDirection>(direction.id ? `/admin/directions/${direction.id}` : '/admin/directions', { method: direction.id ? 'PUT' : 'POST', body: JSON.stringify(direction) })
export const archiveDirection = (id: string) => apiRequest<void>(`/admin/directions/${id}`, { method: 'DELETE' })
