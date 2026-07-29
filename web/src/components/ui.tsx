import { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, SelectHTMLAttributes, TextareaHTMLAttributes } from 'react'
import { cn } from '@/lib/utils'

export function Card({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cn('bg-white rounded-2xl shadow-card border border-gray-100', className)}>
      {children}
    </div>
  )
}

export function Button({ className, variant = 'primary', ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: 'primary' | 'secondary' | 'danger' | 'ghost' }) {
  const styles = {
    primary: 'bg-primary text-white shadow-lg shadow-primary/25 hover:-translate-y-0.5',
    secondary: 'bg-white text-gray-700 border border-gray-200 hover:bg-canvas',
    danger: 'bg-warn/10 text-warn hover:bg-warn/20',
    ghost: 'bg-transparent text-gray-600 hover:bg-canvas',
  }[variant]
  return (
    <button
      className={cn(
        'inline-flex items-center justify-center gap-2 rounded-xl px-4 py-2 text-sm font-medium transition-all duration-200 disabled:opacity-50',
        styles,
        className,
      )}
      {...props}
    />
  )
}

export function Input({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={cn(
        'w-full rounded-xl border border-gray-200 bg-canvas px-3 py-2 text-sm text-gray-800 outline-none focus:ring-2 focus:ring-primary/30 focus:border-primary',
        className,
      )}
      {...props}
    />
  )
}

export function Textarea({ className, ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <textarea
      className={cn(
        'w-full rounded-xl border border-gray-200 bg-canvas px-3 py-2 text-sm text-gray-800 outline-none focus:ring-2 focus:ring-primary/30 focus:border-primary min-h-[100px]',
        className,
      )}
      {...props}
    />
  )
}

export function Select({ className, children, ...props }: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select
      className={cn(
        'w-full rounded-xl border border-gray-200 bg-canvas px-3 py-2 text-sm text-gray-800 outline-none focus:ring-2 focus:ring-primary/30 focus:border-primary',
        className,
      )}
      {...props}
    >
      {children}
    </select>
  )
}

export function Label({ children }: { children: ReactNode }) {
  return <label className="block text-xs font-medium text-gray-500 mb-1.5">{children}</label>
}

export function Badge({ children, tone = 'primary' }: { children: ReactNode; tone?: 'primary' | 'warn' | 'muted' | 'accent' }) {
  const map = {
    primary: 'bg-primary/10 text-primary',
    warn: 'bg-warn/10 text-warn',
    muted: 'bg-muted/10 text-muted',
    accent: 'bg-accent/20 text-amber-700',
  }[tone]
  return <span className={cn('inline-flex items-center rounded-lg px-2 py-0.5 text-xs font-medium', map)}>{children}</span>
}

export function PageHeader({ title, subtitle, actions }: { title: string; subtitle?: string; actions?: ReactNode }) {
  return (
    <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-6">
      <div>
        <h1 className="text-xl font-semibold text-gray-800">{title}</h1>
        {subtitle && <p className="text-sm text-gray-400 mt-1">{subtitle}</p>}
      </div>
      {actions && <div className="flex items-center gap-2 flex-wrap">{actions}</div>}
    </div>
  )
}

export function Table({ headers, children }: { headers: string[]; children: ReactNode }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-gray-100">
            {headers.map((h) => (
              <th key={h} className="text-left text-xs uppercase tracking-wider text-gray-400 font-medium px-4 py-3">
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>{children}</tbody>
      </table>
    </div>
  )
}
