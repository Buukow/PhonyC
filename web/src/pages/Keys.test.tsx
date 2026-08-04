import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { api } from '@/lib/api'
import Keys from './Keys'

vi.mock('@/lib/api', () => ({ api: vi.fn() }))

afterEach(() => {
  cleanup()
  vi.mocked(api).mockReset()
})

describe('user key impersonation modes', () => {
  it('stores Chinese mode values without a frontend translation layer', async () => {
    vi.mocked(api).mockImplementation(async (path: string) => {
      if (path === '/api/keys') return { items: [] }
      if (path === '/api/presets') return { items: [] }
      return {}
    })
    const user = userEvent.setup()
    render(<Keys />)
    await user.click(await screen.findByRole('button', { name: '新建 Key' }))

    const mode = screen.getAllByRole('combobox')[0] as HTMLSelectElement
    expect(mode.value).toBe('透传')
    expect(Array.from(mode.options).map((option) => option.text)).toEqual(['透传', '预设', '自定义'])

    await user.selectOptions(mode, '预设')
    await user.type(screen.getAllByRole('textbox')[0], '中文模式')
    await user.click(screen.getByRole('button', { name: '保存' }))

    const createCall = vi.mocked(api).mock.calls.find(([path, init]) => path === '/api/keys' && init?.method === 'POST')
    expect(JSON.parse(String(createCall?.[1]?.body))).toMatchObject({ impersonation_mode: '预设' })
  })
})
