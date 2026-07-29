import { useEffect, useMemo, useState } from 'react'
import {
  Area,
  CartesianGrid,
  ComposedChart,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { api } from '@/lib/api'
import { Badge, Card, PageHeader, Table } from '@/components/ui'

type Period = '24h' | '7d' | '30d'

type SeriesPoint = {
  start: string
  end: string
  requests: number
  total_tokens: number
  prompt_tokens: number
  completion_tokens: number
}

type Summary = {
  requests_today: number
  errors_today: number
  requests_7d: number
  error_rate_7d: number
  period: Period
  token_usage: {
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
    cached_tokens: number
    reasoning_tokens: number
  }
  series: SeriesPoint[]
  top_models: { name: string; count: number; tokens: number; prompt_tokens: number; completion_tokens: number }[]
  recent_errors: any[]
}

function formatCompact(n: number) {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}

function tickLabel(start: string, period: Period) {
  const d = new Date(start)
  if (Number.isNaN(d.getTime())) return start
  if (period === '24h') {
    return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  }
  return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
}

export default function Dashboard() {
  const [period, setPeriod] = useState<Period>('7d')
  const [data, setData] = useState<Summary | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    setLoading(true)
    api<Summary>(`/api/dashboard/summary?period=${period}`)
      .then(setData)
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [period])

  const cards = [
    { label: '今日请求', value: data?.requests_today ?? '—', tone: 'primary' },
    { label: '今日错误', value: data?.errors_today ?? '—', tone: 'warn' },
    { label: '近 7 日请求', value: data?.requests_7d ?? '—', tone: 'muted' },
    { label: '近 7 日错误率', value: data ? `${(data.error_rate_7d * 100).toFixed(1)}%` : '—', tone: 'accent' },
  ]

  const chartData = useMemo(() => {
    return (data?.series || []).map((p) => ({
      ...p,
      label: tickLabel(p.start, period),
      tokens: p.total_tokens || 0,
    }))
  }, [data, period])

  const usage = data?.token_usage

  return (
    <div>
      <PageHeader title="概览" subtitle="网关运行概况、Token 用量趋势与近期异常" />
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4 mb-6">
        {cards.map((c) => (
          <Card key={c.label} className="p-5 hover:shadow-2xl hover:-translate-y-1 transition-all duration-300">
            <div className="text-xs text-gray-400">{c.label}</div>
            <div className="text-2xl font-semibold text-gray-800 mt-2">{c.value}</div>
          </Card>
        ))}
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-4 gap-6 mb-6 items-stretch">
        <Card className="p-5 xl:col-span-3 min-h-[360px] flex flex-col">
          <div className="flex flex-wrap items-start justify-between gap-3 mb-3">
            <div>
              <div className="text-lg font-semibold">Token 消耗量</div>
              <div className="text-xs text-gray-400 mt-1">
                真实上游 usage 解析（失败请求记 0；缺失 usage 暂不估算）
              </div>
            </div>
            <div className="flex gap-1 bg-canvas rounded-xl p-1">
              {([
                ['24h', '24 小时'],
                ['7d', '7 天'],
                ['30d', '30 天'],
              ] as const).map(([k, label]) => (
                <button
                  key={k}
                  type="button"
                  onClick={() => setPeriod(k)}
                  className={`px-3 py-1.5 text-xs rounded-lg transition ${
                    period === k ? 'bg-white text-primary shadow-sm font-medium' : 'text-gray-500 hover:text-gray-700'
                  }`}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>

          <div className="grid grid-cols-2 md:grid-cols-5 gap-3 mb-4">
            <Metric label="总 Token" value={formatCompact(usage?.total_tokens ?? 0)} />
            <Metric label="输入" value={formatCompact(usage?.prompt_tokens ?? 0)} />
            <Metric label="输出" value={formatCompact(usage?.completion_tokens ?? 0)} />
            <Metric label="缓存" value={formatCompact(usage?.cached_tokens ?? 0)} />
            <Metric label="推理" value={formatCompact(usage?.reasoning_tokens ?? 0)} />
          </div>

          <div className={`flex-1 min-h-[240px] ${loading ? 'opacity-50' : ''}`}>
            {chartData.some((d) => d.tokens > 0 || d.requests > 0) ? (
              <ResponsiveContainer width="100%" height={260}>
                <ComposedChart data={chartData} margin={{ top: 8, right: 12, left: 0, bottom: 0 }}>
                  <defs>
                    <linearGradient id="tokenFill" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#4a9d9a" stopOpacity={0.28} />
                      <stop offset="95%" stopColor="#4a9d9a" stopOpacity={0.02} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#e8e4dc" />
                  <XAxis dataKey="label" tickLine={false} axisLine={false} tick={{ fill: '#9ca3af', fontSize: 11 }} minTickGap={16} />
                  <YAxis
                    yAxisId="tokens"
                    tickLine={false}
                    axisLine={false}
                    width={42}
                    tick={{ fill: '#9ca3af', fontSize: 11 }}
                    tickFormatter={(v) => formatCompact(Number(v))}
                  />
                  <YAxis
                    yAxisId="requests"
                    orientation="right"
                    tickLine={false}
                    axisLine={false}
                    width={36}
                    tick={{ fill: '#9ca3af', fontSize: 11 }}
                    allowDecimals={false}
                  />
                  <Tooltip
                    contentStyle={{ borderRadius: 12, borderColor: '#e8e4dc', fontSize: 12 }}
                    formatter={(value: any, name: string) => {
                      if (name === 'tokens') return [Number(value).toLocaleString(), 'Tokens']
                      if (name === 'requests') return [Number(value).toLocaleString(), '请求']
                      return [value, name]
                    }}
                    labelFormatter={(_, payload) => payload?.[0]?.payload?.start || ''}
                  />
                  <Area
                    yAxisId="tokens"
                    type="monotone"
                    dataKey="tokens"
                    name="tokens"
                    stroke="#4a9d9a"
                    fill="url(#tokenFill)"
                    strokeWidth={2}
                  />
                  <Line
                    yAxisId="requests"
                    type="monotone"
                    dataKey="requests"
                    name="requests"
                    stroke="#c4b5a0"
                    strokeWidth={2}
                    strokeDasharray="4 4"
                    dot={false}
                  />
                </ComposedChart>
              </ResponsiveContainer>
            ) : (
              <div className="h-[260px] grid place-items-center text-sm text-gray-400">
                暂无 Token 趋势数据（部署后产生的真实 usage 才会显示）
              </div>
            )}
          </div>
          <div className="mt-2 flex gap-4 text-xs text-gray-400">
            <span className="inline-flex items-center gap-1.5"><i className="w-3 h-0.5 bg-primary inline-block" /> Tokens</span>
            <span className="inline-flex items-center gap-1.5"><i className="w-3 h-0.5 bg-[#c4b5a0] inline-block" style={{ borderTop: '1px dashed' }} /> 请求数</span>
          </div>
        </Card>

        <Card className="p-5 xl:col-span-1 min-h-[360px]">
          <div className="text-lg font-semibold mb-1">热门模型</div>
          <div className="text-xs text-gray-400 mb-4">近 7 日 · 请求次数与 Token</div>
          <div className="space-y-3">
            {(data?.top_models || []).map((m) => (
              <div key={m.name} className="rounded-xl bg-canvas/70 px-3 py-2">
                <div className="flex items-center justify-between gap-2">
                  <span className="text-sm text-gray-700 truncate" title={m.name}>{m.name}</span>
                  <Badge>{m.count}</Badge>
                </div>
                <div className="mt-1 text-[11px] text-gray-400 flex justify-between">
                  <span>Token {formatCompact(m.tokens || 0)}</span>
                  <span>in/out {formatCompact(m.prompt_tokens || 0)}/{formatCompact(m.completion_tokens || 0)}</span>
                </div>
              </div>
            ))}
            {!data?.top_models?.length && <div className="text-sm text-gray-400">暂无数据</div>}
          </div>
        </Card>
      </div>

      <Card className="p-2">
        <div className="px-4 pt-4 pb-2 text-lg font-semibold">最近错误</div>
        <Table headers={['时间', '路径', '状态', '模型', 'Token', '摘要']}>
          {(data?.recent_errors || []).map((e) => (
            <tr key={e.id} className="border-b border-gray-50 hover:bg-canvas">
              <td className="px-4 py-3 text-gray-500">{e.created_at}</td>
              <td className="px-4 py-3">{e.path}</td>
              <td className="px-4 py-3"><Badge tone="warn">{e.status_code}</Badge></td>
              <td className="px-4 py-3">{e.client_model || '—'}</td>
              <td className="px-4 py-3 text-xs text-gray-500">{e.total_tokens ? e.total_tokens : 0}</td>
              <td className="px-4 py-3 text-gray-500">{e.error_summary || '—'}</td>
            </tr>
          ))}
        </Table>
      </Card>
    </div>
  )
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl bg-canvas px-3 py-2">
      <div className="text-[11px] text-gray-400">{label}</div>
      <div className="text-base font-semibold text-gray-800 mt-0.5">{value}</div>
    </div>
  )
}
