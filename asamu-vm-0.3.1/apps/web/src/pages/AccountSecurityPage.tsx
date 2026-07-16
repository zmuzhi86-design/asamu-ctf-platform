import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { PageContainer, PageHeader, PixelButton, PixelCard, PixelInput, PixelTag, RobotTip } from '../components/ui/System'
import { useAuth } from '../contexts/AuthProvider'
import { ApiError } from '../services/apiClient'
import { changePassword, requestEmailChange, resendVerification } from '../services/authApi'

export function AccountSecurityPage() {
  const auth = useAuth()
  const navigate = useNavigate()
  const [emailCurrentPassword, setEmailCurrentPassword] = useState('')
  const [passwordCurrentPassword, setPasswordCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [newEmail, setNewEmail] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [pendingAction, setPendingAction] = useState<'verification' | 'email' | 'password' | null>(null)
  const run = async (kind: 'verification' | 'email' | 'password', action: () => Promise<unknown>, success: string, afterSuccess?: () => void) => {
    if (pendingAction) return
    setPendingAction(kind); setError(''); setMessage('')
    try { await action(); setMessage(success); afterSuccess?.() } catch (reason) { setError(reason instanceof ApiError ? reason.message : '暂时无法完成操作。') } finally { setPendingAction(null) }
  }
  if (!auth.user) return <PageContainer><PixelCard className="mx-auto max-w-lg" title="需要登录"><Link to="/login"><PixelButton>前往登录</PixelButton></Link></PixelCard></PageContainer>
  return <PageContainer><PageHeader eyebrow="ACCOUNT SECURITY" title="账号与安全" description="管理邮箱验证、登录邮箱和密码；敏感变更会撤销已有会话。" />
    {message && <RobotTip className="mb-5" title="操作已受理">{message}</RobotTip>}{error && <p className="mb-5 border-2 border-red-400 bg-red-50 p-3 text-sm font-black text-red-700">{error}</p>}
    <div className="grid gap-6 lg:grid-cols-2">
      <PixelCard title="邮箱安全"><p className="mb-4 text-sm font-semibold">当前邮箱：{auth.user.email} <PixelTag tone={auth.user.emailVerified ? 'green' : 'yellow'}>{auth.user.emailVerified ? '已验证' : '待验证'}</PixelTag></p>{auth.user.pendingEmail && <p className="mb-4 text-sm">等待确认：{auth.user.pendingEmail}</p>}{!auth.user.emailVerified && <PixelButton className="mb-5" variant="secondary" disabled={pendingAction !== null} onClick={() => run('verification', resendVerification, '验证邮件已进入发送队列。')}>{pendingAction === 'verification' ? '发送中…' : '重新发送验证邮件'}</PixelButton>}<label className="block text-sm font-black">新邮箱<PixelInput className="mt-2 w-full" type="email" autoComplete="email" value={newEmail} onChange={(event) => setNewEmail(event.target.value)} /></label><label className="mt-4 block text-sm font-black">当前密码<PixelInput className="mt-2 w-full" type="password" autoComplete="current-password" value={emailCurrentPassword} onChange={(event) => setEmailCurrentPassword(event.target.value)} /></label><PixelButton className="mt-5" disabled={pendingAction !== null || !newEmail || !emailCurrentPassword} onClick={() => run('email', () => requestEmailChange(emailCurrentPassword, newEmail), '确认邮件已发送至新邮箱。', () => { setEmailCurrentPassword(''); setNewEmail('') })}>{pendingAction === 'email' ? '申请中…' : '申请修改邮箱'}</PixelButton></PixelCard>
      <PixelCard title="修改密码"><label className="block text-sm font-black">当前密码<PixelInput className="mt-2 w-full" type="password" autoComplete="current-password" value={passwordCurrentPassword} onChange={(event) => setPasswordCurrentPassword(event.target.value)} /></label><label className="mt-4 block text-sm font-black">新密码<PixelInput className="mt-2 w-full" type="password" minLength={10} maxLength={128} autoComplete="new-password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} /></label><PixelButton className="mt-5" disabled={pendingAction !== null || !passwordCurrentPassword || newPassword.length < 10} onClick={() => run('password', async () => { await changePassword(passwordCurrentPassword, newPassword); try { await auth.logout() } catch {} }, '密码已修改，请重新登录。', () => { setPasswordCurrentPassword(''); setNewPassword(''); navigate('/login', { replace: true, state: { notice: '密码已修改，请使用新密码重新登录。' } }) })}>{pendingAction === 'password' ? '修改中…' : '修改并撤销旧会话'}</PixelButton></PixelCard>
    </div>
  </PageContainer>
}
