import { FormEvent, useEffect, useState } from 'react'
import { api } from '@/lib/api'
import { Button, Card, Input, Label, PageHeader } from '@/components/ui'

export default function SettingsPage() {
  const [settings, setSettings] = useState<Record<string, string>>({})
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [msg, setMsg] = useState('')
  const [err, setErr] = useState('')

  useEffect(() => {
    api<{ settings: Record<string, string> }>('/api/settings').then((r) => setSettings(r.settings || {})).catch(console.error)
  }, [])

  async function saveSettings(e: FormEvent) {
    e.preventDefault()
    setMsg(''); setErr('')
    try {
      await api('/api/settings', { method: 'PATCH', body: JSON.stringify({ settings }) })
      setMsg('设置已保存')
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

  return (
    <div>
      <PageHeader title="设置" subtitle="系统参数与管理员安全" />
      {msg && <div className="mb-4 text-sm text-primary">{msg}</div>}
      {err && <div className="mb-4 text-sm text-warn">{err}</div>}
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        <Card className="p-6">
          <div className="text-lg font-semibold mb-4">系统设置</div>
          <form className="space-y-4" onSubmit={saveSettings}>
            <div>
              <Label>日志保留天数</Label>
              <Input
                value={settings.log_retention_days || '30'}
                onChange={(e) => setSettings({ ...settings, log_retention_days: e.target.value })}
              />
            </div>
            <Button type="submit">保存设置</Button>
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
  )
}
