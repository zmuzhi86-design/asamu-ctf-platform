import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { DataTable, Metric, Modal, PixelButton, PixelCard, PixelInput, PixelSelect, PixelTag, StatusBadge } from '../components/ui/System'
import type { AdminSection } from '../data/platform'
import { ApiError, apiRequest } from '../services/apiClient'

type Page<T> = { items: T[]; page: number; pageSize: number; total: number; totalPages: number }
type RecordRow = Record<string, unknown>

const meta: Record<AdminSection, { title: string; description: string }> = {
  overview: { title: '管理概览', description: '平台运行、内容审核与资源使用情况。' },
  challenges: { title: '题目管理', description: '维护题目、附件、Hint 与动态环境配置。' },
  competitions: { title: '比赛管理', description: '创建比赛、配置题目池、赛制与封榜。' },
  instances: { title: '动态环境', description: '查看实例状态、资源占用与生命周期。' },
  users: { title: '用户与战队', description: '用户角色、组织信息和违规状态管理。' },
  submissions: { title: '提交记录', description: '检索判题结果、得分与来源信息。' },
  antiCheat: { title: '反作弊中心', description: '审核共享 Flag、异常速度与高风险行为。' },
  writeups: { title: 'WriteUp 审核', description: '审核投稿、设置精选并处理违规内容。' },
  announcements: { title: '公告管理', description: '发布平台公告、比赛通知和维护提醒。' },
  settings: { title: '系统设置', description: '敏感配置由环境变量和密钥服务管理。' },
}

const endpoints: Partial<Record<AdminSection, string>> = {
  challenges: '/admin/challenges?pageSize=100', competitions: '/admin/competitions?pageSize=100', instances: '/admin/instances?pageSize=100', users: '/admin/users?pageSize=100', submissions: '/admin/submissions?pageSize=100', antiCheat: '/admin/anti-cheat?pageSize=100', writeups: '/admin/writeups?pageSize=100', announcements: '/admin/announcements?pageSize=100',
}

export function AdminSectionPage({ section }: { section: AdminSection }) {
  const info = meta[section]
  const [rows, setRows] = useState<RecordRow[]>([])
  const [dashboard, setDashboard] = useState<RecordRow>({})
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [announcement, setAnnouncement] = useState({ title: '', content: '', type: 'platform', status: 'draft' })

  useEffect(() => {
    let active = true
    setLoading(true)
    setError('')
    const request = section === 'overview'
      ? apiRequest<RecordRow>('/admin/dashboard').then((data) => ({ dashboard: data, rows: [] as RecordRow[] }))
      : endpoints[section]
        ? apiRequest<Page<RecordRow>>(endpoints[section]!).then((page) => ({ dashboard: {}, rows: page.items }))
        : Promise.resolve({ dashboard: {}, rows: [] as RecordRow[] })
    request.then((data) => { if (active) { setDashboard(data.dashboard); setRows(data.rows) } }).catch((reason: Error) => active && setError(reason.message)).finally(() => active && setLoading(false))
    return () => { active = false }
  }, [section])

  const filtered = useMemo(() => rows.filter((row) => {
    const text = JSON.stringify(row).toLowerCase()
    return (!search || text.includes(search.toLowerCase())) && (!status || text.includes(status.toLowerCase()))
  }), [rows, search, status])

  async function publishAnnouncement() {
    try {
      const created = await apiRequest<RecordRow>('/admin/announcements', { method: 'POST', body: JSON.stringify(announcement) })
      setRows((current) => [created, ...current])
      setModalOpen(false)
      setAnnouncement({ title: '', content: '', type: 'platform', status: 'draft' })
    } catch (reason) {
      setError(reason instanceof ApiError ? reason.message : '公告创建失败')
    }
  }

  return <>
    <header className="mb-6 flex flex-col gap-4 border-b-2 border-asamu-ink/10 pb-5 sm:flex-row sm:items-end sm:justify-between">
      <div><p className="text-xs font-black uppercase tracking-[.18em] text-asamu-blue">ADMINISTRATION</p><h1 className="mt-2 font-display text-3xl font-black">{info.title}</h1><p className="mt-2 text-sm font-semibold text-asamu-muted">{info.description}</p></div>
      {section === 'announcements' && <PixelButton onClick={() => setModalOpen(true)}>新建公告</PixelButton>}
    </header>

    {error && <div className="mb-5 border-2 border-red-300 bg-red-50 p-3 text-sm font-black text-red-700">{error}</div>}
    {section === 'overview' ? <Overview data={dashboard} loading={loading} /> : section === 'settings' ? <Settings /> : <ResourceTable section={section} rows={filtered} loading={loading} search={search} status={status} onSearch={setSearch} onStatus={setStatus} />}

    <Modal open={modalOpen} title="新建公告" onClose={() => setModalOpen(false)}>
      <div className="space-y-4"><label className="block text-sm font-black">标题<PixelInput className="mt-2" value={announcement.title} onChange={(event) => setAnnouncement({ ...announcement, title: event.target.value })} /></label><label className="block text-sm font-black">类型<PixelSelect className="mt-2" value={announcement.type} onChange={(event) => setAnnouncement({ ...announcement, type: event.target.value })}><option value="platform">平台公告</option><option value="competition">比赛通知</option><option value="maintenance">维护提醒</option></PixelSelect></label><label className="block text-sm font-black">内容<textarea className="pixel-input mt-2 min-h-28" value={announcement.content} onChange={(event) => setAnnouncement({ ...announcement, content: event.target.value })} /></label><div className="flex justify-end gap-3"><PixelButton variant="secondary" onClick={() => setModalOpen(false)}>取消</PixelButton><PixelButton disabled={!announcement.title || !announcement.content} onClick={publishAnnouncement}>保存草稿</PixelButton></div></div>
    </Modal>
  </>
}

