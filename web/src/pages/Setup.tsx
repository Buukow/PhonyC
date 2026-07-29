import { FormEvent, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, setToken } from '@/lib/api'
import { Button, Card, Input, Label } from '@/components/ui'

export default function Setup() {
  const nav = useNavigate()
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('')
  const [err, setErr] = useState('')
  const [loading, setLoading] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setLoading(true)
    setErr('')
    try {
      const res = await api<{ token: string }>('/api/setup', {
        method: 'POST',
        body: JSON.stringify({ username, password }),
      })
      setToken(res.token)
      nav('/')
    } catch (ex: any) {
      setErr(ex.message || '初始化失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen grid place-items-center p-6">
      <Card className="w-full max-w-md p-8">
        <div className="mb-6">
          <div className="w-12 h-12 rounded-2xl bg-primary/10 text-primary grid place-items-center text-lg font-semibold mb-4">P</div>
          <h1 className="text-xl font-semibold text-gray-800">初始化 PhonyC</h1>
          <p className="text-sm text-gray-400 mt-1">首次启动需创建唯一管理员账号</p>
        </div>
        <form className="space-y-4" onSubmit={onSubmit}>
          <div>
            <Label>用户名</Label>
            <Input value={username} onChange={(e) => setUsername(e.target.value)} required />
          </div>
          <div>
            <Label>密码（至少 6 位）</Label>
            <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={6} />
          </div>
          {err && <div className="text-sm text-warn">{err}</div>}
          <Button className="w-full" disabled={loading}>{loading ? '创建中…' : '创建并进入'}</Button>
        </form>
      </Card>
    </div>
  )
}
