import { Navigate, Route, Routes } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { api, getToken } from '@/lib/api'
import Layout from '@/components/Layout'
import Setup from '@/pages/Setup'
import Login from '@/pages/Login'
import Dashboard from '@/pages/Dashboard'
import Channels from '@/pages/Channels'
import ChannelDetail from '@/pages/ChannelDetail'
import Keys from '@/pages/Keys'
import Presets from '@/pages/Presets'
import Logs from '@/pages/Logs'
import SettingsPage from '@/pages/Settings'
import CapturePage from '@/pages/Capture'

function Guard({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<'loading' | 'setup' | 'login' | 'ok'>('loading')
  useEffect(() => {
    ;(async () => {
      try {
        const s = await api<{ initialized: boolean }>('/api/setup/status')
        if (!s.initialized) {
          setState('setup')
          return
        }
        if (!getToken()) {
          setState('login')
          return
        }
        setState('ok')
      } catch {
        setState('login')
      }
    })()
  }, [])
  if (state === 'loading') {
    return <div className="min-h-screen grid place-items-center text-gray-400">加载中…</div>
  }
  if (state === 'setup') return <Navigate to="/setup" replace />
  if (state === 'login') return <Navigate to="/login" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <Routes>
      <Route path="/setup" element={<Setup />} />
      <Route path="/login" element={<Login />} />
      <Route
        path="/"
        element={
          <Guard>
            <Layout />
          </Guard>
        }
      >
        <Route index element={<Dashboard />} />
        <Route path="channels" element={<Channels />} />
        <Route path="channels/:id" element={<ChannelDetail />} />
        <Route path="keys" element={<Keys />} />
        <Route path="presets" element={<Presets />} />
        <Route path="logs" element={<Logs />} />
        <Route path="capture" element={<CapturePage />} />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
