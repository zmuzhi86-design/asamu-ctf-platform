import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { AssetImage, PageContainer, PageHeader, PixelButton, PixelCard, PixelInput, PixelTag, RobotTip } from '../components/ui/System'
import { useAuth } from '../contexts/AuthProvider'
import { useAssetSystem } from '../contexts/AssetProvider'
import { ApiError } from '../services/apiClient'
import { createTeam, fetchMyTeam, inviteTeamMember, leaveTeam, postTeamAnnouncement, removeTeamMember, reviewTeamJoin, transferTeamCaptain, updateTeam, uploadTeamAvatar, type TeamManagementDto, type TeamMutation } from '../services/communityApi'

const emptyTeam: TeamMutation = { name: '', slogan: '', description: '', flagAssetKey: '', bannerAssetKey: '', memberLimit: 30, recruiting: true }

export function TeamManagerPage() {
  const auth = useAuth()
  const { refreshAssets } = useAssetSystem()
  const [data, setData] = useState<TeamManagementDto | null>(null)
  const [form, setForm] = useState<TeamMutation>(emptyTeam)
  const [username, setUsername] = useState('')
  const [announcement, setAnnouncement] = useState({ title: '', content: '', pinned: false })
  const [loading, setLoading] = useState(true)
  const [avatarUploading, setAvatarUploading] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const load = useCallback(() => { if (!auth.user) return; setLoading(true); fetchMyTeam().then((value) => { setData(value); setForm({ name: value.team.name, slogan: value.team.slogan, description: value.team.description, flagAssetKey: value.team.flagAssetKey, bannerAssetKey: value.team.bannerAssetKey, memberLimit: value.team.memberLimit, recruiting: value.team.recruiting }) }).catch((reason: ApiError) => { if (reason.code === 'TEAM_MEMBERSHIP_NOT_FOUND') setData(null); else setError(reason.message) }).finally(() => setLoading(false)) }, [auth.user])
  useEffect(load, [load])
  const run = async (action: () => Promise<unknown>, success: string) => { setError(''); setMessage(''); try { await action(); setMessage(success); await load() } catch (reason) { setError(reason instanceof ApiError ? reason.message : '操作失败') } }
  const uploadAvatar = async (file?: File) => {
    if (!file || !data || data.myRole !== 'captain') return
    if (file.size > 5 * 1024 * 1024) { setError('战队头像不能超过 5 MB。'); return }
    if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) { setError('战队头像仅支持 PNG、JPEG 或 WebP。'); return }
    setAvatarUploading(true); setError(''); setMessage('')
    try {
      const team = await uploadTeamAvatar(data.team.id, file)
      await refreshAssets()
      setData({ ...data, team })
      setForm({ ...form, flagAssetKey: team.flagAssetKey })
      setMessage('战队头像已更新。')
    } catch (reason) {
      setError(reason instanceof ApiError ? reason.message : '战队头像上传失败')
    } finally {
      setAvatarUploading(false)
    }
  }
  if (!auth.user) return <PageContainer><PixelCard className="mx-auto max-w-lg" title="登录后管理战队"><Link to="/login" state={{ from: '/team/manage' }}><PixelButton>前往登录</PixelButton></Link></PixelCard></PageContainer>
  if (loading) return <PageContainer><PixelCard><p>正在读取战队状态…</p></PixelCard></PageContainer>
  if (!data) return <PageContainer><PageHeader eyebrow="TEAM WORKSPACE" title="创建战队" description="建立长期训练阵容，创建成功后你将成为队长。" />{error && <p className="mb-4 border-2 border-red-400 bg-red-50 p-3 text-sm font-black text-red-700">{error}</p>}<PixelCard className="mx-auto max-w-2xl" title="战队资料"><TeamForm value={form} onChange={setForm} /><PixelButton className="mt-5" disabled={!form.name} onClick={() => run(() => createTeam(form), '战队已创建。')}>创建战队</PixelButton></PixelCard></PageContainer>
  const canManage = data.myRole === 'captain' || data.myRole === 'manager'
  return <PageContainer><PageHeader eyebrow="TEAM WORKSPACE" title={`${data.team.name} 工作台`} description="处理招募、成员、公告和战队公开资料。"><div className="flex gap-2"><PixelTag tone="yellow">{data.myRole}</PixelTag><Link to={`/teams/${data.team.slug || data.team.id}`}><PixelButton variant="secondary">公开主页</PixelButton></Link></div></PageHeader>
    {message && <RobotTip className="mb-5" title="操作完成">{message}</RobotTip>}{error && <p className="mb-5 border-2 border-red-400 bg-red-50 p-3 text-sm font-black text-red-700">{error}</p>}
    <div className="grid gap-6 xl:grid-cols-2">{canManage && <PixelCard title="战队资料"><TeamForm value={form} onChange={setForm} /><PixelButton className="mt-5" onClick={() => run(() => updateTeam(data.team.id, form), '战队资料已更新。')}>保存资料</PixelButton></PixelCard>}
      {data.myRole === 'captain' && <PixelCard title="自定义战队头像"><div className="flex items-center gap-5"><div className="grid h-28 w-28 shrink-0 place-items-center border-2 border-asamu-ink bg-asamu-soft"><AssetImage className="h-24 w-24" assetKey={data.team.flagAssetKey || 'team.honor.verified'} alt="当前战队头像" /></div><div><p className="text-sm font-semibold leading-6 text-asamu-muted">仅队长可以上传。支持 PNG、JPEG、WebP，最大 5 MB，建议使用正方形图片。</p><label className="pixel-button pixel-button-primary pixel-button-md mt-4 cursor-pointer">{avatarUploading ? '上传中…' : '选择并上传图片'}<input className="hidden" type="file" accept="image/png,image/jpeg,image/webp" disabled={avatarUploading} onChange={(event) => { void uploadAvatar(event.target.files?.[0]); event.currentTarget.value = '' }} /></label></div></div></PixelCard>}
      {canManage && <PixelCard title="邀请成员"><label className="block text-sm font-black">用户名<PixelInput className="mt-2 w-full" value={username} onChange={(event) => setUsername(event.target.value)} /></label><PixelButton className="mt-4" disabled={!username} onClick={() => run(() => inviteTeamMember(data.team.id, username), '邀请邮件和站内通知已发送。')}>发送邀请</PixelButton><div className="mt-5 space-y-2">{data.invitations.map((item) => <p className="border-t border-asamu-line pt-2 text-sm" key={item.id}><b>{item.username}</b> · {new Date(item.expiresAt).toLocaleDateString('zh-CN')} 到期</p>)}</div></PixelCard>}
      {canManage && <PixelCard title={`待审申请 (${data.joinRequests.length})`}><div className="space-y-3">{data.joinRequests.map((item) => <div className="border-b border-asamu-line pb-3" key={item.id}><b>{item.username}</b><p className="my-2 text-sm text-asamu-muted">{item.message || '未填写申请说明'}</p><div className="flex gap-2"><PixelButton size="sm" onClick={() => run(() => reviewTeamJoin(data.team.id, item.id, true), '申请已通过。')}>通过</PixelButton><PixelButton size="sm" variant="danger" onClick={() => run(() => reviewTeamJoin(data.team.id, item.id, false), '申请已拒绝。')}>拒绝</PixelButton></div></div>)}</div></PixelCard>}
      {canManage && <PixelCard title="发布公告"><label className="block text-sm font-black">标题<PixelInput className="mt-2 w-full" value={announcement.title} onChange={(event) => setAnnouncement({ ...announcement, title: event.target.value })} /></label><label className="mt-4 block text-sm font-black">正文<textarea className="pixel-input mt-2 min-h-28 w-full" value={announcement.content} onChange={(event) => setAnnouncement({ ...announcement, content: event.target.value })} /></label><label className="mt-3 flex gap-2 text-sm font-black"><input type="checkbox" checked={announcement.pinned} onChange={(event) => setAnnouncement({ ...announcement, pinned: event.target.checked })} />置顶</label><PixelButton className="mt-4" disabled={!announcement.title || !announcement.content} onClick={() => run(() => postTeamAnnouncement(data.team.id, announcement.title, announcement.content, announcement.pinned), '公告已发布。')}>发布公告</PixelButton></PixelCard>}
      <PixelCard title="成员管理"><div className="space-y-3">{data.team.members?.map((member) => <div className="flex flex-wrap items-center justify-between gap-2 border-b border-asamu-line pb-3" key={member.userId}><div><b>{member.username}</b><p className="text-xs text-asamu-muted">{member.role}</p></div>{canManage && member.role !== 'captain' && <div className="flex gap-2">{data.myRole === 'captain' && <PixelButton size="sm" variant="yellow" onClick={() => run(() => transferTeamCaptain(data.team.id, member.userId), '队长已转让。')}>转让队长</PixelButton>}<PixelButton size="sm" variant="danger" onClick={() => run(() => removeTeamMember(data.team.id, member.userId), '成员已移除。')}>移除</PixelButton></div>}</div>)}</div>{data.myRole !== 'captain' && <PixelButton className="mt-5" variant="danger" onClick={() => run(() => leaveTeam(data.team.id), '你已退出战队。')}>退出战队</PixelButton>}</PixelCard>
    </div>
  </PageContainer>
}

function TeamForm({ value, onChange }: { value: TeamMutation; onChange: (value: TeamMutation) => void }) {
  return <div className="space-y-4"><label className="block text-sm font-black">名称<PixelInput className="mt-2 w-full" maxLength={80} value={value.name} onChange={(event) => onChange({ ...value, name: event.target.value })} /></label><label className="block text-sm font-black">口号<PixelInput className="mt-2 w-full" maxLength={240} value={value.slogan} onChange={(event) => onChange({ ...value, slogan: event.target.value })} /></label><label className="block text-sm font-black">介绍<textarea className="pixel-input mt-2 min-h-28 w-full" maxLength={5000} value={value.description} onChange={(event) => onChange({ ...value, description: event.target.value })} /></label><label className="block text-sm font-black">人数上限<PixelInput className="mt-2 w-full" type="number" min={2} max={100} value={value.memberLimit} onChange={(event) => onChange({ ...value, memberLimit: Number(event.target.value) })} /></label><label className="flex gap-2 text-sm font-black"><input type="checkbox" checked={value.recruiting ?? true} onChange={(event) => onChange({ ...value, recruiting: event.target.checked })} />开放招募</label></div>
}
