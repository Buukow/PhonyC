import { FormEvent, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Plus } from 'lucide-react'
import { api } from '@/lib/api'
import { Badge, Button, Card, Input, Label, PageHeader, Select, Table, Textarea } from '@/components/ui'

type Channel = {
  id: number
  name: string
  enabled: boolean
  temp_disabled?: boolean
  protocol: string
  base_url: string
  api_key: string
  priority: number
  timeout_ms: number
  extra_headers_json?: string
  last_test_status?: number
  last_test_ms?: number
  last_test_error?: string
}

type ChannelForm = {
  name: string
  protocol: string
  base_url: string
  api_key: string
  priority: number
  timeout_ms: number
  extra_headers_json: string
}

const emptyForm = (): ChannelForm => ({
  name: '',
  protocol: 'openai',
  base_url: '',
  api_key: '',
  priority: 10,
  timeout_ms: 600000,
  extra_headers_json: '{}',
})

export default function Channels() {
  const [items, setItems] = useState<Channel[]>([])
  const [creating, setCreating] = useState(false)
  const [editing, setEditing] = useState<Channel | null>(null)
  const [form, setForm] = useState<ChannelForm>(emptyForm())
  const [err, setErr] = useState('')

  async function load() {
    const res = await api<{ items: Channel[] }>('/api/channels')
    setItems(res.items || [])
  }
  useEffect(() => { load().catch(console.error) }, [])

  function startCreate() {
    setEditing(null)
    setCreating(true)
    setForm(emptyForm())
    setErr('')
  }

  function startEdit(ch: Channel) {
    setCreating(false)
    setEditing(ch)
    setForm({
      name: ch.name,
      protocol: ch.protocol,
      base_url: ch.base_url,
      api_key: '',
      priority: ch.priority,
      timeout_ms: ch.timeout_ms,
      extra_headers_json: ch.extra_headers_json || '{}',
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
    const payload: Record<string, unknown> = {
      name: form.name,
      protocol: form.protocol,
      base_url: form.base_url,
      priority: Number(form.priority),
      timeout_ms: Number(form.timeout_ms),
      extra_headers_json: form.extra_headers_json || '{}',
    }
    if (form.api_key.trim()) {
      payload.api_key = form.api_key
    }
    try {
      if (editing) {
        await api(`/api/channels/${editing.id}`, {
          method: 'PATCH',
          body: JSON.stringify(payload),
        })
      } else {
        await api('/api/channels', {
          method: 'POST',
          body: JSON.stringify({ ...payload, api_key: form.api_key, enabled: true }),
        })
      }
      cancelForm()
      await load()
    } catch (ex: any) {
      setErr(ex.message || '保存失败')
    }
  }

  async function toggle(ch: Channel) {
    await api(`/api/channels/${ch.id}`, { method: 'PATCH', body: JSON.stringify({ enabled: !ch.enabled }) })
    await load()
  }

  async function remove(id: number) {
    if (!confirm('确认删除该渠道？')) return
    await api(`/api/channels/${id}`, { method: 'DELETE' })
    await load()
  }

  const formOpen = creating || !!editing

  return (
    <div>
      <PageHeader
        title="渠道"
        subtitle="配置上游协议、Base URL 与优先级（0 最低，越大越优先；同优先级随机）"
        actions={<Button onClick={startCreate}><Plus className="w-4 h-4" />新建渠道</Button>}
      />
      {err && <div className="mb-4 text-sm text-warn">{err}</div>}
      {formOpen && (
        <Card className="p-6 mb-6">
          <div className="text-base font-semibold mb-4">{editing ? `编辑渠道 · ${editing.name}` : '新建渠道'}</div>
          <form className="grid grid-cols-1 md:grid-cols-2 gap-4" onSubmit={onSave}>
            <div><Label>名称</Label><Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></div>
            <div>
              <Label>协议</Label>
              <Select value={form.protocol} onChange={(e) => setForm({ ...form, protocol: e.target.value })}>
                <option value="openai">openai</option>
                <option value="anthropic">anthropic</option>
              </Select>
            </div>
            <div className="md:col-span-2"><Label>Base URL</Label><Input placeholder="https://api.example.com" value={form.base_url} onChange={(e) => setForm({ ...form, base_url: e.target.value })} required /></div>
            <div className="md:col-span-2">
              <Label>上游 API Key{editing ? '（留空则不修改）' : ''}</Label>
              <Input
                value={form.api_key}
                onChange={(e) => setForm({ ...form, api_key: e.target.value })}
                placeholder={editing ? '•••• 留空保持原 Key' : ''}
              />
            </div>
            <div>
              <Label>优先级（0 最低默认，数字越大越优先，禁止负数）</Label>
              <Input type="number" min={0} value={form.priority} onChange={(e) => setForm({ ...form, priority: Math.max(0, Number(e.target.value)) })} />
            </div>
            <div><Label>超时 (ms)</Label><Input type="number" value={form.timeout_ms} onChange={(e) => setForm({ ...form, timeout_ms: Number(e.target.value) })} /></div>
            <div className="md:col-span-2"><Label>额外 Header JSON</Label><Textarea value={form.extra_headers_json} onChange={(e) => setForm({ ...form, extra_headers_json: e.target.value })} /></div>
            <div className="md:col-span-2 flex gap-2">
              <Button type="submit">保存</Button>
              <Button type="button" variant="secondary" onClick={cancelForm}>取消</Button>
            </div>
          </form>
        </Card>
      )}
      <Card className="p-2">
        <Table headers={['名称', '协议', '优先级', '状态', '最近测活', 'Base URL', '操作']}>
          {items.map((ch) => (
            <tr key={ch.id} className="border-b border-gray-50 hover:bg-canvas">
              <td className="px-4 py-3"><Link className="text-primary hover:underline font-medium" to={`/channels/${ch.id}`}>{ch.name}</Link></td>
              <td className="px-4 py-3"><Badge tone="muted">{ch.protocol}</Badge></td>
              <td className="px-4 py-3">{ch.priority}</td>
              <td className="px-4 py-3 space-x-1">
                {ch.enabled ? <Badge>启用</Badge> : <Badge tone="warn">停用</Badge>}
                {ch.temp_disabled ? <Badge tone="warn">测活临时禁用</Badge> : null}
              </td>
              <td className="px-4 py-3 text-xs text-gray-500">
                {ch.last_test_status ? `${ch.last_test_status} · ${ch.last_test_ms || 0}ms` : '—'}
                {ch.last_test_error ? <div className="text-warn truncate max-w-[10rem]" title={ch.last_test_error}>{ch.last_test_error}</div> : null}
              </td>
              <td className="px-4 py-3 text-gray-500 max-w-xs truncate">{ch.base_url}</td>
              <td className="px-4 py-3 space-x-2 whitespace-nowrap">
                <Button variant="ghost" onClick={() => startEdit(ch)}>编辑</Button>
                <Button variant="ghost" onClick={async () => { await api(`/api/channels/${ch.id}/test`, { method: 'POST', body: '{}' }); await load() }}>测活</Button>
                <Button variant="ghost" onClick={() => toggle(ch)}>{ch.enabled ? '停用' : '启用'}</Button>
                <Button variant="danger" onClick={() => remove(ch.id)}>删除</Button>
              </td>
            </tr>
          ))}
        </Table>
      </Card>
    </div>
  )
}
