import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { PageContainer, PixelButton, PixelCard, PixelInput, RobotTip } from '../components/ui/System'
import { ApiError } from '../services/apiClient'
import { confirmEmailChange, requestPasswordReset, resetPassword, verifyEmail } from '../services/authApi'

type Mode = 'forgot' | 'reset' | 'verify' | 'change-email'

export function AccountActionPage({ mode }: { mode: Mode }) {
  const [params] = useSearchParams()
  const token = params.get('token') ?? ''
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [pending, setPending] = useState(mode === 'verify' || mode === 'change-email')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    if (mode !== 'verify' && mode !== 'change-email') return
    if (!token) { setPending(false); setError('链接中缺少确认令牌。'); return }
    const action = mode === 'verify' ? verifyEmail(token) : confirmEmailChange(token)
    action.then(() => setMessage(mode === 'verify' ? '邮箱验证成功。' : '邮箱修改成功，旧会话已撤销，请重新登录。')).catch((reason) => setError(reason instanceof ApiError ? reason.message : '链接无效或已过期。')).finally(() => setPending(false))
  }, [mode, token])

  const submit = async () => {
    setPending(true); setError(''); setMessage('')
    try {
      if (mode === 'forgot') { await requestPasswordReset(email); setMessage('如果账户存在，重置邮件已进入发送队列。') }
      if (mode === 'reset') { await resetPassword(token, password); setMessage('密码已重置，所有旧会话均已撤销。') }
    } catch (reason) { setError(reason instanceof ApiError ? reason.message : '暂时无法完成操作。') } finally { setPending(false) }
  }

  const title = mode === 'forgot' ? '找回密码' : mode === 'reset' ? '设置新密码' : mode === 'verify' ? '验证邮箱' : '确认新邮箱'
  return <PageContainer className="py-12"><PixelCard className="mx-auto max-w-lg" title={title}>
    {(mode === 'verify' || mode === 'change-email') && pending && <p className="font-semibold text-asamu-muted">正在校验一次性链接…</p>}
    {mode === 'forgot' && <label className="block text-sm font-black">注册邮箱<PixelInput className="mt-2 w-full" type="email" autoComplete="email" value={email} onChange={(event) => setEmail(event.target.value)} /></label>}
    {mode === 'reset' && <label className="block text-sm font-black">新密码<PixelInput className="mt-2 w-full" type="password" minLength={10} maxLength={128} autoComplete="new-password" value={password} onChange={(event) => setPassword(event.target.value)} /></label>}
    {(mode === 'forgot' || mode === 'reset') && <PixelButton className="mt-5 w-full" disabled={pending || (mode === 'forgot' ? !email : !token || password.length < 10)} onClick={submit}>{pending ? '处理中…' : '提交'}</PixelButton>}
    {message && <RobotTip className="mt-5" title="操作完成">{message}</RobotTip>}
    {error && <p className="mt-5 border-2 border-red-400 bg-red-50 p-3 text-sm font-black text-red-700">{error}</p>}
    <p className="mt-5 text-center text-sm"><Link className="font-black text-asamu-blue" to="/login">返回登录</Link></p>
  </PixelCard></PageContainer>
}
