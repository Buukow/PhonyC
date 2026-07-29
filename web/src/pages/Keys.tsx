import { FormEvent, useEffect, useState } from 'react'
import { Eye, EyeOff, Copy } from 'lucide-react'
import { api } from '@/lib/api'
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

export default function Keys() {
  const [items, setItems] = useState<Key[]>([])
  const [presets, setPresets] = useState<any[]>([])
  const [show, setShow] = useState(false)
  const [reveal, setReveal] = useState<Record<number, boolean>>({})
  const [form, setForm] = useState({
    name: '', key: '', remark: '', impersonation_mode: 'passthrough', preset_id: '', custom_headers_json: '{}',
  })

  async function load() {
    const [k, p] = await Promise.all([
      api<{ items: Key[] }>('/api/keys'),
      api<{ items: any[] }>('/api/presets'),
    ])
    setItems(k.items || [])
    setPresets(p.items || [])
  }
  useEffect(() => { load().catch(console.error) }, [])

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    const body: any = {
      name: form.name,
      key: form.key || undefined,
      remark: form.remark,
      impersonation_mode: form.impersonation_mode,
      custom_headers_json: form.custom_headers_json,
      enabled: true,
    }
    if (form.impersonation_mode === 'preset' && form.preset_id) {
      body.preset_id = Number(form.preset_id)
    }
    await api('/api/keys', { method: 'POST', body: JSON.stringify(body) })
    setShow(false)
    setForm({ name: '', key: '', remark: '', impersonation_mode: 'passthrough', preset_id: '', custom_headers_json: '{}' })
    await load()
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

  return (
    <div>
      <PageHeader title="用户 Key" subtitle="分发客户端凭证并绑定伪装策略" actions={<Button onClick={() => setShow((v) => !v)}>新建 Key</Button>} />
      {show && (
        <Card className="p-6 mb-6">
          <form className="grid grid-cols-1 md:grid-cols-2 gap-4" onSubmit={onCreate}>
            <div><Label>名称</Label><Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></div>
            <div><Label>Key（留空自动生成）</Label><Input value={form.key} onChange={(e) => setForm({ ...form, key: e.target.value })} placeholder="sk-..." /></div>
            <div>
              <Label>伪装模式</Label>
              <Select value={form.impersonation_mode} onChange={(e) => setForm({ ...form, impersonation_mode: e.target.value })}>
                <option value="passthrough">passthrough 透传</option>
                <option value="preset">preset 预设</option>
                <option value="custom">custom 自定义</option>
              </Select>
            </div>
            <div>
              <Label>预设</Label>
              <Select value={form.preset_id} onChange={(e) => setForm({ ...form, preset_id: e.target.value })} disabled={form.impersonation_mode !== 'preset'}>
                <option value="">选择预设</option>
                {presets.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
              </Select>
            </div>
            <div className="md:col-span-2"><Label>备注</Label><Input value={form.remark} onChange={(e) => setForm({ ...form, remark: e.target.value })} /></div>
            <div className="md:col-span-2"><Label>自定义 Header JSON</Label><Textarea value={form.custom_headers_json} onChange={(e) => setForm({ ...form, custom_headers_json: e.target.value })} disabled={form.impersonation_mode !== 'custom'} /></div>
            <div className="md:col-span-2 flex gap-2"><Button type="submit">保存</Button><Button type="button" variant="secondary" onClick={() => setShow(false)}>取消</Button></div>
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
                  <button onClick={() => setReveal((r) => ({ ...r, [k.id]: !r[k.id] }))} className="text-gray-400 hover:text-primary">
                    {reveal[k.id] ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                  </button>
                  <button onClick={() => navigator.clipboard.writeText(k.key)} className="text-gray-400 hover:text-primary"><Copy className="w-4 h-4" /></button>
                </div>
              </td>
              <td className="px-4 py-3"><Badge tone="accent">{k.impersonation_mode}</Badge></td>
              <td className="px-4 py-3">{k.enabled ? <Badge>启用</Badge> : <Badge tone="warn">停用</Badge>}</td>
              <td className="px-4 py-3 text-gray-500">{k.remark || '—'}</td>
              <td className="px-4 py-3 space-x-2">
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
