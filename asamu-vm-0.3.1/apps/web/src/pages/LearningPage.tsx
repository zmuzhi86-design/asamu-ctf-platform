import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { AssetImage, Metric, PageContainer, PageHeader, PixelButton, PixelCard, PixelTag, ProgressBar, RobotTip, SceneArtwork, SecondaryCard, SectionHeading, StatusBadge } from '../components/ui/System'
import { assets } from '../data/assetManifest'
import { useAuth } from '../contexts/AuthProvider'
import { apiRequest } from '../services/apiClient'
import { fetchLearningPaths, type LearningPath } from '../services/learningApi'

type Progression = { experience: number; tier: { name: string }; progress: number; medals: Array<{ id: string }> }

export function LearningPage() {
  const auth = useAuth()
  const [paths, setPaths] = useState<LearningPath[]>([])
  const [selectedID, setSelectedID] = useState('')
  const [progression, setProgression] = useState<Progression | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [reloadKey, setReloadKey] = useState(0)

  useEffect(() => {
    let active = true
    setLoading(true); setError('')
    fetchLearningPaths().then((items) => {
      if (!active) return
      setPaths(items)
      setSelectedID((current) => items.some((item) => item.id === current) ? current : (items.find((item) => item.featured) ?? items[0])?.id ?? '')
    }).catch((reason: Error) => active && setError(reason.message)).finally(() => active && setLoading(false))
    return () => { active = false }
  }, [auth.user?.id, reloadKey])

  useEffect(() => {
    if (!auth.user) { setProgression(null); return }
    apiRequest<Progression>('/me/progression').then(setProgression).catch(() => setProgression(null))
  }, [auth.user?.id])

  const selected = paths.find((item) => item.id === selectedID) ?? paths[0]
  const completed = paths.reduce((sum, item) => sum + item.completedChallenges, 0)
  const total = paths.reduce((sum, item) => sum + item.totalChallenges, 0)
  const nextChallenge = selected?.stages.flatMap((stage) => stage.challenges).find((challenge) => !challenge.completed)
  const nextURL = nextChallenge ? `/challenges/${nextChallenge.slug || nextChallenge.id}` : '/challenges'

  if (loading) return <PageContainer><PixelCard><p className="py-20 text-center font-bold text-asamu-muted">正在加载训练路线…</p></PixelCard></PageContainer>
  if (error) return <PageContainer><PixelCard><div className="py-20 text-center"><p className="font-bold text-asamu-danger">学习中心加载失败：{error}</p><PixelButton className="mt-5" variant="secondary" onClick={() => setReloadKey((value) => value + 1)}>重新加载</PixelButton></div></PixelCard></PageContainer>
  if (!selected) return <PageContainer><PageHeader eyebrow="LEARNING PATH" title="训练路线" description="管理员尚未发布训练路线。" /><PixelCard><p className="py-20 text-center font-bold text-asamu-muted">暂无已发布路线，请稍后再来。</p></PixelCard></PageContainer>

  return <PageContainer><PageHeader eyebrow="LEARNING PATH" title="训练路线" description="训练路线、阶段和关卡均由平台后台实时编排，解题后会自动同步完成进度。"><Link to={nextURL}><PixelButton variant="yellow">{nextChallenge ? '继续上次训练' : '浏览公开题库'}</PixelButton></Link></PageHeader>
    <section className="mb-7 grid gap-5 lg:grid-cols-[1fr_1.7fr]"><PixelCard><div className="flex items-center gap-4"><AssetImage className="h-24 w-24" src={assets.characters.studentLaptop} alt="学习者" /><div><PixelTag tone="yellow">{progression ? `${progression.tier.name} · ${progression.experience} XP` : auth.user ? '成长档案加载中' : '登录后记录进度'}</PixelTag><h2 className="mt-3 text-xl font-black">{auth.user?.username || '游客学习者'}</h2><p className="mt-1 text-sm font-semibold text-asamu-muted">已完成 {completed} / {total} 道路线题目</p></div></div><div className="mt-5"><ProgressBar value={total ? Math.round(completed * 100 / total) : 0} label="总训练进度" /></div><div className="mt-4 grid grid-cols-3 gap-2"><Metric label="已完成" value={String(completed)} /><Metric label="路线数" value={String(paths.length)} /><Metric label="徽章" value={String((progression?.medals ?? []).length)} highlight /></div></PixelCard><PixelCard padded={false} className="relative min-h-64 overflow-hidden"><SceneArtwork className="absolute inset-0 h-full w-full" assetKey={selected.heroAssetKey || selected.sceneAssetKey || 'training.route.hero'} alt={selected.title} /><div className="absolute inset-0 bg-gradient-to-r from-asamu-card/95 via-asamu-card/80 to-transparent" /><div className="relative z-10 max-w-lg p-6"><PixelTag tone="green">{selected.featured ? '推荐路线' : selected.directionName}</PixelTag><h2 className="mt-4 font-display text-3xl font-black">{selected.title}</h2><p className="mt-3 text-sm font-semibold leading-6 text-asamu-muted">{selected.summary}</p><p className="mt-2 text-xs font-bold text-asamu-muted">{selected.stages.length} 个阶段 · {selected.totalChallenges} 道挑战 · 预计 {formatMinutes(selected.estimatedMinutes)}</p><Link to={nextURL}><PixelButton className="mt-5">{nextChallenge ? '开始下一关' : '查看题库'}</PixelButton></Link></div></PixelCard></section>
    <SectionHeading eyebrow="TRAINING PATHS" title="选择训练路线" description="后台可自由新增路线、调整阶段顺序并绑定已发布题目。" />
    <div className="mb-8 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">{paths.map((path) => <button className={`scene-card ${selected.id === path.id ? 'scene-card-active' : ''}`} onClick={() => setSelectedID(path.id)} key={path.id}><div className="h-24 overflow-hidden rounded bg-asamu-soft"><SceneArtwork className="h-full w-full" assetKey={path.sceneAssetKey || path.heroAssetKey} alt={path.directionName || path.title} /></div><div className="mt-3 text-left"><b className="block text-sm">{path.title}</b><span className="text-xs text-asamu-muted">{Math.round(path.progress * 100)}% 完成 · {path.totalChallenges} 题</span></div></button>)}</div>
    <div className="grid gap-6 xl:grid-cols-[1fr_300px]"><PixelCard title={`${selected.title} · 路线关卡`} action={<StatusBadge tone={selected.progress >= 1 ? 'green' : 'blue'}>{selected.progress >= 1 ? '已完成' : '进行中'}</StatusBadge>}><div className="relative min-h-[360px] overflow-hidden border border-asamu-line bg-asamu-soft p-5"><SceneArtwork className="pointer-events-none absolute inset-0 h-full w-full opacity-10" assetKey={selected.sceneAssetKey} alt="" /><div className="relative grid gap-5 md:grid-cols-2 xl:grid-cols-3">{selected.stages.map((stage, index) => <div className={`relative min-h-40 border-2 border-asamu-ink p-4 shadow-pixelSm ${stage.completed ? 'bg-green-50' : stage.challenges.some((item) => !item.completed) ? 'bg-asamu-card' : 'bg-yellow-50'}`} key={stage.id}><span className="text-xs font-black">STAGE {String(index + 1).padStart(2, '0')}</span><h3 className="mt-3 font-display font-black">{stage.title}</h3><p className="mt-2 text-xs font-semibold text-asamu-muted">{stage.description}</p><div className="mt-4 space-y-2">{stage.challenges.map((challenge) => <Link className="flex items-center justify-between border-t border-asamu-line pt-2 text-xs font-bold hover:text-asamu-blue" to={`/challenges/${challenge.slug || challenge.id}`} key={challenge.id}><span>{challenge.completed ? '✓ ' : ''}{challenge.title}</span><span>{challenge.score} 分</span></Link>)}{!stage.challenges.length && <p className="border-t border-asamu-line pt-2 text-xs text-asamu-muted">该阶段暂无公开题目</p>}</div></div>)}</div></div></PixelCard><aside className="space-y-5"><SecondaryCard title="路线信息"><Info label="完成度" value={`${Math.round(selected.progress * 100)}%`} /><Info label="预计用时" value={formatMinutes(selected.estimatedMinutes)} /><Info label="前置知识" value={selected.prerequisite || '无'} /><Info label="关卡数量" value={String(selected.totalChallenges)} /></SecondaryCard><SecondaryCard title="推荐下一步">{nextChallenge ? <><h3 className="font-black">{nextChallenge.title}</h3><p className="mt-2 text-sm leading-6 text-asamu-muted">{nextChallenge.difficulty} · {nextChallenge.dynamic ? '动态环境题' : '静态附件题'} · {nextChallenge.score} 分</p><Link to={nextURL}><PixelButton className="mt-4 w-full" size="sm">开始挑战</PixelButton></Link></> : <p className="text-sm font-semibold text-asamu-muted">当前路线没有待完成的公开题目。</p>}</SecondaryCard><RobotTip title="训练说明">完成绑定题目的正确 Flag 提交后，路线进度会自动更新，无需手工打卡。</RobotTip></aside></div>
  </PageContainer>
}

function formatMinutes(value: number) { if (value < 60) return `${value} 分钟`; const hours = Math.round(value / 6) / 10; return `${hours} 小时` }
function Info({ label, value }: { label: string; value: string }) { return <div className="flex justify-between gap-3 border-b border-asamu-line py-2 text-sm last:border-0"><span className="shrink-0 text-asamu-muted">{label}</span><b className="text-right">{value}</b></div> }