function Overview({ data, loading }: { data: RecordRow; loading: boolean }) {
  if (loading) return <PixelCard><p className="py-10 text-center text-asamu-muted">正在读取平台状态…</p></PixelCard>
  const values = [
    ['总用户数', data.users ?? 0], ['战队数量', data.teams ?? 0], ['已发布题目', data.challenges ?? 0], ['进行中比赛', data.competitions ?? 0], ['运行中环境', data.instances ?? 0], ['今日提交', data.submissionsToday ?? 0], ['开放风控案件', data.openCheatCases ?? 0], ['待审 WriteUp', data.pendingWriteups ?? 0],
  ]
  return <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">{values.map(([label, value], index) => <Metric key={String(label)} label={String(label)} value={String(value)} highlight={index === 4 || index === 6} />)}</div>
}

function ResourceTable({ section, rows, loading, search, status, onSearch, onStatus }: { section: AdminSection; rows: RecordRow[]; loading: boolean; search: string; status: string; onSearch: (value: string) => void; onStatus: (value: string) => void }) {
  const columns = columnsFor(section)
  return <PixelCard>
    <div className="mb-5 flex flex-col gap-3 border-b border-asamu-line pb-4 sm:flex-row"><PixelInput className="sm:w-72" value={search} onChange={(event) => onSearch(event.target.value)} placeholder="搜索当前列表" /><PixelSelect className="sm:w-40" value={status} onChange={(event) => onStatus(event.target.value)}><option value="">全部状态</option><option value="published">已发布</option><option value="running">运行中</option><option value="pending">待处理</option><option value="active">正常</option><option value="banned">停用</option></PixelSelect><PixelTag tone="blue">共 {rows.length} 条</PixelTag></div>
    {loading ? <p className="py-10 text-center text-asamu-muted">正在加载真实数据…</p> : <DataTable headers={columns.map((column) => column.label)} rows={rows.map((row) => columns.map((column) => renderCell(row[column.key], column.key)))} />}
  </PixelCard>
}

