import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api } from '@/lib/api'
import ChannelModelEditor, { type ModelDraft } from '@/components/ChannelModelEditor'
import { Badge, Button, Card, PageHeader } from '@/components/ui'

export default function ChannelDetail() {
  const { id } = useParams()
  const [ch, setCh] = useState<any>(null)
  const [models, setModels] = useState<ModelDraft[]>([])
  const [msg, setMsg] = useState('')
  const [err, setErr] = useState('')
  const [saving, setSaving] = useState(false)

  async function load() {
    const c = await api(`/api/channels/${id}`)
    setCh(c)
    const m = await api<{ items: any[] }>(`/api/channels/${id}/models`)
    setModels((m.items || []).map((x) => ({
      client_model: x.client_model,
      upstream_model: x.upstream_model || x.client_model,
      rewrite_model: !!x.rewrite_model,
      enabled: x.enabled !== false,
    })))
  }
  useEffect(() => { load().catch(console.error) }, [id])

  async function saveModels() {
    setSaving(true)
    setMsg('')
    setErr('')
    try {
      await api(`/api/channels/${id}/models`, {
        method: 'PUT',
        body: JSON.stringify({
          items: models
            .filter((m) => m.client_model.trim())
            .map((m) => ({
              client_model: m.client_model.trim(),
              upstream_model: (m.upstream_model || m.client_model).trim(),
              rewrite_model: !!m.rewrite_model,
              enabled: true,
            })),
        }),
      })
      setMsg('模型映射已保存')
      await load()
    } catch (ex: any) {
      setErr(ex.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  if (!ch) return <div className="text-gray-400">加载中…</div>

  return (
    <div>
      <PageHeader
        title={ch.name}
        subtitle={`协议 ${ch.protocol} · 优先级 ${ch.priority}`}
        actions={<Link to="/channels" className="text-sm text-primary">返回列表</Link>}
      />
      {msg && <div className="mb-3 text-sm text-primary">{msg}</div>}
      {err && <div className="mb-3 text-sm text-warn">{err}</div>}
      <Card className="p-5 mb-6 grid grid-cols-1 md:grid-cols-2 gap-3 text-sm">
        <div><span className="text-gray-400">Base URL</span><div className="text-gray-800 break-all">{ch.base_url}</div></div>
        <div><span className="text-gray-400">状态</span><div>{ch.enabled ? <Badge>启用</Badge> : <Badge tone="warn">停用</Badge>}</div></div>
        <div><span className="text-gray-400">超时</span><div>{ch.timeout_ms} ms</div></div>
        <div><span className="text-gray-400">上游 Key</span><div className="font-mono text-xs">{ch.api_key ? '••••' + String(ch.api_key).slice(-4) : '—'}</div></div>
      </Card>

      <Card className="p-6 mb-6">
        <ChannelModelEditor
          models={models}
          onChange={setModels}
          channelId={Number(id)}
          probe={{
            base_url: ch.base_url,
            api_key: ch.api_key || '',
            protocol: ch.protocol,
            extra_headers_json: ch.extra_headers_json || '{}',
          }}
        />
        <div className="mt-4">
          <Button onClick={saveModels} disabled={saving}>{saving ? '保存中…' : '保存模型映射'}</Button>
        </div>
      </Card>
    </div>
  )
}
