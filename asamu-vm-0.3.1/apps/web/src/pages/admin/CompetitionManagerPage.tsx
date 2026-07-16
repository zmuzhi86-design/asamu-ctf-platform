import { useEffect, useState } from 'react'
import { Modal, PixelButton, PixelCard, PixelInput, PixelSelect, PixelTag, StatusBadge } from '../../components/ui/System'
import { apiRequest } from '../../services/apiClient'
import type { ChallengeDto, CompetitionDto, Page } from '../../services/platformApi'

type Draft = { slug: string; name: string; summary: string; description: string; mode: string; scoringMode: string; visibility: string; bannerAssetKey: string; themeKey: string; registrationStartsAt: string; registrationEndsAt: string; startsAt: string; endsAt: string; freezeAt: string; teamMin: number; teamMax: number; challengeIds: string[] }
type ScoreEvent = { id: string; userId: string; username: string; teamName: string; challengeTitle: string; type: string; delta: number; reason: string; corrected: boolean; createdAt: string }
const local = (date: Date) => new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
const blank = (): Draft => { const now = Date.now(); return { slug: '', name: '', summary: '', description: '', mode: 'individual', scoringMode: 'dynamic', visibility: 'public', bannerAssetKey: 'competition.hero', themeKey: '', registrationStartsAt: local(new Date(now)), registrationEndsAt: local(new Date(now + 86400000)), startsAt: local(new Date(now + 2 * 86400000)), endsAt: local(new Date(now + 3 * 86400000)), freezeAt: '', teamMin: 1, teamMax: 5, challengeIds: [] } }

const transitions: Record<string, Array<{ status: string; label: string }>> = {
  draft: [{ status: 'registration', label: '开放报名' }],
  registration: [{ status: 'running', label: '开始比赛' }, { status: 'draft', label: '退回草稿' }],
  running: [{ status: 'frozen', label: '立即封榜' }, { status: 'finished', label: '结束比赛' }],
  frozen: [{ status: 'finished', label: '结束并结算' }],
  finished: [{ status: 'archived', label: '归档比赛' }],
}

