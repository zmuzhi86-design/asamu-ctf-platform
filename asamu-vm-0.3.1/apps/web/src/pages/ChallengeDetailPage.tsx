import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { AssetImage, Modal, PageContainer, PixelButton, PixelCard, PixelInput, PixelTag, RobotTip, SceneArtwork, SecondaryCard, StatusBadge, Toast } from '../components/ui/System'
import { getChallengeDetail, type SubmissionRecord } from '../data/challenges'
import { assets } from '../data/assetManifest'
import { getChallengeInstance, isTransitionalInstanceStatus, resetChallengeInstance, restartChallengeInstance, startChallengeInstance, stopChallengeInstance, type ChallengeInstance, type InstanceStatus } from '../services/instanceApi'
import { downloadChallengeFile, fetchChallenge, fetchChallengeHints, fetchChallengeSubmissions, submitChallengeFlag, unlockChallengeHint, type HintItem } from '../services/platformApi'
import { ApiError, mockApiEnabled } from '../services/apiClient'
import { useAuth } from '../contexts/AuthProvider'

const directionKey = (label: string) => `direction.${label === 'AI Security' ? 'ai_security' : label.toLowerCase()}.scene`
const maskFlag = (value: string) => value.length <= 12 ? 'flag{***}' : `${value.slice(0, 7)}…${value.slice(-3)}`
type AttachmentItem = { name: string; size: string; downloadUrl?: string }
const statusMeta: Record<InstanceStatus, { label: string; tone: 'blue' | 'yellow' | 'green' | 'red' | 'slate'; description: string }> = {
  pending: { label: '等待调度', tone: 'yellow', description: '启动任务已提交，正在选择可用 Worker。' },
  pulling: { label: '准备镜像', tone: 'yellow', description: 'Worker 正在准备容器镜像。' },
  creating: { label: '创建容器', tone: 'yellow', description: '正在创建隔离网络和容器。' },
  starting: { label: '启动中', tone: 'yellow', description: '正在分配容器、端口和动态 Flag。' },
  running: { label: '运行中', tone: 'green', description: '实验舱已就绪，可以开始挑战。' },
  restarting: { label: '重启中', tone: 'yellow', description: '正在重启当前实例，环境数据不会被清空。' },
  resetting: { label: '重置中', tone: 'yellow', description: '正在销毁旧实例并创建干净环境。' },
  stopping: { label: '关闭中', tone: 'yellow', description: '正在释放端口和运行资源。' },
  stopped: { label: '未启动', tone: 'slate', description: '点击按钮创建你的专属靶机环境。' },
  failed: { label: '启动失败', tone: 'red', description: '环境暂时不可用，请重新尝试。' },
  expired: { label: '已过期', tone: 'red', description: '环境已自动回收，请重新启动。' },
  interrupted: { label: '运行中断', tone: 'red', description: 'Worker 检测到容器异常退出，请重新启动。' },
  deleted: { label: '已删除', tone: 'slate', description: '该实例已经被删除。' },
}

