import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import {
  LayoutDashboard, Radio, KeyRound, Fingerprint, ScrollText, Settings, Menu, LogOut, ChevronLeft
} from 'lucide-react'
import { useState } from 'react'
import { clearToken } from '@/lib/api'
import { cn } from '@/lib/utils'
import { Button } from './ui'

const nav = [
  { to: '/', label: '概览', icon: LayoutDashboard },
  { to: '/channels', label: '渠道', icon: Radio },
  { to: '/keys', label: '用户 Key', icon: KeyRound },
  { to: '/presets', label: '客户端预设', icon: Fingerprint },
  { to: '/logs', label: '请求日志', icon: ScrollText },
  { to: '/settings', label: '设置', icon: Settings },
]

export default function Layout() {
  const [open, setOpen] = useState(true)
  const navgate = useNavigate()
  return (
    <div className="min-h-screen bg-canvas">
      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-30 bg-canvas border-r border-gray-200/60 transition-all duration-300 flex flex-col',
          open ? 'w-60' : 'w-16',
        )}
      >
        <div className="h-14 flex items-center gap-2 px-4 border-b border-gray-200/50">
          <div className="w-8 h-8 rounded-xl bg-primary/10 text-primary flex items-center justify-center font-semibold">P</div>
          {open && <div className="font-semibold text-gray-800 tracking-tight">PhonyC</div>}
          <button className="ml-auto text-gray-400 hover:text-gray-700" onClick={() => setOpen((v) => !v)}>
            {open ? <ChevronLeft className="w-4 h-4" /> : <Menu className="w-4 h-4" />}
          </button>
        </div>
        <nav className="flex-1 p-3 space-y-1">
          {nav.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition-all duration-200',
                  isActive ? 'bg-primary text-white shadow-lg shadow-primary/20' : 'text-gray-600 hover:bg-white',
                )
              }
            >
              <item.icon className="w-4 h-4 shrink-0" />
              {open && <span>{item.label}</span>}
            </NavLink>
          ))}
        </nav>
        <div className="p-3 border-t border-gray-200/50">
          <Button
            variant="ghost"
            className="w-full justify-start"
            onClick={() => {
              clearToken()
              navgate('/login')
            }}
          >
            <LogOut className="w-4 h-4" />
            {open && '退出登录'}
          </Button>
        </div>
      </aside>
      <div className={cn('transition-all duration-300', open ? 'ml-60' : 'ml-16')}>
        <header className="sticky top-0 z-20 h-14 flex items-center px-6 md:px-8 bg-canvas/80 backdrop-blur-md border-b border-gray-200/50">
          <div className="text-sm text-gray-400">AI API 轻量中转网关</div>
        </header>
        <main className="p-6 md:p-8">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
