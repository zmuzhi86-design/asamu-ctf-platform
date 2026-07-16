import { useEffect, useState } from 'react'
import { Modal, PixelButton, PixelCard, PixelInput, PixelSelect, PixelTag } from '../../components/ui/System'
import { ApiError } from '../../services/apiClient'
import { archiveDirection, fetchAdminDirections, fetchPlatformDraft, publishPlatformDraft, saveDirection, savePlatformDraft, type ChallengeDirection, type PlatformBootstrap } from '../../services/platformConfigApi'
import { usePlatform } from '../../contexts/PlatformProvider'

const emptyDirection: ChallengeDirection = { slug: '', name: '', subtitle: '', description: '', iconAssetKey: '', cardAssetKey: '', bannerAssetKey: '', backgroundAssetKey: '', sortOrder: 0, status: 'active', showOnHome: true, showOnLibraryHeader: true, showOnLibrarySidebar: true, featured: false }

export function PlatformSettingsPage() {
  const platform = usePlatform()
  const [draft, setDraft] = useState<PlatformBootstrap | null>(null)
  const [directions, setDirections] = useState<ChallengeDirection[]>([])
  const [editing, setEditing] = useState<ChallengeDirection | null>(null)
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  async function load() {
    setError('')
    try {
      const [config, rows] = await Promise.all([fetchPlatformDraft(), fetchAdminDirections()])
      setDraft(config)
      setDirections(rows)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '平台配置加载失败')
    }
  }

  useEffect(() => { void load() }, [])

  async function saveDraft() {
    if (!draft) return
    setBusy(true); setError(''); setMessage('')
    try {
      setDraft(await savePlatformDraft(draft))
      setMessage('草稿已保存，公开站点尚未改变。')
    } catch (reason) { setError(reason instanceof ApiError ? reason.message : '草稿保存失败') } finally { setBusy(false) }
  }

  async function publish() {
    if (!draft) return
    setBusy(true); setError(''); setMessage('')
    try {
      await savePlatformDraft(draft)
      const value = await publishPlatformDraft()
      setDraft(value)
      await platform.reload()
      setMessage(`平台配置 v${value.publishedVersion} 已发布。`)
    } catch (reason) { setError(reason instanceof ApiError ? reason.message : '发布失败') } finally { setBusy(false) }
  }

  async function submitDirection() {
    if (!editing?.slug.trim() || !editing.name.trim()) return
    setBusy(true); setError('')
    try {
      await saveDirection(editing)
      setEditing(null)
      await load()
      setMessage('方向已保存；发布平台配置后会固化到公开快照。')
    } catch (reason) { setError(reason instanceof Error ? reason.message : '方向保存失败') } finally { setBusy(false) }
  }

  async function removeDirection(direction: ChallengeDirection) {
    if (!direction.id || !window.confirm(`确认归档方向“${direction.name}”？已有题目的关联不会被删除。`)) return
    setBusy(true); setError('')
    try {
      await archiveDirection(direction.id)
      const published = await publishPlatformDraft()
      await Promise.all([load(), platform.reload()])
      setMessage(`方向已归档，平台配置 v${published.publishedVersion} 已自动发布。`)
    } catch (reason) { setError(reason instanceof Error ? reason.message : '方向归档失败') } finally { setBusy(false) }
  }

  if (!draft) return <PixelCard><p className="py-10 text-center font-bold text-asamu-muted">{error || '正在加载平台配置…'}</p></PixelCard>

  const updateProfile = (key: keyof PlatformBootstrap['profile'], value: string) => setDraft({ ...draft, profile: { ...draft.profile, [key]: value } })
  const updateLibrary = (key: keyof PlatformBootstrap['challengeLibrary'], value: string | number | boolean) => setDraft({ ...draft, challengeLibrary: { ...draft.challengeLibrary, [key]: value } })

  return <>
    <header className="mb-6 flex flex-col gap-4 border-b-2 border-asamu-ink/10 pb-5 sm:flex-row sm:items-end sm:justify-between"><div><p className="text-xs font-black uppercase tracking-[.18em] text-asamu-blue">PLATFORM CONFIGURATION</p><h1 className="mt-2 font-display text-3xl font-black">平台与方向配置</h1><p className="mt-2 text-sm font-semibold text-asamu-muted">草稿与公开版本隔离；发布后所有访客读取同一份不可变快照。</p></div><div className="flex gap-2"><PixelButton disabled={busy} variant="secondary" onClick={saveDraft}>保存草稿</PixelButton><PixelButton disabled={busy} onClick={publish}>发布配置</PixelButton></div></header>
    {error && <div className="mb-5 border-2 border-red-300 bg-red-50 p-3 text-sm font-black text-red-700">{error}</div>}
    {message && <div className="mb-5 border-2 border-emerald-300 bg-emerald-50 p-3 text-sm font-black text-emerald-700">{message}</div>}

    <div className="grid gap-6 xl:grid-cols-2">
      <PixelCard title="品牌信息"><div className="grid gap-4 sm:grid-cols-2"><Field label="平台名称" value={draft.profile.platformName} onChange={(value) => updateProfile('platformName', value)} /><Field label="简称" value={draft.profile.shortName} onChange={(value) => updateProfile('shortName', value)} /><Field label="口号" value={draft.profile.slogan} onChange={(value) => updateProfile('slogan', value)} /><Field label="浏览器标题" value={draft.profile.homepageTitle} onChange={(value) => updateProfile('homepageTitle', value)} /><Field label="Logo 素材键" value={draft.profile.logoAssetKey} onChange={(value) => updateProfile('logoAssetKey', value)} /><Field label="默认背景键" value={draft.profile.defaultBackgroundKey} onChange={(value) => updateProfile('defaultBackgroundKey', value)} /><label className="block text-sm font-black sm:col-span-2">平台描述<textarea className="pixel-input mt-2 min-h-24" value={draft.profile.description} onChange={(event) => updateProfile('description', event.target.value)} /></label><label className="block text-sm font-black sm:col-span-2">页脚文字<textarea className="pixel-input mt-2 min-h-20" value={draft.profile.footerMarkdown} onChange={(event) => updateProfile('footerMarkdown', event.target.value)} /></label></div></PixelCard>

      <PixelCard title="题库展示"><div className="grid gap-4 sm:grid-cols-2"><Field label="页面标题" value={draft.challengeLibrary.pageTitle} onChange={(value) => updateLibrary('pageTitle', value)} /><Field label="搜索占位文字" value={draft.challengeLibrary.searchPlaceholder} onChange={(value) => updateLibrary('searchPlaceholder', value)} /><label className="block text-sm font-black sm:col-span-2">页面说明<textarea className="pixel-input mt-2 min-h-20" value={draft.challengeLibrary.pageSubtitle} onChange={(event) => updateLibrary('pageSubtitle', event.target.value)} /></label><Field label="每页数量" type="number" value={String(draft.challengeLibrary.pageSize)} onChange={(value) => updateLibrary('pageSize', Math.max(1, Math.min(100, Number(value) || 20)))} /><Field label="默认排序" value={draft.challengeLibrary.defaultSort} onChange={(value) => updateLibrary('defaultSort', value)} /></div><div className="mt-5 flex flex-wrap gap-4"><Check label="显示方向探索区" checked={draft.challengeLibrary.showDirectionSection} onChange={(value) => updateLibrary('showDirectionSection', value)} /><Check label="显示侧栏筛选" checked={draft.challengeLibrary.showSidebar} onChange={(value) => updateLibrary('showSidebar', value)} /></div></PixelCard>
    </div>

    <PixelCard className="mt-6" title="功能开关"><div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">{Object.entries(draft.features).map(([key, enabled]) => <Check key={key} label={key} checked={enabled} onChange={(value) => setDraft({ ...draft, features: { ...draft.features, [key]: value } })} />)}</div></PixelCard>

    <PixelCard className="mt-6" title="主导航" action={<PixelTag tone="blue">{draft.navigation.length} 项</PixelTag>}><div className="space-y-3">{draft.navigation.map((item, index) => <div className="grid gap-2 border-b border-asamu-line pb-3 sm:grid-cols-[1fr_1fr_auto]" key={item.itemKey}><PixelInput value={item.label} aria-label="导航名称" onChange={(event) => setDraft({ ...draft, navigation: draft.navigation.map((row, rowIndex) => rowIndex === index ? { ...row, label: event.target.value } : row) })} /><PixelInput value={item.href} aria-label="导航地址" onChange={(event) => setDraft({ ...draft, navigation: draft.navigation.map((row, rowIndex) => rowIndex === index ? { ...row, href: event.target.value } : row) })} /><Check label="启用" checked={item.enabled} onChange={(value) => setDraft({ ...draft, navigation: draft.navigation.map((row, rowIndex) => rowIndex === index ? { ...row, enabled: value } : row) })} /></div>)}</div></PixelCard>

    <PixelCard className="mt-6" title="挑战方向" action={<PixelButton size="sm" onClick={() => setEditing({ ...emptyDirection })}>新增方向</PixelButton>}><div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">{directions.map((direction) => <article className="border-2 border-asamu-line p-4" key={direction.id || direction.slug}><div className="flex items-start justify-between gap-2"><div><b>{direction.name}</b><p className="text-xs font-bold text-asamu-muted">{direction.slug} · {direction.subtitle}</p></div><PixelTag tone={direction.status === 'active' ? 'green' : 'slate'}>{direction.status}</PixelTag></div><p className="mt-3 line-clamp-2 text-sm text-asamu-muted">{direction.description || '暂无描述'}</p><div className="mt-4 flex gap-2"><PixelButton size="sm" variant="secondary" onClick={() => setEditing({ ...direction })}>编辑</PixelButton>{direction.status !== 'archived' && <PixelButton size="sm" variant="danger" onClick={() => void removeDirection(direction)}>归档</PixelButton>}</div></article>)}</div></PixelCard>

    <Modal open={Boolean(editing)} title={editing?.id ? '编辑方向' : '新增方向'} onClose={() => setEditing(null)}>{editing && <div className="grid gap-4 sm:grid-cols-2"><Field label="名称" value={editing.name} onChange={(value) => setEditing({ ...editing, name: value })} /><Field label="Slug" value={editing.slug} onChange={(value) => setEditing({ ...editing, slug: value.toLowerCase().replace(/[^a-z0-9-]/g, '-') })} /><Field label="副标题" value={editing.subtitle} onChange={(value) => setEditing({ ...editing, subtitle: value })} /><label className="block text-sm font-black">状态<PixelSelect className="mt-2" value={editing.status} onChange={(event) => setEditing({ ...editing, status: event.target.value as ChallengeDirection['status'] })}><option value="active">active</option><option value="disabled">disabled</option><option value="archived">archived</option></PixelSelect></label><Field label="卡片素材键" value={editing.cardAssetKey} onChange={(value) => setEditing({ ...editing, cardAssetKey: value })} /><Field label="排序" type="number" value={String(editing.sortOrder)} onChange={(value) => setEditing({ ...editing, sortOrder: Number(value) || 0 })} /><label className="block text-sm font-black sm:col-span-2">描述<textarea className="pixel-input mt-2 min-h-24" value={editing.description} onChange={(event) => setEditing({ ...editing, description: event.target.value })} /></label><div className="flex flex-wrap gap-4 sm:col-span-2"><Check label="首页显示" checked={editing.showOnHome} onChange={(value) => setEditing({ ...editing, showOnHome: value })} /><Check label="题库顶部显示" checked={editing.showOnLibraryHeader} onChange={(value) => setEditing({ ...editing, showOnLibraryHeader: value })} /><Check label="题库侧栏显示" checked={editing.showOnLibrarySidebar} onChange={(value) => setEditing({ ...editing, showOnLibrarySidebar: value })} /></div><div className="flex justify-end gap-2 sm:col-span-2"><PixelButton variant="secondary" onClick={() => setEditing(null)}>取消</PixelButton><PixelButton disabled={busy || !editing.slug || !editing.name} onClick={() => void submitDirection()}>保存</PixelButton></div></div>}</Modal>
  </>
}

function Field({ label, value, onChange, type = 'text' }: { label: string; value: string; onChange: (value: string) => void; type?: string }) { return <label className="block text-sm font-black">{label}<PixelInput className="mt-2" type={type} value={value} onChange={(event) => onChange(event.target.value)} /></label> }
function Check({ label, checked, onChange }: { label: string; checked: boolean; onChange: (value: boolean) => void }) { return <label className="flex items-center gap-2 text-sm font-black"><input className="h-4 w-4 accent-asamu-blue" type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} />{label}</label> }
