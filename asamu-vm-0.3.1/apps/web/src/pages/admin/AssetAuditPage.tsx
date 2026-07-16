import { DataTable, PixelCard, PixelTag, StatusBadge } from '../../components/ui/System'
import { useAssetSystem } from '../../contexts/AssetProvider'

export function AssetAuditPage() {
  const { audit } = useAssetSystem()
  return <><header className="mb-6 border-b-2 border-asamu-ink/10 pb-5"><p className="text-xs font-black uppercase tracking-[.18em] text-asamu-blue">VISUAL OPERATIONS</p><h1 className="mt-2 font-display text-3xl font-black">素材使用与审计记录</h1><p className="mt-2 text-sm font-semibold text-asamu-muted">记录上传、版本、发布、回滚、槽位与背景配置变化。</p></header><PixelCard title="最近审计"><DataTable headers={['时间','操作','目标','说明','操作人','结果']} rows={audit.map((item) => [new Date(item.createdAt).toLocaleString(),<PixelTag>{item.action}</PixelTag>,<code className="text-xs">{item.target}</code>,item.detail,item.actor,<StatusBadge tone="green">成功</StatusBadge>])} /></PixelCard></>
}
