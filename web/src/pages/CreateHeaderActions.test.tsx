import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import Channels from './Channels'
import Keys from './Keys'

vi.mock('@/lib/api', () => ({
  api: vi.fn(async (path: string) => path === '/api/presets' ? { items: [] } : { items: [] }),
}))

afterEach(cleanup)

describe('create page header actions', () => {
  it('hides New Channel while the channel form is open', async () => {
    const user = userEvent.setup()
    render(<MemoryRouter><Channels /></MemoryRouter>)
    await user.click(screen.getByRole('button', { name: '新建渠道' }))
    expect(screen.queryByRole('button', { name: '新建渠道' })).toBeNull()
    expect(screen.getByRole('button', { name: '保存' })).toBeTruthy()
  })

  it('hides New Key while the key form is open', async () => {
    const user = userEvent.setup()
    render(<Keys />)
    await user.click(screen.getByRole('button', { name: '新建 Key' }))
    expect(screen.queryByRole('button', { name: '新建 Key' })).toBeNull()
    expect(screen.getByRole('button', { name: '保存' })).toBeTruthy()
  })
})
