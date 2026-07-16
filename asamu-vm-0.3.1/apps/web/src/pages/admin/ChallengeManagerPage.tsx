import { useEffect, useState } from 'react'
import { Modal, PixelButton, PixelCard, PixelInput, PixelSelect, PixelTag, StatusBadge } from '../../components/ui/System'
import { useAuth } from '../../contexts/AuthProvider'
import { usePlatform } from '../../contexts/PlatformProvider'
import { archiveAdminChallenge, deleteChallengeFile, getAdminChallenge, listAdminChallenges, publishAdminChallenge, saveAdminChallenge, uploadChallengeFile, type AdminChallenge, type ChallengeMutation } from '../../services/adminChallengeApi'
import { adminInstanceApi } from '../../services/adminInstanceApi'
import type { ChallengeDto } from '../../services/platformApi'
import { registryCredentialApi, type RegistryCredential } from '../../services/registryCredentialApi'

type RuntimeMutation = NonNullable<ChallengeMutation['runtime']>
const emptyRuntime = (): RuntimeMutation => ({ imageRef: '', imageDigest: '', internalPort: 8080, protocol: 'tcp', flagFormat: 'standard', cpuMilli: 250, memoryMB: 128, pidsLimit: 64, diskMB: 64, ttlSeconds: 7200, maxTTLSeconds: 14400, readOnlyRootFS: true, environment: {} })
const empty = (): ChallengeMutation => ({ slug: '', title: '', categoryKey: 'web', difficulty: '入门', summary: '', description: '', author: 'asamu Lab', scoreMode: 'fixed', visibility: 'public', baseScore: 100, minimumScore: 100, maximumScore: 100, dynamicDecay: 50, isDynamic: false, tags: [], knowledgePoints: [], hintConfigs: [], runtime: emptyRuntime() })
const digestPattern = /^sha256:[a-f0-9]{64}$/

