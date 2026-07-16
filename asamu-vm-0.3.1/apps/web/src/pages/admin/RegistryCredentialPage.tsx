import { useEffect, useState } from 'react'
import { Modal, PixelButton, PixelCard, PixelInput, StatusBadge } from '../../components/ui/System'
import { registryCredentialApi, type RegistryCredential } from '../../services/registryCredentialApi'

type Form = { name: string; registryHost: string; username: string; token: string; enabled: boolean; reason: string }
const emptyForm = (): Form => ({ name: '', registryHost: '', username: '', token: '', enabled: true, reason: '' })

export function RegistryCredentialPage() {
  const [items, setItems] = useState<RegistryCredential[]>([])
  const [editing, setEditing] = useState<RegistryCredential>()
  const [form, setForm] = useState<Form>(emptyForm())
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const reload = () => registryCredentialApi.list().then(setItems).catch((reason: Error) => setError(reason.message))
  useEffect(() => { void reload() }, [])

  function create() { setEditing(undefined); setForm(emptyForm()); setError(''); setMessage(''); setOpen(true) }
  function edit(item: RegistryCredential) {
    setEditing(item); setForm({ name: item.name, registryHost: item.registryHost, username: item.username, token: '', enabled: item.enabled, reason: '' }); setError(''); setMessage(''); setOpen(true)
  }
  async function save() {
    setBusy(true); setError(''); setMessage('')
    try {
      if (editing) {
        await registryCredentialApi.update(editing, { name: form.name, username: form.username, token: form.token || undefined, enabled: form.enabled, reason: form.reason })
        setMessage('凭据已更新；留空的 token 未发生变化。')
      } else {
        await registryCredentialApi.create({ name: form.name, registryHost: form.registryHost, username: form.username, token: form.token })
        setMessage('凭据已加密保存。')
      }
      setForm((value) => ({ ...value, token: '' })); setOpen(false); await reload()
    } catch (reason) { setError(reason instanceof Error ? reason.message : '保存失败') } finally { setBusy(false) }
  }

  const valid = form.name.trim() && form.username.trim() && (editing ? form.reason.trim().length >= 4 : form.registryHost.trim() && form.token.length >= 8)
  return <>
    <header className="mb-6 flex items-end justify-between border-b-2 border-asamu-ink/10 pb-5"><div><p className="text-xs font-black tracking-[.18em] text-asamu-blue">PRIVATE REGISTRY</p><h1 className="mt-2 font-display text-3xl font-black">镜像仓库凭据</h1><p className="mt-2 text-sm font-semibold text-asamu-muted">Token 使用独立 AES-GCM 密钥加密，页面和管理 API 永不回显明文。</p></div><PixelButton onClick={create}>添加凭据</PixelButton></header>
    {error && <div className="mb-4 border-2 border-red-300 bg-red-50 p-3 text-sm font-black text-red-700">{error}</div>}{message && <div className="mb-4 border-2 border-emerald-300 bg-emerald-50 p-3 text-sm font-black text-emerald-700">{message}</div>}
    <PixelCard><div className="grid gap-3 lg:grid-cols-2 xl:grid-cols-3">{items.map((item) => <article key={item.id} className="border-2 border-asamu-line p-4"><div className="flex justify-between gap-3"><div><b>{item.name}</b><p className="mt-1 break-all text-xs font-bold text-asamu-muted">{item.registryHost}</p></div><StatusBadge tone={item.enabled ? 'green' : 'yellow'}>{item.enabled ? '启用' : '停用'}</StatusBadge></div><dl className="mt-4 grid grid-cols-2 gap-2 text-xs"><dt className="font-black text-asamu-muted">用户名</dt><dd className="text-right font-bold">{item.username}</dd><dt className="font-black text-asamu-muted">Token</dt><dd className="text-right font-bold">{item.tokenConfigured ? '已配置（不可查看）' : '未配置'}</dd><dt className="font-black text-asamu-muted">最近使用</dt><dd className="text-right font-bold">{item.lastUsedAt ? new Date(item.lastUsedAt).toLocaleString() : '—'}</dd></dl><PixelButton className="mt-4" size="sm" variant="secondary" onClick={() => edit(item)}>编辑 / 轮换</PixelButton></article>)}</div>{items.length === 0 && <p className="py-10 text-center text-sm font-bold text-asamu-muted">尚未配置私有镜像仓库。</p>}</PixelCard>
    <Modal open={open} title={editing ? '编辑或轮换凭据' : '添加镜像仓库凭据'} onClose={() => !busy && setOpen(false)}><div className="grid gap-4"><Field label="显示名称" value={form.name} onChange={(name) => setForm({ ...form, name })} /><Field label="仓库地址（仅主机名，可含端口）" value={form.registryHost} disabled={Boolean(editing)} onChange={(registryHost) => setForm({ ...form, registryHost })} /><Field label="用户名" value={form.username} onChange={(username) => setForm({ ...form, username })} /><Field label={editing ? '新 Token（留空则不轮换）' : 'Token / 密码'} value={form.token} type="password" autoComplete="new-password" onChange={(token) => setForm({ ...form, token })} />{editing && <><label className="flex items-center gap-2 text-sm font-black"><input type="checkbox" checked={form.enabled} onChange={(event) => setForm({ ...form, enabled: event.target.checked })} />启用凭据</label><Field label="变更原因（至少 4 个字符）" value={form.reason} onChange={(reason) => setForm({ ...form, reason })} /></>}<p className="text-xs font-bold text-asamu-muted">保存后无法查看 token。Worker 仅能为归属自己的实例申请短时使用租约，每次申请都会写入审计日志。</p><div className="flex justify-end gap-2"><PixelButton variant="secondary" onClick={() => setOpen(false)}>取消</PixelButton><PixelButton disabled={busy || !valid} onClick={() => void save()}>{busy ? '保存中…' : '保存'}</PixelButton></div></div></Modal>
  </>
}

function Field({ label, value, onChange, type = 'text', disabled = false, autoComplete }: { label: string; value: string; onChange: (value: string) => void; type?: string; disabled?: boolean; autoComplete?: string }) {
  return <label className="text-sm font-black">{label}<PixelInput className="mt-2" type={type} value={value} disabled={disabled} autoComplete={autoComplete} onChange={(event) => onChange(event.target.value)} /></label>
}
