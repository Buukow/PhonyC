import { useEffect, useState } from 'react'
import { api } from '@/lib/api'
import { Badge, Button, Card, Input, PageHeader, Select, Table } from '@/components/ui'

type LogFilterState = {
  q: string
  path: string
  userKeyID: string
  channelID: string
  statusCode: string
}

const emptyFilters: LogFilterState = { q: '', path: '', userKeyID: '', channelID: '', statusCode: '' }

export default function Logs() {
  const [items, setItems] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [q, setQ] = useState('')
  const [path, setPath] = useState('')
  const [userKeyID, setUserKeyID] = useState('')
  const [channelID, setChannelID] = useState('')
  const [statusCode, setStatusCode] = useState('')
  const [keys, setKeys] = useState<any[]>([])
  const [channels, setChannels] = useState<any[]>([])
  const [offset, setOffset] = useState(0)
  const limit = 50

  async function load(nextOffset = offset, filters: LogFilterState = { q, path, userKeyID, channelID, statusCode }) {
    const params = new URLSearchParams({ limit: String(limit), offset: String(nextOffset) })
    if (filters.q) params.set('q', filters.q)
    if (filters.path) params.set('path', filters.path)
    if (filters.userKeyID) params.set('user_key_id', filters.userKeyID)
    if (filters.channelID) params.set('channel_id', filters.channelID)
    if (filters.statusCode) {
      params.set('status_min', filters.statusCode)
      params.set('status_max', filters.statusCode)
    }
    const res = await api<{ items: any[]; total: number }>(`/api/logs?${params}`)
    setItems(res.items || [])
    setTotal(res.total || 0)
    setOffset(nextOffset)
  }

  useEffect(() => {
    Promise.all([
      api<{ items: any[] }>('/api/keys'),
      api<{ items: any[] }>('/api/channels'),
    ]).then(([keyRes, channelRes]) => {
      setKeys(keyRes.items || [])
      setChannels(channelRes.items || [])
      return load(0)
    }).catch(console.error)
  }, [])

  function resetFilters() {
    const next = { ...emptyFilters }
    setQ(next.q)
    setPath(next.path)
    setUserKeyID(next.userKeyID)
    setChannelID(next.channelID)
    setStatusCode(next.statusCode)
    load(0, next).catch(console.error)
  }

  return (
    <div>
      <PageHeader title="请求日志" subtitle="元数据 + Token（input/output/total）" />
      <Card className="p-4 mb-6 grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3">
        <Input placeholder="搜索 model / request_id / 错误" value={q} onChange={(e) => setQ(e.target.value)} />
        <Input placeholder="路径过滤，如 /v1/responses" value={path} onChange={(e) => setPath(e.target.value)} />
        <Select aria-label="按 Key 筛选" value={userKeyID} onChange={(e) => setUserKeyID(e.target.value)}>
          <option value="">全部 Key</option>
          {keys.map((key) => <option key={key.id} value={key.id}>{key.name}（#{key.id}）</option>)}
        </Select>
        <Select aria-label="按渠道筛选" value={channelID} onChange={(e) => setChannelID(e.target.value)}>
          <option value="">全部渠道</option>
          {channels.map((channel) => <option key={channel.id} value={channel.id}>{channel.name}（#{channel.id}）</option>)}
        </Select>
        <Input aria-label="按状态码筛选" inputMode="numeric" pattern="[0-9]*" placeholder="状态码，如 401" value={statusCode} onChange={(e) => setStatusCode(e.target.value.replace(/[^0-9]/g, ''))} />
        <div className="flex gap-2">
          <Button onClick={() => load(0)}>查询</Button>
          <Button type="button" variant="secondary" onClick={resetFilters}>重置筛选</Button>
        </div>
      </Card>
      <Card className="p-2 overflow-x-auto">
        <Table headers={['时间', 'Key', '路径', '模型', '渠道', '状态', '耗时', '输入', '输出', '合计', '摘要']}>
          {items.map((l) => (
            <tr key={l.id} className="border-b border-gray-50 hover:bg-canvas">
              <td className="px-4 py-3 text-xs text-gray-500 whitespace-nowrap">{l.created_at}</td>
              <td className="px-4 py-3">{l.user_key_id ?? '—'}</td>
              <td className="px-4 py-3 font-mono text-xs">{l.path}</td>
              <td className="px-4 py-3">{l.client_model || '—'}</td>
              <td className="px-4 py-3">{l.channel_id ?? '—'}</td>
              <td className="px-4 py-3">{l.status_code >= 400 ? <Badge tone="warn">{l.status_code}</Badge> : <Badge>{l.status_code}</Badge>}</td>
              <td className="px-4 py-3 text-gray-500">{l.total_ms}ms</td>
              <td className="px-4 py-3 text-xs text-gray-600">{l.prompt_tokens ?? 0}</td>
              <td className="px-4 py-3 text-xs text-gray-600">{l.completion_tokens ?? 0}</td>
              <td className="px-4 py-3 text-xs font-medium text-gray-800">{l.total_tokens ?? 0}</td>
              <td className="px-4 py-3 text-gray-500 max-w-xs truncate">{l.error_summary || '—'}</td>
            </tr>
          ))}
        </Table>
        <div className="flex items-center justify-between px-4 py-3 text-sm text-gray-500">
          <span>共 {total} 条</span>
          <div className="flex gap-2">
            <Button variant="secondary" disabled={offset <= 0} onClick={() => load(Math.max(0, offset - limit))}>上一页</Button>
            <Button variant="secondary" disabled={offset + limit >= total} onClick={() => load(offset + limit)}>下一页</Button>
          </div>
        </div>
      </Card>
    </div>
  )
}