export function ChallengeManagerPage() {
  const { config } = usePlatform()
  const auth = useAuth()
  const [rows, setRows] = useState<ChallengeDto[]>([])
  const [registryCredentials, setRegistryCredentials] = useState<RegistryCredential[]>([])
  const [localImages, setLocalImages] = useState<string[]>([])
  const [localImagesLoading, setLocalImagesLoading] = useState(false)
  const [localImagesStatus, setLocalImagesStatus] = useState('')
  const [editingID, setEditingID] = useState<string>()
  const [form, setForm] = useState<ChallengeMutation>(empty())
  const [flag, setFlag] = useState('')
  const [files, setFiles] = useState<File[]>([])
  const [existingFiles, setExistingFiles] = useState<AdminChallenge['files']>([])
  const [environmentText, setEnvironmentText] = useState('')
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const [showArchived, setShowArchived] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const canPublish = auth.hasPermission('challenge.publish')
  const reload = () => listAdminChallenges().then((page) => setRows(page.items)).catch((reason: Error) => setError(reason.message))

  useEffect(() => {
    void reload()
    void registryCredentialApi.list().then((items) => setRegistryCredentials(items.filter((item) => item.enabled))).catch(() => setRegistryCredentials([]))
    void loadLocalImages()
  }, [])

  async function loadLocalImages() {
    setLocalImagesLoading(true)
    try {
      const workers = await adminInstanceApi.workers()
      const onlineWorkers = workers.filter((worker) => worker.status === 'online' || worker.status === 'draining')
      const images = [...new Set(onlineWorkers.flatMap((worker) => worker.cachedImages))].sort()
      setLocalImages(images)
      const failedWorkers = workers.filter((worker) => worker.lastErrorCode)
      if (failedWorkers.length) setLocalImagesStatus(`Worker 无法读取 Docker 镜像（${[...new Set(failedWorkers.map((worker) => worker.lastErrorCode))].join('、')}），请检查 Docker socket 权限后刷新。`)
      else if (!onlineWorkers.length) setLocalImagesStatus('没有在线 Worker，暂时无法核对宿主机镜像；仍可手动填写镜像标签。')
      else if (!images.length) setLocalImagesStatus('在线 Worker 尚未发现带标签的本地镜像，请确认镜像构建在 Worker 所连接的同一个 Docker daemon 中。')
      else setLocalImagesStatus('')
    } catch (reason) {
      setLocalImages([])
      setLocalImagesStatus(reason instanceof Error ? `读取 Worker 镜像失败：${reason.message}` : '读取 Worker 镜像失败。')
    } finally {
      setLocalImagesLoading(false)
    }
  }

  async function edit(id: string) {
    setError('')
    try {
      const item = await getAdminChallenge(id)
      const runtime: RuntimeMutation = item.runtime ? { registryCredentialId: item.runtime.registryCredentialId, imageRef: item.runtime.imageRef || '', imageDigest: item.runtime.imageDigest || '', internalPort: item.runtime.internalPort, protocol: item.runtime.protocol || 'tcp', flagFormat: item.runtime.flagFormat || 'standard', cpuMilli: item.runtime.cpuMilli, memoryMB: item.runtime.memoryMB, pidsLimit: item.runtime.pidsLimit || 64, diskMB: item.runtime.diskMB || 64, ttlSeconds: item.runtime.ttlSeconds, maxTTLSeconds: item.runtime.maxTTLSeconds, readOnlyRootFS: item.runtime.readOnlyRootFS ?? true, environment: item.runtime.environment ?? {} } : emptyRuntime()
      setEditingID(item.id)
      setFlag('')
      setFiles([])
      setExistingFiles(item.files ?? [])
      setEnvironmentText(Object.entries(runtime.environment).map(([key, value]) => `${key}=${value}`).join('\n'))
      setForm({ slug: item.slug, title: item.title, categoryKey: item.categoryKey, difficulty: item.difficulty, summary: item.summary, description: item.description, author: item.author, scoreMode: item.scoreMode || 'fixed', visibility: item.visibility || 'public', baseScore: item.score, minimumScore: item.minimumScore, maximumScore: item.maximumScore || item.score, dynamicDecay: item.dynamicDecay || 50, isDynamic: item.dynamic, tags: item.tags, knowledgePoints: item.knowledgePoints ?? [], hintConfigs: (item.hints ?? []).map((hint) => ({ title: hint.title, content: hint.content, cost: hint.cost })), runtime })
      setOpen(true)
      void loadLocalImages()
    } catch (reason) { setError(reason instanceof Error ? reason.message : '题目读取失败') }
  }

  function newChallenge() { setEditingID(undefined); setForm(empty()); setFlag(''); setFiles([]); setExistingFiles([]); setEnvironmentText(''); setError(''); setOpen(true); void loadLocalImages() }
  function parseEnvironment() {
    const environment: Record<string, string> = {}
    for (const raw of environmentText.split('\n')) {
      const line = raw.trim()
      if (!line) continue
      const separator = line.indexOf('=')
      if (separator < 1) throw new Error(`环境变量格式错误：${line}`)
      environment[line.slice(0, separator).trim()] = line.slice(separator + 1)
    }
    return environment
  }
  function validate(payload: ChallengeMutation) {
    if (!payload.title.trim() || !payload.categoryKey) throw new Error('标题和方向不能为空。')
    if (!editingID && !payload.isDynamic && !flag.trim()) throw new Error('静态题目首次保存必须填写 Flag。')
    if (payload.minimumScore > payload.baseScore || payload.baseScore > payload.maximumScore || payload.baseScore < 1) throw new Error('题目分值必须满足最低分 ≤ 基础分 ≤ 最高分。')
    if (payload.isDynamic && payload.runtime) {
      const runtime = payload.runtime
      if (!runtime.imageRef.trim()) throw new Error('请填写 Docker 镜像名称。')
      if (runtime.imageRef.includes('@') && (!digestPattern.test(runtime.imageDigest) || !runtime.imageRef.endsWith(`@${runtime.imageDigest}`))) throw new Error('固定镜像引用与 Digest 不一致。')
      if (runtime.imageDigest && (!digestPattern.test(runtime.imageDigest) || !runtime.imageRef.endsWith(`@${runtime.imageDigest}`))) throw new Error('镜像 Digest 格式不正确或与镜像引用不一致。')
      if (runtime.internalPort < 1 || runtime.internalPort > 65535) throw new Error('内部端口必须在 1 到 65535 之间。')
      if (runtime.maxTTLSeconds < runtime.ttlSeconds) throw new Error('最大运行时长不能小于默认运行时长。')
    }
  }

  async function save(publishAfter = false) {
    setBusy(true); setError(''); setMessage('')
    try {
      const runtime = form.isDynamic && form.runtime ? { ...form.runtime, environment: parseEnvironment(), registryCredentialId: form.runtime.registryCredentialId || registryCredentials.find((credential) => form.runtime?.imageRef.startsWith(`${credential.registryHost}/`))?.id } : undefined
      const payload: ChallengeMutation = { ...form, flags: flag.trim() ? [{ kind: 'static', value: flag.trim(), stage: 1 }] : undefined, runtime }
      validate(payload)
      const saved = await saveAdminChallenge(editingID, payload)
      for (const file of files) await uploadChallengeFile(saved.id, file)
      if (publishAfter) await publishAdminChallenge(saved.id)
      const refreshed = await getAdminChallenge(saved.id)
      setEditingID(saved.id); setExistingFiles(refreshed.files ?? []); setFiles([]); setFlag(''); setMessage(publishAfter ? '题目已保存并发布新版本。' : '题目草稿已保存。'); await reload()
    } catch (reason) { setError(reason instanceof Error ? reason.message : '题目保存失败') } finally { setBusy(false) }
  }

  async function removeFile(fileId: string) {
    if (!editingID || !window.confirm('确认删除这个草稿附件？已发布历史版本不会被改写。')) return
    setBusy(true); setError('')
    try { await deleteChallengeFile(editingID, fileId); setExistingFiles((items) => items.filter((item) => item.id !== fileId)); setMessage('附件已从草稿移除，重新发布后前台生效。') } catch (reason) { setError(reason instanceof Error ? reason.message : '附件删除失败') } finally { setBusy(false) }
  }
  async function publish(id: string) { if (!window.confirm('发布会生成不可变题目版本，是否继续？')) return; setBusy(true); setError(''); try { await publishAdminChallenge(id); setMessage('题目版本已发布。'); await reload() } catch (reason) { setError(reason instanceof Error ? reason.message : '发布失败') } finally { setBusy(false) } }
  async function archive(item: ChallengeDto) { if (!window.confirm(`确认${item.status === 'published' ? '下架' : '删除'}题目“${item.title}”？历史提交和已发布版本会保留，但题目将不再出现在前台。`)) return; setBusy(true); setError(''); try { await archiveAdminChallenge(item.id); setMessage('题目已归档并从前台下架。'); await reload() } catch (reason) { setError(reason instanceof Error ? reason.message : '题目删除失败') } finally { setBusy(false) } }
  const field = <K extends keyof ChallengeMutation>(key: K, value: ChallengeMutation[K]) => setForm({ ...form, [key]: value })
  const runtimeField = <K extends keyof RuntimeMutation>(key: K, value: RuntimeMutation[K]) => field('runtime', { ...(form.runtime ?? emptyRuntime()), [key]: value })
  const visibleRows = showArchived ? rows : rows.filter((row) => row.status !== 'archived')
  const imageChanged = (value: string) => { const imageRef = value.trim(); const match = imageRef.toLowerCase().match(/@(sha256:[a-f0-9]{64})$/); field('runtime', { ...(form.runtime ?? emptyRuntime()), imageRef, imageDigest: match?.[1] ?? '' }) }

  return <>
    <header className="mb-6 flex flex-wrap items-end justify-between gap-4 border-b-2 border-asamu-ink/10 pb-5"><div><p className="text-xs font-black tracking-[.18em] text-asamu-blue">CHALLENGE OPERATIONS</p><h1 className="mt-2 font-display text-3xl font-black">题目管理</h1><p className="mt-2 text-sm font-semibold text-asamu-muted">静态附件题与 Docker 动态题统一管理；发布会冻结不可变版本。</p></div><div className="flex gap-2"><PixelButton variant="secondary" onClick={() => setShowArchived((value) => !value)}>{showArchived ? '隐藏已归档' : '显示已归档'}</PixelButton><PixelButton onClick={newChallenge}>新建题目</PixelButton></div></header>
    {error && <div className="mb-4 border-2 border-red-300 bg-red-50 p-3 text-sm font-black text-red-700">{error}</div>}{message && <div className="mb-4 border-2 border-emerald-300 bg-emerald-50 p-3 text-sm font-black text-emerald-700">{message}</div>}
    <PixelCard><div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">{visibleRows.map((row) => <article className="border-2 border-asamu-line p-4" key={row.id}><div className="flex justify-between gap-2"><div><b>{row.title}</b><p className="text-xs font-bold text-asamu-muted">{row.category} · {row.difficulty}</p></div><StatusBadge tone={row.status === 'published' ? 'green' : row.status === 'archived' ? 'slate' : 'yellow'}>{row.status}</StatusBadge></div><div className="mt-3 flex gap-2"><PixelTag>{row.score} 分</PixelTag>{row.dynamic && <PixelTag tone="yellow">Docker 动态环境</PixelTag>}</div><div className="mt-4 flex flex-wrap gap-2"><PixelButton size="sm" variant="secondary" onClick={() => void edit(row.id)}>编辑</PixelButton>{canPublish && <PixelButton size="sm" disabled={busy || row.status === 'archived'} onClick={() => void publish(row.id)}>发布版本</PixelButton>}{row.status !== 'archived' && <PixelButton size="sm" variant="danger" disabled={busy} onClick={() => void archive(row)}>{row.status === 'published' ? '下架' : '删除'}</PixelButton>}</div></article>)}</div>{!visibleRows.length && <p className="py-12 text-center font-bold text-asamu-muted">暂无题目。</p>}</PixelCard>
    <Modal open={open} title={editingID ? '编辑题目草稿' : '新建题目'} onClose={() => !busy && setOpen(false)}>
      <div className="grid gap-4 sm:grid-cols-2">
        <Section title="基本信息" />
        <Field label="标题" value={form.title} onChange={(value) => field('title', value)} /><Field label="Slug（创建后不修改）" value={form.slug} onChange={(value) => field('slug', value)} />
        <label className="text-sm font-black">方向<PixelSelect className="mt-2" value={form.categoryKey} onChange={(event) => field('categoryKey', event.target.value)}>{config.directions.filter((direction) => direction.status === 'active').map((direction) => <option key={direction.slug} value={direction.slug}>{direction.name}</option>)}</PixelSelect></label>
        <label className="text-sm font-black">难度<PixelSelect className="mt-2" value={form.difficulty} onChange={(event) => field('difficulty', event.target.value)}>{['入门', '简单', '中等', '困难', '专家'].map((value) => <option key={value}>{value}</option>)}</PixelSelect></label>
        <Field label="出题人" value={form.author} onChange={(value) => field('author', value)} />
        <label className="text-sm font-black">可见性<PixelSelect className="mt-2" value={form.visibility} onChange={(event) => field('visibility', event.target.value)}><option value="public">公开</option><option value="private">仅授权范围</option></PixelSelect></label>
        <label className="text-sm font-black sm:col-span-2">摘要<textarea className="pixel-input mt-2 min-h-16" value={form.summary} onChange={(event) => field('summary', event.target.value)} /></label>
        <label className="text-sm font-black sm:col-span-2">题目正文（Markdown）<textarea className="pixel-input mt-2 min-h-28" value={form.description} onChange={(event) => field('description', event.target.value)} /></label>
        <Field label="标签（逗号分隔）" value={form.tags.join(',')} onChange={(value) => field('tags', value.split(',').map((item) => item.trim()).filter(Boolean))} /><Field label="知识点（逗号分隔）" value={form.knowledgePoints.join(',')} onChange={(value) => field('knowledgePoints', value.split(',').map((item) => item.trim()).filter(Boolean))} />
        <Section title="计分与 Flag" />
        <label className="text-sm font-black">计分模式<PixelSelect className="mt-2" value={form.scoreMode} onChange={(event) => field('scoreMode', event.target.value)}><option value="fixed">固定分</option><option value="dynamic">动态衰减</option></PixelSelect></label><span />
        <Field label="基础分" type="number" value={String(form.baseScore)} onChange={(value) => field('baseScore', Number(value))} /><Field label="最低分" type="number" value={String(form.minimumScore)} onChange={(value) => field('minimumScore', Number(value))} /><Field label="最高分" type="number" value={String(form.maximumScore)} onChange={(value) => field('maximumScore', Number(value))} /><Field label="动态衰减参数" type="number" value={String(form.dynamicDecay)} onChange={(value) => field('dynamicDecay', Number(value))} />
        <label className="flex items-center gap-2 text-sm font-black sm:col-span-2"><input type="checkbox" checked={form.isDynamic} onChange={(event) => field('isDynamic', event.target.checked)} />Docker 动态容器题（每位用户创建隔离实例和动态 Flag）</label>
        {!form.isDynamic && <Field label={editingID ? '新 Flag（留空保持原值）' : '静态 Flag'} value={flag} onChange={setFlag} />}
        {form.isDynamic && form.runtime && <>
          <Section title="Docker 运行环境" />
          <p className="border border-emerald-300 bg-emerald-50 p-3 text-xs font-bold leading-5 text-emerald-800 sm:col-span-2">本地模式直接填写宿主机上由 <code>docker build -t 镜像名:标签</code> 构建好的镜像，不需要仓库、Digest 或重新配置 Worker。远程自动拉取模式才需要配置镜像白名单。</p>
          <label className="text-sm font-black sm:col-span-2">私有镜像仓库<PixelSelect className="mt-2" value={form.runtime.registryCredentialId || ''} onChange={(event) => runtimeField('registryCredentialId', event.target.value || undefined)}><option value="">公开镜像 / 无凭据</option>{registryCredentials.map((credential) => <option key={credential.id} value={credential.id}>{credential.name} · {credential.registryHost}</option>)}</PixelSelect></label>
          <label className="text-sm font-black sm:col-span-2">容器镜像<PixelInput className="mt-2" list="runtime-local-images" value={form.runtime.imageRef} onChange={(event) => imageChanged(event.target.value)} /><datalist id="runtime-local-images">{localImages.map((image) => <option key={image} value={image} />)}</datalist><span className="mt-1 flex flex-wrap items-center justify-between gap-2 text-xs font-semibold text-asamu-muted"><span>可直接输入本地镜像标签；在线 Worker 已发现 {localImages.length} 个可用标签。</span><button className="font-black text-asamu-blue underline disabled:opacity-50" type="button" disabled={localImagesLoading} onClick={() => void loadLocalImages()}>{localImagesLoading ? '读取中…' : '刷新镜像'}</button></span>{localImagesStatus && <span className="mt-2 block border border-amber-300 bg-amber-50 p-2 text-xs font-bold leading-5 text-amber-800">{localImagesStatus}</span>}</label>
          <Field label="镜像 Digest（远程固定镜像可选）" value={form.runtime.imageDigest} onChange={(value) => runtimeField('imageDigest', value.toLowerCase().trim())} />
          <label className="text-sm font-black">协议<PixelSelect className="mt-2" value={form.runtime.protocol} onChange={(event) => runtimeField('protocol', event.target.value)}><option value="tcp">TCP</option><option value="http">HTTP</option><option value="https">HTTPS</option><option value="udp">UDP</option></PixelSelect></label>
          <label className="text-sm font-black">动态 Flag 格式<PixelSelect className="mt-2" value={form.runtime.flagFormat} onChange={(event) => runtimeField('flagFormat', event.target.value as RuntimeMutation['flagFormat'])}><option value="standard">标准随机（flag&#123;cm_...&#125;）</option><option value="uuid">UUID（flag&#123;xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx&#125;）</option></PixelSelect></label>
          <Field label="容器内部端口" type="number" value={String(form.runtime.internalPort)} onChange={(value) => runtimeField('internalPort', Number(value))} /><Field label="CPU（毫核 m）" type="number" value={String(form.runtime.cpuMilli)} onChange={(value) => runtimeField('cpuMilli', Number(value))} /><Field label="内存 MB" type="number" value={String(form.runtime.memoryMB)} onChange={(value) => runtimeField('memoryMB', Number(value))} /><Field label="进程数上限" type="number" value={String(form.runtime.pidsLimit)} onChange={(value) => runtimeField('pidsLimit', Number(value))} /><Field label="磁盘预算 MB" type="number" value={String(form.runtime.diskMB)} onChange={(value) => runtimeField('diskMB', Number(value))} /><Field label="默认时长（分钟）" type="number" value={String(Math.round(form.runtime.ttlSeconds / 60))} onChange={(value) => runtimeField('ttlSeconds', Number(value) * 60)} /><Field label="最大时长（分钟）" type="number" value={String(Math.round(form.runtime.maxTTLSeconds / 60))} onChange={(value) => runtimeField('maxTTLSeconds', Number(value) * 60)} />
          <label className="flex items-center gap-2 text-sm font-black"><input type="checkbox" checked={form.runtime.readOnlyRootFS} onChange={(event) => runtimeField('readOnlyRootFS', event.target.checked)} />只读根文件系统（推荐）</label>
          <label className="text-sm font-black sm:col-span-2">环境变量（每行 KEY=VALUE）<textarea className="pixel-input mt-2 min-h-24 font-mono" value={environmentText} onChange={(event) => setEnvironmentText(event.target.value)} placeholder={'APP_MODE=challenge\nTZ=Asia/Shanghai'} /><p className="mt-1 text-xs font-semibold text-asamu-muted">ASAMU_FLAG 和 ASAMU_INSTANCE_ID 由平台安全注入，不能覆盖。</p></label>
        </>}
        <Section title="Hint 与附件" />
        <label className="text-sm font-black sm:col-span-2">Hint（每行：标题|扣分|正文）<textarea className="pixel-input mt-2 min-h-24" value={form.hintConfigs.map((hint) => `${hint.title}|${hint.cost}|${hint.content}`).join('\n')} onChange={(event) => field('hintConfigs', event.target.value.split('\n').filter(Boolean).map((line, index) => { const [title, cost, ...content] = line.split('|'); return { title: title || `Hint ${index + 1}`, cost: Number(cost) || 0, content: content.join('|') } }))} /></label>
        {existingFiles.length > 0 && <div className="space-y-2 sm:col-span-2">{existingFiles.map((file) => <div className="flex items-center justify-between border border-asamu-line p-2 text-sm font-bold" key={file.id}><span>{file.name} · {Math.ceil(file.size / 1024)} KB</span><PixelButton size="sm" variant="danger" disabled={busy} onClick={() => void removeFile(file.id)}>删除</PixelButton></div>)}</div>}
        <label className="text-sm font-black sm:col-span-2">新增附件（单文件最大 64 MB）<input className="mt-2 block w-full text-sm" type="file" multiple onChange={(event) => setFiles(Array.from(event.target.files ?? []))} /></label>
        <div className="flex flex-wrap justify-end gap-2 sm:col-span-2"><PixelButton variant="secondary" onClick={() => setOpen(false)}>取消</PixelButton><PixelButton disabled={busy || !form.title || !form.categoryKey} onClick={() => void save(false)}>{busy ? '保存中…' : '保存草稿'}</PixelButton>{canPublish && <PixelButton variant="yellow" disabled={busy || !form.title || !form.categoryKey} onClick={() => void save(true)}>保存并发布</PixelButton>}</div>
      </div>
    </Modal>
  </>
}

function Section({ title }: { title: string }) { return <div className="border-b-2 border-asamu-ink/10 pb-2 pt-3 font-display text-lg font-black sm:col-span-2">{title}</div> }
function Field({ label, value, onChange, type = 'text' }: { label: string; value: string; onChange: (value: string) => void; type?: string }) { return <label className="text-sm font-black">{label}<PixelInput className="mt-2" type={type} value={value} onChange={(event) => onChange(event.target.value)} /></label> }
