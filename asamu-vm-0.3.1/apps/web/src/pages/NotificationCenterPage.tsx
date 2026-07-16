import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { EmptyState, PageContainer, PageHeader, PixelButton, PixelCard, PixelTag } from '../components/ui/System'
import { useAuth } from '../contexts/AuthProvider'
import { fetchNotifications, readAllNotifications, readNotification, type NotificationDto } from '../services/communityApi'

export function NotificationCenterPage() {
  const auth = useAuth()
  const [items, setItems] = useState<NotificationDto[]>([])
  const [unreadOnly, setUnreadOnly] = useState(false)
  const [error, setError] = useState('')
  const load = useCallback(() => { if (!auth.user) return; fetchNotifications(unreadOnly).then((page) => { setItems(page.items); setError('') }).catch((reason: Error) => setError(reason.message)) }, [auth.user, unreadOnly])
  useEffect(() => { load(); const timer = window.setInterval(load, 15_000); const visible = () => document.visibilityState === 'visible' && load(); document.addEventListener('visibilitychange', visible); return () => { window.clearInterval(timer); document.removeEventListener('visibilitychange', visible) } }, [load])
  const markOne = async (item: NotificationDto) => { if (item.readAt) return; await readNotification(item.id); setItems((current) => current.map((entry) => entry.id === item.id ? { ...entry, readAt: new Date().toISOString() } : entry)) }
  const markAll = async () => { await readAllNotifications(); setItems((current) => current.map((entry) => ({ ...entry, readAt: entry.readAt ?? new Date().toISOString() }))) }
  if (!auth.user) return <PageContainer><PixelCard className="mx-auto max-w-lg" title="登录后查看通知"><Link to="/login" state={{ from: '/notifications' }}><PixelButton>前往登录</PixelButton></Link></PixelCard></PageContainer>
  return <PageContainer><PageHeader eyebrow="NOTIFICATION CENTER" title="通知中心" description="审核结果、战队邀请和平台事件会汇集在这里；页面可见时每 15 秒同步一次。"><div className="flex gap-2"><PixelButton variant={unreadOnly ? 'yellow' : 'secondary'} onClick={() => setUnreadOnly((value) => !value)}>仅看未读</PixelButton><PixelButton onClick={markAll}>全部已读</PixelButton></div></PageHeader>
    {error && <p className="mb-4 border-2 border-red-400 bg-red-50 p-3 text-sm font-black text-red-700">{error}</p>}
    {!items.length ? <PixelCard><EmptyState title="暂无通知" description={unreadOnly ? '所有通知都已读。' : '新的审核、邀请和系统消息会显示在这里。'} /></PixelCard> : <div className="space-y-3">{items.map((item) => <PixelCard className={item.readAt ? 'opacity-70' : ''} key={item.id} action={<PixelTag tone={item.readAt ? 'slate' : 'yellow'}>{item.readAt ? '已读' : '未读'}</PixelTag>}><div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"><div><h2 className="font-display text-lg font-black">{item.title}</h2><p className="mt-1 text-sm font-semibold text-asamu-muted">{item.body}</p><p className="mt-2 text-xs text-asamu-muted">{new Date(item.createdAt).toLocaleString('zh-CN')}</p></div><div className="flex gap-2">{!item.readAt && <PixelButton size="sm" variant="secondary" onClick={() => markOne(item)}>标为已读</PixelButton>}{item.link && <Link to={item.link} onClick={() => markOne(item)}><PixelButton size="sm">查看</PixelButton></Link>}</div></div></PixelCard>)}</div>}
  </PageContainer>
}
