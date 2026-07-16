import { useEffect, useState, type ButtonHTMLAttributes, type CSSProperties, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes } from 'react'
import { assets } from '../../data/assetManifest'
import { useAssetSystem } from '../../contexts/AssetProvider'

export function PageContainer({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <div className={`mx-auto w-full max-w-[1440px] px-4 py-6 sm:px-6 lg:px-8 ${className}`}>{children}</div>
}

type CardProps = { children: ReactNode; className?: string; title?: ReactNode; action?: ReactNode; padded?: boolean }

export function PixelCard({ children, className = '', title, action, padded = true }: CardProps) {
  return <section className={`asamu-card asamu-card-primary ${padded ? 'p-4 sm:p-5' : ''} ${className}`}>
    {(title || action) && <div className="mb-4 flex items-center justify-between gap-3 border-b border-asamu-line pb-3"><h2 className="font-display text-base font-black tracking-tight">{title}</h2>{action}</div>}
    {children}
  </section>
}

export function SecondaryCard({ children, className = '', title, action, padded = true }: CardProps) {
  return <section className={`asamu-card asamu-card-secondary ${padded ? 'p-4' : ''} ${className}`}>
    {(title || action) && <div className="mb-3 flex items-center justify-between gap-3 border-b border-asamu-line pb-2"><h3 className="text-sm font-black">{title}</h3>{action}</div>}
    {children}
  </section>
}

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & { variant?: 'primary' | 'yellow' | 'secondary' | 'ghost' | 'danger'; size?: 'sm' | 'md' }
export function PixelButton({ className = '', variant = 'primary', size = 'md', ...props }: ButtonProps) {
  return <button className={`pixel-button pixel-button-${variant} pixel-button-${size} ${className}`} {...props} />
}

export function PixelInput(props: InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={`pixel-input ${props.className ?? ''}`} />
}

export function PixelSelect(props: SelectHTMLAttributes<HTMLSelectElement>) {
  return <select {...props} className={`pixel-input appearance-none pr-9 ${props.className ?? ''}`} />
}

export function PixelTag({ children, tone = 'blue' }: { children: ReactNode; tone?: 'blue' | 'yellow' | 'green' | 'red' | 'slate' }) {
  return <span className={`pixel-tag pixel-tag-${tone}`}>{children}</span>
}

export function StatusBadge({ children, tone = 'blue', pulse = false }: { children: ReactNode; tone?: 'blue' | 'yellow' | 'green' | 'red' | 'slate'; pulse?: boolean }) {
  return <span className={`status-badge status-${tone}`}><i className={pulse ? 'animate-pulse' : ''} />{children}</span>
}

export function ProgressBar({ value, tone = 'blue', label }: { value: number; tone?: 'blue' | 'green' | 'yellow'; label?: string }) {
  return <div>{label && <div className="mb-1 flex justify-between text-xs font-bold text-asamu-muted"><span>{label}</span><span>{value}%</span></div>}<div className="pixel-progress"><span className={`progress-${tone}`} style={{ width: `${Math.max(0, Math.min(100, value))}%` }} /></div></div>
}

export function PixelTabs({ items, active, onChange }: { items: string[]; active: string; onChange?: (value: string) => void }) {
  return <div className="flex flex-wrap gap-2" role="tablist">{items.map((item) => <button className={`pixel-tab ${active === item ? 'pixel-tab-active' : ''}`} key={item} onClick={() => onChange?.(item)}>{item}</button>)}</div>
}

export function SectionHeading({ eyebrow, title, description, action }: { eyebrow?: string; title: string; description?: string; action?: ReactNode }) {
  return <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between"><div>{eyebrow && <p className="mb-1 text-xs font-black uppercase tracking-[0.18em] text-asamu-blue">{eyebrow}</p>}<h2 className="font-display text-2xl font-black tracking-tight sm:text-3xl">{title}</h2>{description && <p className="mt-2 max-w-2xl text-sm font-medium leading-6 text-asamu-muted">{description}</p>}</div>{action}</div>
}

export function PageHeader({ title, description, eyebrow, children }: { title: string; description: string; eyebrow?: string; children?: ReactNode }) {
  return <header className="mb-6 flex flex-col gap-4 border-b-2 border-asamu-ink/10 pb-5 lg:flex-row lg:items-end lg:justify-between"><div><p className="mb-2 text-xs font-black uppercase tracking-[0.2em] text-asamu-blue">{eyebrow ?? 'ASAMU PLATFORM'}</p><h1 className="font-display text-3xl font-black tracking-tight sm:text-4xl">{title}</h1><p className="mt-2 max-w-2xl text-sm font-medium leading-6 text-asamu-muted">{description}</p></div>{children}</header>
}

export function RobotTip({ title, children, robot = assets.characters.helperRobot, className = '' }: { title: string; children: ReactNode; robot?: string; className?: string }) {
  return <div className={`robot-tip ${className}`}><img src={robot} alt="像素机器人助手" /><div><b className="font-display text-base">{title}</b><div className="mt-1 text-sm font-medium leading-6 text-asamu-muted">{children}</div></div></div>
}

export function EmptyState({ title, description, image = assets.emptyStates.empty, action }: { title: string; description: string; image?: string; action?: ReactNode }) {
  return <div className="py-10 text-center"><img className="mx-auto h-32 w-32 object-contain" src={image} alt="空状态" /><h3 className="mt-3 text-lg font-black">{title}</h3><p className="mx-auto mt-2 max-w-md text-sm text-asamu-muted">{description}</p>{action && <div className="mt-5">{action}</div>}</div>
}

export function Metric({ label, value, note, highlight = false }: { label: string; value: string; note?: string; highlight?: boolean }) {
  return <div className={`metric-card ${highlight ? 'metric-highlight' : ''}`}><span>{label}</span><b>{value}</b>{note && <small>{note}</small>}</div>
}

export function DataTable({ headers, rows }: { headers: string[]; rows: ReactNode[][] }) {
  return <div className="overflow-x-auto"><table className="data-table"><thead><tr>{headers.map((header) => <th key={header}>{header}</th>)}</tr></thead><tbody>{rows.map((row, rowIndex) => <tr key={rowIndex}>{row.map((cell, cellIndex) => <td key={cellIndex}>{cell}</td>)}</tr>)}</tbody></table></div>
}

export function Pagination({ current = 1, total = 1, onChange }: { current?: number; total?: number; onChange?: (page: number) => void }) {
  const safeTotal = Math.max(1, total)
  const safeCurrent = Math.min(safeTotal, Math.max(1, current))
  return <nav className="flex items-center justify-end gap-2" aria-label="分页"><PixelButton size="sm" variant="secondary" disabled={safeCurrent <= 1 || !onChange} onClick={() => onChange?.(safeCurrent - 1)}>上一页</PixelButton><span className="px-2 text-sm font-black">{safeCurrent} / {safeTotal}</span><PixelButton size="sm" variant="secondary" disabled={safeCurrent >= safeTotal || !onChange} onClick={() => onChange?.(safeCurrent + 1)}>下一页</PixelButton></nav>
}

export function Skeleton({ className = '' }: { className?: string }) { return <span className={`block animate-pulse bg-blue-100 ${className}`} /> }
export function LoadingState() { return <div className="grid gap-3"><Skeleton className="h-5 w-2/5" /><Skeleton className="h-20 w-full" /><Skeleton className="h-20 w-full" /></div> }

export function Modal({ open, title, children, onClose }: { open: boolean; title: string; children: ReactNode; onClose: () => void }) {
  useEffect(() => { if (!open) return; const close = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose() }; window.addEventListener('keydown', close); return () => window.removeEventListener('keydown', close) }, [onClose, open])
  if (!open) return null
  return <div className="fixed inset-0 z-[80] grid place-items-center bg-asamu-ink/45 p-4" role="dialog" aria-modal="true" aria-label={title}><PixelCard className="max-h-[90vh] w-full max-w-2xl overflow-y-auto" title={title} action={<PixelButton size="sm" variant="ghost" onClick={onClose}>关闭</PixelButton>}>{children}</PixelCard></div>
}

