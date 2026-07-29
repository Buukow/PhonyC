import { useEffect, useState } from 'react'
import { Copy, Crosshair, RefreshCw, Save } from 'lucide-react'
import { api } from '@/lib/api'
import { Badge, Button, Card, Input, Label, PageHeader, Table } from '@/components/ui'

type CaptureState = {
  enabled: boolean
  armed: boolean
  key: string
  captured: null | {
    captured_at: string
    method: string
    path: string
    query: string
    headers: Record<string, string>
    model?: string
  }
}

export default function CapturePage() {
  const [data, setData] = useState<CaptureState | null>(null)
  const [presetName, setPresetName] = useState('')
  const [msg, setMsg] = useState('')
  const [err, setErr] = useState('')
  const [loading, setLoading] = useState(false)
  const [overwrite, setOverwrite] = useState(false)

  async function load() {
    const res = await api<CaptureState>('/api/capture')
    setData(res)
  }
  useEffect(() => {
    load().catch((e) => setErr(e.message))
    const t = setInterval(() => load().catch(() => {}), 3000)
    return () => clearInterval(t)
  }, [])

  async function enable() {
    setLoading(true); setMsg(''); setErr('')
    try {
      await api('/api/capture/enable', { method: 'POST', body: '{}' })
      setMsg('已开启捕获，等待第一次请求')
      await load()
    } catch (e: any) { setErr(e.message) }
    finally { setLoading(false) }
  }
  async function disable() {
    setLoading(true); setMsg(''); setErr('')
    try {
      await api('/api/capture/disable', { method: 'POST', body: '{}' })
      setMsg('已关闭捕获')
      await load()
    } catch (e: any) { setErr(e.message) }
    finally { setLoading(false) }
  }
  async function rearm() {
    setLoading(true); setMsg(''); setErr('')
    try {
      await api('/api/capture/arm', { method: 'POST', body: '{}' })
      setMsg('已重新布防，将捕获下一次请求')
      await load()
    } catch (e: any) { setErr(e.message) }
    finally { setLoading(false) }
  }
  async function clear() {
    await api('/api/capture/clear', { method: 'POST', body: '{}' })
    await load()
  }
  async function savePreset() {
    setLoading(true); setMsg(''); setErr('')
    try {
      const p = await api('/api/capture/save-preset', {
        method: 'POST',
        body: JSON.stringify({ name: presetName || undefined, overwrite }),
      })
      setMsg(`已保存预设 #${(p as any).id} ${(p as any).name}`)
      setPresetName('')
    } catch (e: any) { setErr(e.message) }
    finally { setLoading(false) }
  }

  const headers = data?.captured?.headers ? Object.entries(data.captured.headers) : []

  return (
    <div>
      <PageHeader
        title="请求捕获"
        subtitle="使用系统固定 API Key 抓取客户端请求头，一键保存为伪装预设"
        actions={
          <div className="flex gap-2">
            {data?.enabled ? (
              <Button variant="secondary" onClick={disable} disabled={loading}>关闭捕获</Button>
            ) : (
              <Button onClick={enable} disabled={loading}><Crosshair className="w-4 h-4" />开启捕获</Button>
            )}
            <Button variant="ghost" onClick={() => load()}><RefreshCw className="w-4 h-4" /></Button>
          </div>
        }
      />
      {msg && <div className="mb-4 text-sm text-primary">{msg}</div>}
      {err && <div className="mb-4 text-sm text-warn">{err}</div>}

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6 mb-6">
        <Card className="p-6 space-y-4">
          <div className="flex items-center gap-2">
            <div className="text-lg font-semibold">状态</div>
            {data?.enabled ? <Badge>已开启</Badge> : <Badge tone="muted">已关闭</Badge>}
            {data?.armed ? <Badge tone="accent">等待首次请求</Badge> : data?.enabled ? <Badge tone="muted">已捕获/未布防</Badge> : null}
          </div>
          <div>
            <Label>系统固定 API Key</Label>
            <div className="flex gap-2 items-center">
              <Input readOnly value={data?.key || ''} className="font-mono text-xs" />
              <Button variant="secondary" onClick={() => data?.key && navigator.clipboard.writeText(data.key)}>
                <Copy className="w-4 h-4" />
              </Button>
            </div>
            <p className="text-xs text-gray-400 mt-2">
              将客户端 Base URL 指向本服务，Authorization 使用此 Key。布防中：只捕获不转发，返回 captured。捕获后需重新布防；未布防时该 Key 返回 403。已过滤 Authorization/传输类 hop 头。
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" onClick={rearm} disabled={!data?.enabled || loading}>重新布防</Button>
            <Button variant="ghost" onClick={clear} disabled={loading}>清空记录</Button>
          </div>
        </Card>

        <Card className="p-6 space-y-4">
          <div className="text-lg font-semibold">保存为客户端预设</div>
          <div>
            <Label>预设名称</Label>
            <Input value={presetName} onChange={(e) => setPresetName(e.target.value)} placeholder="例如 codex-captured" disabled={!data?.captured} />
          </div>
          <label className="flex items-center gap-2 text-sm text-gray-600">
            <input type="checkbox" checked={overwrite} onChange={(e) => setOverwrite(e.target.checked)} />
            若同名预设已存在则覆盖
          </label>
          <Button onClick={savePreset} disabled={!data?.captured || loading}>
            <Save className="w-4 h-4" />一键保存至预设
          </Button>
        </Card>
      </div>

      <Card className="p-2">
        <div className="px-4 pt-4 pb-2 flex items-center justify-between">
          <div>
            <div className="text-lg font-semibold">捕获到的请求头</div>
            <div className="text-xs text-gray-400 mt-1">
              {data?.captured
                ? `${data.captured.method} ${data.captured.path}${data.captured.query ? '?' + data.captured.query : ''} · ${data.captured.captured_at}${data.captured.model ? ' · model=' + data.captured.model : ''}`
                : '尚无捕获数据'}
            </div>
          </div>
        </div>
        <Table headers={['Header', 'Value']}>
          {headers.map(([k, v]) => (
            <tr key={k} className="border-b border-gray-50 hover:bg-canvas">
              <td className="px-4 py-3 font-mono text-xs text-gray-700">{k}</td>
              <td className="px-4 py-3 font-mono text-xs text-gray-500 break-all">{v}</td>
            </tr>
          ))}
        </Table>
        {!headers.length && <div className="px-4 py-8 text-sm text-gray-400">开启捕获后，用固定 Key 发一次请求即可在此看到客户端头</div>}
      </Card>
    </div>
  )
}