export function CompetitionManagerPage() {
  const [rows, setRows] = useState<CompetitionDto[]>([])
  const [challenges, setChallenges] = useState<ChallengeDto[]>([])
  const [events, setEvents] = useState<ScoreEvent[]>([])
  const [draft, setDraft] = useState<Draft>(blank())
  const [editingID, setEditingID] = useState<string>()
  const [editorOpen, setEditorOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const [adjustment, setAdjustment] = useState({ userId: '', teamId: '', competitionId: '', challengeId: '', delta: '', reason: '' })
  const reload = () => Promise.all([apiRequest<Page<CompetitionDto>>('/admin/competitions?pageSize=100'), apiRequest<Page<ChallengeDto>>('/admin/challenges?status=published&pageSize=100'), apiRequest<Page<ScoreEvent>>('/admin/score-events?pageSize=50')]).then(([competitionPage, challengePage, eventPage]) => { setRows(competitionPage.items); setChallenges(challengePage.items); setEvents(eventPage.items) }).catch((reason: Error) => setError(reason.message))
  useEffect(() => { void reload() }, [])

  async function setStatus(id: string, status: string) {
    const warning = status === 'running' ? '开始比赛将固化题目与计分快照。' : status === 'frozen' ? '封榜后公开排行榜将固定为当前快照。' : status === 'finished' ? '结束比赛将生成最终榜单并结算。' : ''
    if (!window.confirm(`${warning}确认切换为 ${status}？`)) return
    setBusy(true); setError(''); setMessage('')
    try { await apiRequest<void>(`/admin/competitions/${id}/status`, { method: 'POST', body: JSON.stringify({ status }) }); setMessage(`比赛状态已切换为 ${status}。`); await reload() } catch (reason) { setError(reason instanceof Error ? reason.message : '状态切换失败') } finally { setBusy(false) }
  }

  async function adjustScore() {
    setBusy(true); setError(''); setMessage('')
    try {
      const body = { userId: adjustment.userId, teamId: adjustment.teamId || undefined, competitionId: adjustment.competitionId || undefined, challengeId: adjustment.challengeId || undefined, delta: Number(adjustment.delta), reason: adjustment.reason }
      const result = await apiRequest<{ eventId: string; delta: number }>('/admin/score-events/adjust', { method: 'POST', body: JSON.stringify(body) })
      setMessage(`积分事件 ${result.eventId} 已入账，变更 ${result.delta > 0 ? '+' : ''}${result.delta}。`)
      setAdjustment({ userId: '', teamId: '', competitionId: '', challengeId: '', delta: '', reason: '' }); await reload()
    } catch (reason) { setError(reason instanceof Error ? reason.message : '积分调整失败') } finally { setBusy(false) }
  }

  async function editCompetition(id: string) {
    setError('')
    try { const item = await apiRequest<CompetitionDto & { visibility: string; themeKey: string; freezeAt?: string; teamMin: number; teamMax: number; challenges?: Array<{ id: string }> }>(`/admin/competitions/${id}`); setEditingID(id); setDraft({ slug: item.slug, name: item.name, summary: item.summary, description: item.description, mode: item.mode, scoringMode: item.scoringMode, visibility: item.visibility, bannerAssetKey: item.bannerAssetKey, themeKey: item.themeKey || '', registrationStartsAt: local(new Date(item.registrationStartsAt)), registrationEndsAt: local(new Date(item.registrationEndsAt)), startsAt: local(new Date(item.startsAt)), endsAt: local(new Date(item.endsAt)), freezeAt: item.freezeAt ? local(new Date(item.freezeAt)) : '', teamMin: item.teamMin, teamMax: item.teamMax, challengeIds: (item.challenges ?? []).map((challenge) => challenge.id) }); setEditorOpen(true) } catch (reason) { setError(reason instanceof Error ? reason.message : '比赛读取失败') }
  }

  async function saveCompetition() {
    setBusy(true); setError(''); setMessage('')
    try { const body = { ...draft, registrationStartsAt: new Date(draft.registrationStartsAt).toISOString(), registrationEndsAt: new Date(draft.registrationEndsAt).toISOString(), startsAt: new Date(draft.startsAt).toISOString(), endsAt: new Date(draft.endsAt).toISOString(), freezeAt: draft.freezeAt ? new Date(draft.freezeAt).toISOString() : null }; await apiRequest(editingID ? `/admin/competitions/${editingID}` : '/admin/competitions', { method: editingID ? 'PUT' : 'POST', body: JSON.stringify(body) }); setEditorOpen(false); setMessage('比赛草稿与题目池已保存。'); await reload() } catch (reason) { setError(reason instanceof Error ? reason.message : '比赛保存失败') } finally { setBusy(false) }
  }

  async function voidEvent(event: ScoreEvent) {
    const reason = window.prompt(`请输入撤销积分事件 ${event.id} 的原因（至少 4 个字符）`)
    if (!reason || reason.trim().length < 4) return
    setBusy(true); setError('')
    try { await apiRequest(`/admin/score-events/${event.id}/void`, { method: 'POST', body: JSON.stringify({ reason }) }); setMessage('已追加反向 correction 事件。'); await reload() } catch (reason) { setError(reason instanceof Error ? reason.message : '积分事件撤销失败') } finally { setBusy(false) }
  }

  return <><header className="mb-6 flex items-end justify-between border-b-2 border-asamu-ink/10 pb-5"><div><p className="text-xs font-black tracking-[.18em] text-asamu-blue">COMPETITION OPERATIONS</p><h1 className="mt-2 font-display text-3xl font-black">比赛状态与计分</h1><p className="mt-2 text-sm font-semibold text-asamu-muted">高风险状态转换均由服务端状态机校验，并在开始、封榜和结束时生成快照。</p></div><PixelButton onClick={() => { setEditingID(undefined); setDraft(blank()); setEditorOpen(true) }}>新建比赛</PixelButton></header>
    {error && <div className="mb-4 border-2 border-red-300 bg-red-50 p-3 text-sm font-black text-red-700">{error}</div>}{message && <div className="mb-4 border-2 border-emerald-300 bg-emerald-50 p-3 text-sm font-black text-emerald-700">{message}</div>}
    <PixelCard title="比赛生命周期"><div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">{rows.map((row) => <article className="border-2 border-asamu-line p-4" key={row.id}><div className="flex items-start justify-between gap-2"><div><b>{row.name}</b><p className="mt-1 text-xs font-bold text-asamu-muted">{row.mode === 'team' ? '团队赛' : '个人赛'} · {row.challengeCount} 题</p></div><StatusBadge tone={row.status === 'running' ? 'green' : row.status === 'finished' || row.status === 'archived' ? 'slate' : 'yellow'}>{row.status}</StatusBadge></div><div className="mt-4 flex flex-wrap gap-2">{(row.status === 'draft' || row.status === 'registration') && <PixelButton size="sm" variant="secondary" onClick={() => void editCompetition(row.id)}>编辑配置</PixelButton>}{(transitions[row.status] ?? []).map((action) => <PixelButton size="sm" variant={action.status === 'finished' || action.status === 'frozen' ? 'yellow' : 'secondary'} disabled={busy} key={action.status} onClick={() => void setStatus(row.id, action.status)}>{action.label}</PixelButton>)}<PixelButton size="sm" variant="ghost" onClick={() => setAdjustment({ ...adjustment, competitionId: row.id })}>选择计分范围</PixelButton></div></article>)}</div></PixelCard>
    <PixelCard className="mt-6" title="人工积分事件" action={<PixelTag tone="yellow">追加式审计</PixelTag>}><p className="mb-4 text-sm font-semibold text-asamu-muted">不会修改既有事件；误操作需通过反向 correction 撤销。用户 ID 必填，团队赛调整需同时提供该用户所属战队 ID。</p><div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3"><Field label="用户 UUID" value={adjustment.userId} onChange={(value) => setAdjustment({ ...adjustment, userId: value })} /><Field label="战队 UUID（可选）" value={adjustment.teamId} onChange={(value) => setAdjustment({ ...adjustment, teamId: value })} /><Field label="比赛 UUID（可选）" value={adjustment.competitionId} onChange={(value) => setAdjustment({ ...adjustment, competitionId: value })} /><Field label="题目 UUID（可选）" value={adjustment.challengeId} onChange={(value) => setAdjustment({ ...adjustment, challengeId: value })} /><Field label="分值变化（可为负）" type="number" value={adjustment.delta} onChange={(value) => setAdjustment({ ...adjustment, delta: value })} /><Field label="原因（4-500 字）" value={adjustment.reason} onChange={(value) => setAdjustment({ ...adjustment, reason: value })} /></div><div className="mt-5 flex justify-end"><PixelButton variant="danger" disabled={busy || !adjustment.userId || !adjustment.delta || adjustment.reason.trim().length < 4} onClick={() => void adjustScore()}>确认写入积分账本</PixelButton></div></PixelCard>
    <PixelCard className="mt-6" title="最近积分事件"><div className="space-y-2">{events.map((event) => <div className="grid items-center gap-2 border-b border-asamu-line py-3 text-sm md:grid-cols-[1fr_1fr_100px_180px]" key={event.id}><div><b>{event.username}</b><p className="text-xs text-asamu-muted">{event.teamName || event.challengeTitle || event.type}</p></div><span className="text-xs font-semibold text-asamu-muted">{event.reason || event.type}</span><b className={event.delta >= 0 ? 'text-emerald-600' : 'text-red-600'}>{event.delta > 0 ? '+' : ''}{event.delta}</b><div className="flex items-center justify-end gap-2"><span className="text-xs text-asamu-muted">{new Date(event.createdAt).toLocaleString('zh-CN')}</span>{event.type !== 'correction' && !event.corrected && <PixelButton size="sm" variant="danger" disabled={busy} onClick={() => void voidEvent(event)}>反向撤销</PixelButton>}</div></div>)}</div></PixelCard>
    <Modal open={editorOpen} title={editingID ? '编辑比赛与题目池' : '新建比赛'} onClose={() => !busy && setEditorOpen(false)}><div className="grid gap-4 sm:grid-cols-2"><Field label="比赛名称" value={draft.name} onChange={(value) => setDraft({ ...draft, name: value })} /><Field label="Slug" value={draft.slug} onChange={(value) => setDraft({ ...draft, slug: value })} /><label className="text-sm font-black">模式<PixelSelect className="mt-2" value={draft.mode} onChange={(event) => setDraft({ ...draft, mode: event.target.value })}><option value="individual">个人赛</option><option value="team">团队赛</option></PixelSelect></label><label className="text-sm font-black">计分<PixelSelect className="mt-2" value={draft.scoringMode} onChange={(event) => setDraft({ ...draft, scoringMode: event.target.value })}><option value="dynamic">动态分</option><option value="fixed">固定分</option></PixelSelect></label><Field label="报名开始" type="datetime-local" value={draft.registrationStartsAt} onChange={(value) => setDraft({ ...draft, registrationStartsAt: value })} /><Field label="报名结束" type="datetime-local" value={draft.registrationEndsAt} onChange={(value) => setDraft({ ...draft, registrationEndsAt: value })} /><Field label="比赛开始" type="datetime-local" value={draft.startsAt} onChange={(value) => setDraft({ ...draft, startsAt: value })} /><Field label="比赛结束" type="datetime-local" value={draft.endsAt} onChange={(value) => setDraft({ ...draft, endsAt: value })} /><Field label="封榜时间（可选）" type="datetime-local" value={draft.freezeAt} onChange={(value) => setDraft({ ...draft, freezeAt: value })} /><Field label="Banner 素材键" value={draft.bannerAssetKey} onChange={(value) => setDraft({ ...draft, bannerAssetKey: value })} /><label className="text-sm font-black sm:col-span-2">简介<textarea className="pixel-input mt-2 min-h-20" value={draft.summary} onChange={(event) => setDraft({ ...draft, summary: event.target.value })} /></label><div className="sm:col-span-2"><b className="text-sm">题目池（仅已发布题目）</b><div className="mt-2 grid max-h-48 gap-2 overflow-y-auto border border-asamu-line p-3 sm:grid-cols-2">{challenges.map((challenge) => <label className="flex items-center gap-2 text-sm font-bold" key={challenge.id}><input type="checkbox" checked={draft.challengeIds.includes(challenge.id)} onChange={(event) => setDraft({ ...draft, challengeIds: event.target.checked ? [...draft.challengeIds, challenge.id] : draft.challengeIds.filter((id) => id !== challenge.id) })} />{challenge.title} · {challenge.score}</label>)}</div></div><div className="flex justify-end gap-2 sm:col-span-2"><PixelButton variant="secondary" onClick={() => setEditorOpen(false)}>取消</PixelButton><PixelButton disabled={busy || !draft.name || !draft.challengeIds.length} onClick={() => void saveCompetition()}>{busy ? '保存中…' : '保存草稿'}</PixelButton></div></div></Modal>
  </>
}

function Field({ label, value, onChange, type = 'text' }: { label: string; value: string; onChange: (value: string) => void; type?: string }) { return <label className="text-sm font-black">{label}<PixelInput className="mt-2" type={type} value={value} onChange={(event) => onChange(event.target.value)} /></label> }
