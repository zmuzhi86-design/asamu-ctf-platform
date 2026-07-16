import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { PageContainer, PageHeader, PixelButton, PixelCard, PixelInput, PixelSelect, PixelTag, RobotTip } from '../components/ui/System'
import { useAuth } from '../contexts/AuthProvider'
import { ApiError } from '../services/apiClient'
import { createWriteup, fetchMyWriteup, fetchMyWriteups, submitWriteup, updateWriteup, type WriteupMutation } from '../services/communityApi'
import { fetchChallenges, type ChallengeDto, type WriteupDto } from '../services/platformApi'

const emptyForm: WriteupMutation = { title: '', summary: '', contentMarkdown: '', visibility: 'public', challengeId: '' }

export function WriteupEditorPage() {
  const { id } = useParams()
  const auth = useAuth()
  const navigate = useNavigate()
  const [form, setForm] = useState<WriteupMutation>(emptyForm)
  const [mine, setMine] = useState<WriteupDto[]>([])
  const [challenges, setChallenges] = useState<ChallengeDto[]>([])
  const [status, setStatus] = useState('draft')
  const [rejectReason, setRejectReason] = useState('')
  const [preview, setPreview] = useState('')
  const [pending, setPending] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  useEffect(() => { if (!auth.user) return; Promise.all([fetchMyWriteups(), fetchChallenges()]).then(([writeups, challengePage]) => { setMine(writeups.items); setChallenges(challengePage.items) }).catch((reason: Error) => setError(reason.message)) }, [auth.user])
  useEffect(() => { if (!id) { setForm(emptyForm); setStatus('draft'); setPreview(''); return }; fetchMyWriteup(id).then((item) => { setForm({ title: item.title, summary: item.summary, contentMarkdown: item.contentMarkdown ?? '', visibility: item.visibility as WriteupMutation['visibility'], challengeId: item.challengeId ?? '', competitionId: item.competitionId }); setStatus(item.status); setRejectReason(item.rejectReason ?? ''); setPreview(item.contentHTML) }).catch((reason: Error) => setError(reason.message)) }, [id])
  const save = async () => { setPending(true); setError(''); setMessage(''); try { const item = id ? await updateWriteup(id, form) : await createWriteup(form); setPreview(item.contentHTML); setStatus(item.status); setMessage('草稿已保存并生成新修订版本。'); const page = await fetchMyWriteups(); setMine(page.items); if (!id) navigate(`/writeups/${item.id}/edit`, { replace: true }) } catch (reason) { setError(reason instanceof ApiError ? reason.message : '保存失败') } finally { setPending(false) } }
  const submit = async () => { if (!id) return; setPending(true); setError(''); try { await submitWriteup(id); setStatus('review'); setMessage('已提交审核，审核期间内容锁定。') } catch (reason) { setError(reason instanceof ApiError ? reason.message : '提交失败') } finally { setPending(false) } }
  if (!auth.user) return <PageContainer><PixelCard className="mx-auto max-w-lg" title="登录后创作"><Link to="/login" state={{ from: '/writeups/new' }}><PixelButton>前往登录</PixelButton></Link></PixelCard></PageContainer>
  const locked = status === 'review' || status === 'published' || status === 'archived'
  return <PageContainer><PageHeader eyebrow="WRITEUP STUDIO" title="WriteUp 创作台" description="Markdown 原文按修订版本保存，服务端渲染并清洗 HTML 后发布。"><div className="flex gap-2"><PixelTag tone={status === 'rejected' ? 'red' : status === 'published' ? 'green' : status === 'review' ? 'yellow' : 'blue'}>{status}</PixelTag><Link to="/writeups"><PixelButton variant="secondary">返回文章中心</PixelButton></Link></div></PageHeader>
    {rejectReason && <RobotTip className="mb-5" title="审核意见">{rejectReason}</RobotTip>}{message && <RobotTip className="mb-5" title="操作完成">{message}</RobotTip>}{error && <p className="mb-5 border-2 border-red-400 bg-red-50 p-3 text-sm font-black text-red-700">{error}</p>}
    <div className="grid gap-6 xl:grid-cols-[240px_minmax(0,1fr)_minmax(0,1fr)]"><PixelCard title="我的文章"><Link to="/writeups/new"><PixelButton className="mb-4 w-full" size="sm">新建草稿</PixelButton></Link><div className="space-y-2">{mine.map((item) => <Link className={`block border-2 p-3 text-sm ${id === item.id ? 'border-asamu-blue bg-asamu-soft' : 'border-asamu-line'}`} to={`/writeups/${item.id}/edit`} key={item.id}><b>{item.title}</b><p className="mt-1 text-xs text-asamu-muted">{item.status} · {item.visibility}</p></Link>)}</div></PixelCard>
      <PixelCard title="内容编辑"><div className="space-y-4"><label className="block text-sm font-black">标题<PixelInput className="mt-2 w-full" maxLength={160} disabled={locked} value={form.title} onChange={(event) => setForm({ ...form, title: event.target.value })} /></label><label className="block text-sm font-black">关联题目<PixelSelect className="mt-2 w-full" disabled={locked} value={form.challengeId} onChange={(event) => setForm({ ...form, challengeId: event.target.value })}><option value="">请选择已发布题目</option>{challenges.map((item) => <option value={item.id} key={item.id}>{item.title}</option>)}</PixelSelect></label><label className="block text-sm font-black">可见范围<PixelSelect className="mt-2 w-full" disabled={locked} value={form.visibility} onChange={(event) => setForm({ ...form, visibility: event.target.value as WriteupMutation['visibility'] })}><option value="public">公开并进入列表</option><option value="unlisted">仅链接可见</option><option value="private">仅自己可见</option></PixelSelect></label><label className="block text-sm font-black">摘要<textarea className="pixel-input mt-2 min-h-24 w-full" maxLength={1000} disabled={locked} value={form.summary} onChange={(event) => setForm({ ...form, summary: event.target.value })} /></label><label className="block text-sm font-black">Markdown 正文<textarea className="pixel-input mt-2 min-h-[420px] w-full font-mono text-sm" maxLength={500000} disabled={locked} value={form.contentMarkdown} onChange={(event) => setForm({ ...form, contentMarkdown: event.target.value })} /></label><div className="flex gap-2"><PixelButton disabled={pending || locked || !form.title || !form.challengeId} onClick={save}>{pending ? '处理中…' : '保存草稿'}</PixelButton><PixelButton variant="yellow" disabled={pending || !id || (status !== 'draft' && status !== 'rejected')} onClick={submit}>提交审核</PixelButton></div></div></PixelCard>
      <PixelCard title="服务端安全预览">{preview ? <div className="writeup-content leading-8" dangerouslySetInnerHTML={{ __html: preview }} /> : <p className="text-sm font-semibold text-asamu-muted">保存草稿后显示经过服务端清洗的 HTML 预览。</p>}</PixelCard></div>
  </PageContainer>
}
