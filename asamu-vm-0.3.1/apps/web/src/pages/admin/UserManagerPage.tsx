import { useCallback, useEffect, useState } from 'react'
import { PageHeader, PixelButton, PixelCard, PixelInput, PixelTag } from '../../components/ui/System'
import { useAuth } from '../../contexts/AuthProvider'
import { ApiError, apiRequest } from '../../services/apiClient'

type Page<T> = { items: T[] }
type UserRow = { id: string; email: string; username: string; status: string; displayName: string; organization: string; roles: string[]; createdAt: string }
const roles = ['super_admin', 'site_admin', 'visual_operator', 'competition_admin', 'challenge_author', 'reviewer', 'team_captain']

export function UserManagerPage() {
  const auth = useAuth()
  const [items, setItems] = useState<UserRow[]>([])
  const [search, setSearch] = useState('')
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const load = useCallback(() => apiRequest<Page<UserRow>>(`/admin/users?pageSize=100&search=${encodeURIComponent(search)}`).then((page) => setItems(page.items)).catch((reason: Error) => setError(reason.message)), [search])
  useEffect(() => { const timer = window.setTimeout(load, 200); return () => window.clearTimeout(timer) }, [load])
  const run = async (action: () => Promise<unknown>, success: string) => { setError(''); setMessage(''); try { await action(); setMessage(success); await load() } catch (reason) { setError(reason instanceof ApiError ? reason.message : '操作失败') } }
  const status = (user: UserRow) => { if (user.status === 'active') { const reason = window.prompt(`请输入封禁 ${user.username} 的原因`); if (!reason) return; run(() => apiRequest(`/admin/users/${user.id}/status`, { method: 'PATCH', body: JSON.stringify({ status: 'banned', reason }) }), '用户已封禁，现有会话已撤销。') } else if (window.confirm(`确认恢复 ${user.username}？`)) run(() => apiRequest(`/admin/users/${user.id}/status`, { method: 'PATCH', body: JSON.stringify({ status: 'active', reason: 'admin_unban' }) }), '用户已恢复。') }
  return <><PageHeader eyebrow="IDENTITY OPERATIONS" title="用户与角色" description="状态和角色变更会提升 token_version 并撤销用户的现有会话。"><PixelInput className="w-64" placeholder="搜索邮箱或用户名" value={search} onChange={(event) => setSearch(event.target.value)} /></PageHeader>{message && <p className="mb-4 border-2 border-green-400 bg-green-50 p-3 text-sm font-black text-green-700">{message}</p>}{error && <p className="mb-4 border-2 border-red-400 bg-red-50 p-3 text-sm font-black text-red-700">{error}</p>}<div className="space-y-4">{items.map((user) => <PixelCard key={user.id} title={<span>{user.username} <PixelTag tone={user.status === 'active' ? 'green' : 'red'}>{user.status}</PixelTag></span>} action={auth.hasPermission('user.ban') ? <PixelButton size="sm" variant={user.status === 'active' ? 'danger' : 'secondary'} onClick={() => status(user)}>{user.status === 'active' ? '封禁' : '恢复'}</PixelButton> : undefined}><p className="text-sm font-semibold text-asamu-muted">{user.email} · {user.organization || '无组织'} · {new Date(user.createdAt).toLocaleDateString('zh-CN')}</p><div className="mt-4 flex flex-wrap gap-2"><PixelTag>user</PixelTag>{roles.map((role) => { const enabled = user.roles.includes(role); return <button className={`pixel-tag ${enabled ? 'pixel-tag-yellow' : 'pixel-tag-slate'} disabled:cursor-not-allowed`} disabled={!auth.hasPermission('rbac.manage')} onClick={() => run(() => apiRequest(`/admin/users/${user.id}/roles`, { method: 'PATCH', body: JSON.stringify({ role, enabled: !enabled }) }), `${role} 已${enabled ? '移除' : '授予'}。`)} key={role}>{enabled ? '✓ ' : '+ '}{role}</button> })}</div></PixelCard>)}</div></>
}
