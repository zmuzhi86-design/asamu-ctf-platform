import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { PageContainer, PixelButton, PixelCard, RobotTip } from '../components/ui/System'
import { useAuth } from '../contexts/AuthProvider'
import { ApiError, apiRequest } from '../services/apiClient'

export function TeamInvitationPage() {
  const { id = '' } = useParams()
  const auth = useAuth()
  const [pending, setPending] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const accept = async () => { setPending(true); setError(''); try { await apiRequest<void>(`/team-invitations/${id}/accept`, { method: 'POST' }); setMessage('邀请已接受，你已加入战队。') } catch (reason) { setError(reason instanceof ApiError ? reason.message : '暂时无法接受邀请。') } finally { setPending(false) } }
  return <PageContainer className="py-12"><PixelCard className="mx-auto max-w-lg" title="战队邀请">
    {!auth.user ? <><p className="mb-5 font-semibold text-asamu-muted">请先登录收到邀请的账号，再返回此链接接受邀请。</p><Link to="/login" state={{ from: `/team-invitations/${id}` }}><PixelButton>登录后继续</PixelButton></Link></> : <><p className="mb-5 font-semibold text-asamu-muted">该邀请仅对指定账号有效，接受前系统会再次检查战队容量和你的当前战队状态。</p><PixelButton disabled={pending || !id || !!message} onClick={accept}>{pending ? '处理中…' : '接受邀请'}</PixelButton></>}
    {message && <RobotTip className="mt-5" title="加入成功">{message}</RobotTip>}{error && <p className="mt-5 border-2 border-red-400 bg-red-50 p-3 text-sm font-black text-red-700">{error}</p>}
  </PixelCard></PageContainer>
}
