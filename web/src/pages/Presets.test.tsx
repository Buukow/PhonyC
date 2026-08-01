import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { api } from '@/lib/api'
import Presets from './Presets'

const preset = {
  id: 7,
  name: 'Tree preset',
  version_label: '1.0',
  description: 'tree editor test',
  builtin: false,
  rule_json: JSON.stringify({
    schema_version: 1,
    headers: {
      'X-Test': {
        value: { session: { id: 'abc' }, version: '1.0' },
        fill_missing: false,
        children_fill_missing: {},
      },
    },
    remove_headers: [],
    generators: {},
  }),
}

vi.mock('@/lib/api', () => ({ api: vi.fn() }))

afterEach(() => { cleanup(); vi.mocked(api).mockReset() })

function useDefaultAPI() {
  vi.mocked(api).mockImplementation(async (path: string, init?: RequestInit) => {
    if (path === '/api/presets' && !init?.method) return { items: [preset] }
    if (path === '/api/presets/validate') {
      const body = JSON.parse(String(init?.body || '{}'))
      return { rule_json: body.rule_json }
    }
    if (path === '/api/presets' && init?.method === 'POST') {
      return { id: 8, ...JSON.parse(String(init.body || '{}')) }
    }
    return {}
  })
}

async function openEditor() {
  useDefaultAPI()
  const user = userEvent.setup()
  render(<Presets />)
  await user.click(await screen.findByRole('button', { name: '编辑' }))
  return user
}

function rowFor(key: string) {
  const keyNode = screen.getByTitle(key)
  return keyNode.parentElement?.parentElement as HTMLElement
}