function columnsFor(section: AdminSection) {
  const map: Partial<Record<AdminSection, Array<{ key: string; label: string }>>> = {
    challenges: [{ key: 'title', label: '题目' }, { key: 'category', label: '方向' }, { key: 'difficulty', label: '难度' }, { key: 'score', label: '分值' }, { key: 'dynamic', label: '环境' }, { key: 'status', label: '状态' }],
    competitions: [{ key: 'name', label: '比赛' }, { key: 'mode', label: '赛制' }, { key: 'participantCount', label: '参赛主体' }, { key: 'challengeCount', label: '题目' }, { key: 'startsAt', label: '开始时间' }, { key: 'status', label: '状态' }],
    instances: [{ key: 'id', label: '实例' }, { key: 'challengeTitle', label: '题目' }, { key: 'ownerName', label: '所属用户' }, { key: 'status', label: '状态' }, { key: 'hostPort', label: '端口' }, { key: 'expiresAt', label: '到期时间' }],
    users: [{ key: 'username', label: '用户' }, { key: 'email', label: '邮箱' }, { key: 'organization', label: '组织' }, { key: 'roles', label: '角色' }, { key: 'status', label: '状态' }, { key: 'createdAt', label: '注册时间' }],
    submissions: [{ key: 'created_at', label: '时间' }, { key: 'username', label: '用户' }, { key: 'challenge', label: '题目' }, { key: 'result', label: '结果' }, { key: 'awarded_score', label: '得分' }, { key: 'ip', label: '来源 IP' }],
    antiCheat: [{ key: 'id', label: '案件' }, { key: 'risk_score', label: '风险分' }, { key: 'status', label: '状态' }, { key: 'resolution', label: '结论' }, { key: 'created_at', label: '创建时间' }],
    writeups: [{ key: 'title', label: '文章' }, { key: 'author', label: '作者' }, { key: 'category', label: '方向' }, { key: 'views', label: '阅读' }, { key: 'status', label: '状态' }, { key: 'createdAt', label: '提交时间' }],
    announcements: [{ key: 'title', label: '标题' }, { key: 'type', label: '类型' }, { key: 'status', label: '状态' }, { key: 'starts_at', label: '生效时间' }, { key: 'created_at', label: '创建时间' }],
  }
  return map[section] ?? [{ key: 'id', label: 'ID' }]
}

function renderCell(value: unknown, key: string): ReactNode {
  if (key === 'status' || key === 'result') { const text = String(value ?? '—'); const positive = ['active', 'published', 'running', 'correct', 'approved'].includes(text); return <StatusBadge tone={positive ? 'green' : text === 'incorrect' || text === 'failed' || text === 'banned' ? 'red' : 'yellow'}>{text}</StatusBadge> }
  if (key === 'dynamic') return value ? <PixelTag tone="yellow">动态</PixelTag> : <PixelTag>静态</PixelTag>
  if (Array.isArray(value)) return value.join(', ') || '—'
  if (typeof value === 'string' && /^\d{4}-\d{2}-\d{2}T/.test(value)) return new Date(value).toLocaleString('zh-CN')
  if (typeof value === 'object' && value !== null) return JSON.stringify(value)
  return String(value ?? '—')
}

function Settings() {
  return <div className="grid gap-6 xl:grid-cols-2"><PixelCard title="配置来源"><p className="text-sm font-semibold leading-7 text-asamu-muted">数据库、Redis、对象存储、容器运行时和密钥均由部署环境注入。管理端不读取也不回显密码、Token 或私钥。</p></PixelCard><PixelCard title="运行时策略"><div className="space-y-3 text-sm"><Setting label="容器隔离" value="非特权 · 只读根文件系统 · 资源限额" /><Setting label="Flag 存储" value="HMAC 校验 · AES-GCM 加密" /><Setting label="实例生命周期" value="start / restart / stop / reset 独立操作" /></div></PixelCard></div>
}

function Setting({ label, value }: { label: string; value: string }) { return <div className="flex items-start justify-between gap-4 border-b border-asamu-line pb-3 last:border-0"><b>{label}</b><span className="text-right text-asamu-muted">{value}</span></div> }