export function Toast({ message, tone = 'blue', onClose }: { message: string; tone?: 'blue' | 'green' | 'red'; onClose?: () => void }) {
  useEffect(() => { if (!onClose) return; const timer = window.setTimeout(onClose, 2600); return () => window.clearTimeout(timer) }, [onClose])
  return <div className={`fixed bottom-5 right-5 z-[90] max-w-sm border-2 border-asamu-ink bg-white p-4 text-sm font-black shadow-pixel toast-${tone}`}>{message}</div>
}

export type AssetImageProps = { assetKey?: string; src?: string; alt: string; className?: string; fit?: 'contain' | 'cover'; position?: string; safePadding?: number | string; focalPoint?: { x: number; y: number }; fallbackAssetKey?: string; loading?: 'lazy' | 'eager'; decorative?: boolean }

export function AssetImage({ assetKey, src, alt, className = '', fit, position, safePadding, focalPoint, fallbackAssetKey = 'mascot.default', loading = 'lazy', decorative = false }: AssetImageProps) {
  const { resolve } = useAssetSystem()
  const requested = assetKey ? resolve(assetKey, { mobile: window.matchMedia('(max-width: 760px)').matches, theme: document.documentElement.classList.contains('theme-dark') ? 'dark' : 'light', fallbackAssetKey }) : null
  const fallback = resolve(fallbackAssetKey)
  const initialUrl = src ?? requested?.url ?? fallback.url
  const [url, setUrl] = useState(initialUrl)
  useEffect(() => setUrl(initialUrl), [initialUrl])
  const resolvedFit = fit ?? requested?.fit ?? 'contain'
  const resolvedPosition = position ?? (focalPoint ? `${focalPoint.x}% ${focalPoint.y}%` : requested?.position ?? 'center')
  const style: CSSProperties = { objectFit: resolvedFit, objectPosition: resolvedPosition, padding: safePadding }
  return <img className={`select-none ${className}`} src={url} alt={decorative ? '' : alt || requested?.altText || ''} aria-hidden={decorative || undefined} draggable={false} loading={loading} style={style} onError={() => { if (url !== fallback.url) setUrl(fallback.url) }} />
}

export function HeroArtwork(props: AssetImageProps) { return <AssetImage loading="eager" fit="contain" safePadding="clamp(12px, 2vw, 32px)" {...props} /> }
export function SceneArtwork(props: AssetImageProps) { return <AssetImage fit="contain" safePadding="8px" {...props} /> }
export function BadgeArtwork(props: AssetImageProps) { return <AssetImage fit="contain" safePadding="6px" {...props} /> }
export function TransparentPreview({ assetKey, className = '' }: { assetKey: string; className?: string }) { return <div className={`grid grid-cols-2 overflow-hidden border border-asamu-line ${className}`}><div className="grid place-items-center bg-white p-4"><AssetImage className="h-full w-full" assetKey={assetKey} alt="浅色背景预览" /></div><div className="grid place-items-center bg-[#10233F] p-4"><AssetImage className="h-full w-full" assetKey={assetKey} alt="深色背景预览" /></div></div> }
