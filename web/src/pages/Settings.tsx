import { FormEvent, useEffect, useState } from 'react'
import { api } from '@/lib/api'
import { Badge, Button, Card, Input, Label, PageHeader } from '@/components/ui'

export default function SettingsPage() {
  const [settings, setSettings] = useState<Record<string, string>>({})
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [msg, setMsg] = useState('')
  const [err, setErr] = useState('')
  const [hc, setHc] = useState<any>(null)

  async function load() {
    const r = await api<{ settings: Record<string, string> }>('/api/settings')
    setSettings(r.settings || {})
    const h = await api('/api/healthcheck/status')
    setHc(h)
  }
  useEffect(() => { load().catch(console.error) }, [])

  async function saveSettings(e: FormEvent) {
    e.preventDefault()
    setMsg(''); setErr('')
    try {
      await api('/api/settings', { method: 'PATCH', body: JSON.stringify({ settings }) })
      setMsg('设置已保存')
      await load()
    } catch (ex: any) {
      setErr(ex.message)
    }
  }

  async function changePassword(e: FormEvent) {
    e.preventDefault()
    setMsg(''); setErr('')
    try {
      await api('/api/auth/change-password', { method: 'POST', body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }) })
      setMsg('密码已更新')
      setOldPassword(''); setNewPassword('')
    } catch (ex: any) {
      setErr(ex.message)
    }
  }

  async function runNow() {
    setMsg(''); setErr('')
    try {
      await api('/api/healthcheck/run', { method: 'POST', body: '{}' })
      setMsg('已触发一轮测活（后台执行）')
      setTimeout(() => load().catch(() => {}), 2000)
    } catch (ex: any) {
      setErr(ex.message)
    }
  }

  function setBool(key: string, on: boolean) {
    setSettings({ ...settings, [key]: on ? 'true' : 'false' })
  }

  return (
    <div>
      <PageHeader title="设置" subtitle="自动重试、自动测活、日志管理与管理员安全" />
      {msg && <div className="mb-4 text-sm text-primary">{msg}</div>}
      {err && <div className="mb-4 text-sm text-warn">{err}</div>}
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        <Card className="p-6">
          <div className="text-lg font-semibold mb-4">自动测活</div>
          <p className="text-xs text-gray-400 mb-4">
            仅测试「手动启用」的渠道（含测活临时禁用，用于恢复）。命中临时禁用状态码时临时禁用；下次测活成功则恢复。
            <strong>正式转发</strong>若收到相同状态码，也会立即将渠道设为临时禁用。渠道列表「测活」按钮只出报告、不改禁用状态。
          </p>
          <form className="space-y-4" onSubmit={saveSettings}>
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input
                type="checkbox"
                checked={(settings.auto_test_enabled || 'false') === 'true'}
                onChange={(e) => setBool('auto_test_enabled', e.target.checked)}
              />
              开启自动测活
              {hc?.enabled ? <Badge>运行中</Badge> : <Badge tone="muted">关闭</Badge>}
            </label>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label>间隔（分钟）</Label>
                <Input
                  type="number"
                  min={1}
                  value={settings.auto_test_interval_minutes || '10'}
                  onChange={(e) => setSettings({ ...settings, auto_test_interval_minutes: e.target.value })}
                />
              </div>
              <div>
                <Label>随机偏移（分钟）</Label>
                <Input
                  type="number"
                  min={0}
                  value={settings.auto_test_random_offset_minutes || '0'}
                  onChange={(e) => setSettings({ ...settings, auto_test_random_offset_minutes: e.target.value })}
                />
              </div>
            </div>
            <div>
              <Label>测活提问词（默认 hi）</Label>
              <Input
                value={settings.auto_test_prompt ?? 'hi'}
                onChange={(e) => setSettings({ ...settings, auto_test_prompt: e.target.value })}
              />
            </div>
            <div className="text-xs text-gray-400 bg-canvas rounded-xl px-3 py-2">
              测活模型：固定使用<strong>每个渠道模型表中第一个启用映射</strong>。
            </div>
            <div>
              <Label>临时禁用状态码（逗号分隔）</Label>
              <Input
                value={settings.auto_test_disable_status_codes || '401,403,404,503'}
                onChange={(e) => setSettings({ ...settings, auto_test_disable_status_codes: e.target.value })}
              />
              <p className="text-[11px] text-gray-400 mt-1">同时作用于自动测活与正式请求转发。</p>
            </div>
            <div className="flex gap-2">
              <Button type="submit">保存设置</Button>
              <Button type="button" variant="secondary" onClick={runNow}>立即测活一轮</Button>
            </div>
          </form>
          {hc?.last_summary?.finished_at && (
            <div className="mt-4 text-xs text-gray-400 space-y-1">
              <div>上次测活：{hc.last_summary.finished_at}</div>
              <div>
                总计 {hc.last_summary.total} · 成功 {hc.last_summary.ok} · 失败 {hc.last_summary.failed} ·
                临时禁用 {hc.last_summary.disabled} · 恢复 {hc.last_summary.recovered}
              </div>
            </div>
          )}
        </Card>

        <div className="space-y-6">
          <Card className="p-6">
            <div className="text-lg font-semibold mb-4">自动重试</div>
            <p className="text-xs text-gray-400 mb-4">
              正式转发上游返回指定状态码时，可自动换其它可用渠道重试（优先排除已失败渠道）。
              若状态码同时属于「临时禁用状态码」，会先将该渠道临时禁用再重试。
            </p>
            <form className="space-y-4" onSubmit={saveSettings}>
              <label className="flex items-center gap-2 text-sm text-gray-700">
                <input
                  type="checkbox"
                  checked={(settings.auto_retry_enabled || 'false') === 'true'}
                  onChange={(e) => setBool('auto_retry_enabled', e.target.checked)}
                />
                开启自动重试
                {(settings.auto_retry_enabled || 'false') === 'true' ? <Badge>已开启</Badge> : <Badge tone="muted">关闭</Badge>}
              </label>
              <div>
                <Label>重试上限（次）</Label>
                <Input
                  type="number"
                  min={0}
                  max={20}
                  value={settings.auto_retry_max || '2'}
                  onChange={(e) => setSettings({ ...settings, auto_retry_max: e.target.value })}
                />
                <p className="text-[11px] text-gray-400 mt-1">在首次请求失败后，最多再重试的次数（默认 2）。</p>
              </div>
              <div>
                <Label>触发重试的状态码（逗号分隔）</Label>
                <Input
                  value={settings.auto_retry_status_codes || '429,500,502,503,504'}
                  onChange={(e) => setSettings({ ...settings, auto_retry_status_codes: e.target.value })}
                />
              </div>
              <Button type="submit">保存重试设置</Button>
            </form>
          </Card>

          <Card className="p-6">
            <div className="text-lg font-semibold mb-4">日志管理</div>
            <p className="text-xs text-gray-400 mb-4">
              控制请求日志元数据在本地的保留时长；到期日志将由系统清理。
            </p>
            <form className="space-y-4" onSubmit={saveSettings}>
              <div>
                <Label>日志保留天数</Label>
                <Input
                  type="number"
                  min={1}
                  value={settings.log_retention_days || '30'}
                  onChange={(e) => setSettings({ ...settings, log_retention_days: e.target.value })}
                />
              </div>
              <Button type="submit">保存日志设置</Button>
            </form>
          </Card>

          <Card className="p-6">
            <div className="text-lg font-semibold mb-4">修改密码</div>
            <form className="space-y-4" onSubmit={changePassword}>
              <div><Label>当前密码</Label><Input type="password" value={oldPassword} onChange={(e) => setOldPassword(e.target.value)} required /></div>
              <div><Label>新密码</Label><Input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} required minLength={6} /></div>
              <Button type="submit">更新密码</Button>
            </form>
          </Card>
        </div>
      </div>
    </div>
  )
}
