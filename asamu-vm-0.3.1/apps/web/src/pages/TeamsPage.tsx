import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { AssetImage, EmptyState, PageContainer, PageHeader, PixelButton, PixelCard, PixelInput, PixelTag, StatusBadge } from '../components/ui/System'
import { fetchTeams, type TeamDto } from '../services/platformApi'

export function TeamsPage() {
  const [teams, setTeams] = useState<TeamDto[]>([])
  const [search, setSearch] = useState('')
  const [recruitingOnly, setRecruitingOnly] = useState(false)
  const [error, setError] = useState('')
  useEffect(() => { fetchTeams().then((page) => setTeams(page.items)).catch((reason: Error) => setError(reason.message)) }, [])
  const filtered = useMemo(() => teams.filter((team) => (!recruitingOnly || team.recruiting) && `${team.name} ${team.slogan}`.toLowerCase().includes(search.toLowerCase())), [recruitingOnly, search, teams])
  return <PageContainer><PageHeader eyebrow="TEAM BASE" title="战队基地" description="寻找长期训练伙伴、组建比赛阵容并管理属于你们的战队。"><div className="flex flex-wrap gap-2"><PixelInput className="w-56" placeholder="搜索战队" value={search} onChange={(event) => setSearch(event.target.value)} /><PixelButton variant={recruitingOnly ? 'yellow' : 'secondary'} onClick={() => setRecruitingOnly((value) => !value)}>仅看招募中</PixelButton><Link to="/team/manage"><PixelButton>创建 / 管理战队</PixelButton></Link></div></PageHeader>
    {error && <p className="mb-5 border-2 border-red-400 bg-red-50 p-3 text-sm font-black text-red-700">{error}</p>}
    {!filtered.length ? <PixelCard><EmptyState title="没有匹配的战队" description="调整筛选条件，或者创建自己的战队。" /></PixelCard> : <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">{filtered.map((team) => <PixelCard className="flex flex-col" key={team.id}><div className="flex items-start justify-between"><div className="grid h-16 w-16 place-items-center overflow-hidden border-2 border-asamu-ink bg-asamu-soft"><AssetImage className="h-14 w-14" assetKey={team.flagAssetKey || 'team.honor.verified'} alt={`${team.name} 战队头像`} /></div><StatusBadge tone={team.recruiting ? 'green' : 'slate'}>{team.recruiting ? '招募中' : '暂不招募'}</StatusBadge></div><h2 className="mt-4 font-display text-xl font-black">{team.name}</h2><p className="mt-2 min-h-12 text-sm font-semibold leading-6 text-asamu-muted">{team.slogan || team.description}</p><div className="my-4 flex flex-wrap gap-2"><PixelTag>{team.memberCount} / {team.memberLimit} 成员</PixelTag><PixelTag tone="yellow">排名 #{team.rank}</PixelTag></div><p className="mb-4 text-sm font-black text-asamu-blue">{team.score.toLocaleString()} 分</p><Link className="mt-auto" to={`/teams/${team.slug || team.id}`}><PixelButton className="w-full" size="sm" variant="secondary">查看战队</PixelButton></Link></PixelCard>)}</div>}
  </PageContainer>
}
