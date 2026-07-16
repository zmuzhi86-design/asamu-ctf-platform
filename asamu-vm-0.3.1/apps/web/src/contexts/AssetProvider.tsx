import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { apiRequest } from '../services/apiClient'
import { assetCategories as defaultCategories, currentAssetUrl, defaultAssets, defaultBackgrounds, defaultSlots, type AssetRecord, type AssetSlot, type AssetStatus, type AssetTheme, type BackgroundConfig } from '../data/assetSystem'
import { randomUUID } from '../utils/random'
import { useAuth } from './AuthProvider'

type AuditEntry = { id: string; action: string; target: string; detail: string; actor: string; createdAt: string }
type NamedRecord = { key?: string; name?: string }

type AssetContextValue = {
  assets: AssetRecord[]
  slots: AssetSlot[]
  backgrounds: BackgroundConfig[]
  categories: string[]
  tags: string[]
  audit: AuditEntry[]
  resolve: (assetKey: string, options?: { theme?: AssetTheme; mobile?: boolean; fallbackAssetKey?: string }) => AssetRecord & { url: string }
  refreshAssets: () => Promise<void>
  createAssets: (files: File[], draft: Partial<AssetRecord>) => Promise<void>
  createCategory: (name: string) => void
  createTag: (name: string) => void
  createSlot: (slot: Omit<AssetSlot, 'id' | 'version'>) => void
  updateSlot: (id: string, patch: Partial<AssetSlot>) => void
  updateAsset: (id: string, patch: Partial<AssetRecord>) => void
  updateBackground: (id: string, patch: Partial<BackgroundConfig>) => Promise<BackgroundConfig>
  publishBackground: (id: string) => Promise<void>
  rollbackBackground: (id: string) => Promise<void>
  publishAsset: (id: string) => void
  rollbackAsset: (id: string) => void
  archiveAsset: (id: string) => void
  addVersion: (id: string, file?: File) => Promise<void>
}

const AssetContext = createContext<AssetContextValue | null>(null)

function makeAudit(action: string, target: string, detail: string): AuditEntry {
  return { id: randomUUID(), action, target, detail, actor: '当前管理员', createdAt: new Date().toISOString() }
}

function normalizeBackground(item: BackgroundConfig & { pageKey: string; status: AssetStatus; overlayOpacity: number; assetOpacity: number; startsAt?: string }): BackgroundConfig {
  return { ...item, label: item.label || item.pageKey, enabled: item.status !== 'archived', scheduledAt: item.scheduledAt ?? item.startsAt, overlayOpacity: item.overlayOpacity > 1 ? item.overlayOpacity / 100 : item.overlayOpacity, assetOpacity: item.assetOpacity > 1 ? item.assetOpacity / 100 : item.assetOpacity }
}

