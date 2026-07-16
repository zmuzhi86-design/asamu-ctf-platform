import { Link } from 'react-router-dom'
import { AssetImage, PageContainer, PixelButton, PixelCard } from '../components/ui/System'
import { assets } from '../data/assetManifest'

export function NotFoundPage() { return <PageContainer><PixelCard className="mx-auto max-w-xl py-10 text-center"><AssetImage className="mx-auto h-48 w-48" src={assets.emptyStates.search} alt="页面未找到" /><h1 className="mt-4 font-display text-3xl font-black">这条路线暂时不存在</h1><p className="mt-3 text-sm font-semibold text-asamu-muted">小镜没有找到对应页面，可能是链接已更新。</p><Link to="/"><PixelButton className="mt-6">返回首页</PixelButton></Link></PixelCard></PageContainer> }
