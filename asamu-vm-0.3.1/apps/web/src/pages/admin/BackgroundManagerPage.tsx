import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { AssetImage, PixelButton, PixelCard, PixelInput, PixelSelect, PixelTag, SecondaryCard, StatusBadge } from '../../components/ui/System'
import { useAssetSystem } from '../../contexts/AssetProvider'
import type { BackgroundConfig, BackgroundFit } from '../../data/assetSystem'

export function BackgroundManagerPage() {
  const system = useAssetSystem()
  const [selectedId, setSelectedId] = useState(system.backgrounds[0]?.id ?? '')
  const editingId = useRef(selectedId)
  const saveQueue = useRef<Promise<unknown>>(Promise.resolve())
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  useEffect(() => {
    if (system.backgrounds.length && !system.backgrounds.some((item) => item.id === selectedId)) {
      editingId.current = system.backgrounds[0].id
      setSelectedId(system.backgrounds[0].id)
    }
  }, [selectedId, system.backgrounds])
  const selected = useMemo(() => system.backgrounds.find((item) => item.id === selectedId) ?? system.backgrounds[0], [selectedId, system.backgrounds])
  if (!selected) return null
  const patch = (value: Partial<BackgroundConfig>) => {
    setError('')
    const operation = saveQueue.current.then(() => system.updateBackground(editingId.current, value))
    saveQueue.current = operation.then((saved) => { editingId.current = saved.id; setSelectedId(saved.id) }).catch((reason) => { setError(reason instanceof Error ? reason.message : '背景草稿保存失败') })
    return operation
  }
  const publish = async () => { setBusy(true); setError(''); setMessage(''); try { await saveQueue.current; await system.publishBackground(editingId.current); setMessage('背景配置已发布并立即生效。') } catch (reason) { setError(reason instanceof Error ? reason.message : '背景发布失败') } finally { setBusy(false) } }
  const rollback = async () => { setBusy(true); setError(''); setMessage(''); try { await saveQueue.current; await system.rollbackBackground(editingId.current); setMessage('已回滚到上一发布版本。') } catch (reason) { setError(reason instanceof Error ? reason.message : '背景回滚失败') } finally { setBusy(false) } }
  const previewAsset = system.resolve(selected.lightAssetKey)
  const assetOptions = <>{system.assets.map((asset) => <option value={asset.assetKey} key={asset.id}>{asset.assetKey}</option>)}</>
  return <>
    <Header title="页面背景中心" description="为每个页面配置白天、夜间、移动端背景、遮罩、焦点和定时发布。"><div className="flex gap-2"><PixelButton disabled={busy} variant="secondary" onClick={() => void rollback()}>回滚</PixelButton><PixelButton disabled={busy} onClick={() => void publish()}>{busy ? '处理中…' : '发布配置'}</PixelButton></div></Header>
    {error && <div className="mb-4 border-2 border-red-300 bg-red-50 p-3 text-sm font-black text-red-700">{error}</div>}
    {message && <div className="mb-4 border-2 border-emerald-300 bg-emerald-50 p-3 text-sm font-black text-emerald-700">{message}</div>}
    <div className="grid gap-6 xl:grid-cols-[260px_minmax(0,1fr)_330px]">
      <aside><PixelCard title="页面背景"><div className="space-y-1">{system.backgrounds.map((item) => <button className={`admin-nav-item ${item.id === selected.id ? 'admin-nav-item-active' : ''}`} onClick={() => { editingId.current = item.id; setSelectedId(item.id) }} key={item.id}>{item.label}<span>v{item.version}</span></button>)}</div></PixelCard></aside>
      <main className="min-w-0 space-y-5"><PixelCard title="实时背景预览" action={<StatusBadge tone={selected.status === 'published' ? 'green' : 'yellow'}>{selected.status}</StatusBadge>}><div className="relative min-h-[480px] overflow-hidden border-2 border-asamu-ink bg-asamu-canvas"><div className="absolute inset-0" style={{ backgroundImage: `url(${previewAsset.url})`, backgroundSize: selected.fit === 'repeat' || selected.fit === 'repeat-x' ? 'auto' : selected.fit, backgroundRepeat: selected.fit === 'repeat' || selected.fit === 'repeat-x' ? selected.fit : 'no-repeat', backgroundPosition: selected.position, opacity: selected.assetOpacity, filter: selected.blur ? `blur(${selected.blur}px)` : undefined }} /><div className="absolute inset-0" style={{ backgroundColor: selected.overlayColor, opacity: selected.overlayOpacity }} /><div className="relative z-10 p-8"><PixelTag tone="yellow">{selected.label}</PixelTag><h2 className="mt-4 font-display text-4xl font-black">背景可读性预览</h2><p className="mt-3 max-w-xl font-semibold leading-7 text-asamu-muted">标题、正文与按钮必须在不同素材和遮罩下保持清晰。</p><div className="mt-6 grid gap-4 sm:grid-cols-2"><SecondaryCard title="主内容卡"><p className="text-sm leading-6 text-asamu-muted">检查复杂背景是否干扰文字阅读。</p><PixelButton className="mt-4">主要操作</PixelButton></SecondaryCard><SecondaryCard title="素材焦点"><AssetImage className="h-32 w-full" assetKey={selected.lightAssetKey} alt="背景焦点素材" /></SecondaryCard></div></div></div></PixelCard></main>
      <aside className="space-y-5"><PixelCard title="背景配置"><div className="space-y-4">
        <Field label="白天桌面素材"><PixelSelect value={selected.lightAssetKey} onChange={(event) => patch({ lightAssetKey: event.target.value, status: 'draft' })}>{assetOptions}</PixelSelect></Field>
        <Field label="夜间桌面素材"><PixelSelect value={selected.darkAssetKey ?? ''} onChange={(event) => patch({ darkAssetKey: event.target.value || undefined, status: 'draft' })}><option value="">跟随白天</option>{assetOptions}</PixelSelect></Field>
        <Field label="白天移动素材"><PixelSelect value={selected.mobileAssetKey ?? ''} onChange={(event) => patch({ mobileAssetKey: event.target.value || undefined, status: 'draft' })}><option value="">跟随桌面</option>{assetOptions}</PixelSelect></Field>
        <Field label="夜间移动素材"><PixelSelect value={selected.darkMobileAssetKey ?? ''} onChange={(event) => patch({ darkMobileAssetKey: event.target.value || undefined, status: 'draft' })}><option value="">自动回退</option>{assetOptions}</PixelSelect></Field>
        <div className="grid grid-cols-2 gap-3"><Field label="显示模式"><PixelSelect value={selected.fit} onChange={(event) => patch({ fit: event.target.value as BackgroundFit, status: 'draft' })}>{['cover', 'contain', 'repeat', 'repeat-x'].map((item) => <option key={item}>{item}</option>)}</PixelSelect></Field><Field label="对齐"><PixelInput value={selected.position} onChange={(event) => patch({ position: event.target.value, status: 'draft' })} /></Field></div>
        <div className="grid grid-cols-2 gap-3"><Range label="焦点 X" value={selected.focalPoint.x} max={100} step={1} onChange={(x) => patch({ focalPoint: { ...selected.focalPoint, x }, status: 'draft' })} /><Range label="焦点 Y" value={selected.focalPoint.y} max={100} step={1} onChange={(y) => patch({ focalPoint: { ...selected.focalPoint, y }, status: 'draft' })} /></div>
        <Range label="素材透明度" value={selected.assetOpacity} max={1} step={0.01} onChange={(assetOpacity) => patch({ assetOpacity, status: 'draft' })} /><Range label="遮罩透明度" value={selected.overlayOpacity} max={1} step={0.01} onChange={(overlayOpacity) => patch({ overlayOpacity, status: 'draft' })} /><Range label="模糊强度" value={selected.blur} max={20} step={1} onChange={(blur) => patch({ blur, status: 'draft' })} />
        <Field label="遮罩颜色"><PixelInput type="color" value={selected.overlayColor} onChange={(event) => patch({ overlayColor: event.target.value, status: 'draft' })} /></Field><Field label="定时生效"><PixelInput type="datetime-local" value={selected.scheduledAt ?? ''} onChange={(event) => patch({ scheduledAt: event.target.value || undefined, status: 'draft' })} /></Field>
      </div></PixelCard></aside>
    </div>
  </>
}

function Header({ title, description, children }: { title: string; description: string; children?: ReactNode }) { return <header className="mb-6 flex flex-col gap-4 border-b-2 border-asamu-ink/10 pb-5 sm:flex-row sm:items-end sm:justify-between"><div><p className="text-xs font-black uppercase tracking-[.18em] text-asamu-blue">VISUAL OPERATIONS</p><h1 className="mt-2 font-display text-3xl font-black">{title}</h1><p className="mt-2 text-sm font-semibold text-asamu-muted">{description}</p></div>{children}</header> }
function Field({ label, children }: { label: string; children: ReactNode }) { return <label className="block text-xs font-black">{label}<div className="mt-1">{children}</div></label> }
function Range({ label, value, max, step, onChange }: { label: string; value: number; max: number; step: number; onChange: (value: number) => void }) { return <label className="block text-xs font-black"><span className="flex justify-between"><span>{label}</span><span>{value}</span></span><input className="mt-2 w-full accent-asamu-blue" type="range" min="0" max={max} step={step} value={value} onChange={(event) => onChange(Number(event.target.value))} /></label> }
