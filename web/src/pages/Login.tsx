import { FormEvent, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, setToken } from '@/lib/api'
import { Button, Card, Input, Label } from '@/components/ui'

export default function Login() {
  const nav = useNavigate()
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('')
  const [err, setErr] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    api<{ initialized: boolean }>('/api/setup/status').then((s) => {
      if (!s.initialized) nav('/setup')
    }).catch(() => {})
  }, [nav])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setLoading(true)
    setErr('')
    try {
      const res = await api<{ token: string }>('/api/auth/login', {
        method: 'POST',
        body: JSON.stringify({ username, password }),
      })
      setToken(res.token)
      nav('/')
    } catch (ex: any) {
      setErr(ex.message || '登录失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen grid place-items-center p-6">
      <Card className="w-full max-w-md p-8">
        <div className="mb-6">
          <div className="w-12 h-12 rounded-2xl bg-primary/10 text-primary grid place-items-center text-lg font-semibold mb-4">P</div>
          <h1 className="text-xl font-semibold text-gray-800">登录管理台</h1>
          <p className="text-sm text-gray-400 mt-1">使用管理员账号继续</p>
        </div>
        <form className="space-y-4" onSubmit={onSubmit}>
          <div>
            <Label>用户名</Label>
            <Input value={username} onChange={(e) => setUsername(e.target.value)} required />
          </div>
          <div>
            <Label>密码</Label>
            <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
          </div>
          {err && <div className="text-sm text-warn">{err}</div>}
          <Button className="w-full" disabled={loading}>{loading ? '登录中…' : '登录'}</Button>
        </form>
      </Card>
    </div>
  )
}
