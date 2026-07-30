import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import SettingsPage from './Settings'

const defaultLexicon = JSON.stringify({ schema_version: 2, prefix: ['介绍'], target_patterns: ['什么是{target}'], modal_words: [''], short_rules: ['简短'], targets: ['docker'] }, null, 2)

vi.mock('@/lib/api', () => ({
  api: vi.fn(async (path: string, init?: RequestInit) => {
    if (path === '/api/settings') {
      return { settings: { auto_test_enabled: 'false', auto_test_enhanced_enabled: 'false', auto_test_enhanced_lexicon: defaultLexicon }, enhanced_lexicon_default: defaultLexicon }
    }
    if (path === '/api/healthcheck/status') return { enabled: false }
    if (path === '/api/healthcheck/enhanced-preview') {
      const body = JSON.parse(String(init?.body || '{}'))
      if (!body.lexicon.includes('docker')) throw new Error('词库无效')
      return { prompt: '简单介绍docker' }
    }
    return { ok: true }
  }),
}))

afterEach(cleanup)

describe('enhanced healthcheck settings', () => {
  it('expands, previews, edits, and restores the canonical lexicon', async () => {
    const user = userEvent.setup()
    render(<SettingsPage />)
    const toggle = await screen.findByRole('checkbox', { name: /自动测活增强/ })
    expect(screen.queryByText('增强测活 JSON 词库')).toBeNull()
    await user.click(toggle)
    const editor = screen.getByRole('textbox', { name: '增强测活 JSON 词库' }) as HTMLTextAreaElement
    expect(editor.value).toBe(defaultLexicon)
    expect(screen.getByText(/schema_version 由系统维护/)).toBeTruthy()
    expect(screen.getByText('增强模式开启时不使用此固定提问词。')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: '随机预览' }))
    expect(await screen.findByText('简单介绍docker')).toBeTruthy()

    await user.clear(editor)
    await user.type(editor, 'bad')
    expect(screen.getByText(/Unexpected non-whitespace/)).toBeTruthy()
    await user.click(screen.getByRole('button', { name: '恢复默认词库' }))
    await waitFor(() => expect(editor.value).toBe(defaultLexicon))
  })
})
