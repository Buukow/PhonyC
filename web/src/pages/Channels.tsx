import { FormEvent, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Plus } from 'lucide-react'
import { api } from '@/lib/api'
import { Badge, Button, Card, Input, Label, PageHeader, Select, Table, Textarea } from '@/components/ui'

type Channel = {
  id: number
  name: string
  enabled: boolean
  protocol: string
  base_url: string
  api_key: string
  priority: number
  timeout_ms: number
}

export default function Channels() {
  const [items, setItems] = useState<Channel[]>([])
  const [show, setShow] = useState(false)
  const [form, setForm] = useState({
    name: '', protocol: 'openai', base_url: '', api_key: '', priority: 10, timeout_ms: 600000, extra_headers_json: '{}',
  })

  async function load() {
    const res = await api<{ items: Channel[] }>('/api/channels')
    setItems(res.items || [])
  }
  useEffect(() => { load().catch(console.error) }, [])

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    await api('/api/channels', {
      method: 'POST',
      body: JSON.stringify({ ...form, enabled: true, priority: Number(form.priority), timeout_ms: Number(form.timeout_ms) }),
    })
    setShow(false)
    setForm({ name: '', protocol: 'openai', base_url: '', api_key: '', priority: 10, timeout_ms: 600000, extra_headers_json: '{}' })
    await load()
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

  return (
    <div>
      <PageHeader
        title="渠道"
        subtitle="配置上游协议、Base URL 与优先级"
        actions={<Button onClick={() => setShow((v) => !v)}><Plus className="w-4 h-4" />新建渠道</Button>}
      />
      {show && (
        <Card className="p-6 mb-6">
          <form className="grid grid-cols-1 md:grid-cols-2 gap-4" onSubmit={onCreate}>
            <div><Label>名称</Label><Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></div>
            <div>
              <Label>协议</Label>
              <Select value={form.protocol} onChange={(e) => setForm({ ...form, protocol: e.target.value })}>
                <option value="openai">openai</option>
                <option value="anthropic">anthropic</option>
              </Select>
            </div>
            <div className="md:col-span-2"><Label>Base URL</Label><Input placeholder="https://api.example.com" value={form.base_url} onChange={(e) => setForm({ ...form, base_url: e.target.value })} required /></div>
            <div className="md:col-span-2"><Label>上游 API Key</Label><Input value={form.api_key} onChange={(e) => setForm({ ...form, api_key: e.target.value })} /></div>
            <div><Label>优先级</Label><Input type="number" value={form.priority} onChange={(e) => setForm({ ...form, priority: Number(e.target.value) })} /></div>
            <div><Label>超时 (ms)</Label><Input type="number" value={form.timeout_ms} onChange={(e) => setForm({ ...form, timeout_ms: Number(e.target.value) })} /></div>
            <div className="md:col-span-2"><Label>额外 Header JSON</Label><Textarea value={form.extra_headers_json} onChange={(e) => setForm({ ...form, extra_headers_json: e.target.value })} /></div>
            <div className="md:col-span-2 flex gap-2"><Button type="submit">保存</Button><Button type="button" variant="secondary" onClick={() => setShow(false)}>取消</Button></div>
          </form>
        </Card>
      )}
      <Card className="p-2">
        <Table headers={['名称', '协议', '优先级', '状态', 'Base URL', '操作']}>
          {items.map((ch) => (
            <tr key={ch.id} className="border-b border-gray-50 hover:bg-canvas">
              <td className="px-4 py-3"><Link className="text-primary hover:underline font-medium" to={`/channels/${ch.id}`}>{ch.name}</Link></td>
              <td className="px-4 py-3"><Badge tone="muted">{ch.protocol}</Badge></td>
              <td className="px-4 py-3">{ch.priority}</td>
              <td className="px-4 py-3">{ch.enabled ? <Badge>启用</Badge> : <Badge tone="warn">停用</Badge>}</td>
              <td className="px-4 py-3 text-gray-500 max-w-xs truncate">{ch.base_url}</td>
              <td className="px-4 py-3 space-x-2">
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
