import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { AssetImage, Metric, PageContainer, PixelButton, PixelCard, PixelTag, SecondaryCard, StatusBadge, Toast } from '../components/ui/System'
import { assets } from '../data/assetManifest'
import { apiRequest, ApiError } from '../services/apiClient'
import { fetchMyTeam } from '../services/communityApi'
import { fetchCompetition, type CompetitionDto } from '../services/platformApi'

export function CompetitionDetailPage() {
  const { id = '' } = useParams()
  const [competition, setCompetition] = useState<CompetitionDto | null>(null)
  const [toast, setToast] = useState('')
  const [registering, setRegistering] = useState(false)
  const [registered, setRegistered] = useState(false)
  useEffect(() => { let active = true; fetchCompetition(id).then((item) => active && setCompetition(item)).catch((error) => active && setToast(error instanceof Error ? error.message : '比赛加载失败')); return () => { active = false } }, [id])
  const register = async () => {
    if (!competition || registering || registered) return
    setRegistering(true)
    try {
      let teamId: string | undefined
      if (competition.mode === 'team') {
        const membership = await fetchMyTeam()
        if (membership.myRole !== 'captain' && membership.myRole !== 'manager') throw new Error('团队赛需要由战队队长或管理员报名。')
        teamId = membership.team.id
      }
      await apiRequest<void>(`/competitions/${competition.slug || competition.id}/register`, { method: 'POST', body: JSON.stringify({ teamId }) })
      setRegistered(true)
      setCompetition({ ...competition, participantCount: competition.participantCount + 1 })
      setToast('比赛报名成功。')
    } catch (error) { setToast(error instanceof ApiError || error instanceof Error ? error.message : '报名失败') } finally { setRegistering(false) }
  }
  if (!competition) return <PageContainer><PixelCard><p className="font-semibold text-asamu-muted">正在读取比赛资料…</p></PixelCard>{toast && <Toast tone="red" message={toast} onClose={() => setToast('')} />}</PageContainer>
  const statusLabel = ({ registration: '报名中', running: '进行中', frozen: '封榜中', finished: '已结束' } as Record<string, string>)[competition.status] ?? competition.status
  const playable = competition.status === 'running' || competition.status === 'frozen'
  const duration = competition.startsAt && competition.endsAt ? Math.max(1, Math.round((new Date(competition.endsAt).getTime() - new Date(competition.startsAt).getTime()) / 3_600_000)) : 0
  const categories = Array.from(new Set((competition.challenges ?? []).map((challenge) => challenge.category)))
  return <PageContainer>
    <section className="mb-7 overflow-hidden border-2 border-asamu-ink bg-asamu-card shadow-pixel" style={{ borderRadius: 9 }}><div className="relative min-h-[300px] bg-[#0c3c85]"><AssetImage className="absolute inset-0 h-full w-full object-cover opacity-85" assetKey={competition.bannerAssetKey || 'competition.hero'} alt="赛事舞台" /><div className="absolute inset-0 bg-gradient-to-r from-[#082d68]/95 via-[#0c3c85]/75 to-transparent" /><div className="relative z-10 max-w-3xl p-7 text-white sm:p-10"><div className="flex flex-wrap gap-2"><StatusBadge tone={competition.status === 'running' ? 'green' : 'yellow'}>{statusLabel}</StatusBadge><PixelTag tone="yellow">{competition.mode === 'team' ? '团队赛' : '个人赛'}</PixelTag></div><h1 className="mt-5 font-display text-3xl font-black sm:text-5xl">{competition.name}</h1><p className="mt-3 max-w-xl font-semibold leading-7 text-blue-100">{competition.summary || competition.description}</p><div className="mt-6 flex flex-wrap gap-3">{playable && <Link to={`/competitions/${competition.slug || competition.id}/play`}><PixelButton variant="yellow">进入比赛</PixelButton></Link>}{competition.status === 'registration' && <PixelButton variant="yellow" disabled={registering || registered} onClick={() => void register()}>{registered ? '已报名' : registering ? '报名中…' : '立即报名'}</PixelButton>}</div></div><AssetImage className="absolute bottom-0 right-8 z-10 hidden h-52 w-52 lg:block" src={assets.characters.winnerRobot} alt="获奖机器人" /></div></section>
    <div className="mb-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-5"><Metric label="比赛时长" value={duration ? `${duration} 小时` : '待定'} note={formatRange(competition.startsAt, competition.endsAt)} /><Metric label="参赛主体" value={competition.participantCount.toLocaleString()} /><Metric label="题目数量" value={`${competition.challengeCount}`} /><Metric label="计分模式" value={competition.scoringMode === 'dynamic' ? '动态分' : '固定分'} highlight /><Metric label="当前状态" value={statusLabel} /></div>
    <div className="grid gap-6 xl:grid-cols-[1fr_360px]">
      <main className="space-y-6"><PixelCard title="赛事介绍"><p className="whitespace-pre-wrap leading-8 text-asamu-muted">{competition.description || competition.summary || '管理员尚未填写赛事介绍。'}</p></PixelCard><PixelCard title="比赛题目">{competition.challenges?.length ? <div className="grid gap-3 sm:grid-cols-2">{competition.challenges.map((challenge) => <SecondaryCard key={challenge.id}><div className="flex items-center justify-between"><PixelTag>{challenge.category}</PixelTag><b className="text-asamu-blue">{challenge.score} 分</b></div><h3 className="mt-3 font-black">{challenge.title}</h3><p className="mt-2 text-xs text-asamu-muted">{challenge.difficulty} · {challenge.solveCount} 人解出</p></SecondaryCard>)}</div> : <p className="text-sm font-semibold text-asamu-muted">题目将在比赛开放后显示。</p>}</PixelCard><PixelCard title="比赛方向">{categories.length ? <div className="flex flex-wrap gap-3">{categories.map((category) => <PixelTag key={category}>{category}</PixelTag>)}</div> : <p className="text-sm font-semibold text-asamu-muted">比赛题目开放后显示方向。</p>}</PixelCard></main>
      <aside className="space-y-5 xl:sticky xl:top-24"><PixelCard title="比赛时间"><AssetImage className="mx-auto h-24 w-40" src={assets.competition.countdown} alt="比赛时间" /><p className="mt-3 text-center font-mono text-lg font-black text-asamu-blue">{formatRange(competition.startsAt, competition.endsAt)}</p>{playable && <Link to={`/competitions/${competition.slug || competition.id}/play`}><PixelButton className="mt-4 w-full">进入答题区</PixelButton></Link>}</PixelCard><SecondaryCard title="报名信息"><p className="text-sm font-semibold leading-6 text-asamu-muted">当前已有 <b className="text-asamu-blue">{competition.participantCount}</b> 个参赛主体报名。名单按赛事权限保护。</p></SecondaryCard><SecondaryCard title="计分说明"><p className="text-sm leading-6 text-asamu-muted">正确提交、前三血奖励与 Hint 扣分全部记录为不可变积分事件；封榜后展示快照。</p></SecondaryCard></aside>
    </div>
    {toast && <Toast tone={toast.includes('成功') ? 'green' : 'red'} message={toast} onClose={() => setToast('')} />}
  </PageContainer>
}

function formatRange(start: string, end: string) { if (!start || !end) return '时间待定'; const options: Intl.DateTimeFormatOptions = { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }; return `${new Date(start).toLocaleString('zh-CN', options)} - ${new Date(end).toLocaleString('zh-CN', options)}` }