describe('preset visual tree editor', () => {
  it('shows localized names and descriptions for builtin presets', async () => {
    vi.mocked(api).mockResolvedValue({
      items: [
        { id: 1, name: 'codex-tui', version_label: '0.145.0', description: 'backend codex', builtin: true },
        { id: 2, name: 'claude-cli', version_label: '2.1.220', description: 'backend claude', builtin: true },
        { id: 3, name: 'codex-enhanced', version_label: '0.145.0', description: 'backend codex enhanced', builtin: true },
        { id: 4, name: 'claude-enhanced', version_label: '2.1.220', description: 'backend claude enhanced', builtin: true },
        { id: 5, name: 'custom-name', version_label: '1.0', description: 'custom description', builtin: false },
      ],
    })

    render(<Presets />)

    expect(await screen.findByText('Codex 基础')).toBeTruthy()
    expect(screen.getByText('Claude 基础')).toBeTruthy()
    expect(screen.getByText('Codex 增强')).toBeTruthy()
    expect(screen.getByText('Claude 增强')).toBeTruthy()
    expect(screen.getByText('模拟 Codex 客户端的基础请求头')).toBeTruthy()
    expect(screen.getByText('模拟 Claude Code 客户端的基础请求头')).toBeTruthy()
    expect(screen.getByText('模拟 Codex 客户端的完整请求头与动态会话信息，强制覆盖同名请求头')).toBeTruthy()
    expect(screen.getByText('模拟 Claude Code 客户端的完整请求头与动态会话信息，强制覆盖同名请求头')).toBeTruthy()
    expect(screen.getByText('custom-name')).toBeTruthy()
    expect(screen.getByText('custom description')).toBeTruthy()
  })

  it('shows keys and values as an indented expandable tree', async () => {
    const user = await openEditor()

    expect(screen.getByText('headers')).toBeTruthy()
    expect(screen.getByText('对象（1项）')).toBeTruthy()
    expect(screen.getByTitle('X-Test')).toBeTruthy()
    expect(rowFor('X-Test').style.paddingLeft).toBe('24px')
    expect(screen.getByTitle('session')).toBeTruthy()
    expect(within(rowFor('version')).getByRole('textbox')).toHaveProperty('value', '1.0')
    expect(rowFor('session').style.paddingLeft).toBe('48px')
    expect(screen.queryByTitle('id')).toBeNull()

    await user.click(within(rowFor('session')).getByRole('button', { name: '展开' }))
    expect(screen.getByTitle('id')).toBeTruthy()
    expect(screen.getByDisplayValue('abc')).toBeTruthy()
    expect(rowFor('id').style.paddingLeft).toBe('72px')
  })

  it('keeps leaf input focus while typing', async () => {
    const user = await openEditor()
    const input = within(rowFor('version')).getByRole('textbox') as HTMLInputElement

    input.focus()
    await user.type(input, '.1')

    expect(document.activeElement).toBe(input)
    expect(input.value).toBe('1.0.1')
  })

  it('supports inherited and explicit child override modes', async () => {
    const user = await openEditor()
    const rootSwitch = within(rowFor('X-Test')).getByRole('switch')
    const sessionSwitch = within(rowFor('session')).getByRole('switch')

    expect(within(rowFor('X-Test')).getByText('强制覆盖')).toBeTruthy()
    await user.click(rootSwitch)
    expect(within(rowFor('X-Test')).getByText('缺失补全')).toBeTruthy()
    expect(within(rowFor('session')).getByText('继承：缺失补全')).toBeTruthy()

    await user.click(within(rowFor('session')).getByRole('button', { name: '展开' }))
    const idSwitch = within(rowFor('id')).getByRole('switch')
    expect(within(rowFor('id')).getByText('继承：缺失补全')).toBeTruthy()

    await user.click(idSwitch)
    expect(within(rowFor('id')).getByText('强制覆盖')).toBeTruthy()
    expect(within(rowFor('id')).getByRole('button', { name: '恢复继承' })).toBeTruthy()

    await user.click(within(rowFor('id')).getByRole('button', { name: '恢复继承' }))
    expect(within(rowFor('id')).getByText('继承：缺失补全')).toBeTruthy()
  })

  it('defaults newly added headers to force override', async () => {
    useDefaultAPI()
    const user = userEvent.setup()
    render(<Presets />)
    await user.click(await screen.findByRole('button', { name: '新建预设' }))
    await user.click(screen.getByRole('button', { name: '添加 Header' }))
    expect(screen.getByRole('switch', { name: /New-Header 强制覆盖/ })).toBeTruthy()
  })

  it('keeps pasted JSON when a new preset switches to visual mode before saving', async () => {
    useDefaultAPI()
    const user = userEvent.setup()
    render(<Presets />)
    await user.click(await screen.findByRole('button', { name: '新建预设' }))

    const nameInput = screen.getAllByRole('textbox')[0]
    await user.type(nameInput, 'Nested create')
    await user.click(screen.getByRole('button', { name: 'JSON 编辑' }))

    const nested = {
      schema_version: 1,
      headers: {
        'X-New': {
          value: { level1: { level2: { value: 'saved' } } },
          fill_missing: false,
        },
      },
      remove_headers: [],
      generators: {},
    }
    const jsonEditor = screen.getAllByRole('textbox').find((element) => element.tagName === 'TEXTAREA') as HTMLTextAreaElement
    fireEvent.change(jsonEditor, { target: { value: JSON.stringify(nested, null, 2) } })
    await user.click(screen.getByRole('button', { name: '可视化编辑' }))

    expect(screen.getByTitle('X-New')).toBeTruthy()
    expect(screen.getByTitle('level1')).toBeTruthy()
    await user.click(screen.getByRole('button', { name: '保存' }))

    const createCall = vi.mocked(api).mock.calls.find(([path, init]) => path === '/api/presets' && init?.method === 'POST')
    expect(createCall).toBeTruthy()
    const payload = JSON.parse(String(createCall?.[1]?.body))
    expect(JSON.parse(payload.rule_json)).toEqual(nested)
  })

  it('configures interval and increment generators with valid defaults', async () => {
    useDefaultAPI()
    const user = userEvent.setup()
    render(<Presets />)
    await user.click(await screen.findByRole('button', { name: '新建预设' }))
    await user.click(screen.getByRole('button', { name: '添加生成器' }))

    let selects = screen.getAllByRole('combobox') as HTMLSelectElement[]
    const typeSelect = selects[0]
    const charsetSelect = selects[1]
    const modeSelect = selects[2]
    await user.selectOptions(modeSelect, 'increment')

    expect(typeSelect.value).toBe('random')
    expect(charsetSelect.value).toBe('digits')
    expect(charsetSelect.disabled).toBe(true)
    expect(screen.getByDisplayValue('1')).toBeTruthy()
    selects = screen.getAllByRole('combobox') as HTMLSelectElement[]
    expect(selects[3].value).toBe('wrap')

    await user.selectOptions(modeSelect, 'interval')
    expect(screen.getByDisplayValue('30m')).toBeTruthy()
  })
})
