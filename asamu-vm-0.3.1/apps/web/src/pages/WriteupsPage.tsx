import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { AssetImage, EmptyState, PageContainer, PageHeader, PixelButton, PixelCard, PixelInput, PixelTag, RobotTip, SectionHeading } from '../components/ui/System'
import { assets, categoryKeyByLabel } from '../data/assetManifest'
import { fetchWriteups, type WriteupDto } from '../services/platformApi'

export function WriteupsPage() {
  const [writeups, setWriteups] = useState<WriteupDto[]>([])
  const [search, setSearch] = useState('')
  const [error, setError] = useState('')
  useEffect(() => { fetchWriteups().then((page) => setWriteups(page.items)).catch((reason: Error) => setError(reason.message)) }, [])
  const filtered = useMemo(() => writeups.filter((item) => `${item.title} ${item.summary} ${item.author}`.toLowerCase().includes(search.toLowerCase())), [search, writeups])
  const featured = filtered.find((item) => item.featured) ?? filtered[0]
  return <PageContainer><PageHeader eyebrow="KNOWLEDGE BASE" title="WriteUp 中心" description="阅读、讨论并创作经过安全清洗的题解，让每次攻克都成为可复用的知识。"><div className="flex flex-wrap gap-3"><PixelInput className="w-56" placeholder="搜索题解" value={search} onChange={(event) => setSearch(event.target.value)} /><Link to="/writeups/new"><PixelButton>发布 WriteUp</PixelButton></Link></div></PageHeader>
    {error && <p className="mb-5 border-2 border-red-400 bg-red-50 p-3 text-sm font-black text-red-700">{error}</p>}
    {featured && <section className="mb-8 grid gap-5 lg:grid-cols-[1.35fr_.8fr]"><Link to={`/writeups/${featured.slug || featured.id}`}><PixelCard padded={false} className="grid min-h-72 overflow-hidden md:grid-cols-[.9fr_1.1fr]"><div className="relative min-h-52 bg-asamu-soft"><img className="absolute inset-0 h-full w-full object-cover" src={assets.categories.web.scene} alt="WriteUp 专题场景" /><AssetImage className="absolute bottom-2 right-3 h-24 w-24" src={assets.characters.laptopRobot} alt="写作机器人" /></div><div className="flex flex-col justify-center p-6"><PixelTag tone="yellow">精选文章</PixelTag><h2 className="mt-4 font-display text-2xl font-black">{featured.title}</h2><p className="mt-3 text-sm font-medium leading-7 text-asamu-muted">{featured.summary}</p><p className="mt-4 text-xs font-bold text-asamu-muted">{featured.author} · {featured.views.toLocaleString()} 阅读 · {featured.likes} 点赞</p></div></PixelCard></Link><RobotTip title="写作建议" robot={assets.characters.studentLaptop}>好的 WriteUp 不只给出脚本，还会解释观察、假设、验证与失败路径。</RobotTip></section>}
    <SectionHeading eyebrow="LATEST ARTICLES" title="最新文章" />{!filtered.length ? <PixelCard><EmptyState title="没有匹配的文章" description="尝试其他关键词，或者创建第一篇 WriteUp。" /></PixelCard> : <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">{filtered.filter((item) => item.id !== featured?.id).map((item) => { const key = categoryKeyByLabel[item.category as keyof typeof categoryKeyByLabel] ?? 'web'; return <Link to={`/writeups/${item.slug || item.id}`} key={item.id}><PixelCard className="h-full"><div className="flex items-start gap-4"><AssetImage className="h-16 w-16" src={assets.categories[key].icon} alt="" /><div><PixelTag>{item.category}</PixelTag><h3 className="mt-2 font-display text-lg font-black">{item.title}</h3></div></div><p className="mt-4 line-clamp-3 text-sm leading-6 text-asamu-muted">{item.summary}</p><div className="mt-4 flex justify-between border-t border-asamu-line pt-3 text-xs font-bold text-asamu-muted"><span>{item.author}</span><span>{item.views} 阅读 · {item.likes} 赞</span></div></PixelCard></Link>})}</div>}
  </PageContainer>
}
