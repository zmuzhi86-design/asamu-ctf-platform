import { useEffect, useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { AssetImage, EmptyState, PageContainer, PageHeader, PixelButton, PixelInput, PixelSelect, PixelTag, ProgressBar, RobotTip, SceneArtwork, SecondaryCard, StatusBadge } from '../components/ui/System'
import { assets, categoryKeyByLabel } from '../data/assetManifest'
import { categoryLabels } from '../data/platform'
import { fetchChallenges, type ChallengeDto } from '../services/platformApi'
import { usePlatform } from '../contexts/PlatformProvider'

const difficulties = ['全部', '入门', '简单', '中等', '困难', '专家']
const statuses = ['全部', '未解', '已解', '动态环境', '有附件', '有 WriteUp']
const directionKey = (label: string) => `direction.${label === 'AI Security' ? 'ai_security' : label.toLowerCase()}.scene`

export function ChallengeListPage() {
  const { config } = usePlatform()
  const headerDirections = config.directions.filter((direction) => direction.status === 'active' && direction.showOnLibraryHeader)
  const sidebarDirections = config.directions.filter((direction) => direction.status === 'active' && direction.showOnLibrarySidebar)
  const validCategories = useMemo(() => new Set(['全部', ...categoryLabels, ...config.directions.map((direction) => direction.name)]), [config.directions])
  const [params, setParams] = useSearchParams()
  const requestedCategory = params.get('category') ?? '全部'
  const [query, setQuery] = useState(params.get('search') ?? '')
  const [category, setCategory] = useState(validCategories.has(requestedCategory) ? requestedCategory : '全部')
  const [difficulty, setDifficulty] = useState('全部')
  const [status, setStatus] = useState('全部')
  const [sort, setSort] = useState('最新发布')
  const [catalog, setCatalog] = useState<ChallengeDto[]>([])
  const [loadError, setLoadError] = useState('')
  useEffect(() => { let active = true; fetchChallenges().then((page) => { if (active) setCatalog(page.items) }).catch((error) => active && setLoadError(error instanceof Error ? error.message : '题库加载失败')); return () => { active = false } }, [])
  useEffect(() => {
    const nextCategory = params.get('category') ?? '全部'
    setQuery(params.get('search') ?? '')
    setCategory(validCategories.has(nextCategory) ? nextCategory : '全部')
  }, [params, validCategories])

  const updateURLFilter = (key: 'search' | 'category', value: string) => {
    const next = new URLSearchParams(params)
    if (!value || value === '全部') next.delete(key)
    else next.set(key, value)
    setParams(next, { replace: true })
  }
  const updateQuery = (value: string) => { setQuery(value); updateURLFilter('search', value) }
  const updateCategory = (value: string) => { setCategory(value); updateURLFilter('category', value) }

  const filtered = useMemo(() => catalog.filter((challenge) => {
    if (category !== '全部' && challenge.category !== category) return false
    if (difficulty !== '全部' && challenge.difficulty !== difficulty) return false
    const solved = Boolean((challenge as ChallengeDto & { solved?: boolean }).solved)
    if (status === '未解' && solved) return false
    if (status === '已解' && !solved) return false
    if (status === '动态环境' && !challenge.dynamic) return false
    if (status === '有附件' && !challenge.attachment) return false
    if (status === '有 WriteUp' && !challenge.writeup) return false
    const keyword = query.trim().toLowerCase()
    return !keyword || `${challenge.title} ${challenge.category} ${challenge.tags.join(' ')}`.toLowerCase().includes(keyword)
  }).sort((a, b) => {
    if (sort === '分值最高') return b.score - a.score
    if (sort === '解出率最低') return a.solveRate - b.solveRate
    if (sort === '最热门') return b.solves - a.solves
    const publishedA = a.publishedAt ? Date.parse(a.publishedAt) : 0
    const publishedB = b.publishedAt ? Date.parse(b.publishedAt) : 0
    return (Number.isFinite(publishedB) ? publishedB : 0) - (Number.isFinite(publishedA) ? publishedA : 0) || b.id.localeCompare(a.id)
  }), [catalog, category, difficulty, query, sort, status])

  return <PageContainer>
    <PageHeader eyebrow="CHALLENGE LIBRARY" title={config.challengeLibrary.pageTitle || '挑战题库'} description={config.challengeLibrary.pageSubtitle || '从兴趣出发，选择方向、难度和环境类型。'}><div className="w-full max-w-md"><PixelInput value={query} onChange={(event) => updateQuery(event.target.value)} placeholder={config.challengeLibrary.searchPlaceholder || '搜索题目名称、标签或方向…'} /></div></PageHeader>

    {config.challengeLibrary.showDirectionSection && <section className="mb-7"><div className="mb-3 flex items-center justify-between"><h2 className="font-display text-lg font-black">方向探索区</h2><span className="text-xs font-bold text-asamu-muted">选择实验室，快速筛选题目</span></div><div className="no-scrollbar flex gap-3 overflow-x-auto pb-2">{headerDirections.map((item) => <button className={`scene-card w-[190px] shrink-0 ${category === item.name ? 'scene-card-active' : ''}`} onClick={() => updateCategory(category === item.name ? '全部' : item.name)} key={item.slug}><div className="h-28 rounded bg-asamu-soft"><SceneArtwork className="h-full w-full" assetKey={item.cardAssetKey || directionKey(item.name)} alt={item.subtitle || item.name} /></div><div className="mt-2 text-left"><b className="block text-sm">{item.name}</b><span className="text-[11px] font-semibold text-asamu-muted">{item.subtitle || item.description}</span></div></button>)}</div></section>}

    <div className="grid items-start gap-6 lg:grid-cols-[240px_minmax(0,1fr)]">
      {config.challengeLibrary.showSidebar && <aside className="space-y-4 lg:sticky lg:top-24"><SecondaryCard title="方向"><FilterList items={['全部', ...sidebarDirections.map((direction) => direction.name)]} active={category} onChange={updateCategory} /></SecondaryCard><SecondaryCard title="难度"><FilterList items={difficulties} active={difficulty} onChange={setDifficulty} /></SecondaryCard><SecondaryCard title="状态"><FilterList items={statuses} active={status} onChange={setStatus} /></SecondaryCard><RobotTip title="筛选建议" robot={assets.characters.chatRobot}>新手可先选择“入门 + 有 WriteUp”，完成后再进入动态环境题。</RobotTip></aside>}

      <main><div className="mb-4 flex flex-col gap-3 border-b border-asamu-line pb-4 sm:flex-row sm:items-center sm:justify-between"><div><b className="text-lg">找到 {filtered.length} 道题目</b><p className="mt-1 text-xs font-semibold text-asamu-muted">实时题库 · 筛选与搜索即时生效</p>{loadError && <p className="mt-1 text-xs font-black text-red-600">{loadError}</p>}</div><PixelSelect className="sm:w-40" value={sort} onChange={(event) => setSort(event.target.value)}><option>最新发布</option><option>最热门</option><option>分值最高</option><option>解出率最低</option></PixelSelect></div>
        {filtered.length ? <div className="grid gap-4 xl:grid-cols-2">{filtered.map((challenge) => <ChallengeCard key={challenge.id} challenge={challenge} />)}</div> : <SecondaryCard><EmptyState title="没有找到匹配题目" description="换一个方向、难度或关键词再试试。" image={assets.emptyStates.search} action={<PixelButton onClick={() => { const next = new URLSearchParams(params); next.delete('search'); next.delete('category'); setParams(next, { replace: true }); setQuery(''); setCategory('全部'); setDifficulty('全部'); setStatus('全部') }}>清除筛选</PixelButton>} /></SecondaryCard>}
      </main>
    </div>
  </PageContainer>
}

function FilterList({ items, active, onChange }: { items: readonly string[]; active: string; onChange: (item: string) => void }) {
  return <div className="grid grid-cols-2 gap-2 lg:grid-cols-1">{items.map((item) => <button className={`flex items-center justify-between border px-2.5 py-2 text-left text-sm font-bold transition ${active === item ? 'border-asamu-blue bg-asamu-soft text-asamu-blue' : 'border-transparent hover:border-asamu-line hover:bg-asamu-soft'}`} onClick={() => onChange(item)} key={item}><span>{item}</span>{active === item && <span>✓</span>}</button>)}</div>
}

function ChallengeCard({ challenge }: { challenge: ChallengeDto }) {
  const key = categoryKeyByLabel[challenge.category] ?? 'web'
  const category = assets.categories[key]
  const difficultyTone = challenge.difficulty === '专家' || challenge.difficulty === '困难' ? 'red' : challenge.difficulty === '中等' ? 'yellow' : 'blue'
  const solved = Boolean((challenge as ChallengeDto & { solved?: boolean }).solved)
  return <article className="challenge-card flex flex-col"><div className="challenge-card-stripe" /><div className="relative flex min-h-32 gap-4 overflow-hidden border-b border-asamu-line p-4"><div className="relative z-10 min-w-0 flex-1"><div className="flex flex-wrap gap-2"><PixelTag>{challenge.category}</PixelTag><PixelTag tone={difficultyTone}>{challenge.difficulty}</PixelTag>{solved && <PixelTag tone="green">已解</PixelTag>}</div><h2 className="mt-3 truncate font-display text-xl font-black">{challenge.title}</h2><p className="mt-2 flex flex-wrap gap-2">{challenge.tags.map((tag) => <span className="text-xs font-bold text-asamu-muted" key={tag}>#{tag}</span>)}</p></div><AssetImage className="h-24 w-24 shrink-0" src={category.icon} alt={`${challenge.category} 图标`} /></div><div className="grid grid-cols-3 divide-x divide-asamu-line border-b border-asamu-line bg-asamu-soft/60 text-center"><div className="p-3"><b className="block text-xl text-asamu-blue">{challenge.score}</b><span className="text-xs text-asamu-muted">分值</span></div><div className="p-3"><b className="block text-xl">{challenge.solves.toLocaleString()}</b><span className="text-xs text-asamu-muted">解出人数</span></div><div className="p-3"><b className="block text-xl">{challenge.solveRate.toFixed(2)}%</b><span className="text-xs text-asamu-muted">解出率</span></div></div><div className="p-4"><ProgressBar value={Math.round(challenge.solveRate)} label="社区完成度" /><div className="mt-4 flex flex-wrap items-center gap-2">{challenge.dynamic && <StatusBadge tone="green">动态环境</StatusBadge>}{challenge.attachment && <PixelTag tone="slate">附件</PixelTag>}{challenge.writeup && <PixelTag tone="yellow">WriteUp</PixelTag>}<span className="ml-auto" /><Link to={`/challenges/${challenge.slug || challenge.id}`}><PixelButton size="sm">查看题目</PixelButton></Link></div></div></article>
}
