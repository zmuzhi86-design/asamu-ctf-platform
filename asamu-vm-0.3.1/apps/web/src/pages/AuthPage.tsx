import { useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { PageContainer, PixelButton, PixelCard, PixelInput, RobotTip } from '../components/ui/System'
import { useAuth } from '../contexts/AuthProvider'
import { ApiError } from '../services/apiClient'

export function AuthPage({ mode }: { mode: 'login' | 'register' }) {
  const auth = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const locationState = location.state as { from?: string; notice?: string } | null
  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [pending, setPending] = useState(false)
  const submit = async () => {
    setPending(true); setError('')
    try {
      if (mode === 'login') await auth.login(username, password)
      else await auth.register(email, username, password)
      navigate(locationState?.from || '/profile', { replace: true })
    } catch (reason) { setError(reason instanceof ApiError ? reason.message : '暂时无法完成认证') } finally { setPending(false) }
  }
  return <PageContainer><div className="mx-auto grid max-w-5xl gap-6 py-10 lg:grid-cols-[1fr_420px] lg:items-center">
    <div><p className="text-xs font-black tracking-[.22em] text-asamu-blue">ASAMU ACCESS</p><h1 className="mt-4 font-display text-4xl font-black">{mode === 'login' ? '返回你的训练基地' : '创建 CTF 学员档案'}</h1><p className="mt-4 max-w-xl font-semibold leading-7 text-asamu-muted">真实账号用于隔离动态环境、提交判题、比赛积分和战队权限。Refresh Token 由 HttpOnly Cookie 保存。</p><RobotTip className="mt-7" title="安全提示">平台不会在浏览器保存 Refresh Token，也不会向前端返回正确 Flag。</RobotTip></div>
    <PixelCard title={mode === 'login' ? '登录 asamu' : '注册 asamu'}><div className="space-y-4">
      {locationState?.notice && <p className="border-2 border-green-500 bg-green-50 p-3 text-sm font-black text-green-700">{locationState.notice}</p>}
      {mode === 'register' && <label className="block text-sm font-black">邮箱<PixelInput className="mt-2 w-full" type="email" value={email} onChange={(event) => setEmail(event.target.value)} autoComplete="email" /></label>}
      <label className="block text-sm font-black">{mode === 'login' ? '邮箱或用户名' : '用户名'}<PixelInput className="mt-2 w-full" value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" /></label>
      <label className="block text-sm font-black">密码<PixelInput className="mt-2 w-full" type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete={mode === 'login' ? 'current-password' : 'new-password'} onKeyDown={(event) => event.key === 'Enter' && submit()} /></label>
      {mode === 'login' && <div className="text-right"><Link className="text-sm font-black text-asamu-blue" to="/forgot-password">忘记密码？</Link></div>}
      {error && <p className="border-2 border-red-500 bg-red-50 p-3 text-sm font-black text-red-700">{error}</p>}
      <PixelButton className="w-full" onClick={submit} disabled={pending}>{pending ? '处理中…' : mode === 'login' ? '登录' : '创建账号'}</PixelButton>
      <p className="text-center text-sm font-semibold text-asamu-muted">{mode === 'login' ? <>还没有账号？<Link className="font-black text-asamu-blue" to="/register">立即注册</Link></> : <>已有账号？<Link className="font-black text-asamu-blue" to="/login">直接登录</Link></>}</p>
    </div></PixelCard>
  </div></PageContainer>
}
