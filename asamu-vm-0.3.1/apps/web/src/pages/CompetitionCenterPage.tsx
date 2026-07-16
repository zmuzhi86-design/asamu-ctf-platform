import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { EmptyState, HeroArtwork, Metric, PageContainer, PageHeader, PixelButton, PixelCard, PixelTag, SecondaryCard, SectionHeading, StatusBadge } from '../components/ui/System'
import { fetchCompetitions, type CompetitionDto } from '../services/platformApi'

export function CompetitionCenterPage() {
  const [competitions, setCompetitions] = useState<CompetitionDto[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  useEffect(() => { let active = true; fetchCompetitions().then((page) => { if (active) setCompetitions(page.items) }).catch((reason) => active && setError(reason instanceof Error ? reason.message : '赛事加载失败')).finally(() => active && setLoading(false)); return () => { active = false } }, [])
  const featured = competitions.find((item) => item.status === 'running') ?? competitions.find((item) => item.status === 'frozen') ?? competitions[0]
  return <PageContainer>
    <PageHeader eyebrow="COMPETITION HALL" title="比赛中心" description="从练习赛到正式高校赛事，在同一座赛场完成报名、组队、答题和赛后复盘。" />
    {error && <div className="mb-5 border-2 border-red-300 bg-red-50 p-3 text-sm font-black text-red-700">{error}</div>}
    {featured && <section className="mb-8 grid gap-5 xl:grid-cols-[1.55fr_.75fr]">
      <PixelCard padded={false} className="relative min-h-[360px] overflow-hidden bg-gradient-to-br from-[#0b3a83] to-[#126bd1] text-white">
        <div className="absolute inset-y-0 right-0 w-[62%]"><HeroArtwork className="h-full w-full" assetKey={featured.bannerAssetKey || 'competition.hero'} alt="正式 CTF 赛事舞台" position="right center" /></div><div className="absolute inset-0 bg-gradient-to-r from-[#092b63] via-[#0b3a83]/92 to-transparent" />
        <div className="relative z-10 flex min-h-[360px] max-w-xl flex-col justify-center p-7 sm:p-10"><PixelTag tone="yellow">{competitionStatus(featured.status)}</PixelTag><h2 className="mt-4 font-display text-3xl font-black sm:text-4xl">{featured.name}</h2><p className="mt-3 font-semibold text-blue-100">{featured.summary || (featured.mode === 'team' ? '团队赛' : '个人赛')}</p><div className="mt-6 inline-flex w-fit border-2 border-white bg-[#10233F] px-4 py-3 font-mono text-xl font-black">{formatDate(featured.startsAt)} 开赛</div><div className="mt-6"><Link to={`/competitions/${featured.slug || featured.id}`}><PixelButton variant="yellow">进入赛事</PixelButton></Link></div></div>
      </PixelCard>
      <div className="space-y-5"><PixelCard title="赛事日程"><Timeline items={[`${formatDate(featured.registrationEndsAt)} · 报名截止`, `${formatDate(featured.startsAt)} · 比赛开始`, `${formatDate(featured.endsAt)} · 比赛结束`]} /></PixelCard><SecondaryCard title="平台说明"><p className="text-sm font-black">比赛状态由服务端时间统一控制</p><p className="mt-2 text-xs font-semibold leading-5 text-asamu-muted">动态环境、封榜和最终结算均以服务端记录为准。</p></SecondaryCard></div>
    </section>}
    <SectionHeading eyebrow="ALL EVENTS" title="赛事列表" description="正在进行、即将开始、练习赛和历史赛事统一归档。" />
    {loading ? <PixelCard><p className="py-10 text-center font-bold text-asamu-muted">正在加载赛事…</p></PixelCard> : competitions.length ? <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">{competitions.map((competition) => <PixelCard className="flex flex-col" key={competition.id}><div className="flex items-start justify-between"><StatusBadge tone={competition.status === 'running' ? 'green' : competition.status === 'finished' ? 'slate' : 'yellow'}>{competitionStatus(competition.status)}</StatusBadge><PixelTag tone="yellow">{competition.scoringMode === 'dynamic' ? '动态计分' : '固定计分'}</PixelTag></div><h3 className="mt-4 font-display text-lg font-black">{competition.name}</h3><p className="mt-2 text-sm font-semibold text-asamu-muted">{competition.mode === 'team' ? 'Jeopardy 团队赛' : '个人赛'}</p><div className="my-4 grid grid-cols-2 gap-2"><Metric label="参赛" value={competition.participantCount.toLocaleString()} /><Metric label="题目" value={`${competition.challengeCount}`} /></div><p className="text-xs font-semibold leading-5 text-asamu-muted">{formatDate(competition.startsAt)} - {formatDate(competition.endsAt)}</p><Link className="mt-auto pt-4" to={`/competitions/${competition.slug || competition.id}`}><PixelButton className="w-full" size="sm" variant={competition.status === 'finished' ? 'secondary' : 'primary'}>{competition.status === 'finished' ? '查看赛果' : '查看详情'}</PixelButton></Link></PixelCard>)}</div> : <PixelCard><EmptyState title="暂无赛事" description="管理员发布赛事后会显示在这里。" /></PixelCard>}
  </PageContainer>
}

function Timeline({ items }: { items: string[] }) { return <div className="space-y-4">{items.map((item, index) => <div className="timeline-line text-sm font-semibold" key={item}><span className={`timeline-dot ${index === 0 ? 'bg-asamu-success' : ''}`} />{item}</div>)}</div> }
function competitionStatus(status: string) { return ({ draft: '草稿', registration: '报名中', running: '进行中', frozen: '封榜中', finished: '已结束', archived: '已归档' } as Record<string, string>)[status] ?? status }
function formatDate(value: string) { if (!value) return '待定'; return new Date(value).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }) }