export function AssetProvider({ children }: { children: ReactNode }) {
  const auth = useAuth()
  const [assets, setAssets] = useState<AssetRecord[]>(defaultAssets)
  const [slots, setSlots] = useState<AssetSlot[]>(defaultSlots)
  const [backgrounds, setBackgrounds] = useState<BackgroundConfig[]>(defaultBackgrounds)
  const [categories, setCategories] = useState<string[]>(defaultCategories)
  const [tags, setTags] = useState<string[]>(['v3', '默认', '首页', '方向', '战队', '比赛', '等级', '环境'])
  const [audit, setAudit] = useState<AuditEntry[]>([makeAudit('release', '2026.07.11-v3', '发布 v3.0 默认素材清单')])

  const log = useCallback((entry: AuditEntry) => setAudit((current) => [entry, ...current].slice(0, 100)), [])
  const mergeAssets = useCallback((remote: AssetRecord[]) => setAssets([...remote, ...defaultAssets.filter((fallback) => !remote.some((asset) => asset.assetKey === fallback.assetKey))]), [])
  const refreshAssets = useCallback(async () => {
    const manifest = await apiRequest<{ assets?: AssetRecord[]; slots?: AssetSlot[] }>(`/public/assets/manifest?v=${Date.now()}`)
    if (manifest.assets) mergeAssets(manifest.assets)
    if (manifest.slots) setSlots(manifest.slots)
  }, [mergeAssets])

  useEffect(() => {
    if (auth.loading || auth.canAccessAdmin) return
    let active = true
    Promise.all([
      apiRequest<{ assets?: AssetRecord[]; slots?: AssetSlot[] }>('/public/assets/manifest'),
      apiRequest<BackgroundConfig[]>('/public/backgrounds/current'),
    ]).then(([manifest, publicBackgrounds]) => {
        if (!active) return
        if (manifest.assets) setAssets([...manifest.assets, ...defaultAssets.filter((fallback) => !manifest.assets!.some((asset) => asset.assetKey === fallback.assetKey))])
        if (manifest.slots) setSlots(manifest.slots)
        setBackgrounds(publicBackgrounds.length ? publicBackgrounds.map(normalizeBackground) : defaultBackgrounds)
      }).catch(() => undefined)
    return () => { active = false }
  }, [auth.canAccessAdmin, auth.loading])

  useEffect(() => {
    if (auth.loading || !auth.canAccessAdmin) return
    Promise.allSettled([
      apiRequest<{ items: AssetRecord[] }>('/admin/assets?pageSize=100'),
      apiRequest<AssetSlot[]>('/admin/appearance/slots'),
      apiRequest<BackgroundConfig[]>('/admin/appearance/backgrounds'),
      apiRequest<NamedRecord[]>('/admin/asset-categories'),
      apiRequest<NamedRecord[]>('/admin/asset-tags'),
    ]).then(([assetResult, slotResult, backgroundResult, categoryResult, tagResult]) => {
      if (assetResult.status === 'fulfilled') mergeAssets(assetResult.value.items)
      if (slotResult.status === 'fulfilled') setSlots(slotResult.value)
      if (backgroundResult.status === 'fulfilled') setBackgrounds(backgroundResult.value.map(normalizeBackground))
      if (categoryResult.status === 'fulfilled') setCategories(categoryResult.value.map((item) => item.key || item.name || '').filter(Boolean))
      if (tagResult.status === 'fulfilled') setTags(tagResult.value.map((item) => item.name || '').filter(Boolean))
    })
  }, [auth.canAccessAdmin, auth.loading, auth.user?.id, mergeAssets])

  const resolve = useCallback((assetKey: string, options: { theme?: AssetTheme; mobile?: boolean; fallbackAssetKey?: string } = {}) => {
    const requested = assets.find((asset) => asset.assetKey === assetKey && asset.status !== 'archived' && (auth.canAccessAdmin || asset.status === 'published'))
    const fallback = assets.find((asset) => asset.assetKey === (options.fallbackAssetKey ?? requested?.fallbackAssetKey ?? 'mascot.default')) ?? defaultAssets.find((asset) => asset.assetKey === 'mascot.default')!
    const asset = requested ?? fallback
    return { ...asset, url: currentAssetUrl(asset, options.theme ?? 'light', options.mobile ?? false) }
  }, [assets, auth.canAccessAdmin])

  const createAssets = async (files: File[], draft: Partial<AssetRecord>) => {
    const created: AssetRecord[] = []
    for (const [index, file] of files.entries()) {
      const form = new FormData()
      form.append('file', file)
      form.append('assetKey', files.length > 1 ? `${draft.assetKey ?? 'asset.upload'}.${index + 1}` : draft.assetKey ?? `asset.upload.${Date.now()}`)
      form.append('name', files.length > 1 ? `${draft.name ?? file.name} ${index + 1}` : draft.name ?? file.name)
      form.append('category', draft.category ?? 'ui_decoration')
      form.append('altText', draft.altText ?? draft.name ?? file.name)
      form.append('fit', draft.fit ?? 'contain')
      form.append('position', draft.position ?? 'center')
      form.append('fallbackAssetKey', draft.fallbackAssetKey ?? 'mascot.default')
      form.append('tags', (draft.tags ?? ['上传']).join(','))
      form.append('applicablePages', (draft.applicablePages ?? ['global']).join(','))
      created.push(await apiRequest<AssetRecord>('/admin/assets', { method: 'POST', body: form }))
    }
    setAssets((current) => [...created, ...current])
    log(makeAudit('upload', created.map((asset) => asset.assetKey).join(', '), `上传 ${created.length} 个素材`))
  }

  const createCategory = (name: string) => {
    if (!name || categories.includes(name)) return
    void apiRequest<NamedRecord>('/admin/asset-categories', { method: 'POST', body: JSON.stringify({ key: name, name }) }).then(() => { setCategories((current) => [...current, name]); log(makeAudit('create_category', name, '新增素材分类')) })
  }
  const createTag = (name: string) => {
    if (!name || tags.includes(name)) return
    void apiRequest<NamedRecord>('/admin/asset-tags', { method: 'POST', body: JSON.stringify({ name }) }).then(() => { setTags((current) => [...current, name]); log(makeAudit('create_tag', name, '新增素材标签')) })
  }
  const createSlot = (slot: Omit<AssetSlot, 'id' | 'version'>) => {
    void apiRequest<AssetSlot>('/admin/appearance/slots', { method: 'POST', body: JSON.stringify(slot) }).then((created) => { setSlots((current) => [created, ...current]); log(makeAudit('create_slot', created.slotKey, '新增页面素材槽位')) })
  }
  const updateSlot = (id: string, patch: Partial<AssetSlot>) => {
    const current = slots.find((slot) => slot.id === id)
    if (!current) return
    const next = { ...current, ...patch }
    setSlots((items) => items.map((slot) => slot.id === id ? next : slot))
    void apiRequest<AssetSlot>(`/admin/appearance/slots/${id}`, { method: 'PUT', body: JSON.stringify(next) }).then((saved) => setSlots((items) => items.map((slot) => slot.id === id ? saved : slot)))
  }
  const updateAsset = (id: string, patch: Partial<AssetRecord>) => {
    const current = assets.find((asset) => asset.id === id)
    if (!current) return
    const next = { ...current, ...patch, updatedAt: new Date().toISOString() }
    setAssets((items) => items.map((asset) => asset.id === id ? next : asset))
    void apiRequest<AssetRecord>(`/admin/assets/${id}`, { method: 'PUT', body: JSON.stringify(next) }).then((saved) => setAssets((items) => items.map((asset) => asset.id === id ? saved : asset)))
  }
  const updateBackground = async (id: string, patch: Partial<BackgroundConfig>) => {
    const current = backgrounds.find((background) => background.id === id)
    if (!current) throw new Error('背景配置不存在')
    const next = { ...current, ...patch, version: current.version + 1 }
    const payload = { ...next, scopeType: 'platform', overlayOpacity: Math.round(next.overlayOpacity * 100), assetOpacity: Math.round(next.assetOpacity * 100), startsAt: next.scheduledAt || undefined }
    const saved = normalizeBackground(await apiRequest<BackgroundConfig>(`/admin/appearance/backgrounds/${id}`, { method: 'PATCH', body: JSON.stringify(payload) }))
    setBackgrounds((items) => saved.id === id ? items.map((background) => background.id === id ? saved : background) : [saved, ...items.filter((background) => background.id !== saved.id)])
    return saved
  }
  const reloadBackgrounds = async () => {
    const items = await apiRequest<BackgroundConfig[]>('/admin/appearance/backgrounds')
    setBackgrounds(items.map(normalizeBackground))
  }
  const publishBackground = async (id: string) => {
    await apiRequest<void>(`/admin/appearance/backgrounds/${id}/publish`, { method: 'POST' })
    await reloadBackgrounds()
  }
  const rollbackBackground = async (id: string) => {
    await apiRequest<BackgroundConfig>(`/admin/appearance/backgrounds/${id}/rollback`, { method: 'POST' })
    await reloadBackgrounds()
  }

  const setStatus = (id: string, status: AssetStatus, action: string) => { setAssets((current) => current.map((asset) => asset.id === id ? { ...asset, status, updatedAt: new Date().toISOString() } : asset)); log(makeAudit(action, id, `状态变更为 ${status}`)) }
  const publishAsset = (id: string) => { void apiRequest<void>(`/admin/assets/${id}/publish`, { method: 'POST' }).then(() => setStatus(id, 'published', 'publish')) }
  const archiveAsset = (id: string) => { void apiRequest<void>(`/admin/assets/${id}`, { method: 'DELETE' }).then(() => setStatus(id, 'archived', 'archive')) }
  const rollbackAsset = (id: string) => { void apiRequest<void>(`/admin/assets/${id}/rollback`, { method: 'POST' }).then(() => apiRequest<AssetRecord>(`/admin/assets/${id}`)).then((saved) => setAssets((current) => current.map((asset) => asset.id === id ? saved : asset))); log(makeAudit('rollback', id, '回滚到上一版本')) }
  const addVersion = async (id: string, file?: File) => {
    if (!file) {
      file = await new Promise<File | undefined>((resolve) => {
        const input = document.createElement('input')
        input.type = 'file'
        input.accept = 'image/png,image/jpeg,image/webp,image/svg+xml'
        input.onchange = () => resolve(input.files?.[0])
        input.oncancel = () => resolve(undefined)
        input.click()
      })
    }
    if (!file) return
    const form = new FormData()
    form.append('file', file)
    form.append('note', '管理端上传的新版本')
    const saved = await apiRequest<AssetRecord>(`/admin/assets/${id}/versions`, { method: 'POST', body: form })
    setAssets((current) => current.map((asset) => asset.id === id ? saved : asset))
    log(makeAudit('create_version', id, `创建版本 v${saved.currentVersion}`))
  }

  const value = useMemo(() => ({ assets, slots, backgrounds, categories, tags, audit, resolve, refreshAssets, createAssets, createCategory, createTag, createSlot, updateSlot, updateAsset, updateBackground, publishBackground, rollbackBackground, publishAsset, rollbackAsset, archiveAsset, addVersion }), [assets, audit, backgrounds, categories, refreshAssets, resolve, slots, tags])
  return <AssetContext.Provider value={value}>{children}</AssetContext.Provider>
}

export function useAssetSystem() { const value = useContext(AssetContext); if (!value) throw new Error('useAssetSystem must be used inside AssetProvider'); return value }
export function useAsset(assetKey: string, options?: { theme?: AssetTheme; mobile?: boolean; fallbackAssetKey?: string }) { return useAssetSystem().resolve(assetKey, options) }
