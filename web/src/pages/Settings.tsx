import { FormEvent, useEffect, useState } from 'react'
import { CircleHelp } from 'lucide-react'
import { api } from '@/lib/api'
import { Badge, Button, Card, Input, Label, PageHeader, Textarea } from '@/components/ui'

export default function SettingsPage() {
  const [settings, setSettings] = useState<Record<string, string>>({})
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [msg, setMsg] = useState('')
  const [err, setErr] = useState('')
  const [hc, setHc] = useState<any>(null)
  const [defaultLexicon, setDefaultLexicon] = useState('')
  const [lexiconError, setLexiconError] = useState('')
  const [preview, setPreview] = useState('')

  async function load() {
    const r = await api<{ settings: Record<string, string>, enhanced_lexicon_default?: string }>('/api/settings')
    setSettings(r.settings || {})
    setDefaultLexicon(r.enhanced_lexicon_default || '')
    const h = await api('/api/healthcheck/status')
    setHc(h)
  }
  useEffect(() => { load().catch(console.error) }, [])

  async function saveSettings(e: FormEvent) {
    e.preventDefault()
    setMsg(''); setErr('')
    try {
	  if ((settings.auto_test_enhanced_enabled || 'false') === 'true') {
	    JSON.parse(settings.auto_test_enhanced_lexicon || defaultLexicon)
	  }
      await api('/api/settings', { method: 'PATCH', body: JSON.stringify({ settings }) })
      setMsg('设置已保存')
      await load()
    } catch (ex: any) {
      setErr(ex.message)
    }
  }

  function updateLexicon(value: string) {
    setSettings({ ...settings, auto_test_enhanced_lexicon: value })
    setPreview('')
    try {
      JSON.parse(value)
      setLexiconError('')
    } catch (ex: any) {
      setLexiconError(ex.message || 'JSON 格式无效')
    }
  }

  async function previewEnhanced() {
    setPreview(''); setLexiconError('')
    try {
      const r = await api<{ prompt: string }>('/api/healthcheck/enhanced-preview', {
        method: 'POST',
        body: JSON.stringify({ lexicon: settings.auto_test_enhanced_lexicon || defaultLexicon }),
      })
      setPreview(r.prompt)
    } catch (ex: any) {
      setLexiconError(ex.message || '词库无效')
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
          <form className="space-y-4" onSubmit={saveSettings}>
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input
                type="checkbox"
                checked={(settings.auto_test_enabled || 'false') === 'true'}
                onChange={(e) => setBool('auto_test_enabled', e.target.checked)}
              />
              开启自动测活
              {hc?.enabled ? <Badge>运行中</Badge> : null}
            </label>
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input
                type="checkbox"
                checked={(settings.auto_test_enhanced_enabled || 'false') === 'true'}
                onChange={(e) => setBool('auto_test_enhanced_enabled', e.target.checked)}
              />
              自动测活增强
            </label>
            {(settings.auto_test_enhanced_enabled || 'false') === 'true' && (
              <div className="rounded-2xl border border-gray-100 bg-canvas/70 p-4 space-y-3">
                <div className="text-xs text-gray-500 leading-5">
                  随机使用 <code>{'{prefix}{target}'}</code> 或 <code>{'{target}{prefix}'}</code>。
                  语气词与标点随机
                </div>
                <div>
                  <label htmlFor="enhanced-lexicon" className="block text-xs font-medium text-gray-500 mb-1.5">增强测活 JSON 词库</label>
                  <Textarea
                    id="enhanced-lexicon"
                    className="min-h-[360px] font-mono text-xs"
                    value={settings.auto_test_enhanced_lexicon || defaultLexicon}
                    onChange={(e) => updateLexicon(e.target.value)}
                  />
                  <p className="mt-1 text-[11px] text-gray-400">schema_version 由系统维护，保存时会自动更新。</p>
                  {lexiconError && <p className="text-xs text-warn mt-1">{lexiconError}</p>}
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <Button type="button" variant="secondary" onClick={() => updateLexicon(defaultLexicon)}>恢复默认词库</Button>
                  <Button type="button" variant="secondary" onClick={previewEnhanced}>随机预览</Button>
                  <div className="group relative inline-flex">
                    <button
                      type="button"
                      aria-label="随机预览说明"
                      className="rounded-full text-gray-400 transition-colors hover:text-primary focus:outline-none focus:ring-2 focus:ring-primary/30"
                    >
                      <CircleHelp className="h-5 w-5" />
                    </button>
                    <div role="tooltip" className="pointer-events-none absolute bottom-full left-1/2 z-30 mb-2 hidden w-72 -translate-x-1/2 rounded-xl border border-gray-100 bg-white p-3 text-xs leading-5 text-gray-600 shadow-card group-hover:block group-focus-within:block">
                      为什么预览句子有时读起来不够自然？为降低固定特征被识别的概率，系统会随机组合正常语序与倒装语序，并随机添加语气词和标点符号，因此部分句子可能会显得不够通顺。这些变化仅用于模拟更自然、更多样的请求特征。
                    </div>
                  </div>
                </div>
                {preview && <div className="rounded-xl bg-white px-3 py-2 text-sm text-gray-700"><span className="text-gray-400">预览：</span>{preview}</div>}
              </div>
            )}
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
              {(settings.auto_test_enhanced_enabled || 'false') === 'true' && (
                <p className="text-[11px] text-gray-400 mt-1">增强模式开启时不使用此固定提问词。</p>
              )}
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
