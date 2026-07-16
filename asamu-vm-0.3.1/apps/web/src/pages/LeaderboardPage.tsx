import { useEffect, useState } from 'react'
import { AssetImage, DataTable, PageContainer, PageHeader, Pagination, PixelCard, RobotTip, SecondaryCard, StatusBadge } from '../components/ui/System'
import { assets } from '../data/assetManifest'
import { fetchLeaderboard, type LeaderboardDto } from '../services/platformApi'

export function LeaderboardPage() {
  const [rows, setRows] = useState<LeaderboardDto[]>([])
  const [page, setPage] = useState(1)
  const pageSize = 20

  useEffect(() => {
    let active = true
    fetchLeaderboard().then((page) => active && setRows(page.items)).catch(() => undefined)
    return () => { active = false }
  }, [])

  const podium = [rows[1], rows[0], rows[2]]
  return <PageContainer>
    <PageHeader eyebrow="HALL OF FAME" title="排行榜" description="记录每一次突破，见证个人、战队与高校在赛场和训练中的持续成长。">
    </PageHeader>

    <section className="mb-8 grid gap-5 lg:grid-cols-[1fr_1.3fr_1fr] lg:items-end">
      {podium.map((row, index) => row
        ? <PodiumCard key={row.userId} rank={row.rank} name={row.username} score={row.score.toLocaleString()} image={[assets.achievements.silverCup, assets.achievements.goldCup, assets.achievements.bronzeCup][index]} winner={row.rank === 1} />
        : <PixelCard key={index} className="min-h-52"><span className="sr-only">暂无排名数据</span></PixelCard>)}
    </section>

    <div className="mt-6 grid gap-6 xl:grid-cols-[1fr_320px]">
      <main className="space-y-5">
        <PixelCard title="总榜排名">
          <DataTable
            headers={['排名', '选手 / 战队', '学校 / 组织', '总分', '解题数', '一血数', '最近解题']}
            rows={rows.slice((page - 1) * pageSize, page * pageSize).map((row) => [
              <b className={row.rank <= 3 ? 'text-yellow-700' : ''}>#{row.rank}</b>,
              <b>{row.username}</b>,
              row.organization || '—',
              <b className="text-asamu-blue">{row.score.toLocaleString()}</b>,
              row.solves,
              row.bloods,
              <span className="text-asamu-muted">{row.lastSolveAt ? new Date(row.lastSolveAt).toLocaleDateString('zh-CN') : '—'}</span>,
            ])}
          />
          <div className="mt-4"><Pagination current={page} total={Math.max(1, Math.ceil(rows.length / pageSize))} onChange={setPage} /></div>
        </PixelCard>
      </main>
      <aside className="space-y-5">
        <SecondaryCard title="当前领先">
          <div className="flex items-center gap-3"><AssetImage className="h-16 w-16" src={assets.achievements.laurel} alt="领先荣誉" /><div><b>{rows[0]?.username ?? '等待首位选手'}</b><p className="text-sm text-asamu-success">{rows[0] ? `${rows[0].score.toLocaleString()} 分` : '暂无积分'}</p></div></div>
        </SecondaryCard>
        <PixelCard title="一血高手">
          <div className="space-y-3">{rows.filter((row) => row.bloods > 0).slice(0, 3).map((row) => <div className="flex items-center gap-3 border-b border-asamu-line pb-3 last:border-0" key={row.userId}><AssetImage className="h-10 w-10" src={assets.competition.firstBlood} alt="一血" /><div><b className="text-sm">{row.username}</b><p className="text-xs text-asamu-muted">累计 {row.bloods} 次一血</p></div></div>)}</div>
        </PixelCard>
        <RobotTip title="荣誉提示">排行榜由不可变积分事件实时重建；比赛封榜期间只显示封榜快照。</RobotTip>
      </aside>
    </div>
  </PageContainer>
}

function PodiumCard({ rank, name, score, image, winner = false }: { rank: number; name: string; score: string; image: string; winner?: boolean }) {
  return <PixelCard className={`text-center ${winner ? 'border-yellow-500 bg-yellow-50 lg:min-h-64' : ''}`}>
    <StatusBadge tone={winner ? 'yellow' : 'blue'}>TOP {rank}</StatusBadge>
    <AssetImage className={`mx-auto mt-3 ${winner ? 'h-28 w-28' : 'h-24 w-24'}`} src={image} alt={`第 ${rank} 名奖杯`} />
    <h2 className="mt-2 font-display text-xl font-black">{name}</h2>
    <p className="mt-1 text-lg font-black text-asamu-blue">{score} pts</p>
  </PixelCard>
}