export function ChallengeDetailPage() {
  const auth = useAuth()
  const { id = 'sqli-art' } = useParams()
  const [challenge, setChallenge] = useState(() => getChallengeDetail(id))
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [loadedIdentifier, setLoadedIdentifier] = useState('')
  const [instance, setInstance] = useState<ChallengeInstance>({ id: '', challengeId: id, status: 'stopped', version: 0 })
  const [pending, setPending] = useState(false)
  const [moreOpen, setMoreOpen] = useState(false)
  const [confirm, setConfirm] = useState<'stop' | 'reset' | null>(null)
  const [flag, setFlag] = useState('')
  const [records, setRecords] = useState<SubmissionRecord[]>(mockApiEnabled ? challenge.submissions : [])
  const [feedback, setFeedback] = useState<'success' | 'error' | null>(null)
  const [feedbackMessage, setFeedbackMessage] = useState('')
  const [copied, setCopied] = useState(false)
  const [attempts, setAttempts] = useState(0)
  const [solved, setSolved] = useState(false)
  const [hintCards, setHintCards] = useState<HintItem[]>([])
  const [hintPending, setHintPending] = useState<number | null>(null)
  const [downloadPending, setDownloadPending] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    const fallback = getChallengeDetail(id)
    setLoading(true)
    setLoadError('')
    setChallenge(fallback)
    setHintCards([])
    setInstance({ id: '', challengeId: id, status: 'stopped', version: 0 })
    fetchChallenge(id).then((item) => {
      if (!active) return
      const files = (item.files ?? []).filter((file, index, rows) => rows.findIndex((row) => row.name === file.name && row.size === file.size && row.mimeType === file.mimeType) === index)
      setHintCards((item.hints ?? []).map((hint, index) => ({ index, title: hint.title, cost: hint.cost, unlocked: false })))
      setChallenge({ ...fallback, id: item.slug || item.id, title: item.title, category: item.category, difficulty: item.difficulty, score: item.score, solves: item.solves, solveRate: `${item.solveRate.toFixed(2)}%`, tags: item.tags, dynamic: item.dynamic, attachment: item.attachment, writeup: item.writeup, author: item.author, description: item.description, story: item.summary || '请结合题目描述、附件与 Hint 完成挑战。', attachments: files.map((file) => ({ name: file.name, size: file.size ? `${Math.max(1, Math.round(file.size / 1024))} KB` : '附件', downloadUrl: file.downloadUrl })), hints: [], knowledgePoints: item.knowledgePoints ?? item.tags, firstBloods: (item.bloods ?? []).map((entry, index) => ({ label: (['一血', '二血', '三血'][index] ?? '三血') as '一血' | '二血' | '三血', user: entry.username, time: new Date(entry.createdAt).toLocaleTimeString('zh-CN', { hour12: false }) })), similarChallenges: [], discussionPrompt: '', submissions: [] })
      setLoadedIdentifier(id)
      setLoading(false)
      if (auth.user) {
        void fetchChallengeHints(id).then((items) => active && setHintCards(items)).catch(() => undefined)
        void fetchChallengeSubmissions(id).then((page) => { if (!active) return; setAttempts(page.total); setSolved(page.items.some((entry) => entry.result === 'correct')); setRecords(page.items.slice(0, 5).map((entry) => ({ value: 'Flag 已安全隐藏', result: entry.result === 'correct' ? 'success' : 'error', time: new Date(entry.createdAt).toLocaleString('zh-CN') }))) }).catch(() => undefined)
        if (item.dynamic) void getChallengeInstance(id).then((value) => active && setInstance(value)).catch(() => undefined)
      }
    }).catch((error) => {
      if (active) { setLoadError(error instanceof Error ? error.message : '题目加载失败'); setLoadedIdentifier(id); setLoading(false) }
    })
    return () => { active = false }
  }, [auth.user, id])

  useEffect(() => {
	if (!auth.user || !challenge.dynamic || !isTransitionalInstanceStatus(instance.status)) return
	let active = true
	const timer = window.setInterval(() => {
		void getChallengeInstance(id).then((value) => { if (active) setInstance(value) }).catch(() => undefined)
	}, 2_000)
	return () => { active = false; window.clearInterval(timer) }
  }, [auth.user, challenge.dynamic, id, instance.status])

  const operate = async (transitionalStatus: InstanceStatus, action: () => Promise<ChallengeInstance>) => {
    if (pending) return
    const previous = instance
    setPending(true)
    setInstance((current) => ({ ...current, status: transitionalStatus }))
    try { setInstance(await action()) } catch (error) { setFeedback('error'); setFeedbackMessage(error instanceof ApiError ? error.message : '环境操作失败'); try { setInstance(await getChallengeInstance(id)) } catch { setInstance(previous) } } finally { setPending(false); setConfirm(null); setMoreOpen(false) }
  }
  const copyAddress = async () => {
    if (!instance.accessUrl || instance.status !== 'running') return
    try { await navigator.clipboard.writeText(instance.accessUrl) } catch {}
    setCopied(true); window.setTimeout(() => setCopied(false), 1400)
  }
  const submitFlag = async () => {
    if (!flag.trim()) return
    const submitted = flag.trim()
    try { const result = await submitChallengeFlag(id, submitted); const success = result.correct; const nextRecord: SubmissionRecord = { value: maskFlag(submitted), result: success ? 'success' : 'error', time: '刚刚' }; setRecords((current) => [nextRecord, ...current].slice(0, 5)); setAttempts((value) => value + 1); setSolved((value) => value || success); setFeedback(success ? 'success' : 'error'); setFeedbackMessage(result.message + (result.awardedScore ? ` · +${result.awardedScore} 分` : '')) } catch (error) { setFeedback('error'); setFeedbackMessage(error instanceof ApiError ? error.message : '提交失败') } finally { setFlag('') }
  }
  const unlockHint = async (item: HintItem) => {
    if (!auth.user) { setFeedback('error'); setFeedbackMessage('请先登录后解锁 Hint'); return }
    if (item.cost > 0 && !window.confirm(`解锁该 Hint 将扣除 ${item.cost} 分，是否继续？`)) return
    setHintPending(item.index)
    try { const unlocked = await unlockChallengeHint(id, item.index); setHintCards((current) => current.map((row) => row.index === item.index ? unlocked : row)); setFeedback('success'); setFeedbackMessage(unlocked.cost ? `Hint 已解锁，扣除 ${unlocked.cost} 分` : 'Hint 已解锁') } catch (error) { setFeedback('error'); setFeedbackMessage(error instanceof Error ? error.message : 'Hint 解锁失败') } finally { setHintPending(null) }
  }
  const downloadAttachment = async (item: AttachmentItem) => {
    if (!auth.user) { setFeedback('error'); setFeedbackMessage('请先登录后下载附件'); return }
    if (!item.downloadUrl || downloadPending) return
    setDownloadPending(item.downloadUrl)
    try { await downloadChallengeFile(item.downloadUrl, item.name) } catch (error) { setFeedback('error'); setFeedbackMessage(error instanceof Error ? error.message : '附件下载失败') } finally { setDownloadPending(null) }
  }

  if (loading || loadedIdentifier !== id) return <PageContainer><PixelCard><p className="font-semibold text-asamu-muted">正在读取题目资料…</p></PixelCard></PageContainer>
  if (loadError) return <PageContainer><PixelCard title="无法打开题目"><p className="font-semibold text-red-700">{loadError}</p><Link to="/challenges"><PixelButton className="mt-5">返回题库</PixelButton></Link></PixelCard></PageContainer>

  return <PageContainer>
    <section className="mb-6 overflow-hidden border-2 border-asamu-ink bg-asamu-card shadow-pixel" style={{ borderRadius: 8 }}><div className="flex flex-wrap items-center justify-between gap-3 border-b border-asamu-line px-4 py-3 text-sm font-bold text-asamu-muted"><p>题库 <span className="mx-2 text-asamu-blue">›</span> {challenge.category} <span className="mx-2 text-asamu-blue">›</span> {challenge.title}</p><Link to="/challenges"><PixelButton size="sm" variant="secondary">返回题库</PixelButton></Link></div><div className="grid gap-5 p-5 lg:grid-cols-[1fr_auto] lg:items-center"><div className="flex items-center gap-4"><SceneArtwork className="h-24 w-24 shrink-0" assetKey={directionKey(challenge.category)} alt={`${challenge.category} 方向`} /><div><div className="flex flex-wrap gap-2"><PixelTag>{challenge.category}</PixelTag><PixelTag tone="yellow">{challenge.difficulty}</PixelTag>{solved && <PixelTag tone="green">已解</PixelTag>}</div><h1 className="mt-3 font-display text-3xl font-black sm:text-4xl">{challenge.title}</h1><p className="mt-2 text-sm font-semibold text-asamu-muted">出题人：{challenge.author} · 题号 #{challenge.id.toUpperCase()}</p></div></div><div className="grid grid-cols-3 gap-2 text-center"><SummaryMetric value={`${challenge.score}`} label="分值" /><SummaryMetric value={challenge.solves.toLocaleString()} label="解出人数" /><SummaryMetric value={challenge.solveRate} label="解出率" /></div></div></section>

    <div className="desktop-three-columns grid items-start gap-6" style={{ gridTemplateColumns: '240px minmax(0, 1fr) 360px' }}>
      <aside className="sticky top-24 space-y-5"><PixelCard padded={false}><div className="h-44 bg-asamu-soft"><SceneArtwork className="h-full w-full" assetKey={directionKey(challenge.category)} alt={`${challenge.category} 训练场景`} /></div><div className="p-4"><h2 className="font-display text-lg font-black">任务资料包</h2><p className="mt-2 text-sm font-semibold leading-6 text-asamu-muted">{challenge.story}</p></div></PixelCard><SecondaryCard title="基础信息"><InfoRow label="题目类型" value={challenge.category} /><InfoRow label="难度" value={challenge.difficulty} /><InfoRow label="附件" value={`${challenge.attachments.length} 个`} /><InfoRow label="环境" value={challenge.dynamic ? '动态靶机' : '静态题目'} /></SecondaryCard>{challenge.dynamic && challenge.attachments.length > 0 && <SecondaryCard title="附件下载"><AttachmentList attachments={challenge.attachments} pending={downloadPending} onDownload={downloadAttachment} /></SecondaryCard>}{challenge.knowledgePoints.length > 0 && <SecondaryCard title="知识点标签"><div className="flex flex-wrap gap-2">{challenge.knowledgePoints.map((item) => <PixelTag key={item}>{item}</PixelTag>)}</div></SecondaryCard>}<SecondaryCard title="学习中心"><p className="text-sm font-semibold text-asamu-muted">查看管理员发布的学习路线与知识内容。</p><Link to="/learning"><PixelButton className="mt-3 w-full" size="sm" variant="secondary">进入学习中心</PixelButton></Link></SecondaryCard></aside>

      <main className="space-y-6">{challenge.dynamic ? <EnvironmentConsole instance={instance} pending={pending} copied={copied} moreOpen={moreOpen} setMoreOpen={setMoreOpen} onStart={() => operate('starting', () => startChallengeInstance(id))} onRestart={() => operate('restarting', () => restartChallengeInstance(id))} onStop={() => setConfirm('stop')} onReset={() => setConfirm('reset')} onCopy={copyAddress} /> : <StaticAttachmentPanel attachments={challenge.attachments} pending={downloadPending} onDownload={downloadAttachment} />}<FlagTerminal flag={flag} onChange={setFlag} onSubmit={submitFlag} feedback={feedback} /><PixelCard title="最近提交记录"><div className="space-y-2">{records.length ? records.map((record, index) => <div className="flex items-center justify-between gap-3 border-b border-asamu-line py-2 text-sm font-semibold last:border-0" key={`${record.value}-${index}`}><span className={record.result === 'success' ? 'text-asamu-success' : 'text-asamu-danger'}>{record.result === 'success' ? '正确' : '错误'}</span><code className="min-w-0 flex-1 truncate text-asamu-ink">{record.value}</code><span className="text-xs text-asamu-muted">{record.time}</span></div>) : <p className="text-sm font-semibold text-asamu-muted">暂无提交记录。</p>}</div></PixelCard><RobotTip title="作战提示">{challenge.dynamic ? '先启动属于你的隔离实验舱，确认访问地址与端口可用，再根据题目描述和 Hint 验证攻击链。动态 Flag 只属于当前账号。' : '下载题目附件并在本地隔离环境中分析，根据题目描述与 Hint 逐步验证思路。无需启动动态靶机。'}</RobotTip></main>

      <aside className="sticky top-24 space-y-5"><SecondaryCard title="题目描述"><p className="whitespace-pre-wrap text-sm font-medium leading-7 text-asamu-muted">{challenge.description || '管理员尚未填写题目描述。'}</p><div className="mt-4 flex flex-wrap gap-2">{challenge.tags.map((tag) => <PixelTag key={tag}>#{tag}</PixelTag>)}</div></SecondaryCard><SecondaryCard title="Hint 线索卡">{hintCards.length ? <div className="space-y-2">{hintCards.map((item) => <div className="border border-asamu-line bg-asamu-soft p-3 text-sm" key={item.index}><div className="flex items-center justify-between gap-2"><b className="text-asamu-blue">{item.title || `Hint ${item.index + 1}`}</b>{item.unlocked ? <PixelTag tone="green">已解锁</PixelTag> : <PixelButton size="sm" variant="yellow" disabled={hintPending === item.index} onClick={() => void unlockHint(item)}>{hintPending === item.index ? '解锁中…' : item.cost ? `解锁 -${item.cost}` : '免费解锁'}</PixelButton>}</div>{item.unlocked && <p className="mt-2 whitespace-pre-wrap leading-6 text-asamu-muted">{item.content}</p>}</div>)}</div> : <p className="text-sm font-semibold text-asamu-muted">本题没有 Hint。</p>}</SecondaryCard>{challenge.firstBloods.length > 0 && <HonorBoard entries={challenge.firstBloods} />}<SecondaryCard title="我的解题状态"><div className="flex items-center gap-3"><AssetImage className="h-16 w-16" assetKey={solved ? 'flag.feedback.success' : 'challenge.instance.starting'} alt="解题状态" /><div><StatusBadge tone={solved ? 'green' : 'yellow'}>{solved ? '已解' : '未解'}</StatusBadge><p className="mt-2 text-sm font-bold">尝试次数：{attempts}</p><p className="text-xs text-asamu-muted">提交记录：{attempts ? `共 ${attempts} 次` : '暂无'}</p></div></div></SecondaryCard></aside>
    </div>

    <Modal open={challenge.dynamic && confirm === 'reset'} title="确认重置环境？" onClose={() => !pending && setConfirm(null)}><p className="text-sm font-semibold leading-7 text-asamu-muted">重置后，当前环境中的文件、进程、配置修改和操作结果都将被清除，系统会销毁当前实例并重新创建一套全新的初始环境。此操作无法撤销。</p><div className="mt-6 flex justify-end gap-3"><PixelButton variant="secondary" onClick={() => setConfirm(null)} disabled={pending}>取消</PixelButton><PixelButton variant="danger" onClick={() => operate('resetting', () => resetChallengeInstance(id))} disabled={pending}>{pending ? '重置中…' : '确认重置'}</PixelButton></div></Modal>
    <Modal open={challenge.dynamic && confirm === 'stop'} title="确认关闭环境？" onClose={() => !pending && setConfirm(null)}><p className="text-sm font-semibold leading-7 text-asamu-muted">关闭后，当前靶场会停止运行，访问地址立即失效。如需继续挑战，需要重新启动环境。</p><div className="mt-6 flex justify-end gap-3"><PixelButton variant="secondary" onClick={() => setConfirm(null)} disabled={pending}>取消</PixelButton><PixelButton variant="danger" onClick={() => operate('stopping', () => stopChallengeInstance(id))} disabled={pending}>{pending ? '关闭中…' : '确认关闭'}</PixelButton></div></Modal>
    {feedback && <Toast tone={feedback === 'success' ? 'green' : 'red'} message={feedbackMessage || (feedback === 'success' ? 'Flag 正确！本题已点亮。' : '提交失败，请检查格式或继续分析。')} onClose={() => setFeedback(null)} />}
  </PageContainer>
}

function AttachmentList({ attachments, pending, onDownload, prominent = false }: { attachments: AttachmentItem[]; pending: string | null; onDownload: (item: AttachmentItem) => void; prominent?: boolean }) {
  if (!attachments.length) return <p className="border-2 border-dashed border-asamu-line bg-asamu-card/70 p-5 text-center text-sm font-semibold text-asamu-muted">该题暂未发布附件，请联系管理员补充。</p>
  return <div className="space-y-3">{attachments.map((item, index) => {
    const key = item.downloadUrl || `${item.name}-${index}`
    return <button className={`flex w-full items-center justify-between gap-4 border-2 border-asamu-line bg-asamu-card text-left font-bold transition hover:border-asamu-blue hover:bg-blue-50 disabled:cursor-wait disabled:opacity-60 ${prominent ? 'p-4 text-sm' : 'p-3 text-xs'}`} disabled={pending !== null} onClick={() => onDownload(item)} key={key}><span className="min-w-0"><span className="block truncate">{item.name}</span><small className="mt-1 block text-asamu-muted">{item.size}</small></span><span className="shrink-0 text-asamu-blue">{pending === item.downloadUrl ? '下载中…' : '下载附件'}</span></button>
  })}</div>
}

function StaticAttachmentPanel({ attachments, pending, onDownload }: { attachments: AttachmentItem[]; pending: string | null; onDownload: (item: AttachmentItem) => void }) {
  return <PixelCard className="bg-gradient-to-br from-asamu-soft via-asamu-card to-asamu-card" title="静态题目附件" action={<StatusBadge tone="blue">无需靶机</StatusBadge>}>
    <div className="grid gap-5 md:grid-cols-[1fr_190px]"><div><h2 className="font-display text-xl font-black">下载附件开始分析</h2><p className="mt-2 text-sm font-semibold leading-6 text-asamu-muted">本题为静态附件题，不会创建容器或分配端口。请下载资料包后在本地隔离环境中完成分析。</p><div className="mt-5"><AttachmentList attachments={attachments} pending={pending} onDownload={onDownload} prominent /></div></div><AssetImage className="mx-auto h-44 w-44" assetKey="challenge.instance.idle" alt="静态附件题" /></div>
    <div className="mt-5 grid grid-cols-3 gap-2"><SummaryMetric label="题目形态" value="静态附件" /><SummaryMetric label="附件数量" value={String(attachments.length)} /><SummaryMetric label="运行环境" value="本地分析" /></div>
  </PixelCard>
}

function EnvironmentConsole({ instance, pending, copied, moreOpen, setMoreOpen, onStart, onRestart, onStop, onReset, onCopy }: { instance: ChallengeInstance; pending: boolean; copied: boolean; moreOpen: boolean; setMoreOpen: (value: boolean) => void; onStart: () => void; onRestart: () => void; onStop: () => void; onReset: () => void; onCopy: () => void }) {
  const meta = statusMeta[instance.status]
  const running = instance.status === 'running'
  const transitioning = isTransitionalInstanceStatus(instance.status)
  const operationPending = pending || transitioning
  const artworkStatus = instance.status === 'running' ? 'running' : ['failed', 'expired', 'interrupted'].includes(instance.status) ? 'failed' : instance.status === 'stopped' || instance.status === 'deleted' ? 'idle' : 'starting'
  return <PixelCard className="bg-gradient-to-br from-asamu-soft via-asamu-card to-asamu-card" title="动态靶场实验舱" action={<div className="flex items-center gap-2"><StatusBadge pulse={operationPending} tone={meta.tone}>{meta.label}</StatusBadge>{running && <button className="border border-asamu-line bg-asamu-card px-2 py-1 text-xs font-black text-asamu-danger" aria-expanded={moreOpen} onClick={() => { setMoreOpen(false); onReset() }} disabled={operationPending}>重置环境</button>}</div>}>
    <div className="grid gap-5 md:grid-cols-[1fr_190px]"><div><h2 className="font-display text-xl font-black">{running ? '实验舱已就绪' : operationPending ? meta.label : '启动你的专属靶场'}</h2><p className="mt-2 text-sm font-semibold text-asamu-muted">{meta.description}</p>{(instance.status === 'failed' || instance.status === 'interrupted') && instance.errorCode && <div className="mt-4 border-2 border-red-300 bg-red-50 p-3 text-xs text-red-700"><b>{instance.errorCode}</b>{instance.errorMessage && <p className="mt-1 break-words">{instance.errorMessage}</p>}</div>}{running && instance.accessUrl ? <div className="mt-5 border-2 border-asamu-blue bg-asamu-card p-4"><span className="text-xs font-bold text-asamu-muted">访问地址</span>{/^https?:\/\//i.test(instance.accessUrl) ? <a href={instance.accessUrl} target="_blank" rel="noreferrer" className="mt-1 block break-all font-mono text-sm font-black text-asamu-blue">{instance.accessUrl}</a> : <code className="mt-1 block break-all font-mono text-sm font-black text-asamu-blue">{instance.accessUrl}</code>}</div> : <div className="mt-5 border-2 border-dashed border-asamu-line bg-asamu-card/70 p-5 text-center text-sm font-semibold text-asamu-muted">{operationPending ? '生命周期操作执行中，请勿重复提交。' : '启动后将自动分配访问地址与端口。'}</div>}</div><AssetImage className="mx-auto h-44 w-44" assetKey={`challenge.instance.${artworkStatus}`} alt={`环境状态：${meta.label}`} /></div>
    <div className="mt-5 grid grid-cols-3 gap-2"><SummaryMetric label="状态" value={meta.label} /><SummaryMetric label="端口" value={running ? String(instance.port ?? instance.internalPort ?? '—') : '—'} /><SummaryMetric label="剩余" value={running ? formatRemaining(instance.remainingSeconds, instance.expiresAt) : '—'} /></div>
    {running ? <div className="mt-5 grid gap-3 sm:grid-cols-3"><PixelButton variant="secondary" onClick={onRestart} disabled={operationPending}>重新启动</PixelButton><PixelButton variant="danger" onClick={onStop} disabled={operationPending}>关闭环境</PixelButton><PixelButton onClick={onCopy} disabled={operationPending || !instance.accessUrl}>{copied ? '已复制' : '复制地址'}</PixelButton></div> : <div className="mt-5"><PixelButton className="w-full" onClick={onStart} disabled={operationPending || instance.status === 'deleted'}>{operationPending ? meta.label : instance.status === 'failed' || instance.status === 'interrupted' ? '重新尝试' : '启动环境'}</PixelButton></div>}
  </PixelCard>
}

function FlagTerminal({ flag, onChange, onSubmit, feedback }: { flag: string; onChange: (value: string) => void; onSubmit: () => void; feedback: 'success' | 'error' | null }) { return <PixelCard className="bg-gradient-to-r from-yellow-50 via-asamu-card to-asamu-soft" title="Flag 验证终端"><div className="grid gap-5 md:grid-cols-[1fr_150px]"><div><p className="text-sm font-semibold text-asamu-muted">获取 Flag 后在这里验证，正确提交会同步更新积分与排行榜。</p><label className="mt-4 block text-sm font-black" htmlFor="flag-input">Flag 内容</label><div className="mt-2 flex flex-col gap-3 sm:flex-row"><PixelInput id="flag-input" className="font-mono" value={flag} onChange={(event) => onChange(event.target.value)} onKeyDown={(event) => event.key === 'Enter' && onSubmit()} placeholder="flag{...}" /><PixelButton onClick={onSubmit}>提交验证</PixelButton></div>{feedback && <div className={`mt-4 border-2 p-3 text-sm font-black ${feedback === 'success' ? 'border-green-500 bg-green-50 text-green-700' : 'border-red-500 bg-red-50 text-red-600'}`}>{feedback === 'success' ? '验证通过，恭喜完成挑战。' : '验证失败，请继续检查攻击链。'}</div>}</div><AssetImage className="mx-auto h-36 w-36" assetKey={feedback === 'success' ? 'flag.feedback.success' : feedback === 'error' ? 'flag.feedback.error' : 'mascot.default'} alt="Flag 提交反馈" /></div></PixelCard> }

function HonorBoard({ entries }: { entries: Array<{ label: '一血' | '二血' | '三血'; user: string; time: string }> }) { const keys = ['team.honor.gold', 'team.honor.silver', 'team.honor.bronze']; return <SecondaryCard title="前三血 / 最近解题"><div className="space-y-2">{entries.map((entry, index) => <div className="flex items-center gap-3 border-b border-asamu-line py-2 last:border-0" key={entry.label}><AssetImage className="h-10 w-10" assetKey={keys[index]} alt={entry.label} /><div className="min-w-0 flex-1"><b className="block text-sm">{entry.label} · {entry.user}</b><span className="text-xs text-asamu-muted">提交时间 {entry.time}</span></div></div>)}</div></SecondaryCard> }
function SummaryMetric({ value, label }: { value: string; label: string }) { return <div className="border border-asamu-line bg-asamu-soft px-3 py-2"><b className="block text-lg">{value}</b><span className="text-xs font-semibold text-asamu-muted">{label}</span></div> }
function formatRemaining(seconds?: number, expiresAt?: string) { const value = typeof seconds === 'number' ? seconds : expiresAt ? Math.max(0, Math.floor((new Date(expiresAt).getTime() - Date.now()) / 1000)) : 0; if (!value) return '即将到期'; const hours = Math.floor(value / 3600); const minutes = Math.floor((value % 3600) / 60); return `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}` }
function InfoRow({ label, value }: { label: string; value: string }) { return <div className="flex justify-between border-b border-asamu-line py-2 text-sm last:border-0"><span className="text-asamu-muted">{label}</span><b>{value}</b></div> }
