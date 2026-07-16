import { useState } from 'react'
import { Navigate, NavLink, Outlet, useNavigate } from 'react-router-dom'
import { AssetImage, PixelButton } from '../components/ui/System'
import { PageBackground } from '../components/layout/PageBackground'
import { useAuth } from '../contexts/AuthProvider'

const adminLinks = [
  { to: '/admin', label: '管理概览' },
  { to: '/admin/platform', label: '平台与方向', permission: 'platform.read' },
  { to: '/admin/assets', label: '素材中心', permission: 'asset.read' },
  { to: '/admin/appearance/slots', label: '页面槽位', permission: 'appearance.read' },
  { to: '/admin/appearance/backgrounds', label: '页面背景', permission: 'appearance.read' },
  { to: '/admin/assets/audit', label: '素材审计', permission: 'asset.read' },
  { to: '/admin/challenges', label: '题目管理', permission: 'challenge.read' },
  { to: '/admin/learning', label: '学习中心', permission: 'progression.manage' },
  { to: '/admin/competitions', label: '比赛管理', permission: 'competition.read' },
  { to: '/admin/instances', label: '动态环境', permission: 'instance.read' },
  { to: '/admin/registry-credentials', label: '私有镜像仓库', permission: 'registry.read' },
  { to: '/admin/users', label: '用户与角色', permission: 'user.read' },
  { to: '/admin/submissions', label: '提交记录', permission: 'submission.read' },
  { to: '/admin/anti-cheat', label: '反作弊中心', permission: 'anticheat.read' },
  { to: '/admin/writeups', label: 'WriteUp 审核', permission: 'writeup.review' },
  { to: '/admin/announcements', label: '公告管理', permission: 'announcement.write' },
  { to: '/admin/settings', label: '系统设置', permission: 'platform.read' },
]

export function AdminLayout() {
  const auth = useAuth()
  const navigate = useNavigate()
  const [logoutPending, setLogoutPending] = useState(false)
  if (auth.loading) return <div className="grid min-h-screen place-items-center bg-asamu-canvas font-black text-asamu-muted">正在验证管理权限…</div>
  if (!auth.user) return <Navigate to="/login" replace />
  if (!auth.canAccessAdmin) return <Navigate to="/" replace />
  const visibleLinks = adminLinks.filter((item) => !item.permission || auth.hasPermission(item.permission))
  const logout = async () => {
    if (logoutPending) return
    setLogoutPending(true)
    try { await auth.logout() } catch {} finally { setLogoutPending(false); navigate('/', { replace: true }) }
  }
  return <div className="relative isolate min-h-screen bg-asamu-canvas/80 text-asamu-ink">
    <PageBackground />
    <div className="relative z-10">
      <header className="sticky top-0 z-50 flex h-16 items-center border-b-2 border-asamu-ink bg-asamu-card px-4 sm:px-6">
        <NavLink to="/" className="flex items-center gap-3"><AssetImage className="h-10 w-10" assetKey="mascot.default" alt="管理控制台" /><div><b className="block text-sm text-asamu-blue">ASAMU</b><span className="text-xs font-black">ADMIN CONSOLE</span></div></NavLink>
        <div className="ml-auto flex items-center gap-3"><span className="hidden text-xs font-bold text-asamu-muted sm:block">平台状态：正常</span><NavLink to="/profile"><PixelButton size="sm" variant="secondary">{auth.user.username}</PixelButton></NavLink><PixelButton size="sm" variant="danger" disabled={logoutPending} onClick={() => void logout()}>{logoutPending ? '退出中…' : '退出登录'}</PixelButton></div>
      </header>
      <div className="mx-auto grid max-w-[1600px] lg:grid-cols-[230px_minmax(0,1fr)]">
        <aside className="border-r border-asamu-line bg-asamu-card lg:min-h-[calc(100vh-64px)]"><nav className="sticky top-20 py-4">{visibleLinks.map(({ to, label }) => <NavLink key={to} to={to} end={to === '/admin'} className={({ isActive }) => `admin-nav-item ${isActive ? 'admin-nav-item-active' : ''}`}>{label}<span>→</span></NavLink>)}</nav></aside>
        <main className="min-w-0 px-4 py-6 sm:px-6 lg:px-8"><Outlet /></main>
      </div>
    </div>
  </div>
}
