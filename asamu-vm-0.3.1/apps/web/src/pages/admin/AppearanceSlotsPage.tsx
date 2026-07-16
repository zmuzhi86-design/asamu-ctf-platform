import { useState, type ReactNode } from 'react'
import { AssetImage, DataTable, Modal, PixelButton, PixelCard, PixelInput, PixelSelect, PixelTag, StatusBadge } from '../../components/ui/System'
import { useAssetSystem } from '../../contexts/AssetProvider'
import type { AssetFit } from '../../data/assetSystem'

const pages = ['home', 'challenges', 'challenge_detail', 'learning', 'competitions', 'teams', 'leaderboard', 'writeups', 'profile', 'login', 'admin']

export function AppearanceSlotsPage() {
  const system = useAssetSystem()
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState({ slotKey: '', name: '', page: 'home', assetKey: 'home.hero', mobileAssetKey: '', darkAssetKey: '', fit: 'contain' as AssetFit, position: 'center', enabled: true })
  const create = () => {
    if (!draft.slotKey.trim() || !draft.name.trim()) return
    system.createSlot({ ...draft, mobileAssetKey: draft.mobileAssetKey || undefined, darkAssetKey: draft.darkAssetKey || undefined })
    setOpen(false)
  }
  const assetOptions = <>{system.assets.map((asset) => <option value={asset.assetKey} key={asset.id}>{asset.assetKey}</option>)}</>
  return <>
    <Header title="页面素材槽位" description="使用稳定 slotKey 绑定桌面、移动、白天与夜间素材，新增页面时无需修改组件图片路径。"><PixelButton onClick={() => setOpen(true)}>新增槽位</PixelButton></Header>
    <PixelCard title="槽位绑定">
      <DataTable headers={['槽位', '页面', '默认素材', '预览', '适配', '版本', '状态', '操作']} rows={system.slots.map((slot) => [
        <div><b>{slot.name}</b><code className="block text-xs text-asamu-blue">{slot.slotKey}</code></div>,
        <PixelTag>{slot.page}</PixelTag>,
        <div className="max-w-[230px] space-y-1 text-xs"><code className="block truncate">日/桌：{slot.assetKey}</code><code className="block truncate">日/移：{slot.mobileAssetKey || '跟随桌面'}</code><code className="block truncate">夜间：{slot.darkAssetKey || '跟随白天'}</code></div>,
        <AssetImage className="h-12 w-16" assetKey={slot.assetKey} alt="槽位预览" />,
        `${slot.fit} · ${slot.position}`,
        `v${slot.version}`,
        <StatusBadge tone={slot.enabled ? 'green' : 'slate'}>{slot.enabled ? '启用' : '停用'}</StatusBadge>,
        <div className="flex gap-2"><PixelButton size="sm" variant="secondary" onClick={() => system.updateSlot(slot.id, { enabled: !slot.enabled })}>{slot.enabled ? '停用' : '启用'}</PixelButton><PixelButton size="sm" variant="secondary" onClick={() => system.updateSlot(slot.id, {})}>新版本</PixelButton></div>,
      ])} />
    </PixelCard>
    <Modal open={open} title="新增页面素材槽位" onClose={() => setOpen(false)}>
      <div className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-2"><Field label="槽位名称"><PixelInput value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} placeholder="首页活动横幅" /></Field><Field label="slotKey"><PixelInput value={draft.slotKey} onChange={(event) => setDraft({ ...draft, slotKey: event.target.value })} placeholder="home.campaign.banner" /></Field></div>
        <Field label="适用页面"><PixelSelect value={draft.page} onChange={(event) => setDraft({ ...draft, page: event.target.value })}>{pages.map((item) => <option key={item}>{item}</option>)}</PixelSelect></Field>
        <div className="grid gap-3 sm:grid-cols-2"><Field label="白天桌面素材"><PixelSelect value={draft.assetKey} onChange={(event) => setDraft({ ...draft, assetKey: event.target.value })}>{assetOptions}</PixelSelect></Field><Field label="白天移动素材"><PixelSelect value={draft.mobileAssetKey} onChange={(event) => setDraft({ ...draft, mobileAssetKey: event.target.value })}><option value="">跟随桌面</option>{assetOptions}</PixelSelect></Field><Field label="夜间桌面素材"><PixelSelect value={draft.darkAssetKey} onChange={(event) => setDraft({ ...draft, darkAssetKey: event.target.value })}><option value="">跟随白天</option>{assetOptions}</PixelSelect></Field><Field label="显示模式"><PixelSelect value={draft.fit} onChange={(event) => setDraft({ ...draft, fit: event.target.value as AssetFit })}><option>contain</option><option>cover</option></PixelSelect></Field></div>
        <Field label="对齐位置"><PixelInput value={draft.position} onChange={(event) => setDraft({ ...draft, position: event.target.value })} placeholder="center 50%" /></Field>
        <div className="flex justify-end gap-3"><PixelButton variant="secondary" onClick={() => setOpen(false)}>取消</PixelButton><PixelButton onClick={create}>创建槽位</PixelButton></div>
      </div>
    </Modal>
  </>
}

function Header({ title, description, children }: { title: string; description: string; children?: ReactNode }) { return <header className="mb-6 flex flex-col gap-4 border-b-2 border-asamu-ink/10 pb-5 sm:flex-row sm:items-end sm:justify-between"><div><p className="text-xs font-black uppercase tracking-[.18em] text-asamu-blue">VISUAL OPERATIONS</p><h1 className="mt-2 font-display text-3xl font-black">{title}</h1><p className="mt-2 text-sm font-semibold text-asamu-muted">{description}</p></div>{children}</header> }
function Field({ label, children }: { label: string; children: ReactNode }) { return <label className="block text-sm font-black">{label}<div className="mt-2">{children}</div></label> }
