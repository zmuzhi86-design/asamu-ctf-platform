import { useEffect, useState } from 'react'
import { Modal, PageHeader, PixelButton, PixelCard, PixelInput, PixelSelect, PixelTag, StatusBadge } from '../../components/ui/System'
import { usePlatform } from '../../contexts/PlatformProvider'
import { listAdminChallenges } from '../../services/adminChallengeApi'
import { archiveLearningPath, fetchAdminLearningPaths, publishLearningPath, saveLearningPath, type LearningPath, type LearningPathMutation } from '../../services/learningApi'
import type { ChallengeDto } from '../../services/platformApi'

const empty = (): LearningPathMutation => ({ slug: '', directionKey: 'web', title: '', summary: '', description: '', prerequisite: '', estimatedMinutes: 720, heroAssetKey: '', featured: false, sortOrder: 0, stages: [{ title: '基础入门', description: '', sortOrder: 1, challengeIds: [] }] })

export function LearningManagerPage() {
  const { config } = usePlatform()
  const [paths, setPaths] = useState<LearningPath[]>([])
  const [challenges, setChallenges] = useState<ChallengeDto[]>([])
  const [editingID, setEditingID] = useState<string>()
  const [form, setForm] = useState<LearningPathMutation>(empty())
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const [loading, setLoading] = useState(true)
  const [showArchived, setShowArchived] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  async function load() {
    setLoading(true)
    setError('')
    try {
      const [items, challengePage] = await Promise.all([fetchAdminLearningPaths(), listAdminChallenges()])
      setPaths(items)
      setChallenges(challengePage.items)
    } catch (reason) { setError(reason instanceof Error ? reason.message : '学习路线加载失败') } finally { setLoading(false) }
  }
  useEffect(() => { void load() }, [])

  function create() { setEditingID(undefined); setForm(empty()); setOpen(true); setMessage(''); setError('') }
  function edit(path: LearningPath) {
    setEditingID(path.id)
    setForm({ slug: path.slug, directionKey: path.directionKey, title: path.title, summary: path.summary, description: path.description, prerequisite: path.prerequisite, estimatedMinutes: path.estimatedMinutes, heroAssetKey: path.heroAssetKey, featured: path.featured, sortOrder: path.sortOrder, stages: path.stages.map((stage) => ({ title: stage.title, description: stage.description, sortOrder: stage.sortOrder, challengeIds: stage.challenges.map((item) => item.id) })) })
    setOpen(true); setMessage(''); setError('')
  }
  async function save() {
    if (!form.slug || !form.title || !form.directionKey) return
    setBusy(true); setError(''); setMessage('')
    try {
      const saved = await saveLearningPath(editingID, { ...form, stages: form.stages.map((stage, index) => ({ ...stage, sortOrder: index + 1 })) })
      setEditingID(saved.id); setOpen(false); setMessage(saved.status === 'published' ? '学习路线已保存，已发布路线的调整立即生效。' : '学习路线草稿已保存。'); await load()
    } catch (reason) { setError(reason instanceof Error ? reason.message : '学习路线保存失败') } finally { setBusy(false) }
  }
  async function publish(path: LearningPath) {
    if (!window.confirm(`确认发布训练路线“${path.title}”？`)) return
    setBusy(true); setError('')
    try { await publishLearningPath(path.id); setMessage('学习路线已发布，学习中心将立即读取新编排。'); await load() } catch (reason) { setError(reason instanceof Error ? reason.message : '路线发布失败') } finally { setBusy(false) }
  }
  async function archive(path: LearningPath) {
    if (!window.confirm(`确认${path.status === 'published' ? '下架' : '删除'}训练路线“${path.title}”？路线编排会保留，可稍后编辑恢复。`)) return
    setBusy(true); setError('')
    try { await archiveLearningPath(path.id); setMessage('训练路线已从学习中心下架。'); await load() } catch (reason) { setError(reason instanceof Error ? reason.message : '路线下架失败') } finally { setBusy(false) }
  }
  const field = <K extends keyof LearningPathMutation>(key: K, value: LearningPathMutation[K]) => setForm({ ...form, [key]: value })
  const updateStage = (index: number, changes: Partial<LearningPathMutation['stages'][number]>) => field('stages', form.stages.map((stage, row) => row === index ? { ...stage, ...changes } : stage))
  const visiblePaths = showArchived ? paths : paths.filter((path) => path.status !== 'archived')

  return <><PageHeader eyebrow="LEARNING OPERATIONS" title="学习中心管理" description="管理训练路线、阶段顺序和关联题目；只有发布后的路线会出现在前台。"><div className="flex gap-2"><PixelButton variant="secondary" onClick={() => setShowArchived((value) => !value)}>{showArchived ? '隐藏已归档' : '显示已归档'}</PixelButton><PixelButton disabled={loading || Boolean(error)} onClick={create}>新增路线</PixelButton></div></PageHeader>
    {message && <p className="mb-4 border-2 border-green-400 bg-green-50 p-3 text-sm font-black text-green-700">{message}</p>}{error && <div className="mb-4 flex flex-wrap items-center justify-between gap-3 border-2 border-red-400 bg-red-50 p-3 text-sm font-black text-red-700"><span>{error}</span><PixelButton size="sm" variant="secondary" disabled={loading} onClick={() => void load()}>重试加载</PixelButton></div>}
    {loading && <PixelCard><p className="py-16 text-center font-bold text-asamu-muted">正在读取训练路线…</p></PixelCard>}
    {!loading && !error && <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">{visiblePaths.map((path) => <PixelCard key={path.id} title={<span>{path.title} <StatusBadge tone={path.status === 'published' ? 'green' : path.status === 'archived' ? 'slate' : 'yellow'}>{path.status}</StatusBadge></span>}><p className="text-sm font-semibold text-asamu-muted">{path.directionName} · {path.stages.length} 阶段 · {path.totalChallenges} 题</p><p className="mt-3 line-clamp-2 text-sm text-asamu-muted">{path.summary || '暂无路线简介'}</p><div className="mt-4 flex flex-wrap gap-2"><PixelButton size="sm" variant="secondary" onClick={() => edit(path)}>编辑编排</PixelButton><PixelButton size="sm" disabled={busy} onClick={() => void publish(path)}>发布</PixelButton>{path.status !== 'archived' && <PixelButton size="sm" variant="danger" disabled={busy} onClick={() => void archive(path)}>{path.status === 'published' ? '下架' : '删除'}</PixelButton>}</div></PixelCard>)}</div>}
    {!loading && !error && !visiblePaths.length && <PixelCard><p className="py-16 text-center font-bold text-asamu-muted">还没有训练路线，点击“新增路线”开始编排。</p></PixelCard>}

    <Modal open={open} title={editingID ? '编辑训练路线' : '新增训练路线'} onClose={() => !busy && setOpen(false)}><div className="grid gap-4 sm:grid-cols-2"><Field label="路线标题" value={form.title} onChange={(value) => field('title', value)} /><Field label="Slug" value={form.slug} onChange={(value) => field('slug', value.toLowerCase().replace(/[^a-z0-9-]/g, '-'))} /><label className="text-sm font-black">训练方向<PixelSelect className="mt-2" value={form.directionKey} onChange={(event) => field('directionKey', event.target.value)}>{config.directions.filter((item) => item.status === 'active').map((item) => <option key={item.slug} value={item.slug}>{item.name}</option>)}</PixelSelect></label><Field label="预计分钟" type="number" value={String(form.estimatedMinutes)} onChange={(value) => field('estimatedMinutes', Math.max(1, Number(value) || 60))} /><Field label="主视觉素材键" value={form.heroAssetKey} onChange={(value) => field('heroAssetKey', value)} /><Field label="排序" type="number" value={String(form.sortOrder)} onChange={(value) => field('sortOrder', Number(value) || 0)} /><label className="text-sm font-black sm:col-span-2">路线简介<textarea className="pixel-input mt-2 min-h-20" value={form.summary} onChange={(event) => field('summary', event.target.value)} /></label><label className="text-sm font-black sm:col-span-2">详细说明<textarea className="pixel-input mt-2 min-h-24" value={form.description} onChange={(event) => field('description', event.target.value)} /></label><Field label="前置知识" value={form.prerequisite} onChange={(value) => field('prerequisite', value)} /><label className="flex items-center gap-2 text-sm font-black"><input type="checkbox" checked={form.featured} onChange={(event) => field('featured', event.target.checked)} />设为推荐路线</label>
      <div className="space-y-4 sm:col-span-2"><div className="flex items-center justify-between"><b>阶段与题目编排</b><PixelButton size="sm" variant="secondary" onClick={() => field('stages', [...form.stages, { title: `阶段 ${form.stages.length + 1}`, description: '', sortOrder: form.stages.length + 1, challengeIds: [] }])}>新增阶段</PixelButton></div>{form.stages.map((stage, index) => <section className="border-2 border-asamu-line bg-asamu-soft p-4" key={index}><div className="mb-3 flex items-center justify-between"><PixelTag tone="blue">STAGE {index + 1}</PixelTag><PixelButton size="sm" variant="danger" disabled={form.stages.length === 1} onClick={() => field('stages', form.stages.filter((_, row) => row !== index))}>删除阶段</PixelButton></div><div className="grid gap-3 sm:grid-cols-2"><Field label="阶段名称" value={stage.title} onChange={(value) => updateStage(index, { title: value })} /><Field label="阶段说明" value={stage.description} onChange={(value) => updateStage(index, { description: value })} /><label className="text-sm font-black sm:col-span-2">关联题目（可多选）<select className="pixel-input mt-2 min-h-36" multiple value={stage.challengeIds} onChange={(event) => updateStage(index, { challengeIds: Array.from(event.currentTarget.selectedOptions, (option) => option.value) })}>{challenges.map((challenge) => <option key={challenge.id} value={challenge.id}>{challenge.status === 'published' ? '●' : '○'} {challenge.category} · {challenge.title} · {challenge.score} 分</option>)}</select><small className="mt-1 block text-asamu-muted">按住 Ctrl/Command 可选择多道题；路线发布时至少要关联一道已发布题目。</small></label></div></section>)}</div>
      <div className="flex justify-end gap-2 sm:col-span-2"><PixelButton variant="secondary" disabled={busy} onClick={() => setOpen(false)}>取消</PixelButton><PixelButton disabled={busy || !form.title || !form.slug || !form.directionKey || !form.stages.length} onClick={() => void save()}>{busy ? '保存中…' : '保存草稿'}</PixelButton></div></div></Modal>
  </>
}

function Field({ label, value, onChange, type = 'text' }: { label: string; value: string; onChange: (value: string) => void; type?: string }) { return <label className="text-sm font-black">{label}<PixelInput className="mt-2" type={type} value={value} onChange={(event) => onChange(event.target.value)} /></label> }
