import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { api } from '@/lib/api'
import Channels from './Channels'

vi.mock('@/lib/api', () => ({ api: vi.fn() }))

afterEach(() => {
  cleanup()
  vi.mocked(api).mockReset()
})

function presetSelect() {
  return within(screen.getByText('测活所用预设').parentElement as HTMLElement).getByRole('combobox')
}

describe('channel healthcheck preset', () => {
  it('defaults new channels to no preset and lists existing presets', async () => {
    vi.mocked(api).mockImplementation(async (path: string) => {
      if (path === '/api/channels') return { items: [] }
      if (path === '/api/presets') return { items: [{ id: 3, name: 'Codex 基础' }] }
      return { items: [] }
    })
    const user = userEvent.setup()
    render(<MemoryRouter><Channels /></MemoryRouter>)
    await user.click(await screen.findByRole('button', { name: '新建渠道' }))

    const select = presetSelect() as HTMLSelectElement
    expect(select.value).toBe('')
    expect(within(select).getByRole('option', { name: '无（正常请求）' })).toBeTruthy()
    expect(within(select).getByRole('option', { name: 'Codex 基础' })).toBeTruthy()
  })

  it('saves and clears the selected preset while editing', async () => {
    const channel = {
      id: 11, name: 'restricted', enabled: true, protocol: 'openai', base_url: 'https://example.test', api_key: 'saved',
      priority: 10, timeout_ms: 600000, extra_headers_json: '{}', healthcheck_preset_id: 3,
    }
    vi.mocked(api).mockImplementation(async (path: string) => {
      if (path === '/api/channels') return { items: [channel] }
      if (path === '/api/presets') return { items: [{ id: 3, name: 'Codex 基础' }, { id: 4, name: 'Claude 基础' }] }
      if (path === '/api/channels/11/models') return { items: [] }
      return {}
    })
    const user = userEvent.setup()
    render(<MemoryRouter><Channels /></MemoryRouter>)
    await user.click(await screen.findByRole('button', { name: '编辑' }))

    await user.selectOptions(presetSelect(), '4')
    await user.click(screen.getByRole('button', { name: '保存' }))
    const selectedCall = vi.mocked(api).mock.calls.find(([path, init]) => path === '/api/channels/11' && init?.method === 'PATCH')
    expect(JSON.parse(String(selectedCall?.[1]?.body))).toMatchObject({ healthcheck_preset_id: 4 })

    await user.click(await screen.findByRole('button', { name: '编辑' }))
    await user.selectOptions(presetSelect(), '')
    await user.click(screen.getByRole('button', { name: '保存' }))
    const patchCalls = vi.mocked(api).mock.calls.filter(([path, init]) => path === '/api/channels/11' && init?.method === 'PATCH')
    expect(JSON.parse(String(patchCalls[patchCalls.length - 1]?.[1]?.body))).toMatchObject({ clear_healthcheck_preset: true })
  })

  it('shows a disabled spinner until channel testing finishes', async () => {
    let finishTest: (() => void) | undefined
    const pendingTest = new Promise<void>((resolve) => { finishTest = resolve })
    const channel = {
      id: 12, name: 'slow', enabled: true, protocol: 'openai', base_url: 'https://example.test', api_key: 'saved',
      priority: 10, timeout_ms: 600000, extra_headers_json: '{}', healthcheck_preset_id: null,
    }
    vi.mocked(api).mockImplementation(async (path: string, init?: RequestInit) => {
      if (path === '/api/channels') return { items: [channel] }
      if (path === '/api/presets') return { items: [] }
      if (path === '/api/channels/12/test' && init?.method === 'POST') return pendingTest
      return { items: [] }
    })
    const user = userEvent.setup()
    render(<MemoryRouter><Channels /></MemoryRouter>)
    await user.click(await screen.findByRole('button', { name: '测活' }))

    const loadingButton = screen.getByRole('button', { name: '测活中' })
    expect((loadingButton as HTMLButtonElement).disabled).toBe(true)
    expect(loadingButton.querySelector('.animate-spin')).toBeTruthy()

    finishTest?.()
    await waitFor(() => expect(screen.getByRole('button', { name: '测活' })).toBeTruthy())
  })
})
