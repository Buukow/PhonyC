import { useEffect, useState } from 'react'
import { api } from '@/lib/api'
import { Badge, Card, PageHeader, Table } from '@/components/ui'

type Summary = {
  requests_today: number
  errors_today: number
  requests_7d: number
  error_rate_7d: number
  top_keys: { id?: number; name: string; count: number }[]
  top_models: { name: string; count: number }[]
  recent_errors: any[]
}

export default function Dashboard() {
  const [data, setData] = useState<Summary | null>(null)
  useEffect(() => {
    api<Summary>('/api/dashboard/summary').then(setData).catch(console.error)
  }, [])

  const cards = [
    { label: '今日请求', value: data?.requests_today ?? '—', tone: 'primary' },
    { label: '今日错误', value: data?.errors_today ?? '—', tone: 'warn' },
    { label: '近 7 日请求', value: data?.requests_7d ?? '—', tone: 'muted' },
    { label: '近 7 日错误率', value: data ? `${(data.error_rate_7d * 100).toFixed(1)}%` : '—', tone: 'accent' },
  ]

  return (
    <div>
      <PageHeader title="概览" subtitle="网关运行概况与近期异常" />
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4 mb-6">
        {cards.map((c) => (
          <Card key={c.label} className="p-5 hover:shadow-2xl hover:-translate-y-1 transition-all duration-300">
            <div className="text-xs text-gray-400">{c.label}</div>
            <div className="text-2xl font-semibold text-gray-800 mt-2">{c.value}</div>
          </Card>
        ))}
      </div>
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6 mb-6">
        <Card className="p-5">
          <div className="text-lg font-semibold mb-1">热门 Key</div>
          <div className="text-xs text-gray-400 mb-4">近 7 日请求量</div>
          <div className="space-y-3">
            {(data?.top_keys || []).map((k) => (
              <div key={k.name} className="flex items-center justify-between">
                <span className="text-sm text-gray-700">{k.name}</span>
                <Badge>{k.count}</Badge>
              </div>
            ))}
            {!data?.top_keys?.length && <div className="text-sm text-gray-400">暂无数据</div>}
          </div>
        </Card>
        <Card className="p-5">
          <div className="text-lg font-semibold mb-1">热门模型</div>
          <div className="text-xs text-gray-400 mb-4">近 7 日调用</div>
          <div className="space-y-3">
            {(data?.top_models || []).map((m) => (
              <div key={m.name} className="flex items-center justify-between">
                <span className="text-sm text-gray-700">{m.name}</span>
                <Badge tone="accent">{m.count}</Badge>
              </div>
            ))}
            {!data?.top_models?.length && <div className="text-sm text-gray-400">暂无数据</div>}
          </div>
        </Card>
      </div>
      <Card className="p-2">
        <div className="px-4 pt-4 pb-2 text-lg font-semibold">最近错误</div>
        <Table headers={['时间', '路径', '状态', '模型', '摘要']}>
          {(data?.recent_errors || []).map((e) => (
            <tr key={e.id} className="border-b border-gray-50 hover:bg-canvas">
              <td className="px-4 py-3 text-gray-500">{e.created_at}</td>
              <td className="px-4 py-3">{e.path}</td>
              <td className="px-4 py-3"><Badge tone="warn">{e.status_code}</Badge></td>
              <td className="px-4 py-3">{e.client_model || '—'}</td>
              <td className="px-4 py-3 text-gray-500">{e.error_summary || '—'}</td>
            </tr>
          ))}
        </Table>
      </Card>
    </div>
  )
}
