import { useCallback, useEffect, useMemo, useState } from 'react'
import { DataTable, Modal, PageHeader, PixelButton, PixelCard, PixelInput, PixelTag, StatusBadge } from '../../components/ui/System'
import { ApiError } from '../../services/apiClient'
import { adminInstanceApi, type AdminInstance, type AdminInstanceOperation, type AdminRuntimeEvent, type AdminWorker } from '../../services/adminInstanceApi'

const stoppable = new Set(['pending', 'pulling', 'creating', 'starting', 'running', 'restarting', 'resetting'])

export function InstanceManagerPage() {
  const [items, setItems] = useState<AdminInstance[]>([])
  const [selected, setSelected] = useState<AdminInstance>()
  const [operations, setOperations] = useState<AdminInstanceOperation[]>([])
  const [events, setEvents] = useState<AdminRuntimeEvent[]>([])
  const [workers, setWorkers] = useState<AdminWorker[]>([])
  const [search, setSearch] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [action, setAction] = useState<'stop' | 'reset'>()
  const [reason, setReason] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [workerAction, setWorkerAction] = useState<AdminWorker>()

  const loadDetail = useCallback(async (id: string) => {
    const [detail, logs] = await Promise.all([adminInstanceApi.detail(id), adminInstanceApi.logs(id)])
    setSelected(detail.instance)
    setOperations(detail.operations)
    setEvents(logs)
  }, [])

  const load = useCallback(async (preferredId?: string) => {
    setLoading(true)
    setError('')
    try {
      const [page, workerItems] = await Promise.all([adminInstanceApi.list(), adminInstanceApi.workers()])
      setItems(page.items)
      setWorkers(workerItems)
      const id = preferredId && page.items.some((item) => item.id === preferredId) ? preferredId : page.items[0]?.id
      if (id) await loadDetail(id)
      else { setSelected(undefined); setOperations([]); setEvents([]) }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '实例数据加载失败')
    } finally {
      setLoading(false)
    }
  }, [loadDetail])

  useEffect(() => { void load() }, [load])

  const filtered = useMemo(() => {
    const keyword = search.trim().toLowerCase()
    if (!keyword) return items
    return items.filter((item) => [item.id, item.challengeTitle, item.challengeSlug, item.ownerName, item.competitionName, item.status].some((value) => value?.toLowerCase().includes(keyword)))
  }, [items, search])

  async function select(item: AdminInstance) {
    setError('')
    try { await loadDetail(item.id) } catch (cause) { setError(cause instanceof Error ? cause.message : '实例详情加载失败') }
  }

  function openAction(kind: 'stop' | 'reset') {
    setAction(kind)
    setReason('')
    setConfirmation('')
  }

  async function submitAction() {
    if (!selected || !action) return
    setSubmitting(true)
    setError('')
    try {
      await adminInstanceApi.transition(selected, action, reason)
      setAction(undefined)
      await load(selected.id)
    } catch (cause) {
      const message = cause instanceof ApiError && cause.code === 'INSTANCE_VERSION_CONFLICT' ? '实例状态已经变化，页面已刷新，请重新确认操作。' : cause instanceof Error ? cause.message : '操作失败'
      setError(message)
      if (cause instanceof ApiError && cause.code === 'INSTANCE_VERSION_CONFLICT') await load(selected.id)
    } finally {
      setSubmitting(false)
    }
  }

  async function submitWorkerAction() {
    if (!workerAction) return
    setSubmitting(true)
    setError('')
    try {
      await adminInstanceApi.setWorkerDrain(workerAction, !workerAction.draining, reason)
      setWorkerAction(undefined)
      setReason('')
      await load(selected?.id)
    } catch (cause) {
      const message = cause instanceof ApiError && cause.code === 'WORKER_VERSION_CONFLICT' ? 'Worker 状态已经变化，页面已刷新，请重新确认。' : cause instanceof Error ? cause.message : 'Worker 操作失败'
      setError(message)
      if (cause instanceof ApiError && cause.code === 'WORKER_VERSION_CONFLICT') await load(selected?.id)
    } finally {
      setSubmitting(false)
    }
  }

  const confirmToken = selected ? `RESET ${selected.id.slice(0, 8)}` : ''
  const actionReady = reason.trim().length >= 4 && (action !== 'reset' || confirmation === confirmToken)

  return <>
    <PageHeader eyebrow="RUNTIME OPERATIONS" title="动态环境管理" description="查看实例归属、生命周期操作与脱敏运行事件。停止和重置使用乐观锁、幂等键及原子审计。">
      <PixelButton variant="secondary" disabled={loading} onClick={() => void load(selected?.id)}>刷新状态</PixelButton>
    </PageHeader>
    {error && <div className="mb-5 border-2 border-red-300 bg-red-50 p-3 text-sm font-black text-red-700">{error}</div>}
    <PixelCard className="mb-5" title="Worker 节点" action={<PixelTag tone="blue">{workers.length} 个节点</PixelTag>}>
      {workers.length === 0 ? <p className="py-8 text-center text-sm text-asamu-muted">尚无 Worker 注册；网站模式下这是正常状态。</p> : <DataTable headers={['节点', '状态', '实例', 'CPU', '内存', '协议 / 镜像', '心跳', '']} rows={workers.map((worker) => [
        <div><b>{worker.workerId}</b><small className="block text-asamu-muted">{worker.hostname}</small></div>,
        <InstanceStatus status={worker.status} />,
        `${worker.activeInstances} / ${worker.maxInstances}`,
        `${worker.reservedCpuMilli} / ${worker.cpuTotalMilli}m (${worker.cpuPercent}%)`,
        `${worker.reservedMemoryMb} / ${worker.memoryTotalMb} MB (${worker.memoryPercent}%)`,
        <div><b>{worker.supportedProtocols.join(' / ') || '—'}</b><small className="block text-asamu-muted">缓存 {worker.cachedImages.length} 个镜像</small>{worker.lastErrorCode && <small className="block font-black text-red-600">镜像读取异常 · {worker.lastErrorCode}</small>}</div>,
        formatTime(worker.lastHeartbeat),
        <PixelButton size="sm" variant={worker.draining ? 'primary' : 'yellow'} disabled={worker.status === 'offline' || worker.status === 'disabled'} onClick={() => { setWorkerAction(worker); setReason('') }}>{worker.draining ? '恢复接单' : '排空节点'}</PixelButton>,
      ])} />}
    </PixelCard>
    <div className="grid gap-5 xl:grid-cols-[minmax(0,1.25fr)_minmax(360px,.75fr)]">
      <PixelCard title="实例列表" action={<PixelTag tone="blue">{filtered.length} 个</PixelTag>}>
        <PixelInput className="mb-4 w-full" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索实例、题目、用户或比赛" />
        {loading ? <p className="py-10 text-center text-sm font-bold text-asamu-muted">正在加载实例…</p> : <DataTable headers={['题目 / 实例', '归属', '状态', '端口', '到期时间', '']} rows={filtered.map((item) => [
          <div><b>{item.challengeTitle}</b><small className="mt-1 block font-mono text-asamu-muted">{item.id.slice(0, 13)}…</small></div>,
          <div><b>{item.ownerName}</b><small className="block text-asamu-muted">{item.ownerScope === 'team' ? '战队' : '用户'}{item.competitionName ? ` · ${item.competitionName}` : ''}</small></div>,
          <InstanceStatus status={item.status} />,
          item.hostPort ? `${item.hostPort} → ${item.internalPort ?? '—'}` : '—',
          formatTime(item.expiresAt),
          <PixelButton size="sm" variant={selected?.id === item.id ? 'primary' : 'secondary'} onClick={() => void select(item)}>详情</PixelButton>,
        ])} />}
      </PixelCard>

      <PixelCard title="实例详情">
        {!selected ? <p className="py-10 text-center text-sm text-asamu-muted">暂无实例</p> : <div className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3"><div><h2 className="font-display text-xl font-black">{selected.challengeTitle}</h2><p className="mt-1 font-mono text-xs text-asamu-muted">{selected.id}</p></div><InstanceStatus status={selected.status} /></div>
          <dl className="grid grid-cols-2 gap-3 text-sm">
            <Detail label="归属" value={`${selected.ownerName}（${selected.ownerScope}）`} />
            <Detail label="运行时" value={selected.runtimeProvider} />
            <Detail label="版本 / 代次" value={`${selected.version} / ${selected.generation}`} />
            <Detail label="端口" value={selected.hostPort ? `${selected.hostPort} → ${selected.internalPort ?? '—'}` : '未分配'} />
            <Detail label="启动时间" value={formatTime(selected.startedAt)} />
            <Detail label="到期时间" value={formatTime(selected.expiresAt)} />
          </dl>
          {(selected.errorCode || selected.errorMessage) && <div className="border-2 border-red-200 bg-red-50 p-3 text-xs text-red-800"><b>{selected.errorCode || 'RUNTIME_ERROR'}</b><p className="mt-1 break-words">{selected.errorMessage}</p></div>}
          <div className="flex flex-wrap gap-3 border-t border-asamu-line pt-4">
            <PixelButton variant="danger" disabled={!stoppable.has(selected.status)} onClick={() => openAction('stop')}>停止实例</PixelButton>
            <PixelButton variant="yellow" disabled={selected.status !== 'running'} onClick={() => openAction('reset')}>重置并轮换 Flag</PixelButton>
          </div>
        </div>}
      </PixelCard>
    </div>

    <div className="mt-5 grid gap-5 xl:grid-cols-2">
      <PixelCard title="生命周期操作（最近 50 条）"><DataTable headers={['时间', '操作', '状态迁移', '结果']} rows={operations.map((item) => [formatTime(item.requestedAt), item.operation, `${item.fromStatus} → ${item.toStatus}`, <InstanceStatus status={item.result} />])} /></PixelCard>
      <PixelCard title="脱敏运行事件（最近 200 条）"><div className="max-h-[520px] space-y-3 overflow-y-auto">{events.length === 0 ? <p className="py-8 text-center text-sm text-asamu-muted">暂无运行事件</p> : events.map((event) => <div key={event.id} className="border border-asamu-line bg-slate-50 p-3 text-xs"><div className="flex justify-between gap-3"><b>{event.type}</b><time className="text-asamu-muted">{formatTime(event.createdAt)}</time></div>{event.providerStatus && <p className="mt-1 font-bold text-asamu-blue">{event.providerStatus}</p>}<pre className="mt-2 overflow-x-auto whitespace-pre-wrap break-all text-[11px] text-asamu-muted">{JSON.stringify(event.payload, null, 2)}</pre></div>)}</div></PixelCard>
    </div>

    <Modal open={Boolean(action)} title={action === 'reset' ? '确认重置实例' : '确认停止实例'} onClose={() => !submitting && setAction(undefined)}>
      <div className="space-y-4 text-sm">
        <p className="font-semibold leading-6">{action === 'reset' ? '重置会销毁当前容器并轮换 Flag，已有连接将立即失效。' : '停止会终止当前实例，用户将无法继续访问。'}该操作会写入审计日志。</p>
        <label className="block font-black">操作理由（必填）<textarea className="pixel-input mt-2 min-h-24" maxLength={500} value={reason} onChange={(event) => setReason(event.target.value)} placeholder="说明工单、比赛异常或处置依据（至少 4 个字符）" /></label>
        {action === 'reset' && <label className="block font-black">输入 <code>{confirmToken}</code> 进行二次确认<PixelInput className="mt-2 w-full" value={confirmation} onChange={(event) => setConfirmation(event.target.value)} /></label>}
        <div className="flex justify-end gap-3"><PixelButton variant="secondary" disabled={submitting} onClick={() => setAction(undefined)}>取消</PixelButton><PixelButton variant="danger" disabled={!actionReady || submitting} onClick={() => void submitAction()}>{submitting ? '提交中…' : '确认执行'}</PixelButton></div>
      </div>
    </Modal>
    <Modal open={Boolean(workerAction)} title={workerAction?.draining ? '恢复 Worker 接单' : '排空 Worker 节点'} onClose={() => !submitting && setWorkerAction(undefined)}>
      <div className="space-y-4 text-sm">
        <p className="font-semibold leading-6">{workerAction?.draining ? '恢复后节点可以重新接收新实例启动任务。' : '排空后节点不再接收新实例，但仍会处理已有实例的停止、重启和清理任务。'}操作会写入审计日志。</p>
        <label className="block font-black">操作理由（必填）<textarea className="pixel-input mt-2 min-h-24" maxLength={500} value={reason} onChange={(event) => setReason(event.target.value)} placeholder="说明维护窗口、容量调整或恢复依据（至少 4 个字符）" /></label>
        <div className="flex justify-end gap-3"><PixelButton variant="secondary" disabled={submitting} onClick={() => setWorkerAction(undefined)}>取消</PixelButton><PixelButton variant="danger" disabled={reason.trim().length < 4 || submitting} onClick={() => void submitWorkerAction()}>{submitting ? '提交中…' : '确认执行'}</PixelButton></div>
      </div>
    </Modal>
  </>
}

function Detail({ label, value }: { label: string; value: string }) { return <div className="border-b border-asamu-line pb-2"><dt className="text-xs font-black text-asamu-muted">{label}</dt><dd className="mt-1 break-all font-semibold">{value}</dd></div> }
function formatTime(value?: string) { return value ? new Date(value).toLocaleString('zh-CN') : '—' }
function InstanceStatus({ status }: { status: string }) {
  const tone = ['running', 'completed', 'success'].includes(status) ? 'green' : ['failed', 'interrupted', 'expired'].includes(status) ? 'red' : ['stopped', 'deleted'].includes(status) ? 'slate' : 'yellow'
  return <StatusBadge tone={tone}>{status}</StatusBadge>
}
