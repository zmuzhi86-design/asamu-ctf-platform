import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { AssetImage, PageContainer, PageHeader, PixelButton, PixelCard, PixelInput, RobotTip } from '../components/ui/System'
import { useAuth } from '../contexts/AuthProvider'
import { ApiError } from '../services/apiClient'
import { fetchMyProfile, updateMyProfile, type ProfileMutation } from '../services/communityApi'

const empty: ProfileMutation = { displayName: '', bio: '', organizationName: '', avatarAssetKey: '', characterAssetKey: '', signature: '', skills: [], privacy: { showOrganization: true, showSkills: true, showRecentSolves: true, showCompetitionHistory: true } }
const avatarOptions = [
  ['character.student.male.default', '男性学员'],
  ['character.student.male.presenter', '男性讲解员'],
  ['character.student.female.default', '女性学员'],
  ['character.student.female.analyst', '女性分析员'],
  ['mascot.default', '机器人助手'],
] as const

export function ProfileEditPage() {
  const auth = useAuth()
  const [form, setForm] = useState<ProfileMutation>(empty)
  const [skills, setSkills] = useState('')
  const [pending, setPending] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  useEffect(() => { if (!auth.user) return; fetchMyProfile().then((profile) => { const privacy = { ...empty.privacy, ...profile.privacy }; setForm({ displayName: profile.displayName, bio: profile.bio, organizationName: profile.organizationName, avatarAssetKey: profile.avatarAssetKey, characterAssetKey: profile.characterAssetKey, signature: profile.signature, skills: profile.skills, privacy }); setSkills(profile.skills.join(', ')) }).catch((reason: Error) => setError(reason.message)) }, [auth.user])
  const save = async () => { setPending(true); setError(''); setMessage(''); try { const input = { ...form, skills: skills.split(',').map((item) => item.trim()).filter(Boolean) }; await updateMyProfile(input); setMessage('个人资料和公开隐私设置已保存。') } catch (reason) { setError(reason instanceof ApiError ? reason.message : '保存失败') } finally { setPending(false) } }
  if (!auth.user) return <PageContainer><PixelCard className="mx-auto max-w-lg" title="登录后编辑资料"><Link to="/login" state={{ from: '/profile/edit' }}><PixelButton>前往登录</PixelButton></Link></PixelCard></PageContainer>
  return <PageContainer><PageHeader eyebrow="PROFILE SETTINGS" title="编辑个人资料" description="维护公开身份、擅长方向和展示范围。"><div className="flex gap-2"><Link to="/profile"><PixelButton variant="secondary">返回档案</PixelButton></Link><Link to="/account/security"><PixelButton variant="yellow">账号安全</PixelButton></Link></div></PageHeader>
    {message && <RobotTip className="mb-5" title="保存成功">{message}</RobotTip>}{error && <p className="mb-5 border-2 border-red-400 bg-red-50 p-3 text-sm font-black text-red-700">{error}</p>}
    <div className="grid gap-6 lg:grid-cols-2"><PixelCard title="公开资料"><div className="space-y-4"><label className="block text-sm font-black">显示名称<PixelInput className="mt-2 w-full" maxLength={80} value={form.displayName} onChange={(event) => setForm({ ...form, displayName: event.target.value })} /></label><label className="block text-sm font-black">组织<PixelInput className="mt-2 w-full" maxLength={160} value={form.organizationName} onChange={(event) => setForm({ ...form, organizationName: event.target.value })} /></label><label className="block text-sm font-black">个人签名<PixelInput className="mt-2 w-full" maxLength={500} value={form.signature} onChange={(event) => setForm({ ...form, signature: event.target.value })} /></label><label className="block text-sm font-black">简介<textarea className="pixel-input mt-2 min-h-32 w-full" maxLength={2000} value={form.bio} onChange={(event) => setForm({ ...form, bio: event.target.value })} /></label><label className="block text-sm font-black">技能（逗号分隔，最多 20 项）<PixelInput className="mt-2 w-full" value={skills} onChange={(event) => setSkills(event.target.value)} /></label><fieldset><legend className="text-sm font-black">自定义头像</legend><div className="mt-2 grid grid-cols-2 gap-3 sm:grid-cols-3">{avatarOptions.map(([key, label]) => <button type="button" key={key} aria-pressed={form.avatarAssetKey === key} className={`border-2 p-2 text-center text-xs font-black ${form.avatarAssetKey === key ? 'border-asamu-blue bg-asamu-soft text-asamu-blue' : 'border-asamu-line bg-asamu-card'}`} onClick={() => setForm({ ...form, avatarAssetKey: key })}><AssetImage className="mx-auto h-20 w-20" assetKey={key} alt={label} /><span className="mt-2 block">{label}</span></button>)}</div></fieldset><label className="block text-sm font-black">自定义头像素材 Key（可选）<PixelInput className="mt-2 w-full" maxLength={160} value={form.avatarAssetKey} onChange={(event) => setForm({ ...form, avatarAssetKey: event.target.value })} /><span className="mt-1 block text-xs font-semibold text-asamu-muted">可填写管理员已上传的素材 Key；留空时使用默认头像。</span></label></div></PixelCard>
      <PixelCard title="隐私与展示"><div className="space-y-4">{([['showOrganization', '公开组织信息'], ['showSkills', '公开技能方向'], ['showRecentSolves', '公开最近解题'], ['showCompetitionHistory', '公开比赛记录']] as const).map(([key, label]) => <label className="flex items-center gap-3 border-b border-asamu-line pb-3 font-black" key={key}><input type="checkbox" checked={form.privacy[key] !== false} onChange={(event) => setForm({ ...form, privacy: { ...form.privacy, [key]: event.target.checked } })} />{label}</label>)}</div><p className="mt-5 text-sm leading-6 text-asamu-muted">邮箱、收藏和安全状态始终只在本人接口返回，不受公开开关影响。</p></PixelCard></div><PixelButton className="mt-6" disabled={pending} onClick={save}>{pending ? '保存中…' : '保存全部设置'}</PixelButton>
  </PageContainer>
}
