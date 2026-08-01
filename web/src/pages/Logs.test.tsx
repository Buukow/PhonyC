import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import Logs from './Logs'
import { api } from '@/lib/api'

vi.mock('@/lib/api', () => ({ api: vi.fn() }))

afterEach(() => {
  cleanup()
  vi.mocked(api).mockReset()
})

describe('request log filters', () => {
  it('queries by key, channel, and exact status code while preserving filters across pages', async () => {
    vi.mocked(api).mockImplementation(async (path: string) => {
      if (path === '/api/keys') return { items: [{ id: 3, name: 'client-key' }] }
      if (path === '/api/channels') return { items: [{ id: 8, name: 'primary-channel' }] }
      return { items: [], total: 51 }
    })
    const user = userEvent.setup()
    render(<Logs />)

    await user.selectOptions(await screen.findByRole('combobox', { name: '按 Key 筛选' }), '3')
    await user.selectOptions(screen.getByRole('combobox', { name: '按渠道筛选' }), '8')
    await user.type(screen.getByRole('textbox', { name: '按状态码筛选' }), '401')
    await user.click(screen.getByRole('button', { name: '查询' }))

    const requests = vi.mocked(api).mock.calls.filter(([path]) => String(path).startsWith('/api/logs?'))
    const query = new URLSearchParams(String(requests[requests.length - 1]?.[0]).split('?')[1])
    expect(query.get('user_key_id')).toBe('3')
    expect(query.get('channel_id')).toBe('8')
    expect(query.get('status_min')).toBe('401')
    expect(query.get('status_max')).toBe('401')

    await user.click(screen.getByRole('button', { name: '下一页' }))
    const logRequests = vi.mocked(api).mock.calls.filter(([path]) => String(path).startsWith('/api/logs?'))
    const nextQuery = new URLSearchParams(String(logRequests[logRequests.length - 1]?.[0]).split('?')[1])
    expect(nextQuery.get('offset')).toBe('50')
    expect(nextQuery.get('user_key_id')).toBe('3')
    expect(nextQuery.get('channel_id')).toBe('8')
    expect(nextQuery.get('status_min')).toBe('401')
    expect(nextQuery.get('status_max')).toBe('401')
  })

  it('clears all filters with reset', async () => {
    vi.mocked(api).mockImplementation(async (path: string) => {
      if (path === '/api/keys') return { items: [{ id: 3, name: 'client-key' }] }
      if (path === '/api/channels') return { items: [{ id: 8, name: 'primary-channel' }] }
      return { items: [], total: 0 }
    })
    const user = userEvent.setup()
    render(<Logs />)
    await user.selectOptions(await screen.findByRole('combobox', { name: '按 Key 筛选' }), '3')
    await user.click(screen.getByRole('button', { name: '重置筛选' }))

    expect(screen.getByRole('combobox', { name: '按 Key 筛选' })).toHaveProperty('value', '')
    expect(screen.getByRole('combobox', { name: '按渠道筛选' })).toHaveProperty('value', '')
    expect(screen.getByRole('textbox', { name: '按状态码筛选' })).toHaveProperty('value', '')
  })
})
