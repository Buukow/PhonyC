import { useMemo, useState } from 'react'
import { api } from '@/lib/api'
import { Badge, Button, Input, Label } from '@/components/ui'

export type ModelDraft = {
  client_model: string
  upstream_model: string
  rewrite_model: boolean
  enabled: boolean
}

type Props = {
  models: ModelDraft[]
  onChange: (models: ModelDraft[]) => void
  /** credentials used for probe-models (create/edit form) */
  probe?: {
    base_url: string
    api_key: string
    protocol: string
    extra_headers_json?: string
  }
  /** if set, use saved channel fetch endpoint */
  channelId?: number
}

export default function ChannelModelEditor({ models, onChange, probe, channelId }: Props) {
  const [fetched, setFetched] = useState<string[]>([])
  const [filter, setFilter] = useState('')
  const [customClient, setCustomClient] = useState('')
  const [customUpstream, setCustomUpstream] = useState('')
  const [busy, setBusy] = useState(false)
  const [msg, setMsg] = useState('')
  const [err, setErr] = useState('')

  const selectedSet = useMemo(() => new Set(models.map((m) => m.client_model)), [models])

  const visibleFetched = useMemo(() => {
    const q = filter.trim().toLowerCase()
    return fetched.filter((id) => !q || id.toLowerCase().includes(q))
  }, [fetched, filter])

  function addModel(client: string, upstream?: string, rewrite?: boolean) {
    const name = client.trim()
    if (!name) return
    if (selectedSet.has(name)) {
      setErr(`模型 ${name} 已添加`)
      return
    }
    const up = (upstream || name).trim() || name
    const rw = rewrite ?? (up !== name)
    onChange([...models, { client_model: name, upstream_model: up, rewrite_model: rw, enabled: true }])
    setErr('')
    setMsg(`已添加 ${name}`)
  }

  function removeAt(idx: number) {
    onChange(models.filter((_, i) => i !== idx))
  }

  function updateAt(idx: number, patch: Partial<ModelDraft>) {
    onChange(models.map((m, i) => (i === idx ? { ...m, ...patch } : m)))
  }

  async function fetchModels() {
    setBusy(true)
    setErr('')
    setMsg('')
    try {
      if (!probe?.base_url && !channelId) {
        throw new Error('请先填写 Base URL')
      }
      const res = await api<{ items: string[] }>('/api/channels/probe-models', {
        method: 'POST',
        body: JSON.stringify({
          base_url: probe?.base_url || '',
          api_key: probe?.api_key || '',
          protocol: probe?.protocol || 'openai',
          extra_headers_json: probe?.extra_headers_json || '{}',
          channel_id: channelId || undefined,
        }),
      })
      const items = res.items || []
      setFetched(items)
      setMsg(`拉取到 ${items.length} 个上游模型，点击即可添加`)
    } catch (ex: any) {
      setErr(ex.message || '获取模型失败')
    } finally {
      setBusy(false)
    }
  }

  function addAllVisible() {
    const next = [...models]
    const have = new Set(next.map((m) => m.client_model))
    let n = 0
    for (const id of visibleFetched) {
      if (have.has(id)) continue
      next.push({ client_model: id, upstream_model: id, rewrite_model: false, enabled: true })
      have.add(id)
      n++
    }
    onChange(next)
    setMsg(`批量添加 ${n} 个模型`)
  }

  return (
    <div className="space-y-4 md:col-span-2 border-t border-gray-100 pt-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <div className="text-base font-semibold text-gray-800">模型映射</div>
          <div className="text-xs text-gray-400 mt-0.5">
            从上游 <code className="bg-canvas px-1 rounded">/v1/models</code> 拉取，或自定义客户端名 / 上游路由。仅添加的模型会进入测活与网关模型列表。
          </div>
        </div>
        <div className="flex gap-2">
          <Button type="button" variant="secondary" disabled={busy} onClick={fetchModels}>
            {busy ? '拉取中…' : '获取模型列表'}
          </Button>
          {fetched.length > 0 && (
            <Button type="button" variant="ghost" onClick={addAllVisible}>添加筛选结果</Button>
          )}
        </div>
      </div>
      {msg && <div className="text-xs text-primary">{msg}</div>}
      {err && <div className="text-xs text-warn">{err}</div>}

      {fetched.length > 0 && (
        <div className="rounded-2xl bg-canvas p-3 space-y-2">
          <div className="flex items-center gap-2">
            <Input placeholder="筛选上游模型…" value={filter} onChange={(e) => setFilter(e.target.value)} />
            <span className="text-xs text-gray-400 whitespace-nowrap">{visibleFetched.length}/{fetched.length}</span>
          </div>
          <div className="flex flex-wrap gap-2 max-h-40 overflow-y-auto">
            {visibleFetched.map((id) => {
              const selected = selectedSet.has(id)
              return (
                <button
                  key={id}
                  type="button"
                  disabled={selected}
                  onClick={() => addModel(id)}
                  className={`px-2.5 py-1 rounded-lg text-xs border transition ${
                    selected
                      ? 'bg-white/50 text-gray-300 border-gray-100 cursor-not-allowed'
                      : 'bg-white text-gray-700 border-gray-200 hover:border-primary hover:text-primary'
                  }`}
                  title={selected ? '已添加' : '点击添加'}
                >
                  {id}
                </button>
              )
            })}
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-3 gap-3 items-end">
        <div>
          <Label>自定义客户端模型</Label>
          <Input value={customClient} onChange={(e) => setCustomClient(e.target.value)} placeholder="客户端看到的名称" />
        </div>
        <div>
          <Label>上游路由（可选）</Label>
          <Input value={customUpstream} onChange={(e) => setCustomUpstream(e.target.value)} placeholder="默认同客户端名" />
        </div>
        <Button
          type="button"
          onClick={() => {
            addModel(customClient, customUpstream || customClient)
            setCustomClient('')
            setCustomUpstream('')
          }}
        >
          添加自定义
        </Button>
      </div>

      <div className="rounded-2xl border border-gray-100 overflow-hidden">
        <div className="px-3 py-2 bg-canvas text-xs text-gray-500 flex justify-between">
          <span>已选模型（{models.length}）</span>
          <span>客户端名 → 上游路由</span>
        </div>
        {models.length === 0 ? (
          <div className="px-4 py-6 text-sm text-gray-400 text-center">尚未添加模型，测活与 /v1/models 都不会包含此渠道</div>
        ) : (
          <div className="divide-y divide-gray-50">
            {models.map((m, idx) => (
              <div key={`${m.client_model}-${idx}`} className="px-3 py-2 grid grid-cols-1 md:grid-cols-12 gap-2 items-center">
                <div className="md:col-span-3">
                  <Input
                    value={m.client_model}
                    onChange={(e) => updateAt(idx, { client_model: e.target.value })}
                    placeholder="客户端模型"
                  />
                </div>
                <div className="md:col-span-3">
                  <Input
                    value={m.upstream_model}
                    onChange={(e) => {
                      const up = e.target.value
                      updateAt(idx, {
                        upstream_model: up,
                        rewrite_model: up.trim() !== '' && up.trim() !== m.client_model.trim() ? true : m.rewrite_model,
                      })
                    }}
                    placeholder="上游模型"
                  />
                </div>
                <label className="md:col-span-3 flex items-center gap-2 text-xs text-gray-600">
                  <input
                    type="checkbox"
                    checked={m.rewrite_model}
                    onChange={(e) => updateAt(idx, { rewrite_model: e.target.checked })}
                  />
                  改写 body.model
                  {m.rewrite_model ? <Badge>路由</Badge> : <Badge tone="muted">同名</Badge>}
                </label>
                <div className="md:col-span-3 flex justify-end">
                  <Button type="button" variant="danger" onClick={() => removeAt(idx)}>移除</Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
