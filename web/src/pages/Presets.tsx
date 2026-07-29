import { FormEvent, useEffect, useState } from 'react'
import { api } from '@/lib/api'
import { Badge, Button, Card, Input, Label, PageHeader, Table, Textarea } from '@/components/ui'

export default function Presets() {
  const [items, setItems] = useState<any[]>([])
  const [editing, setEditing] = useState<any | null>(null)
  const [creating, setCreating] = useState(false)
  const [form, setForm] = useState({ name: '', description: '', version_label: '', headers_json: '{}', remove_headers: '[]' })

  async function load() {
    const res = await api<{ items: any[] }>('/api/presets')
    setItems(res.items || [])
  }
  useEffect(() => { load().catch(console.error) }, [])

  async function saveEdit(e: FormEvent) {
    e.preventDefault()
    if (editing) {
      await api(`/api/presets/${editing.id}`, { method: 'PATCH', body: JSON.stringify(form) })
    } else {
      await api('/api/presets', { method: 'POST', body: JSON.stringify(form) })
    }
    setEditing(null)
    setCreating(false)
    await load()
  }

  async function remove(id: number) {
    if (!confirm('确认删除？')) return
    await api(`/api/presets/${id}`, { method: 'DELETE' })
    await load()
  }

  function startEdit(p: any) {
    setCreating(false)
    setEditing(p)
    setForm({
      name: p.name,
      description: p.description,
      version_label: p.version_label,
      headers_json: p.headers_json,
      remove_headers: p.remove_headers,
    })
  }

  return (
    <div>
      <PageHeader
        title="客户端预设"
        subtitle="Codex / Claude Code 等指纹模板，可编辑"
        actions={<Button onClick={() => { setCreating(true); setEditing(null); setForm({ name: '', description: '', version_label: '', headers_json: '{}', remove_headers: '[]' }) }}>新建预设</Button>}
      />
      {(creating || editing) && (
        <Card className="p-6 mb-6">
          <form className="space-y-3" onSubmit={saveEdit}>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
              <div><Label>名称</Label><Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required disabled={!!editing?.builtin} /></div>
              <div><Label>版本标签</Label><Input value={form.version_label} onChange={(e) => setForm({ ...form, version_label: e.target.value })} /></div>
              <div><Label>描述</Label><Input value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} /></div>
            </div>
            <div><Label>Headers JSON（可用 {"{{version}}"}）</Label><Textarea className="min-h-[160px] font-mono text-xs" value={form.headers_json} onChange={(e) => setForm({ ...form, headers_json: e.target.value })} /></div>
            <div><Label>Remove headers JSON 数组</Label><Input value={form.remove_headers} onChange={(e) => setForm({ ...form, remove_headers: e.target.value })} /></div>
            <div className="flex gap-2"><Button type="submit">保存</Button><Button type="button" variant="secondary" onClick={() => { setEditing(null); setCreating(false) }}>取消</Button></div>
          </form>
        </Card>
      )}
      <Card className="p-2">
        <Table headers={['名称', '版本', '类型', '描述', '操作']}>
          {items.map((p) => (
            <tr key={p.id} className="border-b border-gray-50 hover:bg-canvas">
              <td className="px-4 py-3 font-medium">{p.name}</td>
              <td className="px-4 py-3">{p.version_label}</td>
              <td className="px-4 py-3">{p.builtin ? <Badge>内置</Badge> : <Badge tone="muted">自定义</Badge>}</td>
              <td className="px-4 py-3 text-gray-500">{p.description}</td>
              <td className="px-4 py-3 space-x-2">
                <Button variant="ghost" onClick={() => startEdit(p)}>编辑</Button>
                {!p.builtin && <Button variant="danger" onClick={() => remove(p.id)}>删除</Button>}
              </td>
            </tr>
          ))}
        </Table>
      </Card>
    </div>
  )
}
