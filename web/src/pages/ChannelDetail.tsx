import { FormEvent, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api } from '@/lib/api'
import { Badge, Button, Card, Input, Label, PageHeader, Table } from '@/components/ui'

export default function ChannelDetail() {
  const { id } = useParams()
  const [ch, setCh] = useState<any>(null)
  const [models, setModels] = useState<any[]>([])
  const [form, setForm] = useState({ client_model: '', upstream_model: '', rewrite_model: false })

  async function load() {
    const c = await api(`/api/channels/${id}`)
    setCh(c)
    const m = await api<{ items: any[] }>(`/api/channels/${id}/models`)
    setModels(m.items || [])
  }
  useEffect(() => { load().catch(console.error) }, [id])

  async function addModel(e: FormEvent) {
    e.preventDefault()
    await api(`/api/channels/${id}/models`, {
      method: 'POST',
      body: JSON.stringify({
        client_model: form.client_model,
        upstream_model: form.upstream_model || form.client_model,
        rewrite_model: form.rewrite_model,
        enabled: true,
      }),
    })
    setForm({ client_model: '', upstream_model: '', rewrite_model: false })
    await load()
  }

  async function toggleRewrite(m: any) {
    await api(`/api/channel-models/${m.id}`, { method: 'PATCH', body: JSON.stringify({ rewrite_model: !m.rewrite_model }) })
    await load()
  }

  async function removeModel(mid: number) {
    await api(`/api/channel-models/${mid}`, { method: 'DELETE' })
    await load()
  }

  if (!ch) return <div className="text-gray-400">加载中…</div>

  return (
    <div>
      <PageHeader
        title={ch.name}
        subtitle={`协议 ${ch.protocol} · 优先级 ${ch.priority}`}
        actions={<Link to="/channels" className="text-sm text-primary">返回列表</Link>}
      />
      <Card className="p-5 mb-6 grid grid-cols-1 md:grid-cols-2 gap-3 text-sm">
        <div><span className="text-gray-400">Base URL</span><div className="text-gray-800 break-all">{ch.base_url}</div></div>
        <div><span className="text-gray-400">状态</span><div>{ch.enabled ? <Badge>启用</Badge> : <Badge tone="warn">停用</Badge>}</div></div>
        <div><span className="text-gray-400">超时</span><div>{ch.timeout_ms} ms</div></div>
        <div><span className="text-gray-400">上游 Key</span><div className="font-mono text-xs">{ch.api_key ? '••••' + String(ch.api_key).slice(-4) : '—'}</div></div>
      </Card>

      <Card className="p-6 mb-6">
        <div className="text-lg font-semibold mb-4">添加模型映射</div>
        <form className="grid grid-cols-1 md:grid-cols-4 gap-3 items-end" onSubmit={addModel}>
          <div><Label>客户端模型</Label><Input value={form.client_model} onChange={(e) => setForm({ ...form, client_model: e.target.value })} required /></div>
          <div><Label>上游模型</Label><Input value={form.upstream_model} onChange={(e) => setForm({ ...form, upstream_model: e.target.value })} placeholder="默认同客户端" /></div>
          <div className="flex items-center gap-2 pb-2">
            <input id="rw" type="checkbox" checked={form.rewrite_model} onChange={(e) => setForm({ ...form, rewrite_model: e.target.checked })} />
            <label htmlFor="rw" className="text-sm text-gray-600">改写 body.model</label>
          </div>
          <Button type="submit">添加</Button>
        </form>
      </Card>

      <Card className="p-2">
        <Table headers={['客户端模型', '上游模型', '改写', '状态', '操作']}>
          {models.map((m) => (
            <tr key={m.id} className="border-b border-gray-50 hover:bg-canvas">
              <td className="px-4 py-3 font-medium">{m.client_model}</td>
              <td className="px-4 py-3">{m.upstream_model}</td>
              <td className="px-4 py-3">{m.rewrite_model ? <Badge>ON</Badge> : <Badge tone="muted">OFF</Badge>}</td>
              <td className="px-4 py-3">{m.enabled ? '启用' : '停用'}</td>
              <td className="px-4 py-3 space-x-2">
                <Button variant="ghost" onClick={() => toggleRewrite(m)}>切换改写</Button>
                <Button variant="danger" onClick={() => removeModel(m.id)}>删除</Button>
              </td>
            </tr>
          ))}
        </Table>
      </Card>
    </div>
  )
}
