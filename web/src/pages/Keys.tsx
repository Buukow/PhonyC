import { FormEvent, useEffect, useState } from 'react'
import { Eye, EyeOff, Copy } from 'lucide-react'
import { api } from '@/lib/api'
import { copyText } from '@/lib/clipboard'
import { Badge, Button, Card, Input, Label, PageHeader, Select, Table, Textarea } from '@/components/ui'

type Key = {
  id: number
  name: string
  key: string
  enabled: boolean
  remark: string
  impersonation_mode: string
  preset_id?: number | null
  custom_headers_json: string
}

type KeyForm = {
  name: string
  key: string
  remark: string
  impersonation_mode: string
  preset_id: string
  custom_headers_json: string
}

const emptyForm = (): KeyForm => ({
  name: '',
  key: '',
  remark: '',
  impersonation_mode: '透传',
  preset_id: '',
  custom_headers_json: '{}',
})

export default function Keys() {
  const [items, setItems] = useState<Key[]>([])
  const [presets, setPresets] = useState<any[]>([])
  const [creating, setCreating] = useState(false)
  const [editing, setEditing] = useState<Key | null>(null)
  const [reveal, setReveal] = useState<Record<number, boolean>>({})
  const [form, setForm] = useState<KeyForm>(emptyForm())
  const [err, setErr] = useState('')
  const [copyMsg, setCopyMsg] = useState('')

  async function load() {
    const [k, p] = await Promise.all([
      api<{ items: Key[] }>('/api/keys'),
      api<{ items: any[] }>('/api/presets'),
    ])
    setItems(k.items || [])
    setPresets(p.items || [])
  }
  useEffect(() => { load().catch(console.error) }, [])

  function startCreate() {
    setEditing(null)
    setCreating(true)
    setForm(emptyForm())
    setErr('')
  }

  function startEdit(k: Key) {
    setCreating(false)
    setEditing(k)
    setForm({
      name: k.name,
      key: '',
      remark: k.remark || '',
      impersonation_mode: k.impersonation_mode || '透传',
      preset_id: k.preset_id ? String(k.preset_id) : '',
      custom_headers_json: k.custom_headers_json || '{}',
    })
    setErr('')
  }

  function cancelForm() {
    setCreating(false)
    setEditing(null)
    setForm(emptyForm())
    setErr('')
  }

  async function onSave(e: FormEvent) {
    e.preventDefault()
    setErr('')
    const body: Record<string, unknown> = {
      name: form.name,
      remark: form.remark,
      impersonation_mode: form.impersonation_mode,
      custom_headers_json: form.custom_headers_json || '{}',
    }
    if (form.key.trim()) {
      body.key = form.key.trim()
    }
    if (form.impersonation_mode === '预设') {
      if (form.preset_id) {
        body.preset_id = Number(form.preset_id)
      }
    } else {
      body.clear_preset = true
    }
    try {
      if (editing) {
        await api(`/api/keys/${editing.id}`, { method: 'PATCH', body: JSON.stringify(body) })
      } else {
        await api('/api/keys', {
          method: 'POST',
          body: JSON.stringify({ ...body, enabled: true, key: form.key || undefined }),
        })
      }
      cancelForm()
      await load()
    } catch (ex: any) {
      setErr(ex.message || '保存失败')
    }
  }

  async function toggle(k: Key) {
    await api(`/api/keys/${k.id}`, { method: 'PATCH', body: JSON.stringify({ enabled: !k.enabled }) })
    await load()
  }

  async function remove(id: number) {
    if (!confirm('确认删除该 Key？')) return
    await api(`/api/keys/${id}`, { method: 'DELETE' })
    await load()
  }

  const formOpen = creating || !!editing

  return (
    <div>
      <PageHeader
        title="用户 Key"
        subtitle="分发客户端凭证并绑定伪装策略"
        actions={formOpen ? undefined : <Button onClick={startCreate}>新建 Key</Button>}
      />
      {copyMsg && <div className="mb-4 text-sm text-primary">{copyMsg}</div>}
      {err && <div className="mb-4 text-sm text-warn">{err}</div>}
      {formOpen && (
        <Card className="p-6 mb-6">
          <div className="text-base font-semibold mb-4">{editing ? `编辑 Key · ${editing.name}` : '新建 Key'}</div>
          <form className="grid grid-cols-1 md:grid-cols-2 gap-4" onSubmit={onSave}>
            <div><Label>名称</Label><Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></div>
            <div>
              <Label>Key{editing ? '（留空则不修改）' : '（留空自动生成）'}</Label>
              <Input
                value={form.key}
                onChange={(e) => setForm({ ...form, key: e.target.value })}
                placeholder={editing ? '•••• 留空保持原 Key' : 'sk-...'}
              />
            </div>
            <div>
              <Label>伪装模式</Label>
              <Select value={form.impersonation_mode} onChange={(e) => setForm({ ...form, impersonation_mode: e.target.value })}>
                <option value="透传">透传</option>
                <option value="预设">预设</option>
                <option value="自定义">自定义</option>
              </Select>
            </div>
            <div>
              <Label>预设</Label>
              <Select value={form.preset_id} onChange={(e) => setForm({ ...form, preset_id: e.target.value })} disabled={form.impersonation_mode !== '预设'}>
                <option value="">选择预设</option>
                {presets.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
              </Select>
            </div>
            <div className="md:col-span-2"><Label>备注</Label><Input value={form.remark} onChange={(e) => setForm({ ...form, remark: e.target.value })} /></div>
            {form.impersonation_mode === '自定义' && (
              <div className="md:col-span-2"><Label>自定义 Header JSON</Label><Textarea value={form.custom_headers_json} onChange={(e) => setForm({ ...form, custom_headers_json: e.target.value })} /></div>
            )}
            <div className="md:col-span-2 flex gap-2">
              <Button type="submit">保存</Button>
              <Button type="button" variant="secondary" onClick={cancelForm}>取消</Button>
            </div>
          </form>
        </Card>
      )}
      <Card className="p-2">
        <Table headers={['名称', 'Key', '伪装', '状态', '备注', '操作']}>
          {items.map((k) => (
            <tr key={k.id} className="border-b border-gray-50 hover:bg-canvas">
              <td className="px-4 py-3 font-medium">{k.name}</td>
              <td className="px-4 py-3">
                <div className="flex items-center gap-2 font-mono text-xs">
                  <span>{reveal[k.id] ? k.key : k.key.slice(0, 6) + '••••' + k.key.slice(-4)}</span>
                  <button type="button" onClick={() => setReveal((r) => ({ ...r, [k.id]: !r[k.id] }))} className="text-gray-400 hover:text-primary">
                    {reveal[k.id] ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                  </button>
                  <button
                    type="button"
                    onClick={async () => {
                      try {
                        await copyText(k.key)
                        setCopyMsg(`已复制 ${k.name} 的 Key`)
                        setTimeout(() => setCopyMsg(''), 2000)
                      } catch (ex: any) {
                        setErr(ex.message || '复制失败')
                      }
                    }}
                    className="text-gray-400 hover:text-primary"
                    title="复制"
                  >
                    <Copy className="w-4 h-4" />
                  </button>
                </div>
              </td>
              <td className="px-4 py-3"><Badge tone="muted">{k.impersonation_mode}</Badge></td>
              <td className="px-4 py-3">{k.enabled ? <Badge>启用</Badge> : <Badge tone="warn">停用</Badge>}</td>
              <td className="px-4 py-3 text-gray-500 max-w-xs truncate">{k.remark || '—'}</td>
              <td className="px-4 py-3 space-x-2 whitespace-nowrap">
                <Button variant="ghost" onClick={() => startEdit(k)}>编辑</Button>
                <Button variant="ghost" onClick={() => toggle(k)}>{k.enabled ? '停用' : '启用'}</Button>
                <Button variant="danger" onClick={() => remove(k.id)}>删除</Button>
              </td>
            </tr>
          ))}
        </Table>
      </Card>
    </div>
  )
}
